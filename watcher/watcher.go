// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redis "github.com/topfreegames/extensions/redis"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/extensions"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

type gracefulShutdown struct {
	wg      *sync.WaitGroup
	timeout time.Duration
}

// Watcher struct for watcher
type Watcher struct {
	AutoScalingPeriod int
	Config            *viper.Viper
	DB                pginterfaces.DB
	KubernetesClient  kubernetes.Interface
	Logger            logrus.FieldLogger
	MetricsReporter   *models.MixedMetricsReporter
	RedisClient       *redis.Client
	LockKey           string
	LockTimeoutMS     int
	Run               bool
	SchedulerName     string
	GameName          string
	gracefulShutdown  *gracefulShutdown
	OccupiedTimeout   int64
	EventForwarders   []eventforwarder.EventForwarder
	ScaleUpInfo       *models.ScaleInfo
	ScaleDownInfo     *models.ScaleInfo
}

// NewWatcher is the watcher constructor
func NewWatcher(
	config *viper.Viper,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	schedulerName, gameName string,
	occupiedTimeout int64,
	eventForwarders []eventforwarder.EventForwarder,
) *Watcher {
	w := &Watcher{
		Config:           config,
		Logger:           logger,
		DB:               db,
		RedisClient:      redisClient,
		KubernetesClient: clientset,
		MetricsReporter:  mr,
		SchedulerName:    schedulerName,
		GameName:         gameName,
		OccupiedTimeout:  occupiedTimeout,
		EventForwarders:  eventForwarders,
	}
	w.loadConfigurationDefaults()
	w.configure()
	return w
}

func (w *Watcher) loadConfigurationDefaults() {
	w.Config.SetDefault("scaleUpTimeoutSeconds", 300)
	w.Config.SetDefault("watcher.autoScalingPeriod", 10)
	w.Config.SetDefault("watcher.lockKey", "maestro-lock-key")
	w.Config.SetDefault("watcher.lockTimeoutMs", 180000)
	w.Config.SetDefault("watcher.gracefulShutdownTimeout", 300)
	w.Config.SetDefault("pingTimeout", 30)
	w.Config.SetDefault("occupiedTimeout", 60*60)
}

func GetLockKey(prefix, schedulerName string) string {
	return fmt.Sprintf("%s-%s", prefix, schedulerName)
}

func (w *Watcher) configure() error {
	w.AutoScalingPeriod = w.Config.GetInt("watcher.autoScalingPeriod")
	w.LockKey = GetLockKey(w.Config.GetString("watcher.lockKey"), w.SchedulerName)
	w.LockTimeoutMS = w.Config.GetInt("watcher.lockTimeoutMs")
	var wg sync.WaitGroup
	w.gracefulShutdown = &gracefulShutdown{
		wg:      &wg,
		timeout: time.Duration(w.Config.GetInt("watcher.gracefulShutdownTimeout")) * time.Second,
	}

	scheduler := models.NewScheduler(w.SchedulerName, "", "")
	err := w.MetricsReporter.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(w.DB)
	})
	if err != nil {
		return err
	}
	configYaml, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return err
	}
	w.configureLogger()
	w.configureTimeout(configYaml)
	w.configureAutoScale(configYaml)
	return nil
}

func (w *Watcher) configureLogger() {
	w.Logger = w.Logger.WithFields(logrus.Fields{
		"source":    "maestro-watcher",
		"version":   metadata.Version,
		"scheduler": w.SchedulerName,
	})
}

func (w *Watcher) configureTimeout(configYaml *models.ConfigYAML) {
	w.OccupiedTimeout = configYaml.OccupiedTimeout
}

func (w *Watcher) configureAutoScale(configYaml *models.ConfigYAML) {
	var capacity int

	capacity = configYaml.AutoScaling.Up.Trigger.Time / w.AutoScalingPeriod
	if capacity <= 0 {
		capacity = 1
	}
	w.ScaleUpInfo = models.NewScaleInfo(capacity)

	capacity = configYaml.AutoScaling.Down.Trigger.Time / w.AutoScalingPeriod
	if capacity <= 0 {
		capacity = 1
	}
	w.ScaleDownInfo = models.NewScaleInfo(capacity)
}

// Start starts the watcher
func (w *Watcher) Start() {
	l := w.Logger.WithFields(logrus.Fields{
		"operation": "start",
	})
	l.Info("starting watcher")
	w.Run = true
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(w.AutoScalingPeriod) * time.Second)

	for w.Run == true {
		select {
		case <-ticker.C:
			lock, err := w.RedisClient.EnterCriticalSection(w.RedisClient.Client, w.LockKey, time.Duration(w.LockTimeoutMS)*time.Millisecond, 0, 0)
			if lock == nil || err != nil {
				if err != nil {
					l.WithError(err).Error("error getting watcher lock")
				} else if lock == nil {
					l.Warnf("unable to get watcher %s lock, maybe some other process has it...", w.SchedulerName)
				}
			} else if lock.IsLocked() {
				w.RemoveDeadRooms()
				w.AutoScale()
				w.RedisClient.LeaveCriticalSection(lock)
			}
		case sig := <-sigchan:
			l.Warnf("caught signal %v: terminating\n", sig)
			w.Run = false
		}
	}
	extensions.GracefulShutdown(l, w.gracefulShutdown.wg, w.gracefulShutdown.timeout)
}

