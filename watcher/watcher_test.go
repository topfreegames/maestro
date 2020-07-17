// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	maestroErrors "github.com/topfreegames/maestro/errors"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	reportersMocks "github.com/topfreegames/maestro/reporters/mocks"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	"github.com/topfreegames/maestro/testing"
	"github.com/topfreegames/maestro/watcher"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

const (
	yaml1 = `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
limits:
  memory: "66Mi"
  cpu: "2"
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 2
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
      threshold: 80
    metricsTrigger:
    - type: legacy
      usage: 60
      time: 100
      threshold: 80
      delta: 2
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
      threshold: 80
    metricsTrigger:
    - type: legacy
      usage: 30
      time: 500
      threshold: 80
      delta: -1
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
forwarders:
  plugin:
    name:
      enabled: true
`
	yamlWithUpLimit = `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
limits:
  memory: "66Mi"
  cpu: "2"
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
      threshold: 80
      limit: 70
    metricsTrigger:
    - type: legacy
      usage: 60
      time: 100
      threshold: 80
      limit: 70
      delta: 2
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
      threshold: 80
    metricsTrigger:
    - type: legacy
      usage: 30
      time: 500
      threshold: 80
      delta: -1
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
	yamlWithLegacyDownAndMetricsUpTrigger = `
name: controller-name
game: controller
image: maestro-dev-room:latest
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
autoscaling:
  min: 2
  max: 10
  up:
    delta: 1
    trigger:
      usage: 50
      time: 200
      limit: 85
      threshold: 80
    cooldown: 30
  down:
    delta: 2
    trigger:
      usage: 30
      time: 100
      threshold: 80
    metricsTrigger:
    - type: legacy
      usage: 30
      time: 100
      threshold: 80
      delta: -2
    cooldown: 60
`
	yamlWithLegacyUpAndMetricsDownTrigger = `
name: controller-name
game: controller
image: maestro-dev-room:latest
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
autoscaling:
  min: 2
  max: 10
  up:
    metricsTrigger:
    - type: legacy
      delta: 2
      usage: 50
      time: 200
      limit: 85
      threshold: 80
    cooldown: 30
  down:
    trigger:
      usage: 30
      time: 100
      threshold: 80
    cooldown: 60
`
	yamlWithRoomMetricsTrigger = `
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
    - type: room
      threshold: 80
      usage: 50
      time: 200
    cooldown: 30
  down:
    metricsTrigger:
    - type: room
      threshold: 80
      usage: 30
      time: 200
    cooldown: 60
`
	yamlWithCPUAndMemoryMetricsTrigger = `
name: controller-name
game: controller
image: controller/controller:v123
imagePullPolicy: Never
requests:
  cpu: 1000m
  memory: 1Gi
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
	occupiedTimeout = 300
)

type triggerSpec struct {
	targetUsagePercent          int
	pointsAboveThresholdPercent int
	limit                       int
	time                        int
	triggerType                 models.AutoScalingPolicyType
}

type simulationSpec struct {
	up                                                                                                        []triggerSpec
	down                                                                                                      []triggerSpec
	lastChangedAt                                                                                             time.Time
	lastScaleOpAt                                                                                             time.Time
	roomCount                                                                                                 *models.RoomsStatusCount
	metricTypeToScale                                                                                         models.AutoScalingPolicyType
	logMessages                                                                                               []string
	shouldScaleUp, shouldScaleDown, shouldCheckMetricsTriggerUp, shouldCheckMetricsTriggerDown, isDownscaling bool
	percentageOfPointsAboveTargetUsage, currentRawUsage, deltaExpected                                        int
	memScale                                                                                                  resource.Scale
}

