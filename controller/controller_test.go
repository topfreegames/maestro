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
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/clock"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	mt "github.com/topfreegames/maestro/testing"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	yamlWithLimit = `
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
  max: 6
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
	yamlWithoutRequests = `
name: controller-name
game: controller
image: controller/controller:v123
imagePullPolicy: Never
occupiedTimeout: 3600
shutdownTimeout: 10
env:
- name: MAESTRO_HOST_PORT
  value: 192.168.64.1:8080
  valueFrom:
    secretKeyRef:
      name: ""
      key: ""
- name: POLLING_INTERVAL_IN_SECONDS
  value: "20"
  valueFrom:
    secretKeyRef:
      name: ""
      key: ""
- name: PING_INTERVAL_IN_SECONDS
  value: "10"
  valueFrom:
    secretKeyRef:
      name: ""
      key: ""
autoscaling:
  min: 2
  max: 20
  up:
    metricsTrigger:
    - type: cpu
      threshold: 80
      usage: 50
      time: 200
    - type: mem
      threshold: 80
      usage: 50
      time: 200
    cooldown: 30
  down:
    metricsTrigger:
    - type: mem
      threshold: 80
      usage: 30
      time: 200
    - type: cpu
      threshold: 80
      usage: 30
      time: 200
    cooldown: 60
`
	yamlWithoutRequestsContainers = `
name: controller-name
game: controller
containers:
- name: game
  image: controller/controller:v123
  imagePullPolicy: Never
  env:
  - name: MAESTRO_HOST_PORT
    value: 192.168.64.1:8080
    valueFrom:
      secretKeyRef:
        name: ""
        key: ""
  - name: POLLING_INTERVAL_IN_SECONDS
    value: "20"
    valueFrom:
      secretKeyRef:
        name: ""
        key: ""
  - name: PING_INTERVAL_IN_SECONDS
    value: "10"
    valueFrom:
      secretKeyRef:
        name: ""
        key: ""
occupiedTimeout: 3600
shutdownTimeout: 10
autoscaling:
  min: 2
  max: 20
  up:
    metricsTrigger:
    - type: cpu
      threshold: 80
      usage: 50
      time: 200
    - type: mem
      threshold: 80
      usage: 50
      time: 200
    cooldown: 30
  down:
    metricsTrigger:
    - type: mem
      threshold: 80
      usage: 30
      time: 200
    - type: cpu
      threshold: 80
      usage: 30
      time: 200
    cooldown: 60
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
		configLockKey    string
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
		configLockKey = models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), "controller-name")
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
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, configYaml1.AutoScaling.Min)

			mt.MockInsertScheduler(mockDb, nil)
			mt.MockUpdateScheduler(mockDb, nil, nil)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

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

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, configYaml1.AutoScaling.Min)

			mt.MockInsertScheduler(mockDb, nil)
			mt.MockUpdateScheduler(mockDb, nil, nil)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

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

				mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, configYaml1.AutoScaling.Min)

				mt.MockInsertScheduler(mockDb, nil)
				mt.MockUpdateScheduler(mockDb, nil, nil)

				mockDb.EXPECT().Query(gomock.Any(), `SELECT name FROM schedulers`)
				mockDb.EXPECT().Query(gomock.Any(), `SELECT * FROM schedulers WHERE name IN (?)`, gomock.Any())

				mockRedisClient.EXPECT().
					Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				err = mt.MockSetScallingAmount(
					mockRedisClient,
					mockPipeline,
					mockDb,
					clientset,
					&configYaml1,
					0,
					yaml1,
				)
				Expect(err).NotTo(HaveOccurred())

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

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				mockPipeline.EXPECT().
					HLen(models.GetPodMapRedisKey(configYaml1.Name)).
					Return(goredis.NewIntResult(0, nil)).
					Times(2)
				mockPipeline.EXPECT().Exec().Times(2)

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

		It("should return error if autoscaling min > max", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			configYaml1.AutoScaling.Max = 1
			configYaml1.AutoScaling.Min = configYaml1.AutoScaling.Max + 1

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("autoscaling min is greater than max")))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})

		It("should return error if using resource autoscaling and requests is not set", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithoutRequests), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			// Empty Requests
			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.cpu in order to use cpu autoscaling")))

			// CPU Up
			configYaml1.Requests = &models.Resources{}
			configYaml1.Requests.CPU = ""

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.cpu in order to use cpu autoscaling")))

			// CPU Down
			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.cpu in order to use cpu autoscaling")))

			// Mem Up
			configYaml1.Requests = &models.Resources{}
			configYaml1.Requests.CPU = "1"

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.memory in order to use mem autoscaling")))

			// Mem Down
			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.memory in order to use mem autoscaling")))

			err = yaml.Unmarshal([]byte(yamlWithoutRequestsContainers), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			// Empty Requests
			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.cpu in order to use cpu autoscaling")))

			// CPU Up
			configYaml1.Containers[0].Requests = &models.Resources{}
			configYaml1.Containers[0].Requests.CPU = ""

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.cpu in order to use cpu autoscaling")))

			// CPU Down
			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.cpu in order to use cpu autoscaling")))

			// Mem Up
			configYaml1.Containers[0].Requests = &models.Resources{}
			configYaml1.Containers[0].Requests.CPU = "1"

			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.memory in order to use mem autoscaling")))

			// Mem Down
			err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprint("must set requests.memory in order to use mem autoscaling")))

			pods, err := clientset.CoreV1().Pods("controller-name").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil)).
				Times(2)
			mockPipeline.EXPECT().Exec().Times(2)

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

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

			mt.MockListPods(mockPipeline, mockRedisClient, configYaml1.Name, []string{}, nil)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())
			errorExec := mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis"))

			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil)).
				Times(2)
			mockPipeline.EXPECT().Exec().Times(2).After(errorExec)

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

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil)).Times(2)
			mockPipeline.EXPECT().Exec().Times(2)

			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			terminationLockKey := models.GetSchedulerTerminationLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			// Get redis lock
			mt.MockRedisLock(mockRedisClient, terminationLockKey, lockTimeoutMs, true, nil)
			// Return redis lock
			mt.MockReturnRedisLock(mockRedisClient, terminationLockKey, nil)

			err = controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))
		})

		It("should return error if scheduler doesn't exist", func() {
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = ""
				})

			err := controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf(`scheduler "%s" not found`, configYaml1.Name)))
		})

		It("should fail if some error retrieving the scheduler", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err = controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should fail if some error deleting the scheduler", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil)).Times(2)
			mockPipeline.EXPECT().Exec().Times(2)

			terminationLockKey := models.GetSchedulerTerminationLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			// Get redis lock
			mt.MockRedisLock(mockRedisClient, terminationLockKey, lockTimeoutMs, true, nil)
			// Return redis lock
			mt.MockReturnRedisLock(mockRedisClient, terminationLockKey, nil)

			mockDb.EXPECT().Exec(
				"DELETE FROM schedulers WHERE name = ?",
				configYaml1.Name,
			).Return(pg.NewTestResult(errors.New("some error deleting in db"), 0), errors.New("some error deleting in db"))
			err = controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error deleting in db"))
		})

		It("should fail if update on existing scheduler fails", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
					scheduler.ID = "random-id"
				})

			terminationLockKey := models.GetSchedulerTerminationLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			// Get redis lock
			mt.MockRedisLock(mockRedisClient, terminationLockKey, lockTimeoutMs, true, nil)
			// Return redis lock
			mt.MockReturnRedisLock(mockRedisClient, terminationLockKey, nil)

			mt.MockUpdateScheduler(mockDb, errDB, nil)

			err = controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating status on schedulers: some error in db"))
		})

		It("should return error if timeout waiting for pods after delete", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil)).Times(2)
			mockPipeline.EXPECT().Exec().Times(2)

			terminationLockKey := models.GetSchedulerTerminationLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			// Get redis lock
			mt.MockRedisLock(mockRedisClient, terminationLockKey, lockTimeoutMs, true, nil)
			// Return redis lock
			mt.MockReturnRedisLock(mockRedisClient, terminationLockKey, nil)

			timeoutSec := 0
			err = controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
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

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(1)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil)).Times(1)
			mockPipeline.EXPECT().Exec().Times(1)

			terminationLockKey := models.GetSchedulerTerminationLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			// Get redis lock
			mt.MockRedisLock(mockRedisClient, terminationLockKey, lockTimeoutMs, true, nil)
			// Return redis lock
			mt.MockReturnRedisLock(mockRedisClient, terminationLockKey, nil)

			err = controller.DeleteScheduler(context.Background(), logger, mr, mockDb, redisClient, clientset, config, configYaml1.Name, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout deleting scheduler pods"))
		})
	})

	Describe("GetSchedulerScalingInfo", func() {
		It("should succeed", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
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
			mt.MockLoadScheduler(name, mockDb).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))
			_, _, _, err := controller.GetSchedulerScalingInfo(logger, mr, mockDb, mockRedisClient, name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should fail if error retrieving rooms count by status", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
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
			mt.MockLoadScheduler(configYaml1.Name, mockDb)
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

			mt.MockUpdateScheduler(mockDb, nil, nil)

			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if fail to update on schedulers table", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()

			mt.MockUpdateScheduler(mockDb, errDB, nil)

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

			mt.MockUpdateScheduler(mockDb, nil, errDB)

			err := controller.UpdateScheduler(logger, mr, mockDb, scheduler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error inserting on scheduler_versions: some error in db"))
		})
	})

	Describe("UpdateSchedulerState", func() {
		It("should succeed", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()

			mt.MockUpdateSchedulerStatus(mockDb, nil, nil)

			err := controller.UpdateSchedulerState(logger, mr, mockDb, scheduler)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if fail to update on schedulers table", func() {
			name := "scheduler-name"
			scheduler := models.NewScheduler(name, name, yaml1)
			scheduler.State = "in-sync"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()

			mt.MockUpdateSchedulerStatus(mockDb, errDB, nil)

			err := controller.UpdateSchedulerState(logger, mr, mockDb, scheduler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating status on schedulers: some error in db"))
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
				mt.MockPodNotFound(mockRedisClient, configYaml.Name, roomName)
				pod, err := models.NewPod(roomName, nil, configYaml, mockRedisClient, mr)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, roomName := range expectedRooms {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				for _, st := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(scheduler.Name, st), gomock.Any())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(scheduler.Name, st), roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), roomName)
				for _, mt := range allMetrics {
					mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(scheduler.Name, mt), gomock.Any())
				}
				mockPipeline.EXPECT().Del(gomock.Any())
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(configYaml.Name), roomName)
				mockPipeline.EXPECT().Exec().Times(2)
			}

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
				mt.MockPodNotFound(mockRedisClient, scheduler.Name, roomName)
				pod, err := models.NewPod(roomName, nil, configYaml, mockRedisClient, mr)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, name := range expectedRooms {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				for _, mt := range allMetrics {
					mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(scheduler.Name, mt), gomock.Any())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Exec().Times(2)
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
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				room := models.NewRoom(name, scheduler.Name)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
				}
				for _, mt := range allMetrics {
					mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(scheduler.Name, mt), gomock.Any())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Exec().Times(2)
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
				mt.MockPodNotFound(mockRedisClient, scheduler.Name, roomName)
				pod, err := models.NewPod(roomName, nil, configYaml, mockRedisClient, mr)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(scheduler.Name), roomName)
				mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis error"))
			}

			err = controller.DeleteUnavailableRooms(logger, roomManager, mr, mockRedisClient, clientset, scheduler, expectedRooms, "deletion_reason")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ScaleUp", func() {
		var maxScaleUpAmount int

		BeforeEach(func() {
			maxScaleUpAmount = config.GetInt("watcher.maxScaleUpAmount")
			config.Set("watcher.maxScaleUpAmount", 100)
		})

		AfterEach(func() {
			config.Set("watcher.maxScaleUpAmount", maxScaleUpAmount)
		})

		It("should fail and return error if error creating pods and initial op", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			firstExec := mt.MockSetScallingAmountAndReturnExec(
				mockRedisClient,
				mockPipeline,
				&configYaml1,
				0,
			)

			secondExec := mt.MockListPods(mockPipeline, mockRedisClient, configYaml1.Name, []string{}, nil).After(firstExec)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any())

			mockPipeline.EXPECT().Exec().
				Return([]goredis.Cmder{}, errors.New("some error in redis")).
				After(secondExec)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
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

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, amount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

			start := time.Now().UnixNano()
			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
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

			firstExec := mt.MockListPods(mockPipeline, mockRedisClient, configYaml1.Name, []string{}, nil)

			secondExec := mt.MockSetScallingAmountAndReturnExec(
				mockRedisClient,
				mockPipeline,
				&configYaml1,
				0,
			).After(firstExec)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(amount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(amount)
			thirdExec := mockPipeline.EXPECT().Exec().Return([]goredis.Cmder{}, errors.New("some error in redis")).After(secondExec)
			mockPipeline.EXPECT().Exec().Times(amount - 1).After(thirdExec)

			mt.MockAnyRunningPod(mockRedisClient, configYaml1.Name, (amount-1)*2)

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, false, config)
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

			mt.MockListPods(mockPipeline, mockRedisClient, configYaml1.Name, []string{}, nil)
			mt.MockAnyRunningPod(mockRedisClient, configYaml1.Name, amount)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(amount)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(amount)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(amount)
			mockPipeline.EXPECT().Exec().Times(amount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

			timeoutSec = 0
			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout scaling up scheduler"))
		})

		It("should return error and not scale up if there are Pending pods", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			pods := make(map[string]string, amount)
			for i := 0; i < amount; i++ {
				pod := &models.Pod{}
				pod.Name = fmt.Sprintf("room-%d", i)
				pod.Status.Phase = v1.PodRunning
				if i == 0 {
					pod.Status.Phase = v1.PodPending
				}
				jsonBytes, err := pod.MarshalToRedis()
				Expect(err).NotTo(HaveOccurred())
				pods[pod.Name] = string(jsonBytes)
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				HGetAll(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewStringStringMapResult(pods, nil))
			mockPipeline.EXPECT().Exec()

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("there are pending pods, check if there are enough CPU and memory to allocate new rooms"))
		})

		It("should scale up to max if scaling amount is higher than max", func() {
			amount := 10
			currentRooms := 4

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, configYaml1.AutoScaling.Max-currentRooms)

			mt.MockSetScallingAmount(mockRedisClient, mockPipeline, mockDb, clientset, &configYaml1, currentRooms, yamlWithLimit)
			for i := 0; i < currentRooms; i++ {
				pod := &v1.Pod{}
				pod.Name = fmt.Sprintf("room-%d", i)
				_, err := clientset.CoreV1().Pods(scheduler.Name).Create(pod)
				Expect(err).NotTo(HaveOccurred())
			}

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
			Expect(err).ToNot(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Max))
			Expect(hook.Entries).To(mt.ContainLogMessage(fmt.Sprintf("amount to scale is higher than max. Maestro will scale up to the max of %d", configYaml1.AutoScaling.Max)))
		})

		It("should maintain scale if number of rooms is higher than max", func() {
			amount := 10
			currentRooms := 10

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			mt.MockListPods(mockPipeline, mockRedisClient, configYaml1.Name, []string{}, nil)

			mt.MockSetScallingAmount(mockRedisClient, mockPipeline, mockDb, clientset, &configYaml1, currentRooms, yamlWithLimit)
			for i := 0; i < currentRooms; i++ {
				pod := &v1.Pod{}
				pod.Name = fmt.Sprintf("room-%d", i)
				_, err := clientset.CoreV1().Pods(scheduler.Name).Create(pod)
				Expect(err).NotTo(HaveOccurred())
			}

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
			Expect(err).ToNot(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(currentRooms))
		})

		It("should maintain scale if number of rooms is equal max", func() {
			amount := 10
			currentRooms := 6

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			mt.MockListPods(mockPipeline, mockRedisClient, configYaml1.Name, []string{}, nil)

			mt.MockSetScallingAmount(mockRedisClient, mockPipeline, mockDb, clientset, &configYaml1, currentRooms, yamlWithLimit)
			for i := 0; i < currentRooms; i++ {
				pod := &v1.Pod{}
				pod.Name = fmt.Sprintf("room-%d", i)
				_, err := clientset.CoreV1().Pods(scheduler.Name).Create(pod)
				Expect(err).NotTo(HaveOccurred())
			}

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
			Expect(err).ToNot(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Max))
		})

		It("should scaleup pods with two containers", func() {
			amount := 5
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml2), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml2)

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, amount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
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

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, amount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, amount, timeoutSec, true, config)
			Expect(err).NotTo(HaveOccurred())

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(amount))
		})
	})

	Describe("ScaleDown", func() {

		var maxScaleUpAmount int

		BeforeEach(func() {
			maxScaleUpAmount = config.GetInt("watcher.maxScaleUpAmount")
			config.Set("watcher.maxScaleUpAmount", 100)
		})

		AfterEach(func() {
			config.Set("watcher.maxScaleUpAmount", maxScaleUpAmount)
		})

		It("should succeed in scaling down", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, scaleUpAmount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true, config)
			Expect(err).NotTo(HaveOccurred())

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
				mt.MockPodNotFound(mockRedisClient, configYaml1.Name, name)

			}
			mockPipeline.EXPECT().Exec()

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				scaleUpAmount,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			timeoutSec = 300
			err = controller.ScaleDown(context.Background(), logger, roomManager, mr, mockDb, redisClient, clientset, scheduler, scaleDownAmount, timeoutSec)
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
			scaleUpAmount := 5
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, scaleUpAmount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true, config)

			// ScaleDown
			scaleDownAmount := 2
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
				mt.MockPodNotFound(mockRedisClient, configYaml.Name, name)

			}
			mockPipeline.EXPECT().Exec()

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				scaleUpAmount,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			timeoutSec = 300
			err = controller.ScaleDown(context.Background(), logger, roomManager, mr, mockDb, redisClient, clientset,
				scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
			pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
		})

		It("should not return error if delete non existing pod", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleDown
			scaleDownAmount := 1

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Exec()

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			timeoutSec = 300
			err = controller.ScaleDown(context.Background(), logger, roomManager, mr, mockDb, redisClient, clientset,
				scheduler, scaleDownAmount, timeoutSec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return timeout error", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, scaleUpAmount)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true, config)

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

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				scaleUpAmount,
				yaml1,
			)
			Expect(err).NotTo(HaveOccurred())

			timeoutSec = 0
			err = controller.ScaleDown(context.Background(), logger, roomManager, mr, mockDb, redisClient, clientset,
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

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml2, len(pods.Items))
			mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd, 0)

			mockRedisClient.EXPECT().
				HGet(models.GetPodMapRedisKey(configYaml2.Name), gomock.Any()).
				Return(goredis.NewStringResult("", goredis.Nil)).
				Times(len(pods.Items))
			scheduler1.Version = "v2.0"
			for _, pod := range pods.Items {
				Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
				Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml2, pod.Name, "deletion_reason")
				Expect(err).NotTo(HaveOccurred())
				_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml2, scheduler1)
				Expect(err).NotTo(HaveOccurred())
			}
			scheduler1.Version = "v1.0"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			calls := mt.NewCalls()

			mt.MockSaveSchedulerFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				opManager,
				config,
				lockTimeoutMs, numberOfVersions,
				yaml1,
				scheduler1,
				false,
				calls,
			)

			// Update scheduler rolling update status
			calls.Append(
				mt.MockUpdateVersionsTable(mockDb, nil))

			// Mock rolling update with rollback
			mt.MockRollingUpdateFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				mockPipeline,
				mockPortChooser,
				opManager,
				config,
				timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
				yaml1, workerPortRange,
				scheduler1,
				&configYaml2,
				nil,
				false,
				false,
				calls,
			)

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

				mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml2, len(pods.Items))
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, configYaml2.PortRange.Start, configYaml2.PortRange.End, 0)

				mockRedisClient.EXPECT().
					HGet(models.GetPodMapRedisKey(configYaml2.Name), gomock.Any()).
					Return(goredis.NewStringResult("", goredis.Nil)).
					Times(len(pods.Items))
				scheduler1.Version = "v2.0"
				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
					err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml2, pod.Name, "deletion_reason")
					Expect(err).NotTo(HaveOccurred())
					_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml2, scheduler1)
					Expect(err).NotTo(HaveOccurred())
				}
				scheduler1.Version = "v1.0"

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
				mockPipeline.EXPECT().Exec()

				// check other scheduler ports
				mt.MockSelectSchedulerNames(mockDb, []string{}, nil)
				mt.MockSelectConfigYamls(mockDb, []models.Scheduler{}, nil)
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				calls := mt.NewCalls()

				mt.MockSaveSchedulerFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					opManager,
					config,
					lockTimeoutMs, numberOfVersions,
					yaml1,
					scheduler1,
					false,
					calls,
				)

				// Update scheduler rolling update status
				calls.Append(
					mt.MockUpdateVersionsTable(mockDb, nil))

				// Mock rolling update with rollback
				mt.MockRollingUpdateFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					mockPipeline,
					mockPortChooser,
					opManager,
					config,
					timeoutSec, lockTimeoutMs, numberOfVersions, configYaml2.PortRange.Start, configYaml2.PortRange.End, 0,
					yaml1, workerPortRange,
					scheduler1,
					&configYaml2,
					nil,
					false,
					false,
					calls,
				)

				calls.Finish()

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

				mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml2, len(pods.Items))
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd, 0)

				mockRedisClient.EXPECT().
					HGet(models.GetPodMapRedisKey(configYaml2.Name), gomock.Any()).
					Return(goredis.NewStringResult("", goredis.Nil)).
					Times(len(pods.Items))

				scheduler1.Version = "v2.0"
				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
					err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml2, pod.Name, "deletion_reason")
					Expect(err).NotTo(HaveOccurred())
					_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml2, scheduler1)
					Expect(err).NotTo(HaveOccurred())
				}
				scheduler1.Version = "v1.0"

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
				mockPipeline.EXPECT().Exec()

				calls := mt.NewCalls()

				mt.MockSaveSchedulerFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					opManager,
					config,
					lockTimeoutMs, numberOfVersions,
					yaml1,
					scheduler1,
					false,
					calls,
				)

				// Update scheduler rolling update status
				calls.Append(
					mt.MockUpdateVersionsTable(mockDb, nil))

				// Mock rolling update with rollback
				mt.MockRollingUpdateFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					mockPipeline,
					mockPortChooser,
					opManager,
					config,
					timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
					yaml1, workerPortRange,
					scheduler1,
					&configYaml2,
					nil,
					false,
					false,
					calls,
				)

				calls.Finish()

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

				mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml2, len(pods.Items))
				mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, configYaml2.PortRange.Start, configYaml2.PortRange.End, 0)

				mockRedisClient.EXPECT().
					HGet(models.GetPodMapRedisKey(configYaml2.Name), gomock.Any()).
					Return(goredis.NewStringResult("", goredis.Nil)).
					Times(len(pods.Items))

				scheduler1.Version = "v2.0"
				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
					err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml2, pod.Name, "deletion_reason")
					Expect(err).NotTo(HaveOccurred())
					_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml2, scheduler1)
					Expect(err).NotTo(HaveOccurred())
				}
				scheduler1.Version = "v1.0"

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
				mockPipeline.EXPECT().Exec()

				// check other scheduler ports
				mt.MockSelectSchedulerNames(mockDb, []string{}, nil)
				mt.MockSelectConfigYamls(mockDb, []models.Scheduler{}, nil)
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil))

				calls := mt.NewCalls()

				mt.MockSaveSchedulerFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					opManager,
					config,
					lockTimeoutMs, numberOfVersions,
					yaml1,
					scheduler1,
					false,
					calls,
				)

				// Update scheduler rolling update status
				calls.Append(
					mt.MockUpdateVersionsTable(mockDb, nil))

				// Mock rolling update with rollback
				mt.MockRollingUpdateFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					mockPipeline,
					mockPortChooser,
					opManager,
					config,
					timeoutSec, lockTimeoutMs, numberOfVersions, 20000, 20020, 0,
					yaml1, workerPortRange,
					scheduler1,
					&configYaml2,
					nil,
					false,
					false,
					calls,
				)

				calls.Finish()

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

			configLockKey := models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), configYaml2.Name)

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRunning,
				}, nil))
			// Get redis lock
			mt.MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil)

			// Select empty scheduler yaml
			mt.MockLoadScheduler(configYaml2.Name, mockDb)

			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, configLockKey, nil)

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

			calls := mt.NewCalls()

			mt.MockSaveSchedulerFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				opManager,
				config,
				lockTimeoutMs, numberOfVersions,
				yaml2,
				scheduler1,
				true,
				calls,
			)

			// Update scheduler rolling update status
			calls.Append(
				mt.MockUpdateVersionsTable(mockDb, nil))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRunning,
				}, nil))
			// Retrieve redis lock
			mt.MockReturnRedisLock(mockRedisClient, configLockKey, nil)

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
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRunning,
				}, nil))
			mt.MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil)
			mt.MockReturnRedisLock(mockRedisClient, configLockKey, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil)

			mt.MockLoadScheduler(configYaml2.Name, mockDb).
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
				SetNX(configLockKey, gomock.Any(), gomock.Any()).
				Return(goredis.NewBoolResult(true, nil)).Times(1)

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
				SetNX(configLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(true, nil))
			mockRedisClient.EXPECT().
				SetNX(configLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
				Return(goredis.NewBoolResult(false, nil))

			mockRedisClient.EXPECT().
				Eval(gomock.Any(), []string{configLockKey}, gomock.Any()).
				Return(goredis.NewCmdResult(nil, nil))

			lock, err := redisClient.EnterCriticalSection(redisClient.Client, configLockKey, time.Duration(lockTimeoutMs)*time.Millisecond, 0, 0)
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
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRunning,
				}, nil))

			mockRedisClient.EXPECT().
				SetNX(configLockKey, gomock.Any(), gomock.Any()).
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

		It("should return error if timeout when creating rooms", func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			calls := mt.NewCalls()

			mt.MockSaveSchedulerFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				opManager,
				config,
				lockTimeoutMs, numberOfVersions,
				yaml1,
				scheduler1,
				false,
				calls,
			)

			// Mock rolling update with rollback
			mt.MockRollingUpdateFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				mockPipeline,
				mockPortChooser,
				opManager,
				config,
				timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
				yaml1, workerPortRange,
				scheduler1,
				&configYaml2,
				nil,
				true,
				true,
				calls,
			)

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
			Expect(err.Error()).To(Equal("operation timedout"))
		})

		It("should return error if timeout when deleting rooms", func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			calls := mt.NewCalls()

			mt.MockSaveSchedulerFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				opManager,
				config,
				lockTimeoutMs, numberOfVersions,
				yaml1,
				scheduler1,
				false,
				calls,
			)

			// Mock rolling update with rollback
			mt.MockRollingUpdateFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				mockPipeline,
				mockPortChooser,
				opManager,
				config,
				timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
				yaml1, workerPortRange,
				scheduler1,
				&configYaml2,
				nil,
				true,
				true,
				calls,
			)

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
			Expect(err.Error()).To(Equal("operation timedout"))
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

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml2, len(pods.Items))
			mt.MockGetPortsFromPool(&configYaml2, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd, 0)

			mockRedisClient.EXPECT().
				HGet(models.GetPodMapRedisKey(configYaml2.Name), gomock.Any()).
				Return(goredis.NewStringResult("", goredis.Nil)).
				Times(len(pods.Items))
			scheduler1.Version = "v2.0"
			for _, pod := range pods.Items {
				err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml2, pod.Name, "deletion_reason")
				Expect(err).NotTo(HaveOccurred())
				_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml2, scheduler1)
				Expect(err).NotTo(HaveOccurred())
			}
			scheduler1.Version = "v1.0"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			calls := mt.NewCalls()

			mt.MockSaveSchedulerFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				opManager,
				config,
				lockTimeoutMs, numberOfVersions,
				yaml1,
				scheduler1,
				false,
				calls,
			)

			// Update scheduler rolling update status
			calls.Append(
				mt.MockUpdateVersionsTable(mockDb, nil))

			// Mock rolling update with rollback
			mt.MockRollingUpdateFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				mockPipeline,
				mockPortChooser,
				opManager,
				config,
				timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
				yaml1, workerPortRange,
				scheduler1,
				&configYaml2,
				nil,
				false,
				false,
				calls,
			)

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

				mockRedisClient.EXPECT().
					HGetAll(opManager.GetOperationKey()).
					Return(goredis.NewStringStringMapResult(map[string]string(nil), nil))
				// Get redis lock
				mt.MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil)

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

			It("should stop on waitCreatingPods", func() {
				yamlString := `