// RemoveDeadRooms remove rooms that have not sent ping requests for a while
func (w *Watcher) RemoveDeadRooms() {
	w.gracefulShutdown.wg.Add(1)
	defer w.gracefulShutdown.wg.Done()

	since := time.Now().Unix() - w.Config.GetInt64("pingTimeout")
	logger := w.Logger.WithFields(logrus.Fields{
		"executionID": uuid.NewV4().String(),
		"operation":   "removeDeadRooms",
		"since":       since,
	})

	var roomsNoPingSince []string
	err := w.MetricsReporter.WithSegment(models.SegmentZRangeBy, func() error {
		var err error
		roomsNoPingSince, err = models.GetRoomsNoPingSince(w.RedisClient.Client, w.SchedulerName, since)
		return err
	})

	if err != nil {
		logger.WithError(err).Error("error listing rooms with no ping since")
	}

	if roomsNoPingSince != nil && len(roomsNoPingSince) > 0 {
		logger.Info("deleting rooms that are not pinging Maestro")

		for _, roomName := range roomsNoPingSince {
			room := &models.Room{
				ID:            roomName,
				SchedulerName: w.SchedulerName,
			}
			eventforwarder.ForwardRoomEvent(w.EventForwarders, w.DB, w.KubernetesClient, room, models.RoomTerminated, map[string]interface{}{})
		}

		err := controller.DeleteUnavailableRooms(
			logger,
			w.MetricsReporter,
			w.RedisClient.Client,
			w.KubernetesClient,
			w.SchedulerName,
			w.GameName,
			roomsNoPingSince,
			models.ReasonPingTimeout,
		)
		if err != nil {
			logger.WithError(err).Error("error removing dead rooms")
		}
	}

	if w.OccupiedTimeout > 0 {
		logger.Info("deleting rooms that are stuck at occupied status")

		since = time.Now().Unix() - w.OccupiedTimeout
		logger = w.Logger.WithFields(logrus.Fields{
			"executionID": uuid.NewV4().String(),
			"operation":   "removeDeadOccupiedRooms",
			"since":       since,
		})

		var roomsOnOccupiedTimeout []string
		err := w.MetricsReporter.WithSegment(models.SegmentZRangeBy, func() error {
			var err error
			roomsOnOccupiedTimeout, err = models.GetRoomsOccupiedTimeout(w.RedisClient.Client, w.SchedulerName, since)
			return err
		})

		if err != nil {
			logger.WithError(err).Error("error listing rooms with no occupied timeout")
		}

		if roomsOnOccupiedTimeout != nil && len(roomsOnOccupiedTimeout) > 0 {
			for _, roomName := range roomsOnOccupiedTimeout {
				room := &models.Room{
					ID:            roomName,
					SchedulerName: w.SchedulerName,
				}
				eventforwarder.ForwardRoomEvent(w.EventForwarders, w.DB, w.KubernetesClient, room, models.RoomTerminated, map[string]interface{}{})
			}

			err = controller.DeleteUnavailableRooms(
				logger,
				w.MetricsReporter,
				w.RedisClient.Client,
				w.KubernetesClient,
				w.SchedulerName,
				w.GameName,
				roomsOnOccupiedTimeout,
				models.ReasonOccupiedTimeout,
			)
			if err != nil {
				logger.WithError(err).Error("error removing old occupied rooms")
			}
		}
	}
}

func (w *Watcher) updateOccupiedTimeout(scheduler *models.Scheduler) error {
	configYaml, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return err
	}
	w.OccupiedTimeout = configYaml.OccupiedTimeout
	return nil
}

