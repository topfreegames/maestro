// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher_test

import (
	"errors"
	"fmt"
	"time"

	"gopkg.in/pg.v5/types"
	yaml "gopkg.in/yaml.v2"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"
	"github.com/topfreegames/maestro/watcher"
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

var _ = Describe("Watcher", func() {
	Describe("NewWatcher", func() {
		It("should return configured new watcher", func() {
			name := "my-scheduler"
			autoScalingPeriod := 1234
			lockKey := "myLockKey"
			lockTimeoutMs := 1000
			config.Set("autoScalingPeriod", autoScalingPeriod)
			config.Set("watcher.lockKey", lockKey)
			config.Set("watcher.lockTimeoutMs", lockTimeoutMs)
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name)
			Expect(w.AutoScalingPeriod).To(Equal(autoScalingPeriod))
			Expect(w.Config).To(Equal(config))
			Expect(w.DB).To(Equal(mockDb))
			Expect(w.KubernetesClient).To(Equal(clientset))
			Expect(w.Logger).NotTo(BeNil())
			Expect(w.MetricsReporter).To(Equal(mr))
			Expect(w.RedisClient).To(Equal(redisClient))
			Expect(w.LockKey).To(Equal(lockKey))
			Expect(w.LockTimeoutMS).To(Equal(lockTimeoutMs))
			Expect(w.SchedulerName).To(Equal(name))
		})

		It("should return configured new watcher using configuration defaults", func() {
			name := "my-scheduler"
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name)
			Expect(w.AutoScalingPeriod).To(Equal(10))
			Expect(w.LockKey).To(Equal("maestro-lock-key"))
			Expect(w.LockTimeoutMS).To(Equal(180000))
		})
	})

	Describe("Start", func() {
		BeforeEach(func() {
			config.Set("autoScalingPeriod", 1)
		})

		It("should start watcher", func() {
			// Testing here if no scaling needs to be done
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, configYaml1.Name)
			Expect(w).NotTo(BeNil())

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().SetNX("maestro-lock-key", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(true, nil)).AnyTimes()

			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
				scheduler.State = models.StateInSync
			}).AnyTimes()
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{4, 3, 20, 1}
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil)).AnyTimes()
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil)).AnyTimes()
			mockPipeline.EXPECT().Exec().AnyTimes()

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				w.Run = false
			}).Return(&types.Result{}, nil)

			// LeaveCriticalSection (unlock done by redis-lock)
			mockRedisClient.EXPECT().Eval(gomock.Any(), []string{"maestro-lock-key"}, gomock.Any()).Return(redis.NewCmdResult(nil, nil)).AnyTimes()
			w.Start()
		})

		It("should not panic if error acquiring lock", func() {
			name := "my-scheduler"
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name)
			Expect(w).NotTo(BeNil())
			defer func() { w.Run = false }()

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().SetNX("maestro-lock-key", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, errors.New("some error in lock"))).AnyTimes()

			Expect(func() { go w.Start() }).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			Eventually(func() bool { return hook.LastEntry() != nil }, 1500*time.Millisecond).Should(BeTrue())
			Expect(hook.LastEntry().Message).To(Equal("error getting watcher lock"))
		})

		It("should not panic if lock is being used", func() {
			name := "my-scheduler"
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name)
			Expect(w).NotTo(BeNil())
			defer func() { w.Run = false }()

			// EnterCriticalSection (lock done by redis-lock)
			mockRedisClient.EXPECT().SetNX("maestro-lock-key", gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(false, nil)).AnyTimes()

			Expect(func() { go w.Start() }).ShouldNot(Panic())
			Eventually(func() bool { return w.Run }).Should(BeTrue())
			Eventually(func() bool { return hook.LastEntry() != nil }, 1500*time.Millisecond).Should(BeTrue())
			Expect(hook.LastEntry().Message).To(Equal("unable to get watcher lock, maybe some other process has it..."))
		})
	})

	Describe("AutoScale", func() {
		var configYaml1 models.ConfigYAML
		var w *watcher.Watcher

		BeforeEach(func() {
			err := yaml.Unmarshal([]byte(yaml1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())
			w = watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, configYaml1.Name)
			Expect(w).NotTo(BeNil())
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), map[string]interface{}{
				"status":   "creating",
				"lastPing": int64(0),
			}).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
			}).Return(&types.Result{}, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			fmt.Sprintf("%v \n", hook.Entries)
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
		})

		It("should scale if roomCount is less than min", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateInSync
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 0, configYaml1.AutoScaling.Min - 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// ScaleUp
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().HMSet(gomock.Any(), map[string]interface{}{
				"status":   "creating",
				"lastPing": int64(0),
			}).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
			}).Return(&types.Result{}, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("subdimensioned"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}).Return(&types.Result{}, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("overdimensioned"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}).Return(&types.Result{}, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}).Return(&types.Result{}, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
		})

		It("should warn if scale down is required", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Cooldown+1) * time.Second)
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateOverdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yaml1
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 100, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is overdimensioned, should scale down"))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
		})

		It("should do nothing if state is creating", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateCreating
				scheduler.YAML = yaml1
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
		})

		It("should do nothing if state is terminating", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateTerminating
				scheduler.YAML = yaml1
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler state is as expected"))
		})

		It("should not panic and exit if error retrieving scheduler scaling info", func() {
			// GetSchedulerScalingInfo
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yaml1
			}).Return(&types.Result{}, errors.New("some cool error in pg"))

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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 2, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("subdimensioned"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(scheduler.LastScaleOpAt).To(Equal(lastScaleOpAt.Unix()))
			}).Return(&types.Result{}, errors.New("some error in pg"))

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("failed to update scheduler info"))
		})
	})
})