var _ = Describe("Watcher", func() {
	var w *watcher.Watcher
	var errDB = errors.New("db failed")

	Describe("NewWatcher", func() {
		It("should return configured new watcher", func() {
			name := "my-scheduler"
			gameName := "game-name"
			autoScalingPeriod := 1234
			lockKey := "myLockKey"
			lockTimeoutMs := 1000
			config.Set("watcher.autoScalingPeriod", autoScalingPeriod)
			config.Set("watcher.lockKey", lockKey)
			config.Set("watcher.lockTimeoutMs", lockTimeoutMs)

			testing.MockLoadScheduler(name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w.AutoScalingPeriod).To(Equal(autoScalingPeriod))
			Expect(w.Config).To(Equal(config))
			Expect(w.DB).To(Equal(mockDb))
			Expect(w.KubernetesClient).To(Equal(clientset))
			Expect(w.Logger).NotTo(BeNil())
			Expect(w.MetricsReporter).To(Equal(mr))
			Expect(w.RedisClient).To(Equal(redisClient))
			Expect(w.LockTimeoutMS).To(Equal(lockTimeoutMs))
			Expect(w.SchedulerName).To(Equal(name))
		})

		It("should return configured new watcher using configuration defaults", func() {
			name := "my-scheduler"
			gameName := "game-name"
			testing.MockLoadScheduler(name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w.AutoScalingPeriod).To(Equal(10))
			Expect(w.LockTimeoutMS).To(Equal(180000))
		})
	})

	Describe("Start", func() {
		var configYaml models.ConfigYAML
		mockAutoScaling := &models.AutoScaling{}

		BeforeEach(func() {
			config.Set("watcher.autoScalingPeriod", 1)
			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
		})
		It("should start watcher", func() {
			// Testing here if no scaling needs to be done
			err := yaml.Unmarshal([]byte(yaml1), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			testing.CopyAutoScaling(configYaml.AutoScaling, mockAutoScaling)
			testing.TransformLegacyInMetricsTrigger(mockAutoScaling)

			// Mock get terminating rooms
			testing.MockRemoveZombieRooms(mockPipeline, mockRedisClient, []string{}, configYaml.Name)

			// Mock send usage percentage
			testing.MockSendUsage(mockPipeline, mockRedisClient, mockAutoScaling)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline,
				configYaml.Name, [][]string{}, nil)

			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset,
				configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			// EnterCriticalSection (lock done by redis-lock)
			terminationLockKey := fmt.Sprintf("maestro-lock-key-%s-termination", configYaml.Name)
			downscalingLockKey := fmt.Sprintf("maestro-lock-key-%s-downscaling", configYaml.Name)
			mockRedisClient.EXPECT().
				SetNX(terminationLockKey, gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(true, nil)).
				AnyTimes()
			mockRedisClient.EXPECT().
				SetNX(downscalingLockKey, gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(true, nil)).
				AnyTimes()

			// DeleteRoomsNoPingSince
			pKey := models.GetRoomPingRedisKey(configYaml.Name)
			// DeleteRoomsNoPingSince
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey, gomock.Any(),
			).Return(redis.NewStringSliceResult([]string{}, nil))

			//DeleteRoomsOccupiedTimeout
			lKey := models.GetLastStatusRedisKey(configYaml.Name, models.StatusOccupied)
			mockRedisClient.EXPECT().ZRangeByScore(
				lKey,
				gomock.Any(),
			).Return(redis.NewStringSliceResult([]string{}, nil))

			// GetSchedulerScalingInfo
			testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
				scheduler.State = models.StateInSync
			}).AnyTimes()

			creating := models.GetRoomStatusSetRedisKey(configYaml.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml.Name, "terminating")
			expC := &models.RoomsStatusCount{4, 3, 20, 1}
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil)).Times(2)
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil)).Times(2)
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil)).Times(2)
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil)).Times(2)
			mockPipeline.EXPECT().Exec().Times(2)

			err = testing.MockSetScallingAmountWithRoomStatusCount(
				mockRedisClient,
				mockPipeline,
				&configYaml,
				expC,
			)
			Expect(err).NotTo(HaveOccurred())

			// Mock MetricsTrigger Up get usage percentages
			for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
				testing.MockGetUsages(
					mockPipeline, mockRedisClient,
					fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
					trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
				)
			}

			// ScaleUp
			testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				w.Run = false
			}, mockDb, nil, nil)

			opManager := models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			testing.MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Del(models.GetInvalidRoomsKey(configYaml.Name))
			mockPipeline.EXPECT().Exec()

			names := make([]string, 0, configYaml.AutoScaling.Up.Delta)
			for i := 0; i < configYaml.AutoScaling.Up.Delta; i++ {
				names = append(names, fmt.Sprintf("room-%d", i))
			}

			testing.MockListPods(mockPipeline, mockRedisClient, configYaml.Name, names, nil)
			mockRedisClient.EXPECT().HMSet(models.GetPodMapRedisKey(configYaml.Name), gomock.Any()).Return(redis.NewStatusResult("", nil)).AnyTimes()

			// LeaveCriticalSection (unlock done by redis-lock)
			mockRedisClient.EXPECT().Eval(gomock.Any(), []string{terminationLockKey}, gomock.Any()).Return(redis.NewCmdResult(nil, nil)).AnyTimes()
			mockRedisClient.EXPECT().Eval(gomock.Any(), []string{downscalingLockKey}, gomock.Any()).Return(redis.NewCmdResult(nil, nil)).AnyTimes()

			Expect(func() {
				go func() {
					defer GinkgoRecover()
					w.Start()
				}()
			}).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			time.Sleep(10 * time.Second)
		})

		It("should not panic if error acquiring lock", func() {
			name := "my-scheduler"
			gameName := "game-name"
			testing.MockLoadScheduler(name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				}).AnyTimes()
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())
			defer func() { w.Run = false }()

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().
				SetNX("maestro-lock-key-my-scheduler-termination", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, errors.New("some error in lock"))).AnyTimes()
			mockRedisClient.EXPECT().
				SetNX("maestro-lock-key-my-scheduler-downscaling", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, errors.New("some error in lock"))).AnyTimes()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
			creating := models.GetRoomStatusSetRedisKey(name, "creating")
			occupied := models.GetRoomStatusSetRedisKey(name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(name, "terminating")
			ready := models.GetRoomStatusSetRedisKey(name, "ready")
			expC := &models.RoomsStatusCount{0, 0, 1, 0}
			mockPipeline.EXPECT().LPush(gomock.Any(), gomock.Any()).AnyTimes()
			mockPipeline.EXPECT().LTrim(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil)).AnyTimes()
			mockPipeline.EXPECT().Exec().AnyTimes()

			Expect(func() {
				go func() {
					defer GinkgoRecover()
					w.Start()
				}()
			}).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			Eventually(func() bool { return hook.LastEntry() != nil }).Should(BeTrue())
			time.Sleep(3 * time.Second)
		})

		It("should not panic if lock is being used", func() {
			name := "my-scheduler"
			gameName := "game-name"
			testing.MockLoadScheduler(name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				}).AnyTimes()
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())
			defer func() { w.Run = false }()

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().SetNX("maestro-lock-key-my-scheduler-downscaling", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, nil)).AnyTimes()
			mockRedisClient.EXPECT().SetNX("maestro-lock-key-my-scheduler-termination", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, nil)).AnyTimes()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
			creating := models.GetRoomStatusSetRedisKey(name, "creating")
			occupied := models.GetRoomStatusSetRedisKey(name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(name, "terminating")
			ready := models.GetRoomStatusSetRedisKey(name, "ready")
			expC := &models.RoomsStatusCount{0, 0, 1, 0}
			mockPipeline.EXPECT().LPush(gomock.Any(), gomock.Any()).AnyTimes()
			mockPipeline.EXPECT().LTrim(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil)).AnyTimes()
			mockPipeline.EXPECT().Exec().AnyTimes()

			Expect(func() {
				go func() {
					defer GinkgoRecover()
					w.Start()
				}()
			}).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			Eventually(func() bool { return hook.LastEntry() != nil }).Should(BeTrue())
			time.Sleep(1 * time.Second)
		})
	})

	Describe("ReportRoomsStatuses", func() {
		It("Should report all 4 statuses", func() {
			var configYaml models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			mockCtrl = gomock.NewController(GinkgoT())
			mockReporter := reportersMocks.NewMockReporter(mockCtrl)

			Creating := models.GetRoomStatusSetRedisKey(w.SchedulerName, models.StatusCreating)
			Ready := models.GetRoomStatusSetRedisKey(w.SchedulerName, models.StatusReady)
			Occupied := models.GetRoomStatusSetRedisKey(w.SchedulerName, models.StatusOccupied)
			Terminating := models.GetRoomStatusSetRedisKey(w.SchedulerName, models.StatusTerminating)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(Creating).Return(redis.NewIntResult(int64(3), nil))
			mockPipeline.EXPECT().SCard(Ready).Return(redis.NewIntResult(int64(2), nil))
			mockPipeline.EXPECT().SCard(Occupied).Return(redis.NewIntResult(int64(1), nil))
			mockPipeline.EXPECT().SCard(Terminating).Return(redis.NewIntResult(int64(5), nil))
			mockPipeline.EXPECT().Exec()

			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]interface{}{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusCreating,
					"gauge":                         "3",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]interface{}{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusReady,
					"gauge":                         "2",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]interface{}{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusOccupied,
					"gauge":                         "1",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]interface{}{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusTerminating,
					"gauge":                         "5",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]interface{}{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusReadyOrOccupied,
					"gauge":                         "3",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventResponseTime,
				gomock.Any(),
			)

			r := reporters.GetInstance()
			r.SetReporter("mockReporter", mockReporter)
			w.ReportRoomsStatuses()
			r.UnsetReporter("mockReporter")
		})
	})

	Describe("AutoScale", func() {

		// createTriggerSpecs populate the lists of triggers in the simulation spec to
		// mock GetUsage() for each of the triggers
		createTriggerSpecs := func(
			upTriggers, downTriggers []*models.ScalingPolicyMetricsTrigger,
			simSpec simulationSpec,
			triggerType models.AutoScalingPolicyType,
			percentageAbove int,
			isDownscaling bool,
		) (up, down []triggerSpec) {
			var triggerMap map[models.AutoScalingPolicyType]triggerSpec
			triggerMap = make(map[models.AutoScalingPolicyType]triggerSpec)

			for _, trigger := range upTriggers {
				pointsAbove := 20
				if trigger.Limit == 0 {
					trigger.Limit = 90
				}
				if trigger.Type == triggerType && !isDownscaling {
					pointsAbove = percentageAbove
					triggerMap[trigger.Type] = triggerSpec{
						targetUsagePercent:          trigger.Usage,
						pointsAboveThresholdPercent: pointsAbove,
						limit:                       trigger.Limit,
						time:                        trigger.Time,
						triggerType:                 trigger.Type,
					}
				}
				if _, exists := triggerMap[trigger.Type]; !exists {
					triggerMap[trigger.Type] = triggerSpec{
						targetUsagePercent:          trigger.Usage,
						pointsAboveThresholdPercent: pointsAbove,
						limit:                       trigger.Limit,
						time:                        trigger.Time,
						triggerType:                 trigger.Type,
					}
				}
			}

			for _, trigger := range downTriggers {
				pointsAbove := 80
				if trigger.Limit == 0 {
					trigger.Limit = 90
				}
				if trigger.Type == triggerType && isDownscaling {
					pointsAbove = percentageAbove
					triggerMap[trigger.Type] = triggerSpec{
						targetUsagePercent:          trigger.Usage,
						pointsAboveThresholdPercent: 100 - pointsAbove,
						limit:                       trigger.Limit,
						time:                        trigger.Time,
						triggerType:                 trigger.Type,
					}
				}
				if _, exists := triggerMap[trigger.Type]; !exists {
					triggerMap[trigger.Type] = triggerSpec{
						targetUsagePercent:          trigger.Usage,
						pointsAboveThresholdPercent: 100 - pointsAbove,
						limit:                       trigger.Limit,
						time:                        trigger.Time,
						triggerType:                 trigger.Type,
					}
				}
				down = append(down, triggerMap[trigger.Type])
			}

			for _, trigger := range upTriggers {
				up = append(up, triggerMap[trigger.Type])
			}

			return up, down
		}

		// simulateMetricsAutoscaling run all the mockoing necessary to test autoscaling with all different types of triggers
		simulateMetricsAutoscaling := func(simSpec simulationSpec, configYaml *models.ConfigYAML, yamlActive, state string) {
			testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, simSpec.roomCount)
			testing.MockGetScheduler(mockDb, configYaml, state, yamlActive, simSpec.lastChangedAt, simSpec.lastScaleOpAt, 2)

			simSpec.up, simSpec.down = createTriggerSpecs(
				configYaml.AutoScaling.Up.MetricsTrigger,
				configYaml.AutoScaling.Down.MetricsTrigger,
				simSpec,
				simSpec.metricTypeToScale,
				simSpec.percentageOfPointsAboveTargetUsage,
				simSpec.isDownscaling,
			)
			if !simSpec.shouldCheckMetricsTriggerUp {
				simSpec.up = []triggerSpec{}
			}
			if !simSpec.shouldCheckMetricsTriggerDown {
				simSpec.down = []triggerSpec{}
			}

			// Mock pod metricses
			containerMetrics := testing.BuildContainerMetricsArray(
				[]testing.ContainerMetricsDefinition{
					testing.ContainerMetricsDefinition{
						Name: configYaml.Name,
						Usage: map[models.AutoScalingPolicyType]int{
							simSpec.metricTypeToScale: simSpec.currentRawUsage,
						},
						MemScale: simSpec.memScale,
					},
				},
			)

			pods := make([]string, simSpec.roomCount.Available())
			for i := 0; i < simSpec.roomCount.Available(); i++ {
				pods[i] = fmt.Sprintf("scheduler:%s:rooms:%s", configYaml.Name, configYaml.Name)
			}
			fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, pods, configYaml.Name)

			// create watcher
			w = watcher.NewWatcher(
				config,
				logger,
				mr,
				mockDb,
				redisClient,
				clientset,
				fakeMetricsClient,
				configYaml.Name,
				configYaml.Game,
				occupiedTimeout,
				[]*eventforwarder.Info{},
			)
			Expect(w).NotTo(BeNil())

			// Mock get usages for each trigger
			for _, up := range simSpec.up {
				// mock GetUsage only if the currentUsage is not above limit or the test is not of this triggerType (cpu, mem...)
				// If currentUsage is above limit, the autoscaler does not get the usage points from redis.
				if (float32(simSpec.currentRawUsage)/1000) < float32(up.limit)/100 || simSpec.metricTypeToScale != up.triggerType {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", up.triggerType, configYaml.Name),
						up.time/w.AutoScalingPeriod, up.targetUsagePercent, up.pointsAboveThresholdPercent, 1,
					)
				}
				if simSpec.metricTypeToScale == up.triggerType && simSpec.shouldScaleUp {
					break
				}
			}

			if !simSpec.shouldScaleUp {
				for _, down := range simSpec.down {
					// mock GetUsage only if the currentUsage is not above limit or the test is not of this triggerType (cpu, mem...)
					// If currentUsage is above limit, the autoscaler does not get the usage points from redis.
					if (float32(simSpec.currentRawUsage)/1000) < float32(down.limit)/100 || simSpec.metricTypeToScale != down.triggerType {
						testing.MockGetUsages(
							mockPipeline, mockRedisClient,
							fmt.Sprintf("maestro:scale:%s:%s", down.triggerType, configYaml.Name),
							down.time/w.AutoScalingPeriod, down.targetUsagePercent, down.pointsAboveThresholdPercent, 1,
						)
					}
					if simSpec.metricTypeToScale == down.triggerType && simSpec.shouldScaleDown {
						break
					}
				}
			}

			if simSpec.shouldScaleUp {
				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					simSpec.roomCount,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					simSpec.deltaExpected,
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)
			} else if simSpec.shouldScaleDown {

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					simSpec.roomCount,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, simSpec.deltaExpected)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, simSpec.deltaExpected)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)
			}

			testing.MockSendUsage(mockPipeline, mockRedisClient, configYaml.AutoScaling)
			Expect(func() { w.AutoScale() }).ShouldNot(Panic())

			for _, logMessage := range simSpec.logMessages {
				Expect(hook.Entries).To(testing.ContainLogMessage(logMessage))
			}
		}

		Context("Legacy", func() {
			var configYaml models.ConfigYAML
			mockAutoScaling := &models.AutoScaling{}

			BeforeEach(func() {
				err := yaml.Unmarshal([]byte(yaml1), &configYaml)
				Expect(err).NotTo(HaveOccurred())
				testing.MockLoadScheduler(configYaml.Name, mockDb).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yaml1
					})
				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())
				testing.CopyAutoScaling(configYaml.AutoScaling, mockAutoScaling)
				testing.TransformLegacyInMetricsTrigger(mockAutoScaling)

				// Mock send usage percentage
				testing.MockSendUsage(mockPipeline, mockRedisClient, mockAutoScaling)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			It("should scale up and update scheduler state", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				// ScaleUp
				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should scale if roomCount is less than min", func() {
				os.Setenv("CONTROLLER_NAME_ENABLE_INFO", "true")

				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 0, configYaml.AutoScaling.Min - 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, 1)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))

				os.Unsetenv("CONTROLLER_NAME_ENABLE_INFO")
			})

			It("should change state and scale if first state change - subdimensioned", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, errDB, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should change state and scale if first state change - overdimensioned", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, 1)
				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, 1)

				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			It("should change state and not scale if in-sync but wrong state reported", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateOverdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should do nothing if in cooldown - subdimensioned", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now()
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should do nothing if in cooldown - overdimensioned", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now()
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateOverdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should warn if scale down is required", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateOverdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yamlWithUpLimit
				})

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Create rooms (ScaleUp)
				timeoutSec := 1000
				var configYaml models.ConfigYAML
				err := yaml.Unmarshal([]byte(yaml1), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				scheduler := models.NewScheduler(configYaml.Name, configYaml.Game, yaml1)

				scaleUpAmount := 5

				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, 5)

				err = testing.MockSetScallingAmount(
					mockRedisClient,
					mockPipeline,
					mockDb,
					clientset,
					&configYaml,
					expC.Ready,
					yamlWithUpLimit,
				)
				Expect(err).NotTo(HaveOccurred())

				err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true, config)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				// ScaleDown
				scaleDownAmount := configYaml.AutoScaling.Down.Delta
				names, err := controller.GetPodNames(scaleDownAmount, scheduler.Name, clientset)
				Expect(err).NotTo(HaveOccurred())

				readyKey := models.GetRoomStatusSetRedisKey(configYaml.Name, models.StatusReady)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, name := range names {
					mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))
				}
				mockPipeline.EXPECT().Exec()

				for _, name := range names {
					testing.MockPodNotFound(mockRedisClient, configYaml.Name, name)
				}

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				err = testing.MockSetScallingAmount(
					mockRedisClient,
					mockPipeline,
					mockDb,
					clientset,
					&configYaml,
					expC.Ready+scaleUpAmount,
					yamlWithUpLimit,
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
				pods, err := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{
					FieldSelector: fields.Everything().String(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(scaleUpAmount - scaleDownAmount))
			})

			It("should do nothing if state is expected", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should do nothing if state is creating", func() {
				// GetSchedulerScalingInfo
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateCreating
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should do nothing if state is terminating", func() {
				// GetSchedulerScalingInfo
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateTerminating
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not panic and log error if failed to change state info", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())


				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, errDB, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("failed to update scheduler info"))
			})

			It("should not scale up if half of the points are below threshold", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 8, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up if 90% of the points are above threshold", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Up.Cooldown+1) * time.Second)
				w.AutoScalingPeriod = configYaml.AutoScaling.Up.Trigger.Time / 10
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}


				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale down if half of the points are below threshold", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down if 90% of the points are above threshold", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Trigger.Time+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateOverdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percenta9es
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, 1)
				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, 1)

				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			It("should scale up, even if lastScaleOpAt is now, if usage is above 90% (default)", func() {
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yaml1
				})
				expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				fmt.Sprintf("%v \n", hook.Entries)
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should scale up, even if lastScaleOpAt is now, if usage is above 70% (from scheduler)", func() {
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateSubdimensioned
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yamlWithUpLimit
				})
				expC := &models.RoomsStatusCount{0, 7, 3, 0} // creating,occupied,ready,terminating

				testing.MockRoomDistribution(&configYaml, mockPipeline, mockRedisClient, expC)

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					&configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				testing.MockScaleUp(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Up.Delta)

				// UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				fmt.Sprintf("%v \n", hook.Entries)
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})
		})

		Context("Legacy error", func() {
			var configYaml models.ConfigYAML
			mockAutoScaling := &models.AutoScaling{}

			BeforeEach(func() {
				err := yaml.Unmarshal([]byte(yaml1), &configYaml)
				Expect(err).NotTo(HaveOccurred())
				testing.MockLoadScheduler(configYaml.Name, mockDb).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yaml1
					})
				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())
				testing.CopyAutoScaling(configYaml.AutoScaling, mockAutoScaling)
				testing.TransformLegacyInMetricsTrigger(mockAutoScaling)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			It("should terminate watcher if scheduler is not in database", func() {
				// GetSchedulerScalingInfo
				testing.MockLoadScheduler(configYaml.Name, mockDb).Return(pg.NewTestResult(fmt.Errorf("scheduler not found"), 0), fmt.Errorf("scheduler not found"))

				w.Run = true
				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(w.Run).To(BeFalse())
			})

			It("should not panic and exit if error retrieving scheduler scaling info", func() {
				// GetSchedulerScalingInfo
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				}).Return(pg.NewTestResult(nil, 0), errors.New("some cool error in pg"))

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("failed to get scheduler scaling info"))
			})
		})

		// Tests for AutoScale with metrics trigger
		Context("LegacyTrigger Down And MetricsTrigger Up", func() {
			var configYaml *models.ConfigYAML
			var yamlActive string
			mockAutoScaling := &models.AutoScaling{}

			BeforeEach(func() {
				yamlActive = yamlWithLegacyDownAndMetricsUpTrigger
				err := yaml.Unmarshal([]byte(yamlActive), &configYaml)
				Expect(err).NotTo(HaveOccurred())
				testing.CopyAutoScaling(configYaml.AutoScaling, mockAutoScaling)
				testing.TransformLegacyInMetricsTrigger(mockAutoScaling)

				// Mock send usage percentage
				testing.MockSendUsage(mockPipeline, mockRedisClient, mockAutoScaling)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			// Down
			It("should scale down if 90% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, configYaml.AutoScaling.Down.Delta)
				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, configYaml.AutoScaling.Down.Delta)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			It("should not scale down if 75% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 75, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not scale down if total rooms is equal min", func() {
				expC := &models.RoomsStatusCount{0, 1, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down to min if total rooms - delta is less than min", func() {
				expC := &models.RoomsStatusCount{0, 1, 2, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Min)
				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Min)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
				Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("amount to scale is lower than min. Maestro will scale down to the min of %d", configYaml.AutoScaling.Min)))
			})

			It("should not scale down if in cooldown period", func() {
				expC := &models.RoomsStatusCount{0, 1, 8, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateOverdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down if total rooms above max", func() {
				expC := &models.RoomsStatusCount{0, 9, 3, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Max)
				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Max)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			// Up
			It("should scale up if 90% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				metricTrigger := mockAutoScaling.Up.MetricsTrigger[0]

				// [Occupied / (Total + Delta)] = Usage/100
				occupied := float64(expC.Occupied)
				total := float64(expC.Total())
				threshold := float64(metricTrigger.Usage) / 100
				delta := occupied - threshold*total
				delta = delta / threshold

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					int(math.Round(float64(delta))),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale up if 75% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 75, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not scale up if total rooms is equal max", func() {
				expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up to max if total rooms + delta is greater than max", func() {
				expC := &models.RoomsStatusCount{0, 6, 3, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Max-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("amount to scale is higher than max. Maestro will scale up to the max of %d", configYaml.AutoScaling.Max)))
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale up if in cooldown period and usage is below limit", func() {
				expC := &models.RoomsStatusCount{0, 6, 2, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 85, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up if in cooldown period and usage is above limit", func() {
				expC := &models.RoomsStatusCount{0, 7, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Max-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should scale up if total rooms below min", func() {
				expC := &models.RoomsStatusCount{0, 0, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Min-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})
		})

		Context("LegacyTrigger Up And MetricsTrigger Down", func() {
			var configYaml *models.ConfigYAML
			var yamlActive string
			mockAutoScaling := &models.AutoScaling{}

			BeforeEach(func() {
				yamlActive = yamlWithLegacyUpAndMetricsDownTrigger
				err := yaml.Unmarshal([]byte(yamlActive), &configYaml)
				Expect(err).NotTo(HaveOccurred())
				testing.CopyAutoScaling(configYaml.AutoScaling, mockAutoScaling)
				testing.TransformLegacyInMetricsTrigger(mockAutoScaling)

				// Mock send usage percentage
				testing.MockSendUsage(mockPipeline, mockRedisClient, mockAutoScaling)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			// Down
			It("should scale down if 90% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				metricTrigger := mockAutoScaling.Down.MetricsTrigger[0]

				// [Occupied / (Total + Delta)] = Usage/100
				occupied := float64(expC.Occupied)
				total := float64(expC.Total())
				threshold := float64(metricTrigger.Usage) / 100
				delta := occupied - threshold*total
				delta = delta / threshold
				deltaInt := -int(math.Round(float64(delta)))

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, deltaInt)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, deltaInt)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			It("should not scale down if 75% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 75, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not scale down if total rooms is equal min", func() {
				expC := &models.RoomsStatusCount{0, 1, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down to min if total rooms - delta is less than min", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				yamlActive = strings.Replace(yamlActive, "min: 2", "min: 4", 1)
				configYaml.AutoScaling.Min = 4
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Min)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Min)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
				Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("amount to scale is lower than min. Maestro will scale down to the min of %d", configYaml.AutoScaling.Min)))
			})

			It("should not scale down if in cooldown period", func() {
				expC := &models.RoomsStatusCount{0, 1, 8, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateOverdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down if total rooms above max", func() {
				expC := &models.RoomsStatusCount{0, 9, 3, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Max)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Max)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			// Up
			It("should scale up if 90% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					mockAutoScaling.Up.MetricsTrigger[0].Delta,
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale up if 75% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 75, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not scale up if total rooms is equal max", func() {
				expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range mockAutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up to max if total rooms + delta is greater than max", func() {
				expC := &models.RoomsStatusCount{0, 6, 3, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Max-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("amount to scale is higher than max. Maestro will scale up to the max of %d", configYaml.AutoScaling.Max)))
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale up if in cooldown period and usage is below limit", func() {
				expC := &models.RoomsStatusCount{0, 6, 2, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range mockAutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 85, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up if in cooldown period and usage is above limit", func() {
				expC := &models.RoomsStatusCount{0, 7, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					mockAutoScaling.Up.MetricsTrigger[0].Delta,
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should scale up if total rooms below min", func() {
				expC := &models.RoomsStatusCount{0, 0, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Min-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})
		})

		Context("Room Metrics Trigger", func() {
			var configYaml *models.ConfigYAML
			var yamlActive string

			BeforeEach(func() {
				yamlActive = yamlWithRoomMetricsTrigger
				err := yaml.Unmarshal([]byte(yamlActive), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				// Mock send usage percentage
				testing.MockSendUsage(mockPipeline, mockRedisClient, configYaml.AutoScaling)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			// Down
			It("should scale down if 90% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range configYaml.AutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				metricTrigger := configYaml.AutoScaling.Down.MetricsTrigger[0]

				// [Occupied / (Total + Delta)] = Usage/100
				occupied := float64(expC.Occupied)
				total := float64(expC.Total())
				threshold := float64(metricTrigger.Usage) / 100
				delta := occupied - threshold*total
				delta = delta / threshold
				deltaInt := -int(math.Round(float64(delta)))

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, deltaInt)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, deltaInt)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			It("should not scale down if 75% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range configYaml.AutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 75, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not scale down if total rooms is equal min", func() {
				expC := &models.RoomsStatusCount{0, 1, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down to min if total rooms - delta is less than min", func() {
				expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
				yamlActive = strings.Replace(yamlActive, "min: 2", "min: 4", 1)
				configYaml.AutoScaling.Min = 4
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range configYaml.AutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Min)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Min)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
				Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("amount to scale is lower than min. Maestro will scale down to the min of %d", configYaml.AutoScaling.Min)))
			})

			It("should not scale down if in cooldown period", func() {
				expC := &models.RoomsStatusCount{0, 1, 8, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateOverdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range configYaml.AutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 10, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale down if total rooms above max", func() {
				expC := &models.RoomsStatusCount{0, 20, 3, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// EnterCriticalSection (lock done by redis-lock)
				downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
				testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
				testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock removal from redis ready set
				testing.MockRedisReadyPop(mockPipeline, mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Max)

				testing.MockPodsNotFound(mockRedisClient, configYaml.Name, expC.Total()-configYaml.AutoScaling.Max)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
			})

			// Up
			It("should scale up if 90% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				metricTrigger := configYaml.AutoScaling.Up.MetricsTrigger[0]

				// [Occupied / (Total + Delta)] = Usage/100
				occupied := float64(expC.Occupied)
				total := float64(expC.Total())
				threshold := float64(metricTrigger.Usage) / 100
				delta := occupied - threshold*total
				delta = delta / threshold

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					int(math.Round(float64(delta))),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale up if 75% of the points are above threshold", func() {
				expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 75, 1,
					)
				}

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range configYaml.AutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should not scale up if total rooms is equal max", func() {
				expC := &models.RoomsStatusCount{0, 19, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Down get usage percentages
				for _, trigger := range configYaml.AutoScaling.Down.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 50, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up to max if total rooms + delta is greater than max", func() {
				expC := &models.RoomsStatusCount{0, 12, 2, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockGetScheduler(mockDb, configYaml, models.StateInSync, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 90, 1,
					)
				}

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Max-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("amount to scale is higher than max. Maestro will scale up to the max of %d", configYaml.AutoScaling.Max)))
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should not scale up if in cooldown period and usage is below limit", func() {
				expC := &models.RoomsStatusCount{0, 6, 2, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				// Mock MetricsTrigger Up get usage percentages
				for _, trigger := range configYaml.AutoScaling.Up.MetricsTrigger {
					testing.MockGetUsages(
						mockPipeline, mockRedisClient,
						fmt.Sprintf("maestro:scale:%s:%s", trigger.Type, configYaml.Name),
						trigger.Time/w.AutoScalingPeriod, trigger.Usage, 85, 1,
					)
				}

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
			})

			It("should scale up if in cooldown period and usage is above limit", func() {
				expC := &models.RoomsStatusCount{0, 10, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				metricTrigger := configYaml.AutoScaling.Up.MetricsTrigger[0]

				// [Occupied / (Total + Delta)] = Usage/100
				occupied := float64(expC.Occupied)
				total := float64(expC.Total())
				threshold := float64(metricTrigger.Usage) / 100
				delta := occupied - threshold*total
				delta = delta / threshold

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					int(math.Round(float64(delta))),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})

			It("should scale up if total rooms below min", func() {
				expC := &models.RoomsStatusCount{0, 0, 1, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock scheduler
				lastChangedAt := time.Now()
				lastScaleOpAt := time.Now()
				testing.MockGetScheduler(mockDb, configYaml, models.StateSubdimensioned, yamlActive, lastChangedAt, lastScaleOpAt, 2)

				w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout, []*eventforwarder.Info{})
				Expect(w).NotTo(BeNil())

				err := testing.MockSetScallingAmountWithRoomStatusCount(
					mockRedisClient,
					mockPipeline,
					configYaml,
					expC,
				)
				Expect(err).NotTo(HaveOccurred())

				// ScaleUp
				testing.MockScaleUp(
					mockPipeline, mockRedisClient,
					configYaml.Name,
					configYaml.AutoScaling.Min-expC.Total(),
				)

				// Mock UpdateScheduler
				testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
					Expect(scheduler.State).To(Equal("in-sync"))
					Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
					Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				}, mockDb, nil, nil)

				Expect(func() { w.AutoScale() }).ShouldNot(Panic())
				Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
			})
		})

		Context("Resource Metrics Trigger", func() {
			var configYaml *models.ConfigYAML
			var yamlActive string

			calculateExpectedDelta := func(metricsTrigger *models.ScalingPolicyMetricsTrigger, simSpec simulationSpec, min, max int) int {
				desiredUsage := float64(metricsTrigger.Usage) / 100
				currentUsage := float64(simSpec.currentRawUsage) / 1000
				roomsAvailable := simSpec.roomCount.Available()
				usageRatio := currentUsage / desiredUsage
				expectedDelta := int(math.Ceil(usageRatio*float64(roomsAvailable))) - roomsAvailable

				if roomsAvailable+expectedDelta < min || roomsAvailable < min {
					return int(math.Abs(float64(roomsAvailable - min)))
				}

				if roomsAvailable+expectedDelta > max || roomsAvailable > max {
					return int(math.Abs(float64(max - roomsAvailable)))
				}

				return int(math.Abs(float64(expectedDelta)))
			}

			BeforeEach(func() {
				yamlActive = yamlWithCPUAndMemoryMetricsTrigger
				err := yaml.Unmarshal([]byte(yamlActive), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				testing.CreatePod(clientset, "1.0", "1Gi", configYaml.Name, "", configYaml.Name)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			// Down
			It("should scale down if 90% of the points are above threshold", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 1, 4, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    true,
					isDownscaling:                      true,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      true,
					memScale:                           resource.Mega,
					currentRawUsage:                    200,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages:                        []string{"scheduler is overdimensioned, should scale down"},
				}

				// EnterCriticalSection (lock done by redis-lock)
				for index := 0; index < 2; index++ {
					downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
					testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
					testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should not scale down if 75% of the points are above threshold", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 1, 4, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    false,
					isDownscaling:                      true,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      true,
					memScale:                           resource.Mega,
					currentRawUsage:                    200,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 75,
					logMessages:                        []string{"scheduler 'controller-name': state is as expected"},
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should not scale down if total rooms is equal min", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 1, 1, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    false,
					isDownscaling:                      true,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    200,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages:                        []string{"scheduler 'controller-name': state is as expected"},
				}

				// CPU
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should scale down to min if total rooms - delta is less than min", func() {
				yamlActive = strings.Replace(yamlActive, "min: 2", "min: 4", 1)
				configYaml.AutoScaling.Min = 4

				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 1, 4, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    true,
					isDownscaling:                      true,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      true,
					memScale:                           resource.Mega,
					currentRawUsage:                    100,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages: []string{
						"scheduler is overdimensioned, should scale down",
						fmt.Sprintf("amount to scale is lower than min. Maestro will scale down to the min of %d", configYaml.AutoScaling.Min),
					},
				}

				// EnterCriticalSection (lock done by redis-lock)
				for index := 0; index < 2; index++ {
					downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
					testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
					testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should not scale down if in cooldown period", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now(),
					lastScaleOpAt:                      time.Now(),
					roomCount:                          &models.RoomsStatusCount{0, 1, 8, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    false,
					isDownscaling:                      true,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      true,
					memScale:                           resource.Mega,
					currentRawUsage:                    200,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages:                        []string{"scheduler 'controller-name': state is as expected"},
				}

				// CPU
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateOverdimensioned)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateOverdimensioned)
			})

			It("should scale down if total rooms above max", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 20, 3, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    true,
					isDownscaling:                      true,
					shouldCheckMetricsTriggerUp:        false,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    900,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 20,
					logMessages:                        []string{"scheduler is overdimensioned, should scale down"},
				}

				// EnterCriticalSection (lock done by redis-lock)
				for index := 0; index < 2; index++ {
					downScalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), configYaml.Name)
					testing.MockRedisLock(mockRedisClient, downScalingLockKey, 5, true, nil)
					testing.MockReturnRedisLock(mockRedisClient, downScalingLockKey, nil)
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Down.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			// Up
			It("should scale up if 90% of the points are above threshold", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 4, 1, 0}, // creating,occupied,ready,terminating
					shouldScaleUp:                      true,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    600,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages:                        []string{"scheduler is subdimensioned, scaling up"},
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should not scale up if 75% of the points are above threshold", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 4, 1, 0}, // creating,occupied,ready,terminating
					shouldScaleUp:                      false,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      true,
					memScale:                           resource.Mega,
					currentRawUsage:                    800,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 75,
					logMessages:                        []string{"scheduler 'controller-name': state is as expected"},
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should not scale up if total rooms is equal max", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 19, 1, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        false,
					shouldCheckMetricsTriggerDown:      true,
					memScale:                           resource.Mega,
					currentRawUsage:                    800,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages:                        []string{"scheduler 'controller-name': state is as expected"},
				}

				// CPU
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should scale up to max if total rooms + delta is greater than max", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 12, 2, 0}, // creating,occupied,ready,terminating
					shouldScaleUp:                      true,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    800,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages: []string{
						fmt.Sprintf("amount to scale is higher than max. Maestro will scale up to the max of %d", configYaml.AutoScaling.Max),
						"scheduler is subdimensioned, scaling up",
					},
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})

			It("should not scale up if in cooldown period and usage is below limit", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now(),
					lastScaleOpAt:                      time.Now(),
					roomCount:                          &models.RoomsStatusCount{0, 6, 2, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      false,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    850,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 90,
					logMessages:                        []string{"scheduler 'controller-name': state is as expected"},
				}

				// CPU
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateSubdimensioned)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateSubdimensioned)
			})

			It("should scale up if in cooldown period and usage is above limit", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now(),
					lastScaleOpAt:                      time.Now(),
					roomCount:                          &models.RoomsStatusCount{0, 10, 1, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      true,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        true,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    980,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 20,
					logMessages:                        []string{"scheduler is subdimensioned, scaling up"},
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateSubdimensioned)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateSubdimensioned)
			})

			It("should scale up if total rooms below min", func() {
				simSpec := simulationSpec{
					lastChangedAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					lastScaleOpAt:                      time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second),
					roomCount:                          &models.RoomsStatusCount{0, 0, 1, 0}, // creating,occupied,ready,terminatings
					shouldScaleUp:                      true,
					shouldScaleDown:                    false,
					isDownscaling:                      false,
					shouldCheckMetricsTriggerUp:        false,
					shouldCheckMetricsTriggerDown:      false,
					memScale:                           resource.Mega,
					currentRawUsage:                    100,
					metricTypeToScale:                  models.CPUAutoScalingPolicyType,
					percentageOfPointsAboveTargetUsage: 20,
					logMessages:                        []string{"scheduler is subdimensioned, scaling up"},
				}

				// CPU
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[0],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)

				// Mem
				simSpec.metricTypeToScale = models.MemAutoScalingPolicyType
				simSpec.deltaExpected = calculateExpectedDelta(
					configYaml.AutoScaling.Up.MetricsTrigger[1],
					simSpec,
					configYaml.AutoScaling.Min,
					configYaml.AutoScaling.Max,
				)
				simulateMetricsAutoscaling(simSpec, configYaml, yamlActive, models.StateInSync)
			})
		})

	})

	Describe("RemoveDeadRooms", func() {
		var configYaml models.ConfigYAML
		var w *watcher.Watcher
		createNamespace := func(name string, clientset kubernetes.Interface) error {
			return models.NewNamespace(name).Create(clientset)
		}
		createPod := func(name, namespace string, clientset kubernetes.Interface) error {
			configYaml := &models.ConfigYAML{
				Name:  namespace,
				Game:  "game",
				Image: "img",
			}
			mockRedisClient.EXPECT().
				HGet(models.GetPodMapRedisKey(configYaml.Name), name).
				Return(redis.NewStringResult("", redis.Nil))
			pod, err := models.NewPod(name, nil, configYaml, clientset, mockRedisClient, mr)
			if err != nil {
				return err
			}
			_, err = pod.Create(clientset)
			return err
		}
		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml)
			Expect(err).NotTo(HaveOccurred())
			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(
				config, logger, mr, mockDb, redisClient, clientset, metricsClientset,
				configYaml.Name, configYaml.Game, occupiedTimeout,
				[]*eventforwarder.Info{
					&eventforwarder.Info{
						Plugin:    "plugin",
						Name:      "name",
						Forwarder: mockEventForwarder,
					},
				},
			)
			Expect(w).NotTo(BeNil())

			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
		})

		It("should call controller roomsWithNoPing and roomsWithOccupationTimeout", func() {
			schedulerName := configYaml.Name
			pKey := models.GetRoomPingRedisKey(schedulerName)
			lKey := models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied)
			ts := time.Now().Unix() - w.Config.GetInt64("pingTimeout")
			createNamespace(schedulerName, clientset)

			// roomsWithNoPing
			expectedRooms := []string{"room1", "room2", "room3"}
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				gomock.Any(),
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult([]string{"room1"}, nil)).AnyTimes()

			// roomsWithOccupationTimeout
			ts = time.Now().Unix() - w.OccupiedTimeout
			mockRedisClient.EXPECT().ZRangeByScore(
				lKey,
				gomock.Any(),
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult([]string{"room2","room3"}, nil)).AnyTimes()

			for _, roomName := range expectedRooms {
				err := createPod(roomName, schedulerName, clientset)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HGet("scheduler:controller-name:rooms:room1", "metadata")
			mockPipeline.EXPECT().HGet("scheduler:controller-name:rooms:room2", "metadata")
			mockPipeline.EXPECT().HGet("scheduler:controller-name:rooms:room3", "metadata")
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{
				redis.NewStringResult(`{"region": "us"}`, nil),
				redis.NewStringResult(`{"region": "us"}`, nil),
				redis.NewStringResult(`{"region": "us"}`, nil),
			}, nil).Times(2)

			// Mock room creation
			testing.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml, 0)

			// Mock get terminating rooms
			testing.MockListPods(mockPipeline, mockRedisClient, schedulerName, []string{"room1","room2","room3"}, nil)
			testing.MockRemoveZombieRooms(mockPipeline, mockRedisClient, []string{"scheduler:controller-name:rooms:room-0"}, schedulerName)

			for _, roomName := range expectedRooms {
				runningCall := testing.MockRunningPod(mockRedisClient, configYaml.Name, roomName)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(configYaml.Name), roomName)
				mockPipeline.EXPECT().Exec()

				testing.MockPodNotFound(mockRedisClient, configYaml.Name, roomName).After(runningCall)

				room := models.NewRoom(roomName, schedulerName)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(
						models.GetRoomStatusSetRedisKey(schedulerName, status),
						room.GetRoomRedisKey())
					mockPipeline.EXPECT().ZRem(
						models.GetLastStatusRedisKey(schedulerName, status),
						roomName)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), roomName)
				for _, mt := range allMetrics {
					mockPipeline.EXPECT().ZRem(
						models.GetRoomMetricsRedisKey(schedulerName, mt),
						roomName)
				}

				mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
				mockPipeline.EXPECT().Exec()
				mockEventForwarder.EXPECT().Forward(gomock.Any(), models.RoomTerminated,
					map[string]interface{}{
						"game":     "controller-name",
						"host":     "",
						"port":     int32(0),
						"roomId":   roomName,
						"metadata": map[string]interface{}{"region": "us"},
					}, map[string]interface{}(nil))
			}

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
					scheduler.Game = schedulerName
				}).Times(4)

			Expect(func() { w.RemoveDeadRooms() }).ShouldNot(Panic())
		})

		It("should log and not panic in case of no ping since error", func() {
			schedulerName := configYaml.Name
			pKey := models.GetRoomPingRedisKey(schedulerName)
			lKey := models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied)
			ts := time.Now().Unix() - w.Config.GetInt64("pingTimeout")
			expectedRooms := []string{"room-0", "room-1", "room-2"}

			// DeleteRoomsNoPingSince
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				gomock.Any(),
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult([]string{}, errors.New("some error")))

			// DeleteRoomsOccupiedTimeout
			ts = time.Now().Unix() - w.OccupiedTimeout
			mockRedisClient.EXPECT().ZRangeByScore(
				lKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(ts, 10)},
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult(expectedRooms, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HGet("scheduler:controller-name:rooms:room-0", "metadata")
			mockPipeline.EXPECT().HGet("scheduler:controller-name:rooms:room-1", "metadata")
			mockPipeline.EXPECT().HGet("scheduler:controller-name:rooms:room-2", "metadata")
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{
				redis.NewStringResult(`{"region": "us"}`, nil),
				redis.NewStringResult(`{"region": "us"}`, nil),
				redis.NewStringResult(`{"region": "us"}`, nil),
			}, nil)
			for _, room := range expectedRooms {
				mockEventForwarder.EXPECT().Forward(
					gomock.Any(),
					models.RoomTerminated,
					map[string]interface{}{
						"game":     "",
						"host":     "",
						"port":     int32(0),
						"roomId":   room,
						"metadata": map[string]interface{}{"region": "us"},
					},
					map[string]interface{}(nil),
				).Do(func(
					ctx context.Context,
					status string,
					infos, fwdMetadata map[string]interface{},
				) {
					Expect(status).To(Equal(models.RoomTerminated))
				})
			}

			testing.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml, 0)

			// Mock get terminating rooms
			testing.MockListPods(mockPipeline, mockRedisClient, schedulerName, []string{}, nil)
			testing.MockRemoveZombieRooms(mockPipeline, mockRedisClient, expectedRooms, schedulerName)

			testing.MockListPods(mockPipeline, mockRedisClient, schedulerName, []string{}, nil)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				}).Times(4)

			Expect(func() { w.RemoveDeadRooms() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("error listing rooms with no ping since"))
		})

		It("should log and not panic in case of occupied error", func() {
			schedulerName := configYaml.Name
			pKey := models.GetRoomPingRedisKey(schedulerName)
			lKey := models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied)
			ts := time.Now().Unix() - w.Config.GetInt64("pingTimeout")
			// DeleteRoomsNoPingSince
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				gomock.Any(),
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult([]string{}, nil))

			// Mock get terminating rooms
			testing.MockRemoveZombieRooms(mockPipeline, mockRedisClient, []string{}, schedulerName)

			// DeleteRoomsOccupiedTimeout
			ts = time.Now().Unix() - w.OccupiedTimeout
			mockRedisClient.EXPECT().ZRangeByScore(
				lKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(ts, 10)},
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult(nil, errors.New("redis error")))
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()

			Expect(func() { w.RemoveDeadRooms() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("error listing rooms with no occupied timeout"))
		})
	})

	Describe("AddUtilizationMetricsToRedis", func() {
		Context("when scheduler uses metrics trigger", func() {
			var configYaml *models.ConfigYAML
			var yamlActive string

			BeforeEach(func() {
				yamlActive = yamlWithCPUAndMemoryMetricsTrigger
				err := yaml.Unmarshal([]byte(yamlActive), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			})

			It("should add utilization metrics to redis", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yamlActive
				}).Times(2)
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock pod metricses
				cpuUsage := 100
				memUsage := 100
				memScale := resource.Mega
				memUsageInBytes := memUsage * int(math.Pow10(int(memScale)))
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: configYaml.Name,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: cpuUsage,
								models.MemAutoScalingPolicyType: memUsage,
							},
							MemScale: memScale,
						},
					},
				)

				// Create test rooms names for each status
				roomNames, rooms := testing.CreateTestRooms(clientset, configYaml.Name, expC)
				testing.MockListPods(mockPipeline, mockRedisClient, configYaml.Name, roomNames, nil)

				// Mock saving CPU for all ready and occupied rooms
				testing.MockSavingRoomsMetricses(
					mockRedisClient,
					mockPipeline,
					models.CPUAutoScalingPolicyType,
					append(rooms[models.StatusReady], rooms[models.StatusOccupied]...),
					float64(cpuUsage),
					configYaml.Name,
				)

				// Mock saving MEM for all ready and occupied rooms
				testing.MockSavingRoomsMetricses(
					mockRedisClient,
					mockPipeline,
					models.MemAutoScalingPolicyType,
					append(rooms[models.StatusReady], rooms[models.StatusOccupied]...),
					float64(memUsageInBytes),
					configYaml.Name,
				)

				// Mock pod metricses list response from kube
				fakeMetricsClient := testing.CreatePodsMetricsList(
					containerMetrics,
					append(rooms[models.StatusReady],
						rooms[models.StatusOccupied]...),
					configYaml.Name,
				)

				w = watcher.NewWatcher(config,
					logger,
					mr,
					mockDb,
					redisClient,
					clientset,
					fakeMetricsClient,
					configYaml.Name,
					configYaml.Game,
					occupiedTimeout,
					[]*eventforwarder.Info{})

				Expect(func() { w.AddUtilizationMetricsToRedis() }).ShouldNot(Panic())
			})

			It("should add utilization metrics to redis if metrics not yet available", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yamlActive
				}).Times(2)
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock pod metricses
				cpuUsage := 100
				memUsage := 100
				memScale := resource.Mega
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: configYaml.Name,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: cpuUsage,
								models.MemAutoScalingPolicyType: memUsage,
							},
							MemScale: memScale,
						},
					},
				)

				// Create test rooms names for each status
				roomNames, rooms := testing.CreateTestRooms(clientset, configYaml.Name, expC)
				testing.MockListPods(mockPipeline, mockRedisClient, configYaml.Name, roomNames, nil)

				// Mock saving CPU for all ready and occupied rooms
				testing.MockSavingRoomsMetricses(
					mockRedisClient,
					mockPipeline,
					models.CPUAutoScalingPolicyType,
					append(rooms[models.StatusReady], rooms[models.StatusOccupied]...),
					float64(math.MaxInt64),
					configYaml.Name,
				)

				// Mock saving MEM for all ready and occupied rooms
				testing.MockSavingRoomsMetricses(
					mockRedisClient,
					mockPipeline,
					models.MemAutoScalingPolicyType,
					append(rooms[models.StatusReady], rooms[models.StatusOccupied]...),
					float64(math.MaxInt64),
					configYaml.Name,
				)

				fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, []string{}, configYaml.Name)

				w = watcher.NewWatcher(config,
					logger,
					mr,
					mockDb,
					redisClient,
					clientset,
					fakeMetricsClient,
					configYaml.Name,
					configYaml.Game,
					occupiedTimeout,
					[]*eventforwarder.Info{})

				Expect(func() { w.AddUtilizationMetricsToRedis() }).ShouldNot(Panic())
			})

			It("should not add utilization metrics to redis if no pods were listed", func() {
				// GetSchedulerScalingInfo
				lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml.AutoScaling.Down.Cooldown+1) * time.Second)
				testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.State = models.StateInSync
					scheduler.StateLastChangedAt = lastChangedAt.Unix()
					scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
					scheduler.YAML = yamlActive
				}).Times(2)
				expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
				testing.MockRoomDistribution(configYaml, mockPipeline, mockRedisClient, expC)

				// Mock pod metricses
				cpuUsage := 100
				memUsage := 100
				memScale := resource.Mega
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: configYaml.Name,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: cpuUsage,
								models.MemAutoScalingPolicyType: memUsage,
							},
							MemScale: memScale,
						},
					},
				)

				fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, []string{}, configYaml.Name, nil)
				testing.MockListPods(mockPipeline, mockRedisClient, configYaml.Name, []string{}, nil)

				w = watcher.NewWatcher(config,
					logger,
					mr,
					mockDb,
					redisClient,
					clientset,
					fakeMetricsClient,
					configYaml.Name,
					configYaml.Game,
					occupiedTimeout,
					[]*eventforwarder.Info{})

				Expect(func() { w.AddUtilizationMetricsToRedis() }).ShouldNot(Panic())
			})
		})
	})

	Describe("EnsureCorrectRooms", func() {
		var configYaml models.ConfigYAML
		var w *watcher.Watcher

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml)
			Expect(err).NotTo(HaveOccurred())
			testing.MockLoadScheduler(configYaml.Name, mockDb).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})

			config.Set("watcher.goroutinePoolSize", 1)

			w = watcher.NewWatcher(
				config, logger, mr, mockDb, redisClient, clientset, metricsClientset,
				configYaml.Name, configYaml.Game, occupiedTimeout,
				[]*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
		})

		It("should return error if fail to get scheduler", func() {
			testing.MockSelectScheduler(yaml1, mockDb, errDB)
			err := w.EnsureCorrectRooms()
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(maestroErrors.NewDatabaseError(errDB)))
		})

		It("should return error if fail to unmarshal yaml", func() {
			testing.MockLoadScheduler(w.SchedulerName, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(w.SchedulerName, "", "}\":{}")
				}).Return(pg.NewTestResult(nil, 1), nil)

			err := w.EnsureCorrectRooms()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("yaml: did not find expected node content"))
		})

		It("should return error if fail to read rooms from redis", func() {
			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockListPods(mockPipeline, mockRedisClient, configYaml.Name, []string{}, errDB)

			err := w.EnsureCorrectRooms()

			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errDB))
		})

		It("should log error and not return if fail to delete pod", func() {
			podNames := []string{"room-1", "room-2"}
			room := models.NewRoom(podNames[0], w.SchedulerName)

			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockListPods(mockPipeline, mockRedisClient, w.SchedulerName, podNames, nil)
			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline, w.SchedulerName, [][]string{ {}, {room.GetRoomRedisKey()} }, nil)

			opManager := models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			testing.MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

			// Create room
			testing.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml, 1)
			testing.MockGetPortsFromPoolAnyTimes(&configYaml, mockRedisClient, mockPortChooser,
				models.NewPortRange(5000, 6000).String(), 5000, 6000)

			testing.MockAnyRunningPod(mockRedisClient, w.SchedulerName, 2)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SIsMember(models.GetRoomStatusSetRedisKey(configYaml.Name, "ready"), gomock.Any()).Return(redis.NewBoolResult(true, nil))
			mockPipeline.EXPECT().SIsMember(models.GetRoomStatusSetRedisKey(configYaml.Name, "occupied"), gomock.Any()).Return(redis.NewBoolResult(false, nil))
			exec1 := mockPipeline.EXPECT().Exec().Return(nil,nil)

			testing.MockRunningPod(mockRedisClient, w.SchedulerName, "room-2")

			for _, podName := range podNames {
				pod := &v1.Pod{}
				pod.SetName(podName)
				pod.SetNamespace(w.SchedulerName)
				pod.SetLabels(map[string]string{"version": "v1.0"})
				_, err := clientset.CoreV1().Pods(w.SchedulerName).Create(pod)
				Expect(err).ToNot(HaveOccurred())
			}

			room = models.NewRoom(podNames[1], w.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(w.SchedulerName), room.ID)
			exec2 := mockPipeline.EXPECT().Exec().Return(nil, nil).After(exec1)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for _, status := range allStatus {
				mockPipeline.EXPECT().
					SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status),
						room.GetRoomRedisKey()).AnyTimes()
				mockPipeline.EXPECT().
					ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status),
						room.ID).AnyTimes()
			}
			mockPipeline.EXPECT().
				ZRem(models.GetRoomPingRedisKey(w.SchedulerName), room.ID).AnyTimes()
			for _, mt := range allMetrics {
				mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(w.SchedulerName, mt), gomock.Any()).AnyTimes()
			}
			mockPipeline.EXPECT().Del(room.GetRoomRedisKey()).AnyTimes()
			mockPipeline.EXPECT().Exec().Return(nil, errDB).After(exec2)

			err := w.EnsureCorrectRooms()

			Expect(err).ToNot(HaveOccurred())
			Expect(hook.Entries).To(testing.ContainLogMessage(fmt.Sprintf("error deleting pod %s", podNames[1])))
		})

		It("should set state to rolling update when there is no invalid pods and a operation is running", func() {
			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockListPods(mockPipeline, mockRedisClient, w.SchedulerName, []string{}, nil)
			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline,
				w.SchedulerName, [][]string{}, nil)

			opKey := fmt.Sprintf("opmanager:%s:1234", configYaml.Name)
			opManager := models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			mockRedisClient.EXPECT().
				Get(opManager.BuildCurrOpKey()).
				Return(redis.NewStringResult(opKey, nil))

			mockRedisClient.EXPECT().
				HGetAll(opKey).
				Return(redis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRunning,
				}, nil))

			mockRedisClient.EXPECT().
				HMSet(opKey, map[string]interface{}{
					"description": models.OpManagerRollingUpdate,
				}).Return(redis.NewStatusResult("", nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Del(models.GetInvalidRoomsKey(configYaml.Name))
			mockPipeline.EXPECT().Exec()

			err := w.EnsureCorrectRooms()

			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete invalid pods", func() {
			podNames := []string{"room-1", "room-2"}
			room := models.NewRoom(podNames[0], w.SchedulerName)

			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockListPods(mockPipeline, mockRedisClient, w.SchedulerName, podNames, nil)
			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline,
				w.SchedulerName, [][]string{{room.GetRoomRedisKey()}}, nil)

			opManager := models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			testing.MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

			// Create room
			testing.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml, 1)
			testing.MockGetPortsFromPoolAnyTimes(&configYaml, mockRedisClient, mockPortChooser,
				models.NewPortRange(5000, 6000).String(), 5000, 6000)

			testing.MockAnyRunningPod(mockRedisClient, w.SchedulerName, 2)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SIsMember(models.GetRoomStatusSetRedisKey(configYaml.Name, "ready"), gomock.Any()).Return(redis.NewBoolResult(true, nil))
			mockPipeline.EXPECT().SIsMember(models.GetRoomStatusSetRedisKey(configYaml.Name, "occupied"), gomock.Any()).Return(redis.NewBoolResult(false, nil))
			mockPipeline.EXPECT().Exec().Return(nil,nil)

			runningPod := testing.MockRunningPod(mockRedisClient, w.SchedulerName, "room-2")

			for _, podName := range podNames {
				pod := &v1.Pod{}
				pod.SetName(podName)
				pod.SetNamespace(w.SchedulerName)
				pod.SetLabels(map[string]string{"version": "v1.0"})
				_, err := clientset.CoreV1().Pods(w.SchedulerName).Create(pod)
				Expect(err).ToNot(HaveOccurred())
			}

			// Delete old rooms
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HDel(models.GetPodMapRedisKey(w.SchedulerName), "room-2")
			mockPipeline.EXPECT().Exec()

			testing.MockRemoveAnyRoomsFromRedisAnyTimes(mockRedisClient, mockPipeline, &configYaml, nil, 1)
			testing.MockPodNotFound(mockRedisClient, w.SchedulerName, "room-2").After(runningPod)

			err := w.EnsureCorrectRooms()

			Expect(err).ToNot(HaveOccurred())
		})

		It("should stop when operation is canceled", func() {
			testing.MockSelectScheduler(yaml1, mockDb, nil)
			//mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HGetAll(
				models.GetPodMapRedisKey(configYaml.Name)).
				Return(redis.NewStringStringMapResult(map[string]string{
					"room-1": `{"name": "room-1", "version": "v2.0"}`,
					"room-2": `{"name": "room-1", "version": "v2.0"}`,
			}, nil))
			//mockPipeline.EXPECT().Exec()

			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline, w.SchedulerName, [][]string{}, nil)

			opKey := fmt.Sprintf("opmanager:%s:operation", configYaml.Name)
			opManager := models.NewOperationManager(configYaml.Name, mockRedisClient, logger)

			mockRedisClient.EXPECT().Get(opManager.BuildCurrOpKey()).Return(redis.NewStringResult(opKey, nil))

			call := mockRedisClient.EXPECT().HGetAll(opKey).Return(
				redis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerRollingUpdate,
				}, nil)).Times(2)
			mockRedisClient.EXPECT().HGetAll(opKey).
				Return(redis.NewStringStringMapResult(map[string]string(nil), nil)).
				After(call)

			// Create room
			testing.MockCreateRoomsAnyTimes(mockRedisClient, mockPipeline, &configYaml, 0)
			testing.MockGetPortsFromPoolAnyTimes(&configYaml, mockRedisClient, mockPortChooser,
				models.NewPortRange(5000, 6000).String(), 5000, 6000)

			testing.MockPodNotFound(mockRedisClient, w.SchedulerName, gomock.Any()).AnyTimes()

			err := w.EnsureCorrectRooms()
			Expect(err).ToNot(HaveOccurred())
			Expect(hook.Entries).To(testing.ContainLogMessage("operation canceled while error replacing chunk of pods"))
		})
	})

	Describe("PodStatesCount", func() {
		var configYaml models.ConfigYAML
		var mockReporter *reportersMocks.MockReporter

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml)
			Expect(err).NotTo(HaveOccurred())
			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient,
				clientset, metricsClientset, configYaml.Name, configYaml.Game, occupiedTimeout,
				[]*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			r := reporters.GetInstance()
			mockReporter = reportersMocks.NewMockReporter(mockCtrl)
			r.SetReporter("mockReporter", mockReporter)

			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
		})

		AfterEach(func() {
			r := reporters.GetInstance()
			r.UnsetReporter("mockReporter")
		})

		It("should send to statsd reason of pods to restart", func() {
			nPods := 3
			reason := "bug"

			pods := make(map[string]string)
			for idx := 1; idx <= nPods; idx++ {
				var pod models.Pod
				pod.Name = fmt.Sprintf("pod-%d", idx)
				pod.Namespace = w.SchedulerName
				pod.Status = v1.PodStatus{
					Phase: v1.PodPending,
					ContainerStatuses: []v1.ContainerStatus{{
						LastTerminationState: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Reason: reason,
							},
						},
					}},
				}
				jsonBytes, err := pod.MarshalToRedis()
				Expect(err).ToNot(HaveOccurred())
				pods[pod.Name] = string(jsonBytes)
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				HGetAll(models.GetPodMapRedisKey(w.SchedulerName)).
				Return(redis.NewStringStringMapResult(pods, nil))
			mockPipeline.EXPECT().Exec()

			mockReporter.EXPECT().Report(reportersConstants.EventResponseTime, gomock.Any())

			stateCount := map[v1.PodPhase]int{
				v1.PodPending:   nPods,
				v1.PodRunning:   0,
				v1.PodSucceeded: 0,
				v1.PodFailed:    0,
				v1.PodUnknown:   0,
			}

			for _ = range stateCount {
				mockReporter.EXPECT().Report(reportersConstants.EventPodStatus, gomock.Any())
			}

			mockReporter.EXPECT().Report(reportersConstants.EventPodLastStatus, map[string]interface{}{
				reportersConstants.TagGame:      w.GameName,
				reportersConstants.TagScheduler: w.SchedulerName,
				reportersConstants.TagReason:    reason,
				reportersConstants.ValueGauge:   fmt.Sprintf("%d", nPods),
			})

			w.PodStatesCount()
		})
	})
})