// AutoScale checks if the GRUs state is as expected and scale up or down if necessary
func (w *Watcher) AutoScale() {
	w.gracefulShutdown.wg.Add(1)
	defer w.gracefulShutdown.wg.Done()

	logger := w.Logger.WithFields(logrus.Fields{
		"executionID": uuid.NewV4().String(),
		"operation":   "autoScale",
		"scheduler":   w.SchedulerName,
	})

	scheduler, autoScalingInfo, roomCountByStatus, err := controller.GetSchedulerScalingInfo(
		logger,
		w.MetricsReporter,
		w.DB,
		w.RedisClient.Client,
		w.SchedulerName,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			w.Run = false
			return
		}
		logger.WithError(err).Error("failed to get scheduler scaling info")
		return
	}

	err = w.updateOccupiedTimeout(scheduler)
	if err != nil {
		logger.WithError(err).Error("failed to update scheduler occupied timeout")
		return
	}

	l := logger.WithFields(logrus.Fields{
		"ready":       roomCountByStatus.Ready,
		"creating":    roomCountByStatus.Creating,
		"occupied":    roomCountByStatus.Occupied,
		"terminating": roomCountByStatus.Terminating,
		"state":       scheduler.State,
	})

	err = controller.CreateNamespaceIfNecessary(
		logger,
		w.MetricsReporter,
		w.KubernetesClient,
		scheduler,
	)
	if err != nil {
		logger.WithError(err).Error("failed to create namespace")
		return
	}

	nowTimestamp := time.Now().Unix()
	shouldScaleUp, shouldScaleDown, changedState := w.checkState(
		autoScalingInfo,
		roomCountByStatus,
		scheduler,
		nowTimestamp,
	)

	if shouldScaleUp {
		l.Info("scheduler is subdimensioned, scaling up")
		timeoutSec := w.Config.GetInt("scaleUpTimeoutSeconds")

		delta := autoScalingInfo.Up.Delta
		currentRooms := roomCountByStatus.Creating + roomCountByStatus.Occupied + roomCountByStatus.Ready
		if currentRooms+delta < autoScalingInfo.Min {
			delta = autoScalingInfo.Min - currentRooms
		}

		err = controller.ScaleUp(
			logger,
			w.MetricsReporter,
			w.DB,
			w.RedisClient.Client,
			w.KubernetesClient,
			scheduler,
			delta,
			timeoutSec,
			false,
		)
		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		changedState = true
		if err == nil {
			scheduler.LastScaleOpAt = nowTimestamp
		}
	} else if shouldScaleDown {
		l.Info("scheduler is overdimensioned, should scale down")
		timeoutSec := w.Config.GetInt("scaleDownTimeoutSeconds")
		err = controller.ScaleDown(
			logger,
			w.MetricsReporter,
			w.DB,
			w.RedisClient.Client,
			w.KubernetesClient,
			scheduler,
			autoScalingInfo.Down.Delta,
			timeoutSec,
		)
		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		changedState = true
		if err == nil {
			scheduler.LastScaleOpAt = nowTimestamp
		}
	} else {
		l.Infof("scheduler '%s': state is as expected", scheduler.Name)
	}

	if err != nil {
		logger.WithError(err).Error("error scaling scheduler")
	}

	if changedState {
		err = controller.UpdateScheduler(logger, w.MetricsReporter, w.DB, scheduler)
		if err != nil {
			logger.WithError(err).Error("failed to update scheduler info")
		}
	}
}

func (w *Watcher) checkState(
	autoScalingInfo *models.AutoScaling,
	roomCount *models.RoomsStatusCount,
	scheduler *models.Scheduler,
	nowTimestamp int64,
) (bool, bool, bool) { //shouldScaleUp, shouldScaleDown, changedState
	var shouldScaleUp, shouldScaleDown, changedState bool
	inSync := true

	if scheduler.State == models.StateCreating || scheduler.State == models.StateTerminating {
		return false, false, changedState
	}
	if roomCount.Total() < autoScalingInfo.Min {
		return true, false, changedState
	}

	threshold := autoScalingInfo.Up.Trigger.Threshold
	usage := float32(autoScalingInfo.Up.Trigger.Usage) / 100

	w.ScaleUpInfo.AddPoint(roomCount.Occupied, roomCount.Total(), usage)
	if w.ScaleUpInfo.IsAboveThreshold(threshold) {
		inSync = false
		if scheduler.State != models.StateSubdimensioned {
			scheduler.State = models.StateSubdimensioned
			scheduler.StateLastChangedAt = nowTimestamp
			changedState = true
		} else if nowTimestamp-scheduler.LastScaleOpAt > int64(autoScalingInfo.Up.Cooldown) {
			shouldScaleUp = true
		}
	}

	threshold = autoScalingInfo.Down.Trigger.Threshold
	usage = float32(autoScalingInfo.Down.Trigger.Usage) / 100

	w.ScaleDownInfo.AddPoint(roomCount.Ready, roomCount.Total(), 1-usage)
	if w.ScaleDownInfo.IsAboveThreshold(threshold) && roomCount.Total()-autoScalingInfo.Down.Delta >= autoScalingInfo.Min {
		inSync = false
		if scheduler.State != models.StateOverdimensioned {
			scheduler.State = models.StateOverdimensioned
			scheduler.StateLastChangedAt = nowTimestamp
			changedState = true
		} else if nowTimestamp-scheduler.LastScaleOpAt > int64(autoScalingInfo.Down.Cooldown) {
			shouldScaleDown = true
		}
	}

	if inSync && scheduler.State != models.StateInSync {
		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		changedState = true
	}

	return shouldScaleUp, shouldScaleDown, changedState
}
