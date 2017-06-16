// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/extensions/clock"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	mt "github.com/topfreegames/maestro/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/pg.v5/types"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	yaml1 = `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
)

var _ = Describe("Controller", func() {
	var clientset *fake.Clientset
	var timeoutSec int
	var lockTimeoutMS int = 600
	var lockKey string = "maestro-test-lock-key"
	var configYaml1 models.ConfigYAML

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		timeoutSec = 300
		err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
		Expect(err).NotTo(HaveOccurred())

		node := &v1.Node{}
		node.SetName(configYaml1.Name)
		node.SetLabels(map[string]string{
			"game": "controller",
		})

		_, err = clientset.CoreV1().Nodes().Create(node)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreateScheduler", func() {
		It("should succeed", func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				gomock.Any(),
			)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			)

			err := controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			for _, svc := range svcs.Items {
				Expect(svc.GetName()).To(ContainSubstring("controller-name-"))
				Expect(svc.GetName()).To(HaveLen(len("controller-name-") + 8))
			}

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))
			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Env[3].Name).To(Equal("MAESTRO_NODE_PORT_1234_UDP"))
				Expect(pod.Spec.Containers[0].Env[3].Value).NotTo(BeNil())
				Expect(pod.Spec.Containers[0].Env[4].Name).To(Equal("MAESTRO_NODE_PORT_7654_TCP"))
				Expect(pod.Spec.Containers[0].Env[4].Value).NotTo(BeNil())
			}
		})

		It("should create pods with node affinity and toleration", func() {
			yaml1 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				gomock.Any(),
			)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			for _, svc := range svcs.Items {
				Expect(svc.GetName()).To(ContainSubstring("controller-name-"))
				Expect(svc.GetName()).To(HaveLen(len("controller-name-") + 8))
			}

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))
			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Env[3].Name).To(Equal("MAESTRO_NODE_PORT_1234_UDP"))
				Expect(pod.Spec.Containers[0].Env[3].Value).NotTo(BeNil())
				Expect(pod.Spec.Containers[0].Env[4].Name).To(Equal("MAESTRO_NODE_PORT_7654_TCP"))
				Expect(pod.Spec.Containers[0].Env[4].Value).NotTo(BeNil())
			}
		})

		It("should return error if namespace already exists", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			namespace := models.NewNamespace(configYaml1.Name)
			err = namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("namespace \"%s\" already exists", configYaml1.Name)))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))

			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(0))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})

		It("should rollback if error creating scheduler", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(
				gomock.Any(),
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				gomock.Any(),
			).Return(&types.Result{}, errors.New("some error in db"))

			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(0))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})

		It("should rollback if error scaling up", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(
				gomock.Any(),
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				gomock.Any(),
			).Do(
				func(scheduler *models.Scheduler, query string, srcScheduler *models.Scheduler) {
					scheduler.ID = uuid.NewV4().String()
				},
			)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(0))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})
	})

	Describe("CreateNamespaceIfNecessary", func() {
		It("should succeed if namespace exists", func() {
			name := "test-123"
			namespace := models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(name))

			err = controller.CreateNamespaceIfNecessary(logger, mr, clientset, name)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if namespace needs to be created", func() {
			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

			name := "test-123"
			err = controller.CreateNamespaceIfNecessary(logger, mr, clientset, name)
			Expect(err).NotTo(HaveOccurred())

			ns, err = clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(name))
		})
	})

	Describe("DeleteScheduler", func() {
		It("should succeed", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.DeleteScheduler(logger, mr, mockDb, clientset, configYaml1.Name, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))
		})

		It("should fail if some error retrieving the scheduler", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				configYaml1.Name,
			).Return(&types.Result{}, errors.New("some error in db"))

			err = controller.DeleteScheduler(logger, mr, mockDb, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should fail if some error deleting the scheduler", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})
			mockDb.EXPECT().Exec(
				"DELETE FROM schedulers WHERE name = ?",
				configYaml1.Name,
			).Return(&types.Result{}, errors.New("some error deleting in db"))
			err = controller.DeleteScheduler(logger, mr, mockDb, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error deleting in db"))
		})

		It("should fail if update on existing scheduler fails", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
				scheduler.ID = "random-id"
			})
			mockDb.EXPECT().
				Query(
					gomock.Any(),
					"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
					gomock.Any(),
				).Return(nil, errors.New("error on updating"))

			err = controller.DeleteScheduler(logger, mr, mockDb, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error on updating"))
		})

		It("should return error if timeout waiting for pods after delete", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})

			timeoutSec := 0
			err = controller.DeleteScheduler(logger, mr, mockDb, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if timeout with 'shutdownTimeout'", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			yaml1 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 0
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})

			err = controller.DeleteScheduler(logger, mr, mockDb, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout deleting scheduler pods"))
		})
	})

	Describe("GetSchedulerScalingInfo", func() {
		It("should succeed", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{
				Creating:    4,
				Occupied:    3,
				Ready:       2,
				Terminating: 1,
			}
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			scheduler, autoScalingPolicy, countByStatus, err := controller.GetSchedulerScalingInfo(logger, mr, mockDb, mockRedisClient, configYaml1.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduler.YAML).To(Equal(yaml1))
			Expect(autoScalingPolicy).To(Equal(configYaml1.AutoScaling))
			Expect(countByStatus).To(Equal(expC))
		})

		It("should fail if error retrieving the scheduler", func() {
			name := "controller-name"
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				name,
			).Return(&types.Result{}, errors.New("some error in db"))
			_, _, _, err := controller.GetSchedulerScalingInfo(logger, mr, mockDb, mockRedisClient, name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should fail if error retrieving rooms count by status", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(gomock.Any()).Times(4)
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))
			_, _, _, err = controller.GetSchedulerScalingInfo(logger, mr, mockDb, mockRedisClient, configYaml1.Name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})

		It("should return error if no scheduler found", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name)
			_, _, _, err = controller.GetSchedulerScalingInfo(logger, mr, mockDb, mockRedisClient, configYaml1.Name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler \"controller-name\" not found"))
		})
	})

	Describe("UpdateScheduler", func() {
		It("should succeed", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			mockDb.EXPECT().Query(
				scheduler,
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				scheduler,
			).Return(&types.Result{}, nil)
			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if error in postgres", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			mockDb.EXPECT().Query(
				scheduler,
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				scheduler,
			).Return(&types.Result{}, errors.New("some error in pg"))
			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("List Schedulers Names", func() {
		It("should get schedulers names from the database", func() {
			expectedNames := []string{"scheduler1", "scheduler2", "scheduler3"}
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Do(
				func(schedulers *[]models.Scheduler, query string) {
					expectedSchedulers := make([]models.Scheduler, len(expectedNames))
					for idx, name := range expectedNames {
						expectedSchedulers[idx] = models.Scheduler{Name: name}
					}
					*schedulers = expectedSchedulers
				},
			)
			names, err := controller.ListSchedulersNames(logger, mr, mockDb)
			Expect(err).NotTo(HaveOccurred())
			Expect(names).To(Equal(expectedNames))
		})

		It("should succeed if error is 'no rows in result set'", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				&types.Result{}, errors.New("pg: no rows in result set"),
			)
			_, err := controller.ListSchedulersNames(logger, mr, mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				&types.Result{}, errors.New("some error in pg"),
			)
			_, err := controller.ListSchedulersNames(logger, mr, mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("DeleteRoomsNoPingSince", func() {
		It("should delete GRUs", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				service := models.NewService(roomName, scheduler, []*models.Port{})
				_, err = service.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
				pod := models.NewPod("", "", roomName, scheduler, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{})
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult(expectedRooms, nil))
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(len(expectedRooms))
			mockPipeline.EXPECT().Del(gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Exec().Times(len(expectedRooms))
			err = controller.DeleteRoomsNoPingSince(logger, mr, mockRedisClient, clientset, scheduler, since)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call redis successfully and exit if no rooms should be deleted", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult([]string{}, nil))
			err := controller.DeleteRoomsNoPingSince(logger, mr, mockRedisClient, clientset, scheduler, since)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if failed to delete service", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod := models.NewPod("", "", roomName, scheduler, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{})
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult(expectedRooms, nil))

			for _, name := range expectedRooms {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err = controller.DeleteRoomsNoPingSince(logger, mr, mockRedisClient, clientset, scheduler, since)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if failed to delete pod", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				service := models.NewService(roomName, scheduler, []*models.Port{})
				_, err = service.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult(expectedRooms, nil))

			for _, name := range expectedRooms {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err = controller.DeleteRoomsNoPingSince(logger, mr, mockRedisClient, clientset, scheduler, since)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult([]string{}, errors.New("some error")))
			err := controller.DeleteRoomsNoPingSince(logger, mr, mockRedisClient, clientset, scheduler, since)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error"))
		})
	})

	Describe("ScaleUp", func() {
		It("should fail and return error if error creating service and pods and initial op", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))

			svcs, err := clientset.CoreV1().Services(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(0))
			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})

		It("should not fail and return error if error creating service and pods and not initial op", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(amount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))
			mockPipeline.EXPECT().Exec().Times(amount - 1)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))

			svcs, err := clientset.CoreV1().Services(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(amount - 1))
			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(amount - 1))
		})

		It("should fail if timeout", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(amount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().Exec().Times(amount)
			timeoutSec = 0
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout scaling up scheduler"))
		})
	})

	Describe("ScaleDown", func() {
		It("should succeed in scaling down", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetServiceNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SPopN(models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady), int64(scaleDownAmount)).
				Return(redis.NewStringSliceResult(names, nil))

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			timeoutSec = 300
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			services, err := clientset.CoreV1().Services(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
		})

		It("should return error if redis fails to clear room statuses", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetServiceNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SPopN(models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady), int64(scaleDownAmount)).
				Return(redis.NewStringSliceResult(names, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			room := models.NewRoom(names[0], scheduler.Name)
			for _, status := range allStatus {
				mockPipeline.EXPECT().
					SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
			}
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
			mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			timeoutSec = 0
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			services, err := clientset.CoreV1().Services(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(scaleUpAmount - 1))
		})

		It("should return error if redis fails to get N ready rooms", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetServiceNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SPopN(models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady), int64(scaleDownAmount)).
				Return(redis.NewStringSliceResult(names, errors.New("some error in redis")))

			timeoutSec = 0
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})

		It("should return error if redis fails to get string slice", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2

			mockRedisClient.EXPECT().
				SPopN(models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady), int64(scaleDownAmount)).
				Return(redis.NewStringSliceResult(nil, errors.New("some error in redis")))

			timeoutSec = 0
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})

		It("should not return error if delete non existing service and pod", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleDown
			scaleDownAmount := 1
			names := []string{"non-existing-service"}
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SPopN(models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady), int64(scaleDownAmount)).
				Return(redis.NewStringSliceResult(names, nil))

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			timeoutSec = 300
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return timeout error", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetServiceNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SPopN(models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady), int64(scaleDownAmount)).
				Return(redis.NewStringSliceResult(names, nil))

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			timeoutSec = 0
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout scaling down scheduler"))
		})
	})

	Describe("MustUpdatePods", func() {
		var configYaml1 *models.ConfigYAML

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return true if image is different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v124
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return true if the ports are different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1235
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return true if the limits are different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "68Mi"
  cpu: "3"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return true if the requests are different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "68Mi"
  cpu: "3"
requests: 
  memory: "70Mi"
  cpu: "1"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return true if command is different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./rom"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return true if command has different length", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
  - "exec"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return true if envs are different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
  - name: MY_ENV_VAR_2
    value: myvalue2
cmd:
  - "./room"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return false if auto scaling are different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 4
  up:
    delta: 3
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 2
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(configYaml1, &configYaml2)).To(BeFalse())
		})
	})

	Describe("UpdateSchedulerConfig", func() {
		var configYaml1, configYaml2 models.ConfigYAML
		var yaml2 string

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				gomock.Any(),
			)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			)
			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			yaml2 = `
name: controller-name
game: controller
image: controller/controller:v124
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 4
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
  - name: MY_NEW_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should recreate rooms with new ENV VARS and image and old number os rooms (and scale up later)", func() {
			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			for _, svc := range svcs.Items {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(svc.GetName(), svc.GetNamespace())
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(svc.GetNamespace()), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml2.AutoScaling.Min)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml2.AutoScaling.Min)
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml2.Name), gomock.Any()).
				Times(configYaml2.AutoScaling.Min)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml2.Name, "creating"), gomock.Any()).
				Times(configYaml2.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml2.AutoScaling.Min)

			mockDb.EXPECT().
				Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())

			mockRedisClient.EXPECT().Ping().AnyTimes()
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(redis.NewCmdResult(nil, nil))

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			svcs, err = clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(configYaml2.AutoScaling.Min))

			for _, svc := range svcs.Items {
				Expect(svc.GetName()).To(ContainSubstring("controller-name-"))
				Expect(svc.GetName()).To(HaveLen(len("controller-name-") + 8))
			}

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(4))
			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MY_NEW_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[3].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[3].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Env[4].Name).To(Equal("MAESTRO_NODE_PORT_1234_UDP"))
				Expect(pod.Spec.Containers[0].Env[4].Value).NotTo(BeNil())
				Expect(pod.Spec.Containers[0].Env[5].Name).To(Equal("MAESTRO_NODE_PORT_7654_TCP"))
				Expect(pod.Spec.Containers[0].Env[5].Value).NotTo(BeNil())
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(6))
			}
		})

		It("should return error if updating unexisting scheduler", func() {
			configYaml2 := models.ConfigYAML{Name: "another-name"}
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name)

			err := controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler another-name not found, create it first"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.ValidationFailedError"))
		})

		It("should not delete rooms if only scaling changes (leave it to ScaleUp or ScaleDown)", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 4
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"`

			var configYaml2 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())

			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})
			mockDb.EXPECT().
				Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			svcs, err = clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(configYaml1.AutoScaling.Min))

			for _, svc := range svcs.Items {
				Expect(svc.GetName()).To(ContainSubstring("controller-name-"))
				Expect(svc.GetName()).To(HaveLen(len("controller-name-") + 8))
			}

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))
			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Env[3].Name).To(Equal("MAESTRO_NODE_PORT_1234_UDP"))
				Expect(pod.Spec.Containers[0].Env[3].Value).NotTo(BeNil())
				Expect(pod.Spec.Containers[0].Env[4].Name).To(Equal("MAESTRO_NODE_PORT_7654_TCP"))
				Expect(pod.Spec.Containers[0].Env[4].Value).NotTo(BeNil())
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(5))
			}
		})

		It("should return error if db fails to select scheduler", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Return(&types.Result{}, errors.New("error on select"))

			err := controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error on select"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.DatabaseError"))
		})

		It("should return error if timeout waiting for lock", func() {
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			timeoutSec := 0
			err := controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout while wating for redis lock"))
		})

		It("should timeout after lock is always connected", func() {
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(false, nil))
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(redis.NewCmdResult(nil, nil))

			lock, err := redisClient.EnterCriticalSection(redisClient.Client, lockKey, time.Duration(lockTimeoutMS)*time.Millisecond, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			defer redisClient.LeaveCriticalSection(lock)

			timeoutSec := 1
			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout while wating for redis lock"))
		})

		It("should return error if error occurred on lock", func() {
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, errors.New("error getting lock")))

			err := controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error getting lock"))
		})

		It("should return error if timeout when deleting rooms", func() {
			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			// Update scheduler
			calls := mt.NewCalls()
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
					}))
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))
			calls.Add(
				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil)))
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec), 0)))
			calls.Add(
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil)))
			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				mockClock,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout during room deletion"))
		})

		It("should return error if timeout when creating rooms", func() {
			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			calls := mt.NewCalls()

			// Update scheduler
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
					}))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil)))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for _, svc := range svcs.Items {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(svc.GetName(), svc.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(svc.GetNamespace()), room.ID))
				calls.Add(mockPipeline.EXPECT().Del(room.GetRoomRedisKey()))
				calls.Add(mockPipeline.EXPECT().Exec())
			}
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec), 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil)))

			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				mockClock,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout during new room creation"))

			svcs, err = clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(BeEmpty())
		})

		It("should return error if timeout after creating rooms", func() {
			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			calls := mt.NewCalls()

			// Update scheduler
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
					}))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil)))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for _, svc := range svcs.Items {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(svc.GetName(), svc.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(svc.GetNamespace()), room.ID))
				calls.Add(mockPipeline.EXPECT().Del(room.GetRoomRedisKey()))
				calls.Add(mockPipeline.EXPECT().Exec())
			}
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for i := 0; i < configYaml2.AutoScaling.Min; i++ {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
						func(schedulerName string, statusInfo map[string]interface{}) {
							Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
							Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
						},
					))
				calls.Add(
					mockPipeline.EXPECT().
						SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()))
				calls.Add(
					mockPipeline.EXPECT().
						ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()))
				calls.Add(
					mockPipeline.EXPECT().Exec())
			}

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec+100), 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil)))

			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				mockClock,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout scaling up scheduler"))
		})

		It("should return error if timeout while waiting for pods", func() {
			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			calls := mt.NewCalls()

			// Update scheduler
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
					}))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil)))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for _, svc := range svcs.Items {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(svc.GetName(), svc.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(svc.GetNamespace()), room.ID))
				calls.Add(mockPipeline.EXPECT().Del(room.GetRoomRedisKey()))
				calls.Add(mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("redis error")))
			}
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for i := 0; i < configYaml2.AutoScaling.Min; i++ {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
						func(schedulerName string, statusInfo map[string]interface{}) {
							Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
							Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
						},
					))
				calls.Add(
					mockPipeline.EXPECT().
						SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()))
				calls.Add(
					mockPipeline.EXPECT().
						ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()))
				calls.Add(
					mockPipeline.EXPECT().Exec())
			}

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec), 0)))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec+100), 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil)))

			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				mockClock,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout waiting for rooms to be created"))

			svcs, err = clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(BeEmpty())
		})

		It("should not return error if ClearAll fails in deleting old rooms", func() {
			svcs, err := clientset.CoreV1().Services("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(3))

			calls := mt.NewCalls()

			// Update scheduler
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
					}))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))

			calls.Add(
				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil)))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for _, svc := range svcs.Items {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(svc.GetName(), svc.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(svc.GetNamespace()), room.ID))
				calls.Add(mockPipeline.EXPECT().Del(room.GetRoomRedisKey()))
				calls.Add(mockPipeline.EXPECT().Exec())
			}
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			for i := 0; i < configYaml2.AutoScaling.Min; i++ {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
						func(schedulerName string, statusInfo map[string]interface{}) {
							Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
							Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
						},
					))
				calls.Add(
					mockPipeline.EXPECT().
						SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()))
				calls.Add(
					mockPipeline.EXPECT().
						ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()))
				calls.Add(
					mockPipeline.EXPECT().Exec())
			}
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any()))
			calls.Add(
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil)))
			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS,
				lockKey,
				mockClock,
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
