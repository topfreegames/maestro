// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher_test

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/pg.v5/types"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"
	"github.com/topfreegames/maestro/watcher"
)

const (
	yaml1 = `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
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
	occupiedTimeout = 300
)

var _ = Describe("Watcher", func() {
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
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
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
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
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

			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, configYaml1.Name, configYaml1.Game, occupiedTimeout, []*eventforwarder.Info{})
			Expect(w).NotTo(BeNil())

			// EnterCriticalSection (lock done by redis-lock)
			lockKey := fmt.Sprintf("maestro-lock-key-%s", configYaml1.Name)
			mockRedisClient.EXPECT().SetNX(lockKey, gomock.Any(), gomock.Any()).Return(redis.NewBoolResult(true, nil)).AnyTimes()

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
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
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
			w := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, name, gameName, occupiedTimeout, []*eventforwarder.Info{})
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

	Describe("AutoScale", func() {
		var configYaml1 models.ConfigYAML
		var w *watcher.Watcher

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
			).Return(&types.Result{}, errors.New(fmt.Sprintf("scheduler \"%s\" not found")))

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
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Up.Delta * len(configYaml1.Ports))
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
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal("creating"))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Up.Delta * len(configYaml1.Ports))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(scaleUpAmount)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).AnyTimes()
			mockPipeline.EXPECT().Exec().Times(scaleUpAmount)

			err = controller.ScaleUp(logger, mr, mockDb, mockRedisClient, clientset, scheduler, scaleUpAmount, timeoutSec, true)
			Expect(err).NotTo(HaveOccurred())

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

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any()).Times(len(configYaml1.Ports))
				mockPipeline.EXPECT().Exec()
			}

			// UpdateScheduler
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
			}).Return(&types.Result{}, nil)

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
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
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
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
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
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should reset scaleInfo when time is updated to half and become subdimensioned because there is one point of all s.length = 1 above usage", func() {
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			yamlString1 := `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
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
      threshold: 60
      limit: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 100
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlString1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating

			for i := 0; i < 9; i++ {
				w.ScaleUpInfo.AddPoint(10, 10, 10, 0.6)
			}
			for i := 0; i < 1; i++ {
				w.ScaleUpInfo.AddPoint(10, 0, 10, 0.6)
			}

			Expect(w.ScaleUpInfo.IsAboveThreshold(50)).To(BeTrue())
			yamlWithNewTime := `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
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
      time: 50
      threshold: 60
      limit: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 200
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithNewTime
			})

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Up.Delta * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
			}).Return(&types.Result{}, nil)
			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			fmt.Sprintf("%v \n", hook.Entries)
			Expect(w.ScaleUpInfo.IsAboveThreshold(60)).To(BeTrue())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))

			Expect(w.ScaleUpInfo.GetPointer()).To(Equal(1))
			Expect(w.ScaleUpInfo.GetPoints()).To(HaveLen(5))
			Expect(w.ScaleUpInfo.GetPointsAboveUsage()).To(Equal(1))
		})

		It("should reset scaleInfo when time is updated to double and become insync", func() {
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Up.Cooldown+1) * time.Second)
			yamlString1 := `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
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
      threshold: 60
      limit: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 100
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(yamlString1), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating

			for i := 0; i < 9; i++ {
				w.ScaleUpInfo.AddPoint(10, 10, 10, 0.6)
			}
			for i := 0; i < 1; i++ {
				w.ScaleUpInfo.AddPoint(10, 0, 10, 0.6)
			}

			Expect(w.ScaleUpInfo.IsAboveThreshold(50)).To(BeTrue())
			yamlWithNewTime := `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
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
      time: 200
      threshold: 60
      limit: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 100
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithNewTime
			})

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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Up.Delta * len(configYaml1.Ports))
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Up.Delta)

			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
			}).Return(&types.Result{}, nil)
			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			fmt.Sprintf("%v \n", hook.Entries)
			Expect(w.ScaleUpInfo.IsAboveThreshold(60)).To(BeTrue())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler is subdimensioned, scaling up"))

			Expect(w.ScaleUpInfo.GetPointer()).To(Equal(1))
			Expect(w.ScaleUpInfo.GetPoints()).To(HaveLen(20))
			Expect(w.ScaleUpInfo.GetPointsAboveUsage()).To(Equal(1))
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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 8, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

			total := configYaml1.AutoScaling.Up.Trigger.Time/w.AutoScalingPeriod - 1
			usage := float32(configYaml1.AutoScaling.Up.Trigger.Usage) / 100
			capac := configYaml1.AutoScaling.Up.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 50*total {
					w.ScaleUpInfo.AddPoint(capac, 0, 10, usage)
				} else {
					w.ScaleUpInfo.AddPoint(capac, 10, 10, usage)
				}
			}
			total = configYaml1.AutoScaling.Down.Trigger.Time/w.AutoScalingPeriod - 1
			usage = float32(configYaml1.AutoScaling.Down.Trigger.Usage) / 100
			capac = configYaml1.AutoScaling.Down.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 50*total {
					w.ScaleDownInfo.AddPoint(capac, 10, 10, usage)
				} else {
					w.ScaleDownInfo.AddPoint(capac, 0, 10, usage)
				}
			}

			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			)

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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 4, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
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

			total := configYaml1.AutoScaling.Up.Trigger.Time/w.AutoScalingPeriod - 1
			usage := float32(configYaml1.AutoScaling.Up.Trigger.Usage) / 100
			capac := configYaml1.AutoScaling.Up.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 90*total {
					w.ScaleUpInfo.AddPoint(capac, 10, 10, usage)
				} else {
					w.ScaleUpInfo.AddPoint(capac, 0, 10, usage)
				}
			}
			total = configYaml1.AutoScaling.Down.Trigger.Time/w.AutoScalingPeriod - 1
			usage = float32(configYaml1.AutoScaling.Down.Trigger.Usage) / 100
			capac = configYaml1.AutoScaling.Down.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 90*total {
					w.ScaleDownInfo.AddPoint(capac, 0, 10, usage)
				} else {
					w.ScaleDownInfo.AddPoint(capac, 10, 10, usage)
				}
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5002", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5003", nil))
			mockPipeline.EXPECT().Exec().Times(2)

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

			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			)

			total := configYaml1.AutoScaling.Up.Trigger.Time/w.AutoScalingPeriod - 1
			usage := float32(configYaml1.AutoScaling.Up.Trigger.Usage) / 100
			capac := configYaml1.AutoScaling.Up.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 50*total {
					w.ScaleUpInfo.AddPoint(capac, 0, 10, usage)
				} else {
					w.ScaleUpInfo.AddPoint(capac, 10, 10, usage)
				}
			}
			total = configYaml1.AutoScaling.Down.Trigger.Time/w.AutoScalingPeriod - 1
			usage = float32(configYaml1.AutoScaling.Down.Trigger.Usage) / 100
			capac = configYaml1.AutoScaling.Down.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 50*total {
					w.ScaleDownInfo.AddPoint(capac, 10, 10, usage)
				} else {
					w.ScaleDownInfo.AddPoint(capac, 0, 10, usage)
				}
			}

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			Expect(hook.Entries).To(testing.ContainLogMessage("scheduler 'controller-name': state is as expected"))
		})

		It("should scale down if 90% of the points are above threshold", func() {
			// GetSchedulerScalingInfo
			lastChangedAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Trigger.Time+1) * time.Second)
			lastScaleOpAt := time.Now().Add(-1 * time.Duration(configYaml1.AutoScaling.Down.Cooldown+1) * time.Second)
			w.AutoScalingPeriod = configYaml1.AutoScaling.Down.Trigger.Time / 10
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
			expC := &models.RoomsStatusCount{0, 1, 4, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
			mockPipeline.EXPECT().Exec()

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

			total := configYaml1.AutoScaling.Up.Trigger.Time/w.AutoScalingPeriod - 1
			usage := float32(configYaml1.AutoScaling.Up.Trigger.Usage) / 100
			capac := configYaml1.AutoScaling.Up.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 90*total {
					w.ScaleUpInfo.AddPoint(capac, 0, 10, usage)
				} else {
					w.ScaleUpInfo.AddPoint(capac, 10, 10, usage)
				}
			}
			total = configYaml1.AutoScaling.Down.Trigger.Time/w.AutoScalingPeriod - 1
			usage = float32(configYaml1.AutoScaling.Down.Trigger.Usage) / 100
			capac = configYaml1.AutoScaling.Down.Trigger.Time / w.AutoScalingPeriod
			for i := 0; i < total; i++ {
				if 100*i <= 90*total {
					w.ScaleDownInfo.AddPoint(capac, 10, 10, usage)
				} else {
					w.ScaleDownInfo.AddPoint(capac, 0, 10, usage)
				}
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
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any())
			mockPipeline.EXPECT().Exec()

			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Do(func(base *models.Scheduler, query string, scheduler *models.Scheduler) {
				Expect(scheduler.State).To(Equal("in-sync"))
				Expect(scheduler.StateLastChangedAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1*time.Second))
			}).Return(&types.Result{}, nil)

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
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 9, 1, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Up.Delta * len(configYaml1.Ports))
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

		It("should scale up, even if lastScaleOpAt is now, if usage is above 70% (from scheduler)", func() {
			lastChangedAt := time.Now()
			lastScaleOpAt := time.Now()
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithUpLimit
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 7, 3, 0} // creating,occupied,ready,terminating
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(expC.Creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(expC.Ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(expC.Occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(expC.Terminating), nil))
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

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Up.Delta)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil)).Times(configYaml1.AutoScaling.Up.Delta * len(configYaml1.Ports))
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

		It("should not scale up is min is 0 (scheduler is disabled)", func() {
			lastChangedAt := time.Now()
			lastScaleOpAt := time.Now()
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.State = models.StateSubdimensioned
				scheduler.StateLastChangedAt = lastChangedAt.Unix()
				scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
				scheduler.YAML = yamlWithMinZero
			})
			kCreating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating")
			kReady := models.GetRoomStatusSetRedisKey(configYaml1.Name, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(configYaml1.Name, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(configYaml1.Name, "terminating")
			expC := &models.RoomsStatusCount{0, 0, 0, 0} // creating,occupied,ready,terminating
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
				Expect(scheduler.LastScaleOpAt).To(BeNumerically("~", time.Now().Unix(), 1))
			}).Return(&types.Result{}, nil)

			Expect(func() { w.AutoScale() }).ShouldNot(Panic())
			fmt.Sprintf("%v \n", hook.Entries)
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
			pod, err := models.NewPod(
				"game",
				"img",
				name,
				namespace,
				nil,
				nil,
				0,
				[]*models.Port{
					&models.Port{
						ContainerPort: 1234,
						Name:          "port1",
						Protocol:      "UDP",
					}},
				nil,
				nil,
				clientset,
				mockRedisClient,
			)
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
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).Return(redis.NewStringResult("5000", nil))
				mockPipeline.EXPECT().Exec()

				err := createPod(roomName, schedulerName, clientset)
				Expect(err).NotTo(HaveOccurred())

				room := models.NewRoom(roomName, schedulerName)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				for _, status := range allStatus {
					mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, status), room.GetRoomRedisKey()).Times(2)
					mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, status), roomName).Times(2)
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), roomName).Times(2)
				mockPipeline.EXPECT().Del(room.GetRoomRedisKey()).Times(2)
				mockPipeline.EXPECT().Exec().Times(2)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), gomock.Any())
				mockPipeline.EXPECT().Exec()
			}

			mockEventForwarder.EXPECT().Forward(models.RoomTerminated, gomock.Any()).Do(
				func(status string, infos map[string]interface{}) {
					Expect(status).To(Equal(models.RoomTerminated))
					Expect(infos["game"]).To(Equal(schedulerName))
					Expect(expectedRooms).To(ContainElement(infos["roomId"]))
				}).Times(len(expectedRooms))
			mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = yaml1
					scheduler.Game = schedulerName
				}).Times(6)

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
				}).Times(3)

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
})