name: scheduler-name-cancel
autoscaling:
  min: 3
  up:
    trigger:
      limit: 10
containers:
- name: container1
  image: image1
`
				newYamlString := `
name: scheduler-name-cancel
autoscaling:
  min: 3
  up:
    trigger:
      limit: 10
containers:
- name: container1
  image: image2
`
				configYaml, _ := models.NewConfigYAML(yamlString)

				mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, roomManager, mr, yamlString, timeoutSec, mockPortChooser, workerPortRange, portStart, portEnd)

				scheduler := models.NewScheduler(configYaml.Name, configYaml.Game, yamlString)
				pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(3))

				for _, pod := range pods.Items {
					Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				}

				mt.MockRemoveInvalidRoomsKey(mockRedisClient, mockPipeline, configYaml.Name)

				mockRedisClient.EXPECT().
					HGetAll(opManager.GetOperationKey()).
					Return(goredis.NewStringStringMapResult(map[string]string{
						"description": models.OpManagerRunning,
					}, nil))
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(
					goredis.NewStringStringMapResult(nil, nil)).AnyTimes()

				calls := mt.NewCalls()

				mt.MockSaveSchedulerFlow(
					mockRedisClient,
					mockDb,
					mockClock,
					opManager,
					config,
					lockTimeoutMs, numberOfVersions,
					newYamlString,
					scheduler,
					false,
					calls,
				)

				configLockKey = models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), scheduler.Name)

				// Update scheduler rolling update status
				calls.Append(
					mt.MockUpdateVersionsTable(mockDb, nil))

				// Update scheduler
				calls.Append(
					mt.MockUpdateSchedulersTable(mockDb, nil))

				// Add new version into versions table
				scheduler.NextMajorVersion()
				calls.Append(
					mt.MockInsertIntoVersionsTable(scheduler, mockDb, nil))

				// Count to delete old versions if necessary
				calls.Append(
					mt.MockCountNumberOfVersions(scheduler, numberOfVersions, mockDb, nil))

				// Update scheduler rolling update status
				calls.Append(
					mt.MockUpdateVersionsTable(mockDb, nil))

				// release configLockKey
				calls.Append(
					mt.MockReturnRedisLock(mockRedisClient, configLockKey, nil))

				calls.Finish()

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
				Expect(err.Error()).To(Equal("operation canceled"))

				pods, err = clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				// Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min))

				for _, pod := range pods.Items {
					Expect(pod.GetName()).To(ContainSubstring("scheduler-name-cancel-"))
					Expect(pod.GetName()).To(HaveLen(len("scheduler-name-cancel-") + 8))
					Expect(pod.Spec.Containers[0].Env[0].Name).To(Equal("MAESTRO_SCHEDULER_NAME"))
					Expect(pod.Spec.Containers[0].Env[0].Value).To(Equal("scheduler-name-cancel"))
					Expect(pod.Spec.Containers[0].Env[1].Name).To(Equal("MAESTRO_ROOM_ID"))
					Expect(pod.Spec.Containers[0].Env[1].Value).To(Equal(pod.GetName()))
					Expect(pod.Spec.Containers[0].Env).To(HaveLen(2))
					Expect(pod.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
					// Expect(pod.ObjectMeta.Labels["version"]).To(Equal("v3.0"))
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
				mt.MockPodNotFound(mockRedisClient, configYaml.Name, roomName)
				pod, err := models.NewPod(roomName, nil, configYaml, mockRedisClient, mr)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, scheduler.Name)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(configYaml.Name), room.ID)
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(scheduler.Name, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(scheduler.Name, status), roomName)
				}
				for _, mt := range allMetrics {
					mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(scheduler.Name, mt), gomock.Any())
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
				mt.MockPodNotFound(mockRedisClient, configYaml.Name, roomName)
				pod, err := models.NewPod(roomName, nil, configYaml, mockRedisClient, mr)
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
				mt.MockPodNotFound(mockRedisClient, configYaml.Name, roomName)
				pod, err := models.NewPod(roomName, nil, configYaml, mockRedisClient, mr)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
			}
			for _, roomName := range expectedRooms {
				room := models.NewRoom(roomName, scheduler.Name)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(configYaml.Name), room.ID)
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
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(scheduler.Name), room.ID)
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(scheduler.Name, status), room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(scheduler.Name, status), roomName)
				}
				for _, mt := range allMetrics {
					mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(scheduler.Name, mt), gomock.Any())
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

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml1, len(pods.Items))
			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd, 0)

			mockRedisClient.EXPECT().
				HGet(models.GetPodMapRedisKey(configYaml1.Name), gomock.Any()).
				Return(goredis.NewStringResult("", goredis.Nil)).
				Times(len(pods.Items))
			scheduler1.Version = "v2.0"
			configYaml1.Image = imageParams.Image
			for _, pod := range pods.Items {
				err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml1, pod.Name, "deletion_reason")
				Expect(err).NotTo(HaveOccurred())
				_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml1, scheduler1)
				Expect(err).NotTo(HaveOccurred())
			}
			scheduler1.Version = "v1.0"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			calls := mt.NewCalls()

			// Select current scheduler yaml
			calls.Append(
				mt.MockSelectScheduler(yaml1, mockDb, nil),
			)

			// Get config lock
			calls.Append(
				mt.MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil))

			// Set new operation manager description
			calls.Append(
				mt.MockSetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil))

			// Update scheduler
			calls.Append(
				mt.MockUpdateSchedulersTable(mockDb, nil))

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			calls.Append(
				mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil))

			// Count to delete old versions if necessary
			calls.Append(
				mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil))

			// Update scheduler rolling update status
			calls.Append(
				mt.MockUpdateVersionsTable(mockDb, nil))

			// Mock rolling update with rollback
			mt.MockRollingUpdateFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				mockPipeline,
				mockPortChooser,
				opManager,
				config,
				timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
				yaml1, workerPortRange,
				scheduler1,
				&configYaml1,
				nil,
				false,
				false,
				calls,
			)

			calls.Finish()

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
			mt.MockLoadScheduler(newSchedulerName, mockDb).
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
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml1, len(pods.Items))
			mt.MockGetPortsFromPool(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd, 4)

			mockRedisClient.EXPECT().
				HGet(models.GetPodMapRedisKey(configYaml1.Name), gomock.Any()).
				Return(goredis.NewStringResult("", goredis.Nil)).
				Times(len(pods.Items))
			scheduler1.Version = "v2.0"
			configYaml1.Image = imageParams.Image
			for _, pod := range pods.Items {
				err = roomManager.Delete(logger, mr, clientset, mockRedisClient, &configYaml1, pod.Name, "deletion_reason")
				Expect(err).NotTo(HaveOccurred())
				_, err = roomManager.Create(logger, mr, mockRedisClient, mockDb, clientset, &configYaml1, scheduler1)
				Expect(err).NotTo(HaveOccurred())
			}
			scheduler1.Version = "v1.0"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HLen(models.GetPodMapRedisKey(scheduler1.Name)).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			calls := mt.NewCalls()

			// Select current scheduler yaml
			calls.Append(
				mt.MockSelectScheduler(yaml1, mockDb, nil),
			)

			// Get config lock
			calls.Append(
				mt.MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil))

			// Set new operation manager description
			calls.Append(
				mt.MockSetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil))

			// Update scheduler
			calls.Append(
				mt.MockUpdateSchedulersTable(mockDb, nil))

			// Add new version into versions table
			scheduler1.NextMajorVersion()
			calls.Append(
				mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil))

			// Count to delete old versions if necessary
			calls.Append(
				mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil))

			mt.MockUpdateVersionsTable(mockDb, nil)

			// Mock rolling update with rollback
			mt.MockRollingUpdateFlow(
				mockRedisClient,
				mockDb,
				mockClock,
				mockPipeline,
				mockPortChooser,
				opManager,
				config,
				timeoutSec, lockTimeoutMs, numberOfVersions, portStart, portEnd, 0,
				yaml1, workerPortRange,
				scheduler1,
				&configYaml1,
				nil,
				false,
				false,
				calls,
			)

			calls.Finish()

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
		var scheduler1 *models.Scheduler

		BeforeEach(func() {
			mt.MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb, logger, roomManager, mr, yaml1, timeoutSec,
				mockPortChooser, workerPortRange, portStart, portEnd)

			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
		})

		It("should update min", func() {
			newMin := 10

			// Select current scheduler yaml
			mt.MockSelectScheduler(yaml1, mockDb, nil)

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRunning,
				}, nil))

			// Get redis lock
			mt.MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			mt.MockSetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil)

			// Update new config on schedulers table
			mt.MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMinorVersion()
			mt.MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			mt.MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Update scheduler rolling update status
			mt.MockUpdateVersionsTable(mockDb, nil)

			mt.MockReturnRedisLock(mockRedisClient, configLockKey, nil)

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
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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

		It("should not update if min is greater than max", func() {
			newMin := 10
			// Update scheduler
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)
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
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid parameter: autoscaling max must be greater than min"))
		})

		It("should return error if DB fails", func() {
			newMin := configYaml1.AutoScaling.Min
			// Update scheduler
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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
		var maxScaleUpAmount int

		BeforeEach(func() {
			maxScaleUpAmount = config.GetInt("watcher.maxScaleUpAmount")
			config.Set("watcher.maxScaleUpAmount", 100)
		})

		AfterEach(func() {
			config.Set("watcher.maxScaleUpAmount", maxScaleUpAmount)
		})

		It("should return error if more than one parameter is set", func() {
			var amountUp, amountDown, replicas uint = 1, 1, 1
			err := controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid scale parameter: can't handle more than one parameter"))
		})

		It("should return error if DB fails", func() {
			var amountUp, amountDown, replicas uint = 1, 0, 0
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

			err := controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in db"))
		})

		It("should return error if yaml not found", func() {
			var amountUp, amountDown, replicas uint = 1, 0, 0
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, "", "")
				})

			err := controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler 'controller-name' not found"))
		})

		It("should scaleup if amounUp is positive", func() {
			var amountUp, amountDown, replicas uint = 4, 0, 0

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, int(amountUp))

			err := mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
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
			scaleUpAmount := 6
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, scaleUpAmount)

			err := mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true, config)
			Expect(err).NotTo(HaveOccurred())

			downscalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			mt.MockRedisLock(mockRedisClient, downscalingLockKey, 0, true, nil)
			mt.MockReturnRedisLock(mockRedisClient, downscalingLockKey, nil)

			// ScaleDown
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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
				mt.MockPodNotFound(mockRedisClient, configYaml1.Name, name)
			}
			mockPipeline.EXPECT().Exec()

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				scaleUpAmount,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
		})

		It("should scaleUp if replicas is above current number of pods", func() {
			var amountUp, amountDown, replicas uint = 0, 0, 4

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()

			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, int(replicas))

			err := mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
				60, 60,
				amountUp, amountDown, replicas,
				configYaml1.Name,
			)

			pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(int(replicas)))
		})

		It("should scaleDown if replicas is below current number of pods", func() {
			var amountUp, amountDown, replicas uint = 0, 0, 4

			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)

			// ScaleUp
			scaleUpAmount := 5
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, scaleUpAmount)

			err := mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				0,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset,
				scheduler, scaleUpAmount, timeoutSec, true, config)
			Expect(err).NotTo(HaveOccurred())

			// ScaleDown
			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			downscalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml1.Name)
			mt.MockRedisLock(mockRedisClient, downscalingLockKey, 0, true, nil)
			mt.MockReturnRedisLock(mockRedisClient, downscalingLockKey, nil)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				HLen(models.GetPodMapRedisKey(configYaml1.Name)).
				Return(goredis.NewIntResult(int64(scaleUpAmount), nil))
			mockPipeline.EXPECT().Exec()

			scaleDownAmount := scaleUpAmount - int(replicas)
			names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
			Expect(err).NotTo(HaveOccurred())

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, name := range names {
				mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(name, nil))
				mt.MockPodNotFound(mockRedisClient, configYaml1.Name, name)
			}
			mockPipeline.EXPECT().Exec()

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				scaleUpAmount,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.ScaleScheduler(
				context.Background(),
				logger,
				roomManager,
				mr,
				mockDb,
				redisClient,
				clientset,
				config,
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

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
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

			err = controller.SetRoomStatus(
				logger,
				roomManager,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				&models.RoomStatusPayload{Status: status},
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
				Occupied:    10,
				Ready:       1,
				Terminating: 0,
			}

			mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			// ScaleUp
			scaleUpAmount := configYaml1.AutoScaling.Up.Delta
			mt.MockScaleUp(mockPipeline, mockRedisClient, configYaml1.Name, scaleUpAmount)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)

			key := fmt.Sprintf("maestro:panic:lock:%s", room.SchedulerName)
			mockRedisClient.EXPECT().HGetAll(key).
				Return(goredis.NewStringStringMapResult(nil, nil))
			mockPipeline.EXPECT().HMSet(key, gomock.Any())
			mockPipeline.EXPECT().Expire(key, 1*time.Minute)
			mockPipeline.EXPECT().Exec()
			mockRedisClient.EXPECT().Del(key).Return(goredis.NewIntResult(0, nil))

			err = mt.MockSetScallingAmountWithRoomStatusCount(
				mockRedisClient,
				mockPipeline,
				&configYaml1,
				expC,
			)
			Expect(err).NotTo(HaveOccurred())

			err = controller.SetRoomStatus(
				logger,
				roomManager,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				&models.RoomStatusPayload{Status: status},
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

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
				})

			err = controller.SetRoomStatus(
				logger,
				roomManager,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				&models.RoomStatusPayload{Status: status},
				config,
				room,
				schedulerCache,
			)

			time.Sleep(1 * time.Second)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not scale up if error on db", func() {
			status := "occupied"

			mt.MockLoadScheduler(configYaml1.Name, mockDb).
				Return(pg.NewTestResult(errors.New("some error on db"), 0), errors.New("some error on db"))

			err = controller.SetRoomStatus(
				logger,
				roomManager,
				mockRedisClient,
				mockDb,
				mr,
				clientset,
				&models.RoomStatusPayload{Status: status},
				config,
				room,
				schedulerCache,
			)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error on db"))
		})
	})

	Describe("SetScalingAmount", func() {
		It("Should scale down to min if current - amount is less than min", func() {

			amount := 10
			current := 5

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(current - configYaml1.AutoScaling.Min))
		})

		It("Should scale down amount", func() {

			amount := 2
			current := 6

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(amount))
		})

		It("Should scale down to (current - max) if (current - amount) is greater than max", func() {

			amount := 1
			current := 8

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(current - configYaml1.AutoScaling.Max))
		})

		It("Should not scale if current is less than or equal min", func() {

			amount := 10
			current := 3

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			// equal
			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(0))

			// less
			current = 2

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err = controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(0))
		})

		It("Should scale up amount", func() {

			amount := 4
			current := 2

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(amount))
		})

		It("Should scale up to (max - current) if (current + amount) is greater than max", func() {

			amount := 4
			current := 4

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(configYaml1.AutoScaling.Max - current))
		})

		It("Should scale up to (min - current) if (current + amount) is less than min", func() {

			amount := 1
			current := 1

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(configYaml1.AutoScaling.Min - current))
		})

		It("Should not scale if current is greater than or equal max", func() {

			amount := 10
			current := 6

			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlWithLimit), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlWithLimit)

			// equal
			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err := controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(0))

			// greater
			current = 8

			err = mt.MockSetScallingAmount(
				mockRedisClient,
				mockPipeline,
				mockDb,
				clientset,
				&configYaml1,
				current,
				yamlWithLimit,
			)
			Expect(err).NotTo(HaveOccurred())

			newAmount, err = controller.SetScalingAmount(
				logger,
				mr,
				mockDb,
				mockRedisClient,
				scheduler,
				configYaml1.AutoScaling.Max, configYaml1.AutoScaling.Min, amount,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newAmount).To(Equal(0))
		})
	})

	Describe("SegmentAndReplacePods", func() {
		It("should return timeout error when it timeouts", func() {
			pods := []*models.Pod{
				&models.Pod{Name: "room-1"},
			}
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, string(configYaml1.ToYAML()))

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml1, 0)
			mt.MockGetPortsFromPoolAnyTimes(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)
			mt.MockPodNotFound(mockRedisClient, configYaml1.Name, gomock.Any()).AnyTimes()

			timeoutErr, cancelErr, err := controller.SegmentAndReplacePods(
				logger,
				roomManager,
				mr,
				clientset,
				mockDb,
				mockRedisClient,
				time.Now(),
				&configYaml1,
				pods,
				scheduler,
				nil,
				10*time.Second,
				10,
				1,
			)
			Expect(timeoutErr).To(HaveOccurred())
			Expect(cancelErr).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return cancel error when it is canceled while waiting for pods to be created", func() {
			pods := []*models.Pod{
				&models.Pod{Name: "room-1"},
			}

			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, string(configYaml1.ToYAML()))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRollingUpdate,
				}, nil))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string(nil), nil))

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml1, 0)
			mt.MockGetPortsFromPoolAnyTimes(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)
			mt.MockPodNotFound(mockRedisClient, configYaml1.Name, gomock.Any()).AnyTimes()

			timeoutErr, cancelErr, err := controller.SegmentAndReplacePods(
				logger,
				roomManager,
				mr,
				clientset,
				mockDb,
				mockRedisClient,
				time.Now().Add(time.Minute),
				&configYaml1,
				pods,
				scheduler,
				opManager,
				10*time.Second,
				10,
				1,
			)
			Expect(timeoutErr).ToNot(HaveOccurred())
			Expect(cancelErr).To(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return cancel error when it is canceled while waiting for pods to be deleted", func() {
			pods := []*models.Pod{
				&models.Pod{Name: "room-1"},
			}

			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, string(configYaml1.ToYAML()))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRollingUpdate,
				}, nil))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string(nil), nil))

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml1, 0)
			mt.MockGetPortsFromPoolAnyTimes(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)
			mt.MockAnyRunningPod(mockRedisClient, configYaml1.Name, 0)

			mockPipeline.EXPECT().
				SIsMember(models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready"), gomock.Any()).
				Return(goredis.NewBoolResult(true, nil))
			mockPipeline.EXPECT().
				SIsMember(models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied"), gomock.Any()).
				Return(goredis.NewBoolResult(false, nil))

			for _, pod := range pods {
				podv1 := &v1.Pod{}
				podv1.SetName(pod.Name)
				podv1.SetNamespace(configYaml1.Name)
				podv1.SetLabels(map[string]string{"version": "v1.0"})
				_, err := clientset.CoreV1().Pods(configYaml1.Name).Create(podv1)
				Expect(err).ToNot(HaveOccurred())
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
			mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(configYaml1.Name), "room-1").AnyTimes()
			mockPipeline.EXPECT().Exec().AnyTimes()
			mt.MockRemoveAnyRoomsFromRedisAnyTimes(mockRedisClient, mockPipeline, &configYaml1, nil, 0)

			timeoutErr, cancelErr, err := controller.SegmentAndReplacePods(
				logger,
				roomManager,
				mr,
				clientset,
				mockDb,
				mockRedisClient,
				time.Now().Add(time.Minute),
				&configYaml1,
				pods,
				scheduler,
				opManager,
				10*time.Second,
				10,
				1,
			)
			Expect(timeoutErr).ToNot(HaveOccurred())
			Expect(cancelErr).To(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when it fails to create pod", func() {
			pods := []*models.Pod{
				&models.Pod{Name: "room-1"},
			}

			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, string(configYaml1.ToYAML()))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRollingUpdate,
				}, nil)).AnyTimes()

			mockRedisClient.EXPECT().
				TxPipeline().
				Return(mockPipeline)

			mockPipeline.EXPECT().
				HMSet(gomock.Any(), gomock.Any()).
				Do(func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				})

			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"),
					gomock.Any())

			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())

			mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis error"))

			timeoutErr, cancelErr, err := controller.SegmentAndReplacePods(
				logger,
				roomManager,
				mr,
				clientset,
				mockDb,
				mockRedisClient,
				time.Now().Add(time.Minute),
				&configYaml1,
				pods,
				scheduler,
				opManager,
				10*time.Second,
				10,
				1,
			)
			Expect(timeoutErr).ToNot(HaveOccurred())
			Expect(cancelErr).ToNot(HaveOccurred())
			Expect(err).To(HaveOccurred())
		})

		It("should not return error when it suceeds", func() {
			pods := []*models.Pod{
				&models.Pod{Name: "room-1"},
			}

			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, string(configYaml1.ToYAML()))

			mockRedisClient.EXPECT().
				HGetAll(opManager.GetOperationKey()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRollingUpdate,
				}, nil)).AnyTimes()

			mt.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml1, 1)
			mt.MockGetPortsFromPoolAnyTimes(&configYaml1, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd)
			mt.MockAnyRunningPod(mockRedisClient, configYaml1.Name, 2)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				SIsMember(models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready"), gomock.Any()).
				Return(goredis.NewBoolResult(true, nil))
			mockPipeline.EXPECT().
				SIsMember(models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied"), gomock.Any()).
				Return(goredis.NewBoolResult(false, nil))
			mockPipeline.EXPECT().Exec().Return(nil, nil)

			runningPod := mt.MockRunningPod(mockRedisClient, configYaml1.Name, "room-1")

			for _, pod := range pods {
				podv1 := &v1.Pod{}
				podv1.SetName(pod.Name)
				podv1.SetNamespace(configYaml1.Name)
				podv1.SetLabels(map[string]string{"version": "v1.0"})
				_, err := clientset.CoreV1().Pods(configYaml1.Name).Create(podv1)
				Expect(err).ToNot(HaveOccurred())
			}

			// Delete old rooms
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(configYaml1.Name), "room-1")
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SRem(models.GetInvalidRoomsKey(configYaml1.Name), []string{"room-1"})
			mockPipeline.EXPECT().Exec()

			mt.MockRemoveAnyRoomsFromRedisAnyTimes(mockRedisClient, mockPipeline, &configYaml1, nil, 1)
			mt.MockPodNotFound(mockRedisClient, configYaml1.Name, "room-1").After(runningPod)

			timeoutErr, cancelErr, err := controller.SegmentAndReplacePods(
				logger,
				roomManager,
				mr,
				clientset,
				mockDb,
				mockRedisClient,
				time.Now().Add(time.Minute),
				&configYaml1,
				pods,
				scheduler,
				opManager,
				10*time.Second,
				10,
				1,
			)
			Expect(timeoutErr).ToNot(HaveOccurred())
			Expect(cancelErr).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
