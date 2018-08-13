// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"fmt"
	"time"

	goredis "github.com/go-redis/redis"
	mt "github.com/topfreegames/maestro/testing"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/clock"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/fake"
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
	yaml2 = `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port3
    - containerPort: 7655
      protocol: TCP
      name: port4
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
`
)

var _ = Describe("Controller", func() {
	var (
		clientset        *fake.Clientset
		configYaml1      models.ConfigYAML
		opManager        *models.OperationManager
		roomManager      models.RoomManager
		timeoutSec       int
		lockTimeoutMs    int
		lockKey          string
		maxSurge         int
		errDB            error
		numberOfVersions int
		portStart        int
		portEnd          int
		workerPortRange  string
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
		Expect(err).NotTo(HaveOccurred())

		timeoutSec = 300
		lockTimeoutMs = config.GetInt("watcher.lockTimeoutMs")
		lockKey = controller.GetLockKey(config.GetString("watcher.lockKey"), "controller-name")
		maxSurge = 100
		errDB = errors.New("some error in db")
		numberOfVersions = 1
		portStart = 5000
		portEnd = 6000
		workerPortRange = models.NewPortRange(portStart, portEnd).String()

		node := &v1.Node{}
		node.SetName("controller-name")
		node.SetLabels(map[string]string{
			"game": "controller",
		})

		_, err = clientset.CoreV1().Nodes().Create(node)
		Expect(err).NotTo(HaveOccurred())

		mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
		opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)

		roomManager = &models.GameRoom{}
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
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
				Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
				Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			mt.MockInsertScheduler(mockDb, nil)
			mt.MockUpdateSchedulerStatus(mockDb, nil, nil)

			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			err := controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
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

			mt.MockInsertScheduler(mockDb, nil)
			mt.MockUpdateSchedulerStatus(mockDb, nil, nil)

			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
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

		Context("scheduler with port range", func() {
			It("should create scheduler with port range", func() {
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
portRange:
  start: 10000
  end: 10010
`
				var configYaml1 models.ConfigYAML
				err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					}).Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
					Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
					Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

				mt.MockInsertScheduler(mockDb, nil)
				mt.MockUpdateSchedulerStatus(mockDb, nil, nil)

				mockDb.EXPECT().Query(gomock.Any(), `SELECT name FROM schedulers`)
				mockDb.EXPECT().Query(gomock.Any(), `SELECT * FROM schedulers WHERE name IN (?)`, gomock.Any())

				mockRedisClient.EXPECT().
					Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				schedulerPortStart := configYaml1.PortRange.Start
				schedulerPortEnd := configYaml1.PortRange.End
				mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser,
					workerPortRange, schedulerPortStart, schedulerPortEnd)

				err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
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

			It("should return error if port range is the same as worker", func() {
				yamlStr := `
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
portRange:
  start: 5000
  end: 5010
`
				var configYaml1 models.ConfigYAML
				err := yaml.Unmarshal([]byte(yamlStr), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().Query(gomock.Any(), `SELECT name FROM schedulers`)
				mockDb.EXPECT().Query(gomock.Any(), `SELECT * FROM schedulers WHERE name IN (?)`, gomock.Any())
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).Return(goredis.NewStringResult(workerPortRange, nil))

				mt.MockInsertScheduler(mockDb, nil)

				mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

				err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("scheduler trying to use ports used by pool 'global'"))

				ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ns.Items).To(HaveLen(0))

				pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(0))
			})
		})

		It("should return error if namespace already exists", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			namespace := models.NewNamespace(configYaml1.Name)
			err = namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
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

			mt.MockInsertScheduler(mockDb, errDB)

			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errDB.Error()))

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

			mt.MockInsertScheduler(mockDb, nil)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())
			mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis"))
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
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

			mt.MockUpdateSchedulerStatus(mockDb, errDB, nil)

			err = controller.DeleteScheduler(logger, mr, mockDb, mockRedisClient, clientset, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating status on schedulers: some error in db"))
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
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{
				Creating:    4,
				Occupied:    3,
				Ready:       2,
				Terminating: 1,
			}
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(expC.Terminating), nil))
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
			mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis"))
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

			mt.MockUpdateSchedulerStatus(mockDb, nil, nil)

			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if fail to update on schedulers table", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()

			mt.MockUpdateSchedulerStatus(mockDb, errDB, nil)

			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating status on schedulers: some error in db"))
		})

		It("should fail if fail to insert on scheduler_versions table", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()

			mt.MockUpdateSchedulerStatus(mockDb, nil, errDB)

			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error inserting on scheduler_versions: some error in db"))
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
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}
			configYaml, _ := models.NewConfigYAML(scheduler.YAML)

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod(roomName, nil, configYaml, clientset, mockRedisClient)
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
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Del(gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Exec().Times(len(expectedRooms))
			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset, scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit if no rooms should be deleted", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}

			err := controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset, scheduler, []string{}, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if failed to delete pod", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}
			configYaml, _ := models.NewConfigYAML(scheduler.YAML)

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod(roomName, nil, configYaml, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, name := range expectedRooms {
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

			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset, scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if failed to delete pod", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			for _, name := range expectedRooms {
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

			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset, scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error if redis returns an error when deleting old rooms", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}
			configYaml, _ := models.NewConfigYAML(scheduler.YAML)

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod(roomName, nil, configYaml, clientset, mockRedisClient)
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
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Del(gomock.Any()).Times(len(expectedRooms))
			mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis error")).Times(len(expectedRooms))
			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset, scheduler, expectedRooms, "deletion_reason")
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
			mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis"))

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
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

			mockRedisClient.EXPECT().
				Get(models.GlobalPortsPoolKey).
				Return(goredis.NewStringResult(workerPortRange, nil)).
				Times(amount)
			mockPortChooser.EXPECT().
				Choose(portStart, portEnd, 2).
				Return([]int{5000, 5001}).
				Times(amount)

			start := time.Now().UnixNano()
			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
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
			mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis"))
			mockPipeline.EXPECT().Exec().Times(amount - 1)

			mockRedisClient.EXPECT().
				Get(models.GlobalPortsPoolKey).
				Return(goredis.NewStringResult(workerPortRange, nil)).
				Times(amount - 1)
			mockPortChooser.EXPECT().
				Choose(portStart, portEnd, 2).
				Return([]int{5000, 5001}).
				Times(amount - 1)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, false)
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

			mockRedisClient.EXPECT().
				Get(models.GlobalPortsPoolKey).
				Return(goredis.NewStringResult(workerPortRange, nil)).
				Times(amount)
			mockPortChooser.EXPECT().
				Choose(portStart, portEnd, 2).
				Return([]int{5000, 5001}).
				Times(amount)

			timeoutSec = 0
			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
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

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("there are pending pods, check if there are enough CPU and memory to allocate new rooms"))
		})

		It("should scaleup pods with two containers", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml2)

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

			mockRedisClient.EXPECT().
				Get(models.GlobalPortsPoolKey).
				Return(goredis.NewStringResult(workerPortRange, nil)).
				Times(amount)
			mockPortChooser.EXPECT().
				Choose(portStart, portEnd, 2).
				Return([]int{5000, 5001}).
				Times(amount * 2)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).NotTo(HaveOccurred())

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(amount))
		})

		It("should scale up using correct port when scheduler with port range", func() {
			yamlStr := `
name: controller-name
game: controller
autoscaling:
  min: 5
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
containers:
- name: container1
  image: controller/controller:v123
  ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
portRange:
  start: 10000
  end: 10010
`
			amount := 5
			var configYaml models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlStr), &configYaml)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml.Name, configYaml.Game, yamlStr)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(amount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().Exec().Times(amount)

			schedulerPortStart := configYaml.PortRange.Start
			schedulerPortEnd := configYaml.PortRange.End
			mt.MockGetPortsFromPool(&configYaml, mockRedisClient, mockPortChooser,
				workerPortRange, schedulerPortStart, schedulerPortEnd)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true)
			Expect(err).NotTo(HaveOccurred())

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(amount))
		})
	})

	Describe("ScaleDown", func() {
		It("should succeed in scaling down", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 3
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser,
				workerPortRange, portStart, portEnd)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))

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
			err = controller.ScaleDown(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
		})

		It("should scale down and scheduler with port range", func() {
			yamlStr := `
name: controller-name
game: controller
image: controller/controller:v123
ports:
- containerPort: 1234
  protocol: UDP
  name: port1
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
portRange:
  start: 10000
  end: 10010
`
			var configYaml models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlStr), &configYaml)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml.Name, configYaml.Game, yamlStr)

			// ScaleUp
			scaleUpAmount := 3
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			mt.MockGetPortsFromPool(&configYaml, mockRedisClient, mockPortChooser,
				workerPortRange, configYaml.PortRange.Start, configYaml.PortRange.End)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))

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
			err = controller.ScaleDown(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleDownAmount, timeoutSec)
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
			scaleUpAmount := 3
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
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
			mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis"))

			err = controller.ScaleDown(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
			Expect(err).To(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - 1))
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
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
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
			err = controller.ScaleDown(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return timeout error", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 3
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(scaleUpAmount)
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
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

			timeoutSec = 0
			err = controller.ScaleDown(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleDownAmount, timeoutSec)
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

		It(`should return true if configs of different versions and new version
		has more than one container`, func() {
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
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port1
    - containerPort: 7655
      protocol: TCP
      name: port2
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
`
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeTrue())
			Expect(controller.MustUpdatePods(&configYaml2, &configYaml1)).To(BeTrue())
		})

		It(`should return true if both configs of version 2 but with different
		number of containers`, func() {
			yaml1 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
`
			yaml2 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port1
    - containerPort: 7655
      protocol: TCP
      name: port2
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
`
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeTrue())
		})

		It(`should return true if both configs of version 2 but affinity
		or toleration changes`, func() {
			yaml1 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
`
			yaml2 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
`
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())

			configYaml1.NodeAffinity = "affinity1"
			configYaml2.NodeAffinity = "affinity2"
			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeTrue())

			configYaml1.NodeAffinity = "affinity"
			configYaml2.NodeAffinity = "affinity"

			configYaml1.NodeToleration = "toleration1"
			configYaml2.NodeToleration = "toleration2"
			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeTrue())
		})

		It(`should return false if both configs of version 2 and autoscaling
		parameters change`, func() {
			yaml1 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
      usage: 20
      time: 500
    cooldown: 500
containers:
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port1
    - containerPort: 7655
      protocol: TCP
      name: port2
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
`
			yaml2 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
containers:
- name: container1
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
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port1
    - containerPort: 7655
      protocol: TCP
      name: port2
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
`

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())

			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeFalse())
		})

		It(`should return true if both configs of version 2 and image
		of one container changes`, func() {
			yaml1 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
      usage: 20
      time: 500
    cooldown: 500
containers:
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port1
    - containerPort: 7655
      protocol: TCP
      name: port2
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
- name: container1
  image: controller/controller:v1
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
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
`
			yaml2 := `
name: controller-name
game: controller
affinity: maestro-dedicated
toleration: maestro
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
      usage: 20
      time: 500
    cooldown: 500
containers:
- name: container1
  image: controller/controller:v2
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
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./room"
- name: container2
  image: helper/helper:v1
  ports:
    - containerPort: 1235
      protocol: UDP
      name: port1
    - containerPort: 7655
      protocol: TCP
      name: port2
  limits:
    memory: "66Mi"
    cpu: "2"
  requests:
    memory: "66Mi"
    cpu: "2"
  env:
    - name: MY_ENV_VAR
      value: myvalue
  cmd:
    - "./helper"
`
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())

			Expect(controller.MustUpdatePods(&configYaml1, &configYaml2)).To(BeTrue())
		})
	})

	Describe("UpdateSchedulerConfig", func() {
		var configYaml1, configYaml2 models.ConfigYAML
		var scheduler1 *models.Scheduler
		var yaml2 string
		var err error

		BeforeEach(func() {
			err = yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
				logger, roomManager, mr, yaml1, timeoutSec, mockPortChooser, workerPortRange, portStart, portEnd)

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
  - name: MY_NEW_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should recreate rooms with new ENV VARS and image", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			for _, pod := range pods.Items {
				Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
				Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
			}

			// Select current scheduler yaml
			mt.MockSelectScheduler(yaml1, mockDb, nil)

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Remove old rooms
			mt.MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml2)

			// Create new roome
			// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
			mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml2)
			mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(configYaml2.Name))

			pods, err = clientset.CoreV1().Pods(configYaml2.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

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
				Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
				Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v2.0"))
			}
		})

		Context("Port Range", func() {
			It("should recreate rooms with new ports if added port range", func() {
				yaml2 = `
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
portRange:
  start: 20000
  end: 20010
`
				var configYaml2 models.ConfigYAML
				yaml.Unmarshal([]byte(yaml2), &configYaml2)

				pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				// Select current scheduler yaml
				mt.MockSelectScheduler(yaml1, mockDb, nil)

				// Get redis lock
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

				// Set new operation manager description
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

				// check other scheduler ports
				mt.MockSelectSchedulerNames(mockDb, []string{}, nil)
				mt.MockSelectConfigYamls(mockDb, []models.Scheduler{}, nil)
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				// Remove old rooms
				mt.MockRemoveRoomStatusFromRedis(mockRedisClient, mockPipeline, pods, &configYaml2)

				// Create new rooms
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				mt.MockCreateRoomsWithPorts(mockRedisClient, mockPipeline, &configYaml2)
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser,
					workerPortRange, configYaml2.PortRange.Start, configYaml2.PortRange.End)

				// Update new config on schedulers table
				mt.MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

				err = controller.UpdateSchedulerConfig(context.Background(), logger,
					roomManager, mr, mockDb, redisClient, clientset, &configYaml2,
					maxSurge, &clock.Clock{}, nil, config, opManager)
				Expect(err).NotTo(HaveOccurred())

				ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ns.Items).To(HaveLen(1))
				Expect(ns.Items[0].GetName()).To(Equal(configYaml2.Name))

				pods, err = clientset.CoreV1().Pods(configYaml2.Name).List(metav1.ListOptions{})
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
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v2.0"))
				}
			})

			It("should recreate rooms with new ports if remove port range", func() {
				yaml1 := `
name: controller-name-ports
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
portRange:
  start: 20000
  end: 20010
`
				var configYaml1 models.ConfigYAML
				yaml.Unmarshal([]byte(yaml1), &configYaml1)

				scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

				mt.MockSelectSchedulerNames(mockDb, []string{}, nil)
				mt.MockSelectConfigYamls(mockDb, []models.Scheduler{}, nil)
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, roomManager, mr, yaml1, timeoutSec, mockPortChooser, workerPortRange, 20000, 20010)

				yaml2 = `
name: controller-name-ports
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
				var configYaml2 models.ConfigYAML
				yaml.Unmarshal([]byte(yaml2), &configYaml2)

				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				// Select current scheduler yaml
				mt.MockSelectScheduler(yaml1, mockDb, nil)

				// Get redis lock
				lockKey = controller.GetLockKey(config.GetString("watcher.lockKey"), "controller-name-ports")
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

				// Set new operation manager description
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old rooms
				mt.MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml1)

				// Create new rooms
				mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml2)
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser,
					workerPortRange, portStart, portEnd)

				// Update new config on schedulers table
				mt.MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

				err = controller.UpdateSchedulerConfig(context.Background(), logger,
					roomManager, mr, mockDb, redisClient,
					clientset, &configYaml2, maxSurge, &clock.Clock{}, nil, config, opManager)
				Expect(err).NotTo(HaveOccurred())

				pods, err = clientset.CoreV1().Pods(configYaml2.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				for _, pod := range pods.Items {
					Expect(pod.GetName()).To(ContainSubstring("controller-name-ports"))
					Expect(pod.GetName()).To(HaveLen(len("controller-name-ports-") + 8))
					Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
					Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
					Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
					Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name-ports"))
					Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
					Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
					Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v2.0"))
				}
			})

			It("should recreate rooms with new ports if change port range", func() {
				yaml1 := `
name: controller-name-ports
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
portRange:
  start: 20000
  end: 20010
`
				var configYaml1 models.ConfigYAML
				err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

				mt.MockSelectSchedulerNames(mockDb, []string{}, nil)
				mt.MockSelectConfigYamls(mockDb, []models.Scheduler{}, nil)
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, roomManager, mr, yaml1, timeoutSec, mockPortChooser, workerPortRange, 20000, 20010)

				yaml2 = `
name: controller-name-ports
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
portRange:
  start: 20000
  end: 20020
`
				var configYaml2 models.ConfigYAML
				err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
				Expect(err).NotTo(HaveOccurred())

				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				// Select current scheduler yaml
				mt.MockSelectScheduler(yaml1, mockDb, nil)

				// Get redis lock
				lockKey = controller.GetLockKey(config.GetString("watcher.lockKey"), "controller-name-ports")
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

				// Set new operation manager description
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

				// check other scheduler ports
				mt.MockSelectSchedulerNames(mockDb, []string{}, nil)
				mt.MockSelectConfigYamls(mockDb, []models.Scheduler{}, nil)
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				// Remove old rooms
				mt.MockRemoveRoomStatusFromRedis(mockRedisClient, mockPipeline, pods, &configYaml1)

				// Create new rooms
				mt.MockCreateRoomsWithPorts(mockRedisClient, mockPipeline, &configYaml2)
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser,
					workerPortRange, 20000, 20020)

				// Update new config on schedulers table
				mt.MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

				err = controller.UpdateSchedulerConfig(context.Background(), logger,
					roomManager, mr, mockDb, redisClient,
					clientset, &configYaml2, maxSurge, &clock.Clock{}, nil, config, opManager)
				Expect(err).NotTo(HaveOccurred())

				pods, err = clientset.CoreV1().Pods(configYaml2.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				for _, pod := range pods.Items {
					Expect(pod.GetName()).To(ContainSubstring("controller-name-ports"))
					Expect(pod.GetName()).To(HaveLen(len("controller-name-ports-") + 8))
					Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
					Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
					Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
					Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name-ports"))
					Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
					Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
					Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v2.0"))
				}
			})
		})

		It("should return error if updating unexisting scheduler", func() {
			yaml2 := `name: another-name`
			configYaml2, err := models.NewConfigYAML(yaml2)
			Expect(err).NotTo(HaveOccurred())

			lockKey := controller.GetLockKey(config.GetString("watcher.lockKey"), configYaml2.Name)

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Select empty scheduler yaml
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
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
  - "./room"
`
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			var configYaml2 models.ConfigYAML
			err = yaml.Unmarshal([]byte(yaml2), &configYaml2)
			mt.MockSelectScheduler(yaml2, mockDb, nil)

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMinorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
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
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(true, nil))

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(goredis.NewCmdResult(nil, nil))
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
				Return(pg.NewTestResult(errors.New("error on select"), 0), errors.New("error on select"))

			err := controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error on select"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.DatabaseError"))
		})

		It("should return error if timeout waiting for lock", func() {
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(true, nil))

			config.Set("updateTimeoutSeconds", 0)
			err := controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout while wating for redis lock"))
			config.Set("updateTimeoutSeconds", timeoutSec)
		})

		It("should timeout after lock is always connected", func() {
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(true, nil))
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(false, nil))

			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
				Return(goredis.NewCmdResult(nil, nil))

			lock, err := redisClient.EnterCriticalSection(redisClient.Client, lockKey, time.Duration(lockTimeoutMs)*time.Millisecond, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			defer redisClient.LeaveCriticalSection(lock)

			config.Set("updateTimeoutSeconds", 1)
			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout while wating for redis lock"))
			config.Set("updateTimeoutSeconds", timeoutSec)
		})

		It("should return error if error occurred on lock", func() {
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(true, errors.New("error getting lock")))

			err := controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
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

			// Get lock
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))
			calls.Append(
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil))

			// Set new operation manager description
			calls.Append(
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil))

			// Get scheduler from DB
			calls.Append(
				mt.MockSelectScheduler(yaml1, mockDb, nil))

			// Delete first room
			calls.Append(
				mt.MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml1))

			// Get timeout for waiting pod to be deleted and create new one
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec+100), 0)))

			// Timeod out, so rollback
			calls.Append(
				mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml1))
			calls.Append(mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser,
				workerPortRange, portStart, portEnd))

			calls.Append(
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil))

			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				mockClock,
				nil,
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timedout waiting rooms to be replaced, rolled back"))
		})

		It("should return error if timeout when creating rooms", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Update scheduler
			calls := mt.NewCalls()

			// Get lock
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))
			calls.Add(
				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
					Return(goredis.NewBoolResult(true, nil)))

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Get scheduler from DB
			calls.Add(
				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
					}))

			// Delete first room
			for _, pod := range pods.Items {
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
				break
			}

			// Get timeout for waiting pod to be deleted and create new one
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec-100), 0)))

			// Create new room with updated scheduler
			for range pods.Items {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
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
				calls.Add(
					mockRedisClient.EXPECT().
						Get(models.GlobalPortsPoolKey).
						Return(goredis.NewStringResult(workerPortRange, nil)))
				calls.Add(
					mockPortChooser.EXPECT().
						Choose(portStart, portEnd, 2).
						Return([]int{5000, 5001}))
				break
			}

			// Get timeout for waiting new pod to be created
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(int64(timeoutSec+100), 0)))

			// Timed out, so rollback
			// Delete newly created room
			for range pods.Items {
				calls.Add(
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))

				for range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(gomock.Any(), gomock.Any()))
					calls.Add(mockPipeline.EXPECT().
						ZRem(gomock.Any(), gomock.Any()))
				}
				calls.Add(mockPipeline.EXPECT().ZRem(gomock.Any(), gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Del(gomock.Any()))
				calls.Add(mockPipeline.EXPECT().Exec())
				break
			}

			// Create new room to replace the one with old version deleted
			for range pods.Items {
				calls.Add(mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline))
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
				calls.Add(
					mockRedisClient.EXPECT().
						Get(models.GlobalPortsPoolKey).
						Return(goredis.NewStringResult(workerPortRange, nil)))
				calls.Add(
					mockPortChooser.EXPECT().
						Choose(portStart, portEnd, 2).
						Return([]int{5000, 5001}))
				break
			}

			calls.Add(
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(goredis.NewCmdResult(nil, nil)))
			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				10,
				mockClock,
				nil,
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timedout waiting rooms to be replaced, rolled back"))
		})

		It("should not return error if ClearAll fails in deleting old rooms", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Update scheduler
			calls := mt.NewCalls()

			// Get lock
			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))
			calls.Append(
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil))

			// Set new operation manager description
			calls.Append(
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil))

			// Get scheduler from DB
			calls.Append(mt.MockSelectScheduler(yaml1, mockDb, nil))

			// Delete rooms
			errRedis := errors.New("redis error")
			for _, pod := range pods.Items {
				// Retrieve ports to pool
				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				calls.Add(
					mockRedisClient.EXPECT().
						TxPipeline().
						Return(mockPipeline))
				for _, status := range allStatus {
					calls.Add(
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
					calls.Add(
						mockPipeline.EXPECT().
							ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID))
				}
				calls.Add(
					mockPipeline.EXPECT().
						ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID))
				calls.Add(
					mockPipeline.EXPECT().
						Del(room.GetRoomRedisKey()))
				calls.Add(
					mockPipeline.EXPECT().
						Exec().
						Return(nil, errRedis))
			}

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))

			// Create new pods
			calls.Append(
				mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml2))
			calls.Append(
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser,
					workerPortRange, portStart, portEnd))

			calls.Add(
				mockClock.EXPECT().
					Now().
					Return(time.Unix(0, 0)))

			calls.Append(
				mt.MockUpdateSchedulersTable(mockDb, nil))

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			calls.Append(
				mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil))

			// Count to delete old versions if necessary
			calls.Append(
				mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil))

			calls.Append(
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil))

			calls.Finish()

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				100,
				mockClock,
				nil,
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

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

		It("should update in two steps if maxSurge is 50%", func() {
			maxSurge := 50

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			oldPodNames := make([]string, len(pods.Items))
			for i, pod := range pods.Items {
				oldPodNames[i] = pod.GetName()
			}

			// Select current scheduler yaml
			mt.MockSelectScheduler(yaml1, mockDb, nil)

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Remove old rooms
			mt.MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml2)

			// Create new roome
			// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
			mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml2)
			mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			err = controller.UpdateSchedulerConfig(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				&configYaml2,
				maxSurge,
				&clock.Clock{},
				nil,
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("controller-name"))

			pods, err = clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

			names := make([]string, len(pods.Items))
			for i, pod := range pods.Items {
				names[i] = pod.GetName()

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

		Context("canceled operation", func() {
			It("should stop on redis lock", func() {
				pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				// Get redis lock
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

				mt.MockDeleteRedisKey(opManager, mockRedisClient, mockPipeline, nil)
				opManager.Cancel(opManager.GetOperationKey())

				err = controller.UpdateSchedulerConfig(
					context.Background(),
					logger,
					roomManager,
					mr,
					mockDb,
					redisClient,
					clientset,
					&configYaml2,
					maxSurge,
					&clock.Clock{},
					nil,
					config,
					opManager,
				)
				Expect(err).NotTo(HaveOccurred())

				ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ns.Items).To(HaveLen(1))
				Expect(ns.Items[0].GetName()).To(Equal(configYaml2.Name))

				pods, err = clientset.CoreV1().Pods(configYaml2.Name).List(metav1.ListOptions{})
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
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}
			})

			It("should stop on createPodsAsTheyAreDeleted", func() {
				yamlString := `
name: scheduler-name-cancel
autoscaling:
  min: 3
containers:
- name: container1
  image: image1
`
				newYamlString := `
name: scheduler-name-cancel
autoscaling:
  min: 3
containers:
- name: container1
  image: image2
`
				configYaml, _ := models.NewConfigYAML(yamlString)
				newConfigYaml, err := models.NewConfigYAML(newYamlString)
				Expect(err).ToNot(HaveOccurred())

				mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, roomManager, mr, yamlString, timeoutSec, mockPortChooser, workerPortRange, portStart, portEnd)

				pods, err := clientset.CoreV1().Pods("scheduler-name-cancel").List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				// Select current scheduler yaml
				mt.MockSelectScheduler(newYamlString, mockDb, nil)

				// Get redis lock
				lockKey := "maestro-lock-key-scheduler-name-cancel"
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

				// Set new operation manager description
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old room
				for i, pod := range pods.Items {
					room := models.NewRoom(pod.GetName(), pod.GetNamespace())
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)

					for _, status := range allStatus {
						mockPipeline.EXPECT().
							SRem(
								models.GetRoomStatusSetRedisKey(room.SchedulerName, status),
								room.GetRoomRedisKey())
						mockPipeline.EXPECT().ZRem(
							models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
					}
					mockPipeline.EXPECT().
						ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID)

					if i == len(pods.Items)-1 {
						mockPipeline.EXPECT().Del(room.GetRoomRedisKey()).Do(func(_ string) {
							opManager.Cancel(opManager.GetOperationKey())
						})
					} else {
						mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
					}

					mockPipeline.EXPECT().Exec()
				}

				// Delete keys from OperationManager (to cancel it)
				mt.MockDeleteRedisKey(opManager, mockRedisClient, mockPipeline, nil)

				// Create rooms to rollback
				mt.MockCreateRooms(mockRedisClient, mockPipeline, newConfigYaml)
				mt.MockGetPortsFromPool(newConfigYaml, mockRedisClient, mockPortChooser,
					workerPortRange, portStart, portEnd)

				// Retrieve redis lock
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

				err = controller.UpdateSchedulerConfig(
					context.Background(),
					logger,
					roomManager,
					mr,
					mockDb,
					redisClient,
					clientset,
					configYaml,
					maxSurge,
					&clock.Clock{},
					nil,
					config,
					opManager,
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation was canceled, rolled back"))

				pods, err = clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				for _, pod := range pods.Items {
					Expect(pod.GetName()).To(ContainSubstring("scheduler-name-cancel-"))
					Expect(pod.GetName()).To(HaveLen(len("scheduler-name-cancel-") + 8))
					Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
					Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("scheduler-name-cancel"))
					Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_ROOM_ID"))
					Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal(pod.GetName()))
					Expect(pod.Spec.Containers[0].Env).To(HaveLen(2))
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}
			})

			It("should stop on waitCreatingPods", func() {
				yamlString := `
name: scheduler-name-cancel
autoscaling:
  min: 3
containers:
- name: container1
  image: image1
`
				newYamlString := `
name: scheduler-name-cancel
autoscaling:
  min: 3
containers:
- name: container1
  image: image2
`
				configYaml, _ := models.NewConfigYAML(yamlString)
				newConfigYaml, _ := models.NewConfigYAML(newYamlString)

				mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, roomManager, mr, yamlString, timeoutSec, mockPortChooser, workerPortRange, portStart, portEnd)

				pods, err := clientset.CoreV1().Pods("scheduler-name-cancel").List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				// Select current scheduler yaml
				mt.MockSelectScheduler(newYamlString, mockDb, nil)

				// Get redis lock
				lockKey := "maestro-lock-key-scheduler-name-cancel"
				mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

				// Set new operation manager description
				mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

				// Delete keys from OperationManager (to cancel it)
				mt.MockDeleteRedisKey(opManager, mockRedisClient, mockPipeline, nil)

				// Create rooms to rollback
				mt.MockCreateRooms(mockRedisClient, mockPipeline, newConfigYaml)

				// But first, create rooms
				for i := 0; i < configYaml.AutoScaling.Min; i++ {
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any())
					mockPipeline.EXPECT().SAdd(
						models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"), gomock.Any())
					if i == configYaml.AutoScaling.Min-1 {
						mockPipeline.EXPECT().
							ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any()).
							Do(func(_ string, _ ...goredis.Z) {
								opManager.Cancel(opManager.GetOperationKey())
							})
					} else {
						mockPipeline.EXPECT().
							ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any())
					}
					mockPipeline.EXPECT().Exec()
				}

				// Delete newly created rooms
				for i := 0; i < configYaml2.AutoScaling.Min; i++ {
					mockRedisClient.EXPECT().TxPipeline().
						Return(mockPipeline)
					for _, status := range allStatus {
						mockPipeline.EXPECT().SRem(
							models.GetRoomStatusSetRedisKey(configYaml.Name, status), gomock.Any())
						mockPipeline.EXPECT().ZRem(
							models.GetLastStatusRedisKey(configYaml.Name, status), gomock.Any())
					}
					mockPipeline.EXPECT().ZRem(
						models.GetRoomPingRedisKey(configYaml.Name), gomock.Any())
					mockPipeline.EXPECT().Del(gomock.Any())
					mockPipeline.EXPECT().Exec()
				}

				// But first, remove old rooms
				for i := 0; i < configYaml2.AutoScaling.Min; i++ {
					mockRedisClient.EXPECT().TxPipeline().
						Return(mockPipeline)
					for _, status := range allStatus {
						mockPipeline.EXPECT().SRem(
							models.GetRoomStatusSetRedisKey(configYaml.Name, status), gomock.Any())
						mockPipeline.EXPECT().ZRem(
							models.GetLastStatusRedisKey(configYaml.Name, status), gomock.Any())
					}
					mockPipeline.EXPECT().ZRem(
						models.GetRoomPingRedisKey(configYaml.Name), gomock.Any())
					mockPipeline.EXPECT().Del(gomock.Any())
					mockPipeline.EXPECT().Exec()
				}

				// Retrieve redis lock
				mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

				err = controller.UpdateSchedulerConfig(
					context.Background(),
					logger,
					roomManager,
					mr,
					mockDb,
					redisClient,
					clientset,
					configYaml,
					maxSurge,
					&clock.Clock{},
					nil,
					config,
					opManager,
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation was canceled, rolled back"))

				pods, err = clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				for _, pod := range pods.Items {
					Expect(pod.GetName()).To(ContainSubstring("scheduler-name-cancel-"))
					Expect(pod.GetName()).To(HaveLen(len("scheduler-name-cancel-") + 8))
					Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
					Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("scheduler-name-cancel"))
					Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_ROOM_ID"))
					Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal(pod.GetName()))
					Expect(pod.Spec.Containers[0].Env).To(HaveLen(2))
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}
			})
		})
	})

	Describe("DeleteRoomsOccupiedTimeout", func() {
		It("should delete rooms that timed out", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}
			configYaml, _ := models.NewConfigYAML(scheduler.YAML)

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			for _, roomName := range expectedRooms {
				pod, err := models.NewPod(roomName, nil, configYaml, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, scheduler.Name)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(scheduler.Name, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(scheduler.Name, status), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), roomName)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset,
				scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil if there is no dead rooms", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}
			configYaml, _ := models.NewConfigYAML(scheduler.YAML)

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			for _, roomName := range expectedRooms {
				pod, err := models.NewPod(roomName, nil, configYaml, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}
			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset,
				scheduler, []string{}, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return error if error when cleaning room", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}
			configYaml, _ := models.NewConfigYAML(scheduler.YAML)

			expectedRooms := []string{"room1", "room2", "room3"}
			namespace := models.NewNamespace(scheduler.Name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			for _, roomName := range expectedRooms {
				pod, err := models.NewPod(roomName, nil, configYaml, clientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}
			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, scheduler.Name)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(scheduler.Name, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(scheduler.Name, status), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), roomName)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis error"))
			}

			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset,
				scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete rooms that were not found on kube", func() {
			scheduler := &models.Scheduler{Name: "scheduler-name", YAML: `name: scheduler-name`}

			expectedRooms := []string{"room1", "room2", "room3"}

			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, scheduler.Name)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(scheduler.Name, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(scheduler.Name, status), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), roomName)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
			}

			err := controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset,
				scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateSchedulerImage", func() {
		var configYaml1 models.ConfigYAML
		var imageParams *models.SchedulerImageParams
		var scheduler1 *models.Scheduler

		BeforeEach(func() {
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
				logger, roomManager, mr, yaml1, timeoutSec, mockPortChooser, workerPortRange, portStart, portEnd)

			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			imageParams = &models.SchedulerImageParams{
				Image: "new-image",
			}
		})

		It("should update image", func() {
			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(3))

			// Select current scheduler yaml
			mt.MockSelectScheduler(yaml1, mockDb, nil)

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Remove old rooms
			mt.MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml1)

			// Create new roome
			// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
			mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml1)
			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			err = controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
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
				Expect(pod.Spec.Containers[0].Image).To(Equal(imageParams.Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("should return error if scheduler does not exist", func() {
			newSchedulerName := "new-name"
			configYaml1.Name = newSchedulerName

			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", newSchedulerName).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, "", "")
				})

			err := controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
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
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
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
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
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

			imageParams.Image = configYaml1.Image
			err := controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml1.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateSchedulerImage for configYaml v2", func() {
		var configYaml models.ConfigYAML
		var imageParams *models.SchedulerImageParams
		var scheduler1 *models.Scheduler

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml2), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			scheduler1 = models.NewScheduler(configYaml.Name, configYaml.Game, yaml1)
			imageParams = &models.SchedulerImageParams{
				Image: "new-image",
			}
		})

		It("Should update scheduler with configYaml v2", func() {
			var namespace = configYaml.Name

			// Create scheduler
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb, logger, roomManager, mr, yaml2, timeoutSec,
				mockPortChooser, workerPortRange, portStart, portEnd)

			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			// Update scheduler
			mt.MockSelectScheduler(yaml2, mockDb, nil)

			// Get lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Remove old rooms
			mt.MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

			// Create new rooms
			mt.MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
			mt.MockGetPortsFromPool(&configYaml, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// return lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			imageParams.Container = "container1"
			err = controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(namespace))

			pods, err = clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring(namespace))
				Expect(pod.GetName()).To(HaveLen(len(namespace) + 9))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal(configYaml.Name))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(imageParams.Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("Should not update scheduler with configYaml v2 if image is the same", func() {
			err := yaml.Unmarshal([]byte(yaml2), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			var namespace = configYaml.Name

			// Create scheduler
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb, logger, roomManager, mr, yaml2, timeoutSec,
				mockPortChooser, workerPortRange, portStart, portEnd)

			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			// Update scheduler
			mt.MockSelectScheduler(yaml2, mockDb, nil)

			imageParams.Container = configYaml.Containers[0].Name
			imageParams.Image = configYaml.Containers[0].Image
			err = controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(namespace))

			pods, err = clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring(namespace))
				Expect(pod.GetName()).To(HaveLen(len(namespace) + 9))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(imageParams.Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("Should not update scheduler with configYaml v2 if container name is invalid", func() {
			err := yaml.Unmarshal([]byte(yaml2), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			var namespace = configYaml.Name

			// Create scheduler
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb, logger, roomManager, mr, yaml2, timeoutSec,
				mockPortChooser, workerPortRange, portStart, portEnd)

			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			// Update scheduler
			mt.MockSelectScheduler(yaml2, mockDb, nil)

			imageParams.Container = "invalid-container"
			imageParams.Image = configYaml.Containers[0].Image
			err = controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no container with name invalid-container"))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(namespace))

			pods, err = clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring(namespace))
				Expect(pod.GetName()).To(HaveLen(len(namespace) + 9))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(configYaml.Containers[0].Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})

		It("Should not update scheduler and return error if configYaml v2 and container name is empty", func() {
			err := yaml.Unmarshal([]byte(yaml2), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			var namespace = configYaml.Name

			// Create scheduler
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb, logger, roomManager, mr, yaml2, timeoutSec,
				mockPortChooser, workerPortRange, portStart, portEnd)

			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			// Update scheduler
			mt.MockSelectScheduler(yaml2, mockDb, nil)

			imageParams.Container = ""
			imageParams.Image = configYaml.Containers[0].Image
			err = controller.UpdateSchedulerImage(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				configYaml.Name,
				imageParams,
				maxSurge,
				&clock.Clock{},
				config,
				opManager,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("need to specify container name"))

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(namespace))

			pods, err = clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

			for _, pod := range pods.Items {
				Expect(pod.GetName()).To(ContainSubstring(namespace))
				Expect(pod.GetName()).To(HaveLen(len(namespace) + 9))
				Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MY_ENV_VAR"))
				Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("myvalue"))
				Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
				Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal("controller-name"))
				Expect(pod.Spec.Containers[0].Env[2].Name).To(Equal("MAESTRO_ROOM_ID"))
				Expect(pod.Spec.Containers[0].Env[2].Value).To(Equal(pod.GetName()))
				Expect(pod.Spec.Containers[0].Image).To(Equal(configYaml.Containers[0].Image))
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(3))
			}
		})
	})

	Describe("UpdateSchedulerMin", func() {
		var configYaml1 models.ConfigYAML
		var imageParams *models.SchedulerImageParams
		var scheduler1 *models.Scheduler

		BeforeEach(func() {
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb, logger, roomManager, mr, yaml1, timeoutSec,
				mockPortChooser, workerPortRange, portStart, portEnd)

			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			imageParams = &models.SchedulerImageParams{
				Image: "new-image",
			}
		})

		It("should update min", func() {
			newMin := 10

			// Select current scheduler yaml
			mt.MockSelectScheduler(yaml1, mockDb, nil)

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, lockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, "running", nil)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMinorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, lockKey, nil)

			err := controller.UpdateSchedulerMin(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				configYaml1.Name,
				newMin,
				&clock.Clock{},
				config,
				opManager,
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

			err := controller.UpdateSchedulerMin(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				configYaml1.Name,
				newMin,
				&clock.Clock{},
				config,
				opManager,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if DB fails", func() {
			newMin := configYaml1.AutoScaling.Min
			// Update scheduler
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err := controller.UpdateSchedulerMin(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				configYaml1.Name,
				newMin,
				&clock.Clock{},
				config,
				opManager,
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
				roomManager,
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
				roomManager,
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
				roomManager,
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

			for i := 0; i < int(amountUp); i++ {
				mockRedisClient.EXPECT().
					Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))
				nPorts := len(configYaml1.Ports)
				ports := make([]int, nPorts)
				for i := 0; i < nPorts; i++ {
					ports[i] = portStart + i
				}
				mockPortChooser.EXPECT().Choose(portStart, portEnd, nPorts).Return(ports)
			}

			err := controller.ScaleScheduler(
				logger,
				roomManager,
				mr,
				mockDb,
				mockRedisClient,
				clientset,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).NotTo(HaveOccurred())

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(int(amountUp)))
		})

		It("should scaledown if amountDown is positive", func() {
			var amountUp, amountDown, replicas uint = 0, 2, 0

			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 3
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

			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)

			err := controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)

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
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))

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

			err = controller.ScaleScheduler(
				logger,
				roomManager,
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

			for i := 0; i < int(replicas); i++ {
				mockRedisClient.EXPECT().
					Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))
				nPorts := len(configYaml1.Ports)
				ports := make([]int, nPorts)
				for i := 0; i < nPorts; i++ {
					ports[i] = portStart + i
				}
				mockPortChooser.EXPECT().Choose(portStart, portEnd, nPorts).Return(ports)
			}

			err := controller.ScaleScheduler(
				logger,
				roomManager,
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
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
				Times(scaleUpAmount)
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			for i := 0; i < scaleUpAmount; i++ {
				mockRedisClient.EXPECT().
					Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))
				nPorts := len(configYaml1.Ports)
				ports := make([]int, nPorts)
				for i := 0; i < nPorts; i++ {
					ports[i] = portStart + i
				}
				mockPortChooser.EXPECT().Choose(portStart, portEnd, nPorts).Return(ports)
			}

			err := controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true)

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
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
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

			err = controller.ScaleScheduler(
				logger,
				roomManager,
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
				roomManager,
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
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
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
			mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(expC.Terminating), nil))
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

			mockRedisClient.EXPECT().
				Get(models.GlobalPortsPoolKey).
				Return(goredis.NewStringResult(workerPortRange, nil)).
				Times(scaleUpAmount)
			mockPortChooser.EXPECT().
				Choose(portStart, portEnd, 2).
				Return([]int{5000, 5001}).
				Times(scaleUpAmount)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)

			key := fmt.Sprintf("maestro:panic:lock:%s", room.SchedulerName)
			mockRedisClient.EXPECT().HGetAll(key).
				Return(goredis.NewStringStringMapResult(nil, nil))
			mockPipeline.EXPECT().HMSet(key, gomock.Any())
			mockPipeline.EXPECT().Expire(key, 1*time.Minute)
			mockPipeline.EXPECT().Exec()
			mockRedisClient.EXPECT().Del(key).Return(goredis.NewIntResult(0, nil))

			err := controller.SetRoomStatus(
				logger,
				roomManager,
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
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
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
			mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			err := controller.SetRoomStatus(
				logger,
				roomManager,
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
				roomManager,
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
