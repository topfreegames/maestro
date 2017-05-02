// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redis "github.com/topfreegames/extensions/redis"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

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
	run               bool
	SchedulerName     string
}

// NewWatcher is the watcher constructor
func NewWatcher(
	config *viper.Viper,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	schedulerName string,
) *Watcher {
	w := &Watcher{
		Config:           config,
		Logger:           logger,
		DB:               db,
		RedisClient:      redisClient,
		KubernetesClient: clientset,
		MetricsReporter:  mr,
		SchedulerName:    schedulerName,
	}
	w.loadConfigurationDefaults()
	w.configure()
	w.AutoScalingPeriod = w.Config.GetInt("autoScalingPeriod")
	w.configureLogger()
	return w
}

func (w *Watcher) loadConfigurationDefaults() {
	w.Config.SetDefault("autoScalingPeriod", 10)
	w.Config.SetDefault("watcher.lockKey", "maestro-lock-key")
	w.Config.SetDefault("watcher.lockTimeoutMs", 180000)
}

func (w *Watcher) configure() {
	w.LockKey = w.Config.GetString("watcher.lockKey")
	w.LockTimeoutMS = w.Config.GetInt("watcher.lockTimeoutMs")
}

func (w *Watcher) configureLogger() {
	w.Logger = w.Logger.WithFields(logrus.Fields{
		"source":  "maestro-watcher",
		"version": metadata.Version,
	})
}

// Start starts the watcher
func (w *Watcher) Start() {
	l := w.Logger.WithFields(logrus.Fields{
		"source":  "maestro-watcher",
		"version": metadata.Version,
	})
	w.run = true
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(w.AutoScalingPeriod) * time.Second)

	// TODO better use that buckets algorithm?
	for w.run == true {
		select {
		case <-ticker.C:
			lock, err := w.RedisClient.EnterCriticalSection(w.RedisClient.Client, w.LockKey, time.Duration(w.LockTimeoutMS)*time.Millisecond, 0, 0)
			if lock == nil || err != nil {
				if lock == nil {
					l.Warn("unnable to get watcher lock, maybe some other process has it...")
				} else if err != nil {
					l.Errorf("error getting watcher lock: %s", err.Error())
				}
			} else if lock.IsLocked() {
				w.AutoScale()
				w.RedisClient.LeaveCriticalSection(lock)
			}
		case sig := <-sigchan:
			w.Logger.Warnf("caught signal %v: terminating\n", sig)
			w.run = false
		}
	}
	// TODO: implement graceful shutdown
}

// AutoScale checks if the GRUs state is as expected and scale up or down if necessary
func (w *Watcher) AutoScale() {
	logger := w.Logger.WithFields(logrus.Fields{
		"executionID": uuid.NewV4().String(),
		"operation":   "autoScale",
	})

	// check cooldown
	autoScalingInfo, roomCountByStatus, err := controller.GetSchedulerScalingInfo(
		logger,
		w.MetricsReporter,
		w.DB,
		w.SchedulerName,
	)
	if err != nil {
		logger.WithError(err).Error("Failed to get scheduler scaling info.")
	}

	state, err := controller.GetSchedulerStateInfo(
		logger,
		w.MetricsReporter,
		w.RedisClient.Client,
		w.SchedulerName,
	)
	if err != nil {
		logger.WithError(err).Error("Failed to get scheduler state info.")
	}

	l := logger.WithFields(logrus.Fields{
		"roomsByStatus": roomCountByStatus,
		"state":         state,
	})
	// TODO: we should not try to scale a cluster that is being created, check is somewhere
	// Maybe we should set a cooldown for cluster creation
	shouldScaleUp, shouldScaleDown, changedState := w.checkState(
		autoScalingInfo,
		roomCountByStatus,
		state,
	)
	if changedState {
		err = controller.SaveSchedulerStateInfo(logger, w.MetricsReporter, w.RedisClient.Client, state)
		if err != nil {
			logger.WithError(err).Error("Failed to save scheduler state info.")
		}
	}
	if shouldScaleUp {
		l.Info("Scheduler is subdimensioned, scaling up. ")
		controller.ScaleUp(logger, w.MetricsReporter, w.DB, w.KubernetesClient, w.SchedulerName)
	} else if shouldScaleDown {
		l.Warn("Scheduler is overdimensioned, should scale down.")
	} else {
		l.Info("Scheduler state is as expected. ")
	}
}

func (w *Watcher) checkState(
	autoScalingInfo *models.AutoScaling,
	roomCount *models.RoomsStatusCount,
	state *models.SchedulerState,
) (bool, bool, bool) {
	if roomCount.Total < autoScalingInfo.Min { // this should never happen
		return true, false, false
	}
	if roomCount.Ready/roomCount.Total < 1-(autoScalingInfo.Up.Trigger.Usage/100) {
		if state.State != "subdimensioned" {
			state.State = "subdimensioned"
			state.LastChangedAt = time.Now().Unix()
			return false, false, true
		}
		if time.Now().Unix()-state.LastScaleOpAt > int64(autoScalingInfo.Up.Cooldown) &&
			time.Now().Unix()-state.LastChangedAt > int64(autoScalingInfo.Up.Trigger.Time) {
			return true, false, false
		}
	}

	if roomCount.Ready/roomCount.Total > 1-(autoScalingInfo.Down.Trigger.Usage/100) &&
		roomCount.Total-autoScalingInfo.Down.Delta > autoScalingInfo.Min {
		if state.State != "overdimensioned" {
			state.State = "overdimensioned"
			state.LastChangedAt = time.Now().Unix()
			return false, false, true
		}
		if time.Now().Unix()-state.LastScaleOpAt > int64(autoScalingInfo.Down.Cooldown) &&
			time.Now().Unix()-state.LastChangedAt > int64(autoScalingInfo.Down.Trigger.Time) {
			return false, true, false
		}
	}
	return false, false, false
}
