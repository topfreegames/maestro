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
	Run               bool
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
	w.configureLogger()
	return w
}

func (w *Watcher) loadConfigurationDefaults() {
	w.Config.SetDefault("autoScalingPeriod", 10)
	w.Config.SetDefault("scaleUpTimeout", 300)
	w.Config.SetDefault("watcher.lockKey", "maestro-lock-key")
	w.Config.SetDefault("watcher.lockTimeoutMs", 180000)
}

func (w *Watcher) configure() {
	w.AutoScalingPeriod = w.Config.GetInt("autoScalingPeriod")
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
		"operation": "start",
	})
	w.Run = true
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(w.AutoScalingPeriod) * time.Second)

	// TODO better use that buckets algorithm?
	for w.Run == true {
		select {
		case <-ticker.C:
			lock, err := w.RedisClient.EnterCriticalSection(w.RedisClient.Client, w.LockKey, time.Duration(w.LockTimeoutMS)*time.Millisecond, 0, 0)
			if lock == nil || err != nil {
				if err != nil {
					l.WithError(err).Error("error getting watcher lock")
				} else if lock == nil {
					l.Warn("unable to get watcher lock, maybe some other process has it...")
				}
			} else if lock.IsLocked() {
				w.AutoScale()
				w.RedisClient.LeaveCriticalSection(lock)
			}
		case sig := <-sigchan:
			l.Warnf("caught signal %v: terminating\n", sig)
			w.Run = false
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

	err := controller.CreateNamespaceIfNecessary(
		logger,
		w.MetricsReporter,
		w.KubernetesClient,
		w.SchedulerName,
	)
	if err != nil {
		logger.WithError(err).Error("failed to create namespace")
		return
	}

	scheduler, autoScalingInfo, roomCountByStatus, err := controller.GetSchedulerScalingInfo(
		logger,
		w.MetricsReporter,
		w.DB,
		w.RedisClient.Client,
		w.SchedulerName,
	)
	if err != nil && err.Error() == "pg: no rows in result set" {
		w.Run = false
		return
	}
	if err != nil {
		logger.WithError(err).Error("failed to get scheduler scaling info")
		return
	}

	l := logger.WithFields(logrus.Fields{
		"roomsByStatus": roomCountByStatus,
		"state":         scheduler.State,
	})

	nowTimestamp := time.Now().Unix()
	shouldScaleUp, shouldScaleDown, changedState := w.checkState(
		autoScalingInfo,
		roomCountByStatus,
		scheduler,
		nowTimestamp,
	)

	if shouldScaleUp {
		l.Info("scheduler is subdimensioned, scaling up")
		timeoutSec := w.Config.GetInt("scaleUpTimeout")
		err = controller.ScaleUp(
			logger,
			w.MetricsReporter,
			w.DB,
			w.RedisClient.Client,
			w.KubernetesClient,
			scheduler,
			autoScalingInfo.Up.Delta,
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
		l.Warn("scheduler is overdimensioned, should scale down")
	} else {
		l.Info("scheduler state is as expected")
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
) (bool, bool, bool) {
	inCooldownPeriod := false

	if scheduler.State == models.StateCreating || scheduler.State == models.StateTerminating {
		return false, false, false
	}

	if roomCount.Total() < autoScalingInfo.Min {
		return true, false, false
	}

	if float64(roomCount.Ready)/float64(roomCount.Total()) < 1.0-(float64(autoScalingInfo.Up.Trigger.Usage)/100.0) {
		if scheduler.State != models.StateSubdimensioned {
			scheduler.State = models.StateSubdimensioned
			scheduler.StateLastChangedAt = nowTimestamp
			return false, false, true
		}
		if nowTimestamp-scheduler.LastScaleOpAt > int64(autoScalingInfo.Up.Cooldown) &&
			nowTimestamp-scheduler.StateLastChangedAt > int64(autoScalingInfo.Up.Trigger.Time) {
			return true, false, false
		}
		inCooldownPeriod = true
	}

	if float64(roomCount.Ready)/float64(roomCount.Total()) > 1.0-float64(autoScalingInfo.Down.Trigger.Usage)/100.0 &&
		roomCount.Total()-autoScalingInfo.Down.Delta > autoScalingInfo.Min {
		if scheduler.State != models.StateOverdimensioned {
			scheduler.State = models.StateOverdimensioned
			scheduler.StateLastChangedAt = nowTimestamp
			return false, false, true
		}
		if nowTimestamp-scheduler.LastScaleOpAt > int64(autoScalingInfo.Down.Cooldown) &&
			nowTimestamp-scheduler.StateLastChangedAt > int64(autoScalingInfo.Down.Trigger.Time) {
			return false, true, false
		}
		inCooldownPeriod = true
	}

	if !inCooldownPeriod && scheduler.State != models.StateInSync {
		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		return false, false, true
	}

	return false, false, false
}
