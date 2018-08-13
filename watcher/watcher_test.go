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
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	goredis "github.com/go-redis/redis"
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
	"k8s.io/api/core/v1"
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
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
      threshold: 80
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
      threshold: 80
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
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
	yamlWithMinZero = `
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
  min: 0
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
      threshold: 80
      limit: 70
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
	yamlWithDownDelta5 = `
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
    cooldown: 200
  down:
    delta: 5
    trigger:
      usage: 30
      time: 500
      threshold: 80
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
	occupiedTimeout = 300
)

var _ = Describe("Watcher", func() {
	var w *watcher.Watcher
	var errDB = errors.New("db failed")

	buildRedisScaleInfo := func(
		percentageAboveUp, percentageAboveDown int,
	) {
		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
		mockPipeline.EXPECT().LPush(gomock.Any(), gomock.Any()).Times(2)
		mockPipeline.EXPECT().LTrim(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
		mockPipeline.EXPECT().Exec().Times(2)

		mid := (w.ScaleUpInfo.Size() * percentageAboveUp) / 100
		usages := make([]string, w.ScaleUpInfo.Size())
		for idx := range usages {
			if idx < mid {
				usages[idx] = "0.9"
			} else {
				usages[idx] = "0.3"
			}
		}
		mockPipeline.EXPECT().LRange("maestro:scale:up:controller-name", gomock.Any(), gomock.Any()).Return(goredis.NewStringSliceResult(
			usages, nil,
		))

		mid = (w.ScaleDownInfo.Size() * percentageAboveDown) / 100
		usages = make([]string, w.ScaleDownInfo.Size())
		for idx := range usages {
			if idx < mid {
				usages[idx] = "0.9"
			} else {
				usages[idx] = "0.3"
			}
		}
		mockPipeline.EXPECT().LRange("maestro:scale:down:controller-name", gomock.Any(), gomock.Any()).Return(goredis.NewStringSliceResult(
			usages, nil,
		))
	}

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

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w.AutoScalingPeriod).To(Equal(autoScalingPeriod))
			Expect(w.Config).To(Equal(config))
			Expect(w.DB).To(Equal(mockDb))
			Expect(w.KubernetesClient).To(Equal(clientset))
			Expect(w.Logger).NotTo(BeNil())
			Expect(w.MetricsReporter).To(Equal(mr))
			Expect(w.RedisClient).To(Equal(redisClient))
			Expect(w.LockKey).To(Equal(fmt.Sprintf("%s-%s", lockKey, name)))
			Expect(w.LockTimeoutMS).To(Equal(lockTimeoutMs))
			Expect(w.SchedulerName).To(Equal(name))
		})

		It("should return configured new watcher using configuration defaults", func() {
			name := "my-scheduler"
			gameName := "game-name"
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w.AutoScalingPeriod).To(Equal(10))
			Expect(w.LockKey).To(Equal("maestro-lock-key-my-scheduler"))
			Expect(w.LockTimeoutMS).To(Equal(180000))
		})
	})

	Describe("Start", func() {
		BeforeEach(func() {
			config.Set("watcher.autoScalingPeriod", 1)
		})

		It("should start watcher", func() {
			// Testing here if no scaling needs to be done
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset,
				configYaml1.Name, configYaml1.Game, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			// EnterCriticalSection (lock done by redis-lock)
			lockKey := fmt.Sprintf("maestro-lock-key-%s", configYaml1.Name)
			mockRedisClient.EXPECT().
				SetNX(lockKey, gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(true, nil)).
				AnyTimes()

			// check scale infos and if should scale
			buildRedisScaleInfo(90, 50)

			// DeleteRoomsNoPingSince
			pKey := models.GetRoomPingRedisKey(configYaml1.Name)
			// DeleteRoomsNoPingSince
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey, gomock.Any(),
			).Return(redis.NewStringSliceResult([]string{}, nil))

			//DeleteRoomsOccupiedTimeout
			lKey := models.GetLastStatusRedisKey(configYaml1.Name, models.StatusOccupied)
			mockRedisClient.EXPECT().ZRangeByScore(
				lKey,
				gomock.Any(),
			).Return(redis.NewStringSliceResult([]string{}, nil))

			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
				scheduler.State = models.StateInSync
			}).AnyTimes()
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{4, 3, 20, 1}
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil)).AnyTimes()
			mockPipeline.EXPECT().Exec().AnyTimes()

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				w.Run = false
			}, mockDb, nil, nil)

			// LeaveCriticalSection (unlock done by redis-lock)
			mockRedisClient.EXPECT().Eval(gomock.Any(), []string{lockKey}, gomock.Any()).Return(redis.NewCmdResult(nil, nil)).AnyTimes()
			w.Start()
		})

		It("should not panic if error acquiring lock", func() {
			name := "my-scheduler"
			gameName := "game-name"
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())
			defer func() { w.Run = false }()

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().SetNX("maestro-lock-key-my-scheduler", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, errors.New("some error in lock"))).AnyTimes()

			Expect(func() { go w.Start() }).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			Eventually(func() bool { return hook.LastEntry() != nil }).Should(BeTrue())
			Eventually(func() string { return hook.LastEntry().Message }, 1500*time.Millisecond).Should(Equal("error getting watcher lock"))
		})

		It("should not panic if lock is being used", func() {
			name := "my-scheduler"
			gameName := "game-name"
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())
			defer func() { w.Run = false }()

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().SetNX("maestro-lock-key-my-scheduler", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, nil)).AnyTimes()

			Expect(func() { go w.Start() }).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			Eventually(func() bool { return hook.LastEntry() != nil }).Should(BeTrue())
			Eventually(func() string { return hook.LastEntry().Message }, 1500*time.Millisecond).
				Should(Equal("unable to get watcher my-scheduler lock, maybe some other process has it..."))
		})
	})

	Describe("ReportRoomsStatuses", func() {
		It("Should report all 4 statuses", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})

			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, configYaml1.Name, configYaml1.Game, occupiedTimeout, []*eventforwarder.Info{})
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
				map[string]string{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusCreating,
					"gauge":                         "3",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]string{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusReady,
					"gauge":                         "2",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]string{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusOccupied,
					"gauge":                         "1",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]string{
					reportersConstants.TagGame:      w.GameName,
					reportersConstants.TagScheduler: w.SchedulerName,
					"status":                        models.StatusTerminating,
					"gauge":                         "5",
				},
			)
			mockReporter.EXPECT().Report(
				reportersConstants.EventGruStatus,
				map[string]string{
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
		var configYaml1 models.ConfigYAML

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, configYaml1.Name, configYaml1.Game, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())
		})

		It("should terminate watcher if scheduler is not in database", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				configYaml1.Name,
			).Return(pg.NewTestResult(fmt.Errorf("scheduler not found"), 0), fmt.Errorf("scheduler not found"))

			w.Run = true
			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(w.Run).To(BeFalse())
		})

		It("should scale up and update scheduler state", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(90, 50)

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

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
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateInSync
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 0, configYaml1.AutoScaling.Min - 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

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

		It("should change state and not scale if first state change - subdimensioned", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateInSync
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(90, 50)

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("subdimensioned"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}, mockDb, nil, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should change state and not scale if first state change - overdimensioned", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateInSync
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 90)

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("overdimensioned"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}, mockDb, nil, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should change state and not scale if in-sync but wrong state reported", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateOverdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 50)

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
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now()
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(90, 50)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should do nothing if in cooldown - overdimensioned", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now()
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateOverdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 90)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should warn if scale down is required", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateOverdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithUpLimit
			})

			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// Create rooms (ScaleUp)
			timeoutSec := 1000
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			scheduler := models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
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

			err = controller.ScaleUp(logger, roomManager, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)
			Expect(err).NotTo(HaveOccurred())

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 90)

			// ScaleDown
			scaleDownAmount := configYaml1.AutoScaling.Down.Delta
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
			}

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
			}, mockDb, nil, nil)

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
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateInSync
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 50)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should do nothing if state is creating", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateCreating
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should do nothing if state is terminating", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateTerminating
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should not panic and exit if error retrieving scheduler scaling info", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			}).Return(pg.NewTestResult(nil, 0), errors.New("some cool error in pg"))

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("failed to get scheduler scaling info"))
		})

		It("should not panic and log error if failed to change state info", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateInSync
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(90, 50)

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("subdimensioned"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}, mockDb, errDB, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("failed to update scheduler info"))
		})

		It("should not scale up if half of the points are below threshold", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 8, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().LPush(gomock.Any(), gomock.Any()).Times(2)
			mockPipeline.EXPECT().LTrim(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
			mockPipeline.EXPECT().Exec().Times(2)

			usages := make([]string, w.ScaleUpInfo.Size())
			for idx := range usages {
				if idx < len(usages)/2 {
					usages[idx] = "0.9"
				} else {
					usages[idx] = "0.3"
				}
			}
			mockPipeline.EXPECT().LRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(goredis.NewStringSliceResult(
				usages, nil,
			))

			usages = make([]string, w.ScaleDownInfo.Size())
			for idx := range usages {
				if idx < len(usages)/2 {
					usages[idx] = "0.9"
				} else {
					usages[idx] = "0.3"
				}
			}
			mockPipeline.EXPECT().LRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(goredis.NewStringSliceResult(
				usages, nil,
			))

			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
			}, mockDb, nil, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should scale up if 90% of the points are above threshold", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			w.AutoScalingPeriod = configYaml1.AutoScaling.Up.Trigger.Time / 10
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(90, 50)

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

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
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 50)

			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
			}, mockDb, nil, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should scale down if 90% of the points are above threshold", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateOverdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(50, 90)

			for i := 0; i < 5; i++ {
				pod := &v1.Pod{}
				pod.Name = fmt.Sprintf("room-%d", i)
				pod.Spec.Containers = []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: int32(5000 + i), Name: "TCP"},
					}},
				}
				pod.Status.Phase = v1.PodPending
				_, err := clientset.CoreV1().Pods(configYaml1.Name).Create(pod)
				Expect(err).NotTo(HaveOccurred())
			}

			readyKey := models.GetRoomStatusSetRedisKey(configYaml1.Name, models.StatusReady)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult("room-0", nil))
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for range allStatus {
				mockPipeline.EXPECT().
					SRem(gomock.Any(), gomock.Any())
				mockPipeline.EXPECT().
					ZRem(gomock.Any(), gomock.Any())
			}
			mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any())
			mockPipeline.EXPECT().Del(gomock.Any())
			mockPipeline.EXPECT().Exec()

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
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

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
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithUpLimit
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 7, 3, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

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

		It("should not scale up if min is 0 (scheduler is disabled)", func() {
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithMinZero
			})
			creating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			ready := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			occupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			terminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 0, 0, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(creating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(ready).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(occupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(terminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// check scale infos and if should scale
			buildRedisScaleInfo(0, 0)

			// UpdateScheduler
			testing.MockUpdateSchedulerStatusAndDo(func(_ *models.Scheduler, _ string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
			}, mockDb, nil, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})
	})

	Describe("RemoveDeadRooms", func() {
		var configYaml1 models.ConfigYAML
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
			pod, err := models.NewPod(name, nil, configYaml, clientset, mockRedisClient)
			if err != nil {
				return err
			}
			_, err = pod.Create(clientset)
			return err
		}
		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(
				config, logger, mr, mockDb, redisClient, clientset,
				configYaml1.Name, configYaml1.Game, occupiedTimeout,
				[]*eventforwarder.Info{
					&eventforwarder.Info{
						Plugin:    "plugin",
						Name:      "name",
						Forwarder: mockEventForwarder,
					},
				},
			)
			Expect(w).NotTo(BeNil())
		})

		It("should call controller DeleteRoomsNoPingSince and DeleteRoomsOccupiedTimeout", func() {
			schedulerName := configYaml1.Name
			pKey := models.GetRoomPingRedisKey(schedulerName)
			lKey := models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied)
			ts := time.Now().Unix() - w.Config.GetInt64("pingTimeout")
			createNamespace(schedulerName, clientset)
			// DeleteRoomsNoPingSince
			expectedRooms := []string{"room1", "room2", "room3"}
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				gomock.Any(),
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult(expectedRooms, nil))

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

			for _, roomName := range expectedRooms {
				err := createPod(roomName, schedulerName, clientset)
				Expect(err).NotTo(HaveOccurred())

				room := models.NewRoom(roomName, schedulerName)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(schedulerName, status), room.GetRoomRedisKey()).
						Times(2)
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, status), roomName).Times(2)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), roomName).Times(2)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey()).Times(2)
				mockPipeline.EXPECT().Exec().Times(2)
			}

			mockEventForwarder.EXPECT().Forward(gomock.Any(), models.RoomTerminated, gomock.Any(), gomock.Any()).Do(
				func(ctx context.Context, status string, infos, fwdMetadata map[string]interface{}) {
					Expect(status).To(Equal(models.RoomTerminated))
					Expect(infos["game"]).To(Equal(schedulerName))
					Expect(expectedRooms).To(ContainElement(infos["roomId"]))
				}).Times(len(expectedRooms))
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
					scheduler.Game = schedulerName
				}).Times(8)

			Expect(func() { w.RemoveDeadRooms() }).ShouldNot(Panic())
		})

		It("should log and not panic in case of no ping since error", func() {
			schedulerName := configYaml1.Name
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
			}).Return(redis.NewStringSliceResult([]string{}, errors.New("some error")))

			// DeleteRoomsOccupiedTimeout
			ts = time.Now().Unix() - w.OccupiedTimeout
			expectedRooms := []string{"room1", "room2", "room3"}
			mockRedisClient.EXPECT().ZRangeByScore(
				lKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(ts, 10)},
			).Do(func(key string, zrangeby redis.ZRangeBy) {
				Expect(zrangeby.Min).To(Equal("-inf"))
				max, err := strconv.Atoi(zrangeby.Max)
				Expect(err).NotTo(HaveOccurred())
				Expect(max).To(BeNumerically("~", ts, 1*time.Second))
			}).Return(redis.NewStringSliceResult(expectedRooms, nil))

			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				}).Times(4)

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

			Expect(func() { w.RemoveDeadRooms() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("error listing rooms with no ping since"))
		})

		It("should log and not panic in case of occupied error", func() {
			schedulerName := configYaml1.Name
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

	Describe("EnsureCorrectRooms", func() {
		var configYaml1 models.ConfigYAML
		var w *watcher.Watcher

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				configYaml1.Name,
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			})
			w = watcher.NewWatcher(
				config, logger, mr, mockDb, redisClient, clientset,
				configYaml1.Name, configYaml1.Game, occupiedTimeout,
				[]*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())
		})

		It("should return error if fail to get scheduler", func() {
			testing.MockSelectScheduler(yaml1, mockDb, errDB)
			err := w.EnsureCorrectRooms()
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errDB))
		})

		It("should return error if fail to unmarshal yaml", func() {
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", w.SchedulerName).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(w.SchedulerName, "", "}\":{}")
				}).Return(pg.NewTestResult(nil, 1), nil)

			err := w.EnsureCorrectRooms()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("yaml: did not find expected node content"))
		})

		It("should return error if fail to read rooms from redis", func() {
			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline,
				w.SchedulerName, [][]string{}, errDB)

			err := w.EnsureCorrectRooms()

			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errDB))
		})

		It("should log error and not return if fail to delete pod", func() {
			podNames := []string{"room-1", "room-2"}
			room := models.NewRoom(podNames[0], w.SchedulerName)

			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline,
				w.SchedulerName, [][]string{{room.GetRoomRedisKey()}}, nil)

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
			for _, status := range allStatus {
				mockPipeline.EXPECT().
					SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status),
						room.GetRoomRedisKey())
				mockPipeline.EXPECT().
					ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status),
						room.ID)
			}
			mockPipeline.EXPECT().
				ZRem(models.GetRoomPingRedisKey(w.SchedulerName), room.ID)
			mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
			mockPipeline.EXPECT().Exec().Return(nil, errDB)

			err := w.EnsureCorrectRooms()

			Expect(err).ToNot(HaveOccurred())
			Expect(hook.LastEntry().Message).To(Equal("failed to delete pod"))
		})

		It("should delete invalid pods", func() {
			podNames := []string{"room-1", "room-2"}
			room := models.NewRoom(podNames[0], w.SchedulerName)

			testing.MockSelectScheduler(yaml1, mockDb, nil)
			testing.MockGetRegisteredRooms(mockRedisClient, mockPipeline,
				w.SchedulerName, [][]string{{room.GetRoomRedisKey()}}, nil)

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
			for _, status := range allStatus {
				mockPipeline.EXPECT().
					SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status),
						room.GetRoomRedisKey())
				mockPipeline.EXPECT().
					ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status),
						room.ID)
			}
			mockPipeline.EXPECT().
				ZRem(models.GetRoomPingRedisKey(w.SchedulerName), room.ID)
			mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
			mockPipeline.EXPECT().Exec()

			err := w.EnsureCorrectRooms()

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("PodStatesCount", func() {
		var configYaml models.ConfigYAML
		var mockReporter *reportersMocks.MockReporter

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml)
			Expect(err).NotTo(HaveOccurred())
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
				})
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient,
				clientset, configYaml.Name, configYaml.Game, occupiedTimeout,
				[]*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			r := reporters.GetInstance()
			mockReporter = reportersMocks.NewMockReporter(mockCtrl)
			r.SetReporter("mockReporter", mockReporter)
		})

		AfterEach(func() {
			r := reporters.GetInstance()
			r.UnsetReporter("mockReporter")
		})

		It("should send to statsd reason of pods to restart", func() {
			nPods := 3
			reason := "bug"

			for idx := 1; idx <= nPods; idx++ {
				pod := &v1.Pod{}
				pod.SetName(fmt.Sprintf("pod-%d", idx))
				pod.SetNamespace(w.SchedulerName)
				pod.Status = v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{{
						LastTerminationState: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Reason: reason,
							},
						},
					}},
				}

				_, err := clientset.CoreV1().Pods(w.SchedulerName).Create(pod)
				Expect(err).ToNot(HaveOccurred())
			}

			mockReporter.EXPECT().Report(reportersConstants.EventPodLastStatus, map[string]string{
				reportersConstants.TagGame:      w.GameName,
				reportersConstants.TagScheduler: w.SchedulerName,
				reportersConstants.TagReason:    reason,
				reportersConstants.ValueGauge:   fmt.Sprintf("%d", nPods),
			})

			w.PodStatesCount()
		})
	})
})
