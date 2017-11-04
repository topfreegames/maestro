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
	"strings"
	"time"

	"github.com/go-redis/redis"
	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/extensions/clock"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	mt "github.com/topfreegames/maestro/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/extensions/pg"
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
affinity: maestro-dedicated
toleration: maestro
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
requests:
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
	var maxSurge int = 100

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).
				Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil)).
				Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().
				Times(configYaml1.AutoScaling.Min)

			err := controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).
				Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil)).
				Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().
				Times(configYaml1.AutoScaling.Min)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

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
			).Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

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

			scheduler := models.NewScheduler(name, "", "")
			err = controller.CreateNamespaceIfNecessary(logger, mr, clientset, scheduler)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if namespace needs to be created", func() {
			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

			name := "test-123"
			scheduler := models.NewScheduler(name, "", "")
			err = controller.CreateNamespaceIfNecessary(logger, mr, clientset, scheduler)
			Expect(err).NotTo(HaveOccurred())

			ns, err = clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(name))
		})

		It("should return nil and not create if scheduler is at Terminating state", func() {
			name := "test-123"
			scheduler := models.NewScheduler(name, "", "")
			scheduler.State = models.StateTerminating
			err := controller.CreateNamespaceIfNecessary(logger, mr, clientset, scheduler)
			Expect(err).NotTo(HaveOccurred())

			ns := models.NewNamespace(name)
			exists, err := ns.Exists(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
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

			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))
		})

		It("should return error if scheduler doesn't exist", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = ""
			})

			err := controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf(`scheduler "%s" not found`, configYaml1.Name)))
		})

		It("should fail if some error retrieving the scheduler", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				configYaml1.Name,
			).Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
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
			).Return(pg.NewTestResult(errors.New("some error deleting in db"), 0), errors.New("some error deleting in db"))
			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
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

			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
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
			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
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

			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
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
			configYaml1.AutoScaling.Up.Trigger.Limit = 90
			Expect(autoScalingPolicy).To(Equal(configYaml1.AutoScaling))
			Expect(countByStatus).To(Equal(expC))
		})

		It("should fail if error retrieving the scheduler", func() {
			name := "controller-name"
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				name,
			).Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))
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
			).Return(pg.NewTestResult(nil, 0), nil)
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
			).Return(pg.NewTestResult(errors.New("some error in pg"), 0), errors.New("some error in pg"))
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
				pg.NewTestResult(errors.New("pg: no rows in result set"), 0), errors.New("pg: no rows in result set"),
			)
			_, err := controller.ListSchedulersNames(logger, mr, mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				pg.NewTestResult(errors.New("some error in pg"), 0), errors.New("some error in pg"),
			)
			_, err := controller.ListSchedulersNames(logger, mr, mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("DeleteUnavailableRooms", func() {
		It("should delete GRUs", func() {
			scheduler := "pong-free-for-all"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod("", "", roomName, scheduler, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Del(gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Exec().Times(len(expectedRooms))
			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, scheduler, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call redis successfully and exit if no rooms should be deleted", func() {
			scheduler := "pong-free-for-all"

			err := controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, scheduler, "", []string{}, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if failed to delete pod", func() {
			scheduler := "pong-free-for-all"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod("", "", roomName, scheduler, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, name := range expectedRooms {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, scheduler, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if failed to delete pod", func() {
			scheduler := "pong-free-for-all"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			for _, name := range expectedRooms {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, scheduler, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if redis returns an error when deleting old rooms", func() {
			scheduler := "pong-free-for-all"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod("", "", roomName, scheduler, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Del(gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis error")).Times(len(expectedRooms))
			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, scheduler, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ScaleUp", func() {
		It("should fail and return error if error creating pods and initial op", func() {
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

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})

		It("should wait 100ms to create 11th pod", func() {
			amount := 20
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
			mockPipeline.EXPECT().
				SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(amount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(amount)

			start := time.Now().UnixNano()
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			elapsed := time.Now().UnixNano() - start
			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically(">=", 100*time.Millisecond))

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(amount))
		})

		It("should not fail and return error if error creating pods and not initial op", func() {
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount - 1)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times((amount - 1) * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(amount - 1)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(amount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(amount)

			timeoutSec = 0
			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout scaling up scheduler"))
		})

		It("should return error and not scale up if there are Pending pods", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			pod := &v1.Pod{}
			pod.Name = "room-0"
			pod.Status.Phase = v1.PodPending
			_, err = clientset.CoreV1().Pods(scheduler.Name).Create(pod)
			Expect(err).NotTo(HaveOccurred())

			for i := 1; i < amount; i++ {
				pod := &v1.Pod{}
				pod.Name = fmt.Sprintf("room-%d", i)
				_, err := clientset.CoreV1().Pods(scheduler.Name).Create(pod)
				Expect(err).NotTo(HaveOccurred())
			}

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("there are pending pods, check if there are enough CPU and memory to allocate new rooms"))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))

			}
			mockPipeline.EXPECT().Exec()

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(len(configYaml1.Ports))
				mockPipeline.EXPECT().Exec()
			}

			timeoutSec = 300
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))
			}
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for range allStatus {
				mockPipeline.EXPECT().
					SRem(gomock.Any(), gomock.Any())
				mockPipeline.EXPECT().
					ZRem(gomock.Any(), gomock.Any())
			}
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), gomock.Any())
			mockPipeline.EXPECT().Del(gomock.Any())
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec()

			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - 1))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))
			}
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for i := 0; i < scaleDownAmount; i++ {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult("", nil))
			}
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			timeoutSec = 0
			err = controller.ScaleDown(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})

		It("should not return error if delete non existing pod", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleDown
			scaleDownAmount := 1
			names := []string{"non-existing-pod"}
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))
			}
			mockPipeline.EXPECT().Exec()

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))
			}
			mockPipeline.EXPECT().Exec()

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(len(configYaml1.Ports))
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
affinity: maestro-dedicated
toleration: maestro
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
affinity: maestro-dedicated
toleration: maestro
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
affinity: maestro-dedicated
toleration: maestro
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
affinity: maestro-dedicated
toleration: maestro
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
affinity: maestro-dedicated
toleration: maestro
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
affinity: maestro-dedicated
toleration: maestro
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

		It("should return true if env is different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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

		It("should return true if new env", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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

		It("should return true if affinity changes", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-other
toleration: maestro
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
requests:
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

		It("should return true if toleration changes", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro-other
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
requests:
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

		It("should return false if auto scaling are different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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

		It("should return false if delta is different", func() {
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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

		It("should return true if secret vars change", func() {
			yaml1 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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
  - name: MY_SECRET_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: secretname
        value: secretkey
cmd:
  - "./room"
`
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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
  - name: MY_SECRET_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: newsecretname
        value: newsecretkey
cmd:
  - "./room"
`

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeTrue())
		})

		It("should return false if min changes with secret vars", func() {
			yaml1 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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
  - name: MY_SECRET_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: secretname
        value: secretkey
cmd:
  - "./room"
`
			yaml2 := `
name: controller-name
game: controller
image: controller/controller:v123
affinity: maestro-dedicated
toleration: maestro
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
  min: 10
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
  - name: MY_SECRET_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: secretname
        value: secretkey
cmd:
  - "./room"
`

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeFalse())
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			yaml2 = `
name: controller-name
game: controller
image: controller/controller:v124
affinity: maestro-dedicated
toleration: maestro
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
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			for _, pod := range pods.Items {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID)
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml2.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(configYaml2.AutoScaling.Min * len(configYaml2.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml2.AutoScaling.Min)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml2.AutoScaling.Min))

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
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(4))
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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
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
affinity: maestro-dedicated
toleration: maestro
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

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})
			mockDb.EXPECT().
				Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(redis.NewCmdResult(nil, nil))
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("should return error if db fails to select scheduler", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Return(pg.NewTestResult(errors.New("error on select"), 0), errors.New("error on select"))

			err := controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error getting lock"))
		})

		It("should return error if timeout when deleting rooms", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				mockClock,
				nil,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout during room deletion"))
		})

		It("should return error if timeout when creating rooms", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

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

			for _, pod := range pods.Items {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec())

				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(pod.GetName(), pod.GetNamespace())

				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID))
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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				mockClock,
				nil,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout during new room creation"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(BeEmpty())
		})

		It("should return error if timeout after creating rooms", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

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

			for _, pod := range pods.Items {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec())

				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID))
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

				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil)))
				calls.Add(
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil)))
				calls.Add(mockPipeline.EXPECT().Exec())

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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				mockClock,
				nil,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout scaling up scheduler"))
		})

		It("should return error if timeout while waiting for pods", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

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

			for _, pod := range pods.Items {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec())

				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID))
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

				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil)))
				calls.Add(
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil)))
				calls.Add(mockPipeline.EXPECT().Exec())
			}

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec), 0)))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec+100), 0)))

			for i := 0; i < configYaml2.AutoScaling.Min; i++ {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec())

				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				for range allStatus {
					calls.Add(mockPipeline.EXPECT().SRem(gomock.Any(), gomock.Any()))
					calls.Add(mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()))
				}
				calls.Add(mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Del(gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("redis error")))
			}

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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				mockClock,
				nil,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout waiting for rooms to be created"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(BeEmpty())
		})

		It("should not return error if ClearAll fails in deleting old rooms", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

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

			for _, pod := range pods.Items {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec())

				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				calls.Add(mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID))
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

				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
				calls.Add(mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)))
				calls.Add(mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)))
				calls.Add(mockPipeline.EXPECT().Exec())
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
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				mockClock,
				nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update in two steps if maxSurge is 50%", func() {
			maxSurge := 50

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			for _, pod := range pods.Items {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID)
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml2.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(configYaml2.AutoScaling.Min * len(configYaml2.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml2.AutoScaling.Min)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			err = controller.UpdateSchedulerConfig(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				timeoutSec, lockTimeoutMS, maxSurge,
				lockKey,
				&clock.Clock{},
				nil,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml2.AutoScaling.Min))

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
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(4))
			}
		})
	})

	Describe("DeleteRoomsOccupiedTimeout", func() {
		It("should delete rooms that timed out", func() {
			schedulerName := "scheduler-name"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(schedulerName)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			for _, roomName := range expectedRooms {
				pod, err := models.NewPod("", "", roomName, schedulerName, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, schedulerName)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, status), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), roomName)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, schedulerName, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil if there is no dead rooms", func() {
			schedulerName := "scheduler-name"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(schedulerName)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod("", "", roomName, schedulerName, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}
			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, schedulerName, "", []string{}, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return error if error when cleaning room", func() {
			schedulerName := "scheduler-name"

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(schedulerName)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			for _, roomName := range expectedRooms {
				pod, err := models.NewPod("", "", roomName, schedulerName, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}
			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, schedulerName)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, status), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), roomName)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis error"))
			}

			err = controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, schedulerName, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete rooms that were not found on kube", func() {
			schedulerName := "scheduler-name"
			expectedRooms := []string{"room1", "room2", "room3"}

			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, schedulerName)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, status), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), roomName)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err := controller.DeleteUnavailableRooms(logger, mr, mockRedisClient, clientset, schedulerName, "", expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateSchedulerImage", func() {
		var configYaml1 models.ConfigYAML
		var newImageName string

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())

			newImageName = "new-image"
		})

		It("should update image", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			for _, pod := range pods.Items {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
				Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
				Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			mockDb.EXPECT().
				Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())

			mockRedisClient.EXPECT().Ping().AnyTimes()
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(redis.NewCmdResult(nil, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			err = controller.UpdateSchedulerImage(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name, newImageName, lockKey,
				timeoutSec, lockTimeoutMS, maxSurge,
				&clock.Clock{},
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(newImageName))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("should return error if scheduler does not exist", func() {
			newSchedulerName := "new-name"
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", newSchedulerName).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, "", "")
				})

			err := controller.UpdateSchedulerImage(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				newSchedulerName, newImageName, lockKey,
				timeoutSec, lockTimeoutMS, maxSurge,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler new-name not found, create it first"))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(configYaml1.Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("should return error if db fails", func() {
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err := controller.UpdateSchedulerImage(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name, newImageName, lockKey,
				timeoutSec, lockTimeoutMS, maxSurge,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should return config is not yaml nor json", func() {
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, "{invalid: this, is invalid{")
				})

			err := controller.UpdateSchedulerImage(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name, newImageName, lockKey,
				timeoutSec, lockTimeoutMS, maxSurge,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("yaml: did not find expected ',' or '}'"))
		})

		It("should not update scheduler if the image is the same", func() {
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			err := controller.UpdateSchedulerImage(
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name, configYaml1.Image, lockKey,
				timeoutSec, lockTimeoutMS, maxSurge,
				&clock.Clock{},
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateSchedulerMin", func() {
		var configYaml1 models.ConfigYAML

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Min * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update min", func() {
			newMin := 10
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockDb.EXPECT().
				Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())

			mockRedisClient.EXPECT().Ping().AnyTimes()
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(redis.NewCmdResult(nil, nil))
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
				Return(redis.NewBoolResult(true, nil))

			err := controller.UpdateSchedulerMin(logger,
				mr,
				mockDb,
				redisClient,
				configYaml1.Name, lockKey,
				timeoutSec, lockTimeoutMS, newMin,
				&clock.Clock{},
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring("controller-name-"))
				Expect(pod.GetName()).To(HaveLen(len("controller-name-") + 8))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(configYaml1.Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("should not update if min is the same", func() {
			newMin := configYaml1.AutoScaling.Min
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			err := controller.UpdateSchedulerMin(logger,
				mr,
				mockDb,
				redisClient,
				configYaml1.Name, lockKey,
				timeoutSec, lockTimeoutMS, newMin,
				&clock.Clock{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if DB fails", func() {
			newMin := configYaml1.AutoScaling.Min
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err := controller.UpdateSchedulerMin(logger,
				mr,
				mockDb,
				redisClient,
				configYaml1.Name, lockKey,
				timeoutSec, lockTimeoutMS, newMin,
				&clock.Clock{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})
	})

	Describe("ScaleScheduler", func() {
		It("should return error if more than one parameter is set", func() {
			var amountUp, amountDown, replicas uint = 1, 1, 1
			err := controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid scale parameter: can't handle more than one parameter"))
		})

		It("should return error if DB fails", func() {
			var amountUp, amountDown, replicas uint = 1, 0, 0
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err := controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should return error if yaml not found", func() {
			var amountUp, amountDown, replicas uint = 1, 0, 0
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, "", "")
				})

			err := controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler 'controller-name' not found"))
		})

		It("should scaleup if amounUp is positive", func() {
			var amountUp, amountDown, replicas uint = 1, 0, 0

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(2)
			mockPipeline.EXPECT().Exec()

			err := controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(int(amountUp)))
		})

		It("should scaledown if amountDown is positive", func() {
			var amountUp, amountDown, replicas uint = 0, 2, 0

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err := controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			scaleDownAmount := int(amountDown)
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))

			}
			mockPipeline.EXPECT().Exec()

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(len(configYaml1.Ports))
				mockPipeline.EXPECT().Exec()
			}

			err = controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
		})

		It("should scaleUp if replicas is above current number of pods", func() {
			var amountUp, amountDown, replicas uint = 0, 0, 1

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(2)
			mockPipeline.EXPECT().Exec()

			err := controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(int(replicas)))
		})

		It("should scaleDown if replicas is below current number of pods", func() {
			var amountUp, amountDown, replicas uint = 0, 0, 1

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err := controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			scaleDownAmount := scaleUpAmount - int(replicas)
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))
			}
			mockPipeline.EXPECT().Exec()

			for _, name := range names {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(len(configYaml1.Ports))
				mockPipeline.EXPECT().Exec()
			}

			err = controller.ScaleScheduler(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(int(replicas)))
		})
	})

	Describe("SetRoomStatus", func() {
		pKey := "scheduler:controller-name:ping"
		sKey := "scheduler:controller-name:status:ready"
		oKey := "scheduler:controller-name:status:occupied"
		rKey := "scheduler:controller-name:rooms:roomName"
		lKey := "scheduler:controller-name:last:status:occupied"
		roomName := "roomName"
		schedulerName := "controller-name"
		allStatusKeys := []string{
			"scheduler:controller-name:status:ready",
			"scheduler:controller-name:status:creating",
			"scheduler:controller-name:status:occupied",
			"scheduler:controller-name:status:terminating",
			"scheduler:controller-name:status:terminated",
		}
		room := models.NewRoom(roomName, schedulerName)

		It("should not scale up if has enough ready rooms", func() {
			status := "ready"

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"lastPing": time.Now().Unix(),
				"status":   status,
			})
			mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
			mockPipeline.EXPECT().ZRem(lKey, roomName)
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			for _, key := range allStatusKeys {
				if !strings.Contains(key, status) {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
			}
			mockPipeline.EXPECT().Exec()

			err := controller.SetRoomStatus(
				logger,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				status,
				config,
				room,
				schedulerCache,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should scale up if doesn't have enough ready rooms", func() {
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			status := "occupied"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"lastPing": time.Now().Unix(),
				"status":   status,
			})
			mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
			mockPipeline.EXPECT().Eval(models.ZaddIfNotExists, gomock.Any(), roomName)
			mockPipeline.EXPECT().SAdd(oKey, rKey)
			for _, key := range allStatusKeys {
				if !strings.Contains(key, status) {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
			}
			expC := &models.RoomsStatusCount{
				Creating:    0,
				Occupied:    3,
				Ready:       0,
				Terminating: 0,
			}
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			// ScaleUp
			scaleUpAmount := configYaml1.AutoScaling.Up.Delta
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(goredis.NewStringResult("5000", nil)).Times(scaleUpAmount * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err := controller.SetRoomStatus(
				logger,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				status,
				config,
				room,
				schedulerCache,
			)

			Expect(err).NotTo(HaveOccurred())
			time.Sleep(1 * time.Second)
		})

		It("should not scale up if has enough ready rooms", func() {
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			status := "occupied"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"lastPing": time.Now().Unix(),
				"status":   status,
			})
			mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
			mockPipeline.EXPECT().Eval(models.ZaddIfNotExists, gomock.Any(), roomName)
			mockPipeline.EXPECT().SAdd(oKey, rKey)
			for _, key := range allStatusKeys {
				if !strings.Contains(key, status) {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
			}
			expC := &models.RoomsStatusCount{
				Creating:    0,
				Occupied:    3,
				Ready:       3,
				Terminating: 0,
			}
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			err := controller.SetRoomStatus(
				logger,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				status,
				config,
				room,
				schedulerCache,
			)

			time.Sleep(1 * time.Second)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not scale up if error on db", func() {
			status := "occupied"

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Return(pg.NewTestResult(errors.New("some error on db"), 0), errors.New("some error on db"))

			err := controller.SetRoomStatus(
				logger,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				status,
				config,
				room,
				schedulerCache,
			)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error on db"))
		})
	})
})
