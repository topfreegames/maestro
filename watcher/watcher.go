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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	uuid "github.com/satori/go.uuid"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redis "github.com/topfreegames/extensions/redis"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/extensions"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type gracefulShutdown struct {
	wg      *sync.WaitGroup
	timeout time.Duration
}

// Watcher struct for watcher
type Watcher struct {
	AutoScalingPeriod         int
	RoomsStatusesReportPeriod int
	EnsureCorrectRoomsPeriod  time.Duration
	PodStatesCountPeriod      time.Duration
	Config                    *viper.Viper
	DB                        pginterfaces.DB
	KubernetesClient          kubernetes.Interface
	Logger                    logrus.FieldLogger
	MetricsReporter           *models.MixedMetricsReporter
	RedisClient               *redis.Client
	LockKey                   string
	LockTimeoutMS             int
	Run                       bool
	SchedulerName             string
	GameName                  string
	gracefulShutdown          *gracefulShutdown
	OccupiedTimeout           int64
	EventForwarders           []*eventforwarder.Info
	ScaleUpInfo               *models.ScaleInfo
	ScaleDownInfo             *models.ScaleInfo
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
	eventForwarders []*eventforwarder.Info,
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
	w.Config.SetDefault("watcher.roomsStatusesReportPeriod", 10)
	w.Config.SetDefault("watcher.ensureCorrectRoomsPeriod", 10*time.Minute)
	w.Config.SetDefault("watcher.podStatesCountPeriod", 1*time.Minute)
	w.Config.SetDefault("watcher.lockKey", "maestro-lock-key")
	w.Config.SetDefault("watcher.lockTimeoutMs", 180000)
	w.Config.SetDefault("watcher.gracefulShutdownTimeout", 300)
	w.Config.SetDefault("pingTimeout", 30)
	w.Config.SetDefault("occupiedTimeout", 60*60)
}

func (w *Watcher) configure() error {
	w.AutoScalingPeriod = w.Config.GetInt("watcher.autoScalingPeriod")
	w.RoomsStatusesReportPeriod = w.Config.GetInt("watcher.roomsStatusesReportPeriod")
	w.EnsureCorrectRoomsPeriod = w.Config.GetDuration("watcher.ensureCorrectRoomsPeriod")
	w.PodStatesCountPeriod = w.Config.GetDuration("watcher.podStatesCountPeriod")
	w.LockKey = controller.GetLockKey(w.Config.GetString("watcher.lockKey"), w.SchedulerName)
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
	w.ScaleUpInfo = models.NewScaleUpInfo(capacity, w.RedisClient.Client)

	capacity = configYaml.AutoScaling.Down.Trigger.Time / w.AutoScalingPeriod
	if capacity <= 0 {
		capacity = 1
	}
	w.ScaleDownInfo = models.NewScaleDownInfo(capacity, w.RedisClient.Client)
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
	defer ticker.Stop()
	tickerRs := time.NewTicker(time.Duration(w.RoomsStatusesReportPeriod) * time.Second)
	defer tickerRs.Stop()
	tickerEnsure := time.NewTicker(w.EnsureCorrectRoomsPeriod)
	defer tickerEnsure.Stop()
	tickerStateCount := time.NewTicker(w.PodStatesCountPeriod)
	defer tickerStateCount.Stop()

	for w.Run == true {
		select {
		case <-ticker.C:
			w.WithRedisLock(l, w.watchRooms)
		case <-tickerRs.C:
			w.WithRedisLock(l, w.ReportRoomsStatuses)
		case <-tickerEnsure.C:
			w.WithRedisLock(l, w.EnsureCorrectRooms)
		case <-tickerStateCount.C:
			w.PodStatesCount()
		case sig := <-sigchan:
			l.Warnf("caught signal %v: terminating\n", sig)
			w.Run = false
		}
	}
	extensions.GracefulShutdown(l, w.gracefulShutdown.wg, w.gracefulShutdown.timeout)
}

func (w *Watcher) watchRooms() error {
	w.RemoveDeadRooms()
	w.AutoScale()
	return nil
}

// ReportRoomsStatuses runs as a block of code inside WithRedisLock
// inside a timer tick in w.Start()
func (w *Watcher) ReportRoomsStatuses() error {
	if !reporters.HasReporters() {
		return nil
	}

	var roomCountByStatus *models.RoomsStatusCount
	err := w.MetricsReporter.WithSegment(models.SegmentGroupBy, func() error {
		var err error
		roomCountByStatus, err = models.GetRoomsCountByStatus(w.RedisClient.Client, w.SchedulerName)
		return err
	})

	if err != nil {
		return err
	}

	type RoomData struct {
		Status string
		Gauge  string
	}

	roomDataSlice := []RoomData{
		RoomData{
			models.StatusCreating,
			fmt.Sprint(roomCountByStatus.Creating),
		},
		RoomData{
			models.StatusReady,
			fmt.Sprint(roomCountByStatus.Ready),
		},
		RoomData{
			models.StatusOccupied,
			fmt.Sprint(roomCountByStatus.Occupied),
		},
		RoomData{
			models.StatusTerminating,
			fmt.Sprint(roomCountByStatus.Terminating),
		},
		RoomData{
			models.StatusReadyOrOccupied,
			fmt.Sprint(roomCountByStatus.Ready + roomCountByStatus.Occupied),
		},
	}

	for _, r := range roomDataSlice {
		reporters.Report(reportersConstants.EventGruStatus, map[string]string{
			reportersConstants.TagGame:      w.GameName,
			reportersConstants.TagScheduler: w.SchedulerName,
			"status":                        r.Status,
			"gauge":                         r.Gauge,
		})
	}

	return nil
}

// WithRedisLock is a helper function that runs a block of code
// that needs to hold a lock to redis
func (w *Watcher) WithRedisLock(l *logrus.Entry, f func() error) {
	lock, err := w.RedisClient.EnterCriticalSection(
		w.RedisClient.Client, w.LockKey,
		time.Duration(w.LockTimeoutMS)*time.Millisecond, 0, 0,
	)
	if lock == nil || err != nil {
		if err != nil {
			l.WithError(err).Error("error getting watcher lock")
		} else if lock == nil {
			l.Warnf("unable to get watcher %s lock, maybe some other process has it...", w.SchedulerName)
		}
	} else if lock.IsLocked() {
		err = f()
		if err != nil {
			l.WithError(err).Error("WithRedisLock block function failed")
		}
		w.RedisClient.LeaveCriticalSection(lock)
	}
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
		logger.WithFields(logrus.Fields{
			"quantity": len(roomsNoPingSince),
		}).Info("deleting rooms that are not pinging Maestro")

		for _, roomName := range roomsNoPingSince {
			room := &models.Room{
				ID:            roomName,
				SchedulerName: w.SchedulerName,
			}
			_, err := eventforwarder.ForwardRoomEvent(w.EventForwarders, w.DB, w.KubernetesClient, room,
				models.RoomTerminated, map[string]interface{}{}, nil, w.Logger)
			if err != nil {
				logger.WithError(err).Error("event forwarder failed")
			}
		}

		logger.WithFields(logrus.Fields{
			"rooms": fmt.Sprintf("%v", roomsNoPingSince),
		}).Info("rooms that are not pinging")

		scheduler := models.NewScheduler(w.SchedulerName, "", "")
		err := scheduler.Load(w.DB)
		if err != nil {
			logger.WithError(err).Error("error accessing db while removing dead rooms")
		} else {
			err := controller.DeleteUnavailableRooms(
				logger,
				w.MetricsReporter,
				w.RedisClient.Client,
				w.KubernetesClient,
				scheduler,
				roomsNoPingSince,
				reportersConstants.ReasonPingTimeout,
			)
			if err != nil {
				logger.WithError(err).Error("error removing dead rooms")
			} else {
				logger.WithFields(logrus.Fields{
					"rooms": fmt.Sprintf("%v", roomsNoPingSince),
				}).Info("successfully deleted rooms that were not pinging")
			}
		}

	}

	if w.OccupiedTimeout > 0 {
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
			logger.Info("deleting rooms that are stuck at occupied status")

			for _, roomName := range roomsOnOccupiedTimeout {
				room := &models.Room{
					ID:            roomName,
					SchedulerName: w.SchedulerName,
				}
				eventforwarder.ForwardRoomEvent(w.EventForwarders, w.DB, w.KubernetesClient, room,
					models.RoomTerminated, map[string]interface{}{}, nil, w.Logger)
			}

			scheduler := models.NewScheduler(w.SchedulerName, "", "")
			err := scheduler.Load(w.DB)
			if err != nil {
				logger.WithError(err).Error("error accessing db while removing occupied timeout rooms")
			} else {
				err = controller.DeleteUnavailableRooms(
					logger,
					w.MetricsReporter,
					w.RedisClient.Client,
					w.KubernetesClient,
					scheduler,
					roomsOnOccupiedTimeout,
					reportersConstants.ReasonOccupiedTimeout,
				)
				if err != nil {
					logger.WithError(err).Error("error removing old occupied rooms")
				}
			}
		}
	}

	logger.Info("finish check of dead rooms")
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
	logger.Info("starting auto scale")

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
	shouldScaleUp, shouldScaleDown, changedState, err := w.checkState(
		autoScalingInfo,
		roomCountByStatus,
		scheduler,
		nowTimestamp,
	)
	if err != nil {
		logger.WithError(err).Error("failed to get scheduler occupancy info")
		return
	}

	if shouldScaleUp {
		l.Info("scheduler is subdimensioned, scaling up")
		timeoutSec := w.Config.GetInt("scaleUpTimeoutSeconds")

		delta := autoScalingInfo.Up.Delta
		currentRooms := roomCountByStatus.Available()
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

		delta := autoScalingInfo.Down.Delta
		currentRooms := roomCountByStatus.Available()
		if currentRooms-delta < autoScalingInfo.Min {
			delta = currentRooms - autoScalingInfo.Min
		}

		err = controller.ScaleDown(
			logger,
			w.MetricsReporter,
			w.DB,
			w.RedisClient.Client,
			w.KubernetesClient,
			scheduler,
			delta,
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
) (bool, bool, bool, error) { //shouldScaleUp, shouldScaleDown, changedState
	var shouldScaleUp, shouldScaleDown, changedState bool
	inSync := true

	if scheduler.State == models.StateCreating || scheduler.State == models.StateTerminating {
		return false, false, changedState, nil
	}
	if roomCount.Total() < autoScalingInfo.Min {
		return true, false, changedState, nil
	}
	if autoScalingInfo.Min > 0 && 100*roomCount.Occupied >= roomCount.Total()*autoScalingInfo.Up.Trigger.Limit {
		return true, false, changedState, nil
	}

	threshold := autoScalingInfo.Up.Trigger.Threshold
	usage := float32(autoScalingInfo.Up.Trigger.Usage) / 100
	capacity := autoScalingInfo.Up.Trigger.Time / w.AutoScalingPeriod

	isAboveThreshold, err := w.ScaleUpInfo.SendUsageAndReturnStatus(
		w.SchedulerName,
		capacity, roomCount.Occupied, roomCount.Total(),
		threshold, usage,
	)
	if err != nil {
		return false, false, false, err
	}
	if isAboveThreshold {
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
	capacity = autoScalingInfo.Down.Trigger.Time / w.AutoScalingPeriod

	isAboveThreshold, err = w.ScaleDownInfo.SendUsageAndReturnStatus(
		w.SchedulerName,
		capacity, roomCount.Ready, roomCount.Total(),
		threshold, 1-usage,
	)
	if err != nil {
		return false, false, false, err
	}
	// If total>min, should scale down. But if total-scaleDownDelta < min, scaleDownDelta must be
	// equal to total-min
	if isAboveThreshold && roomCount.Available() > autoScalingInfo.Min {
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

	return shouldScaleUp, shouldScaleDown, changedState, nil
}

// EnsureCorrectRooms walks through the pods on the namespace and
// delete those that have incorrect version and those pods that
// are not registered on Maestro
func (w *Watcher) EnsureCorrectRooms() error {
	w.gracefulShutdown.wg.Add(1)
	defer w.gracefulShutdown.wg.Done()

	logger := w.Logger.WithField("operation", "EnsureCorrectRooms")
	logger.Info("loading scheduler from database")

	scheduler := models.NewScheduler(w.SchedulerName, "", "")
	err := scheduler.Load(w.DB)
	if err != nil {
		logger.WithError(err).Error("failed to load scheduler from database")
		return err
	}

	configYaml, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		logger.WithError(err).Error("failed to unmarshal config yaml")
		return err
	}

	pods, err := w.KubernetesClient.CoreV1().Pods(w.SchedulerName).List(
		metav1.ListOptions{})
	if err != nil {
		logger.WithError(err).Error("failed to list pods on namespace")
		return err
	}

	podsToDelete := []string{}
	concat := func(podNames []string, err error) error {
		if err != nil {
			return err
		}
		podsToDelete = append(podsToDelete, podNames...)
		return nil
	}

	logger.Info("searching for invalid pods")

	err = concat(w.podsOfIncorrectVersion(pods, scheduler))
	if err != nil {
		return err
	}

	err = concat(w.podsNotRegistered(pods))
	if err != nil {
		return err
	}

	if len(podsToDelete) > 0 {
		logger.WithField("podsToDelete", podsToDelete).Info("deleting invalid pods")
	} else {
		logger.Info("no invalid pods to delete")
	}

	for _, podName := range podsToDelete {
		err = controller.DeletePodAndRoom(logger, w.MetricsReporter,
			w.KubernetesClient, w.RedisClient.Client, configYaml,
			podName, reportersConstants.ReasonInvalidPod)
		if err != nil {
			logger.WithError(err).WithField("podName", podName).Error("failed to delete pod")
		}
	}

	return nil
}

func (w *Watcher) podsNotRegistered(
	pods *v1.PodList,
) ([]string, error) {
	registered, err := models.GetAllRegisteredRooms(w.RedisClient.Client,
		w.SchedulerName)
	if err != nil {
		return nil, err
	}

	notRegistered := []string{}
	for _, pod := range pods.Items {
		if _, ok := registered[pod.Name]; !ok {
			notRegistered = append(notRegistered, pod.Name)
		}
	}
	return notRegistered, nil
}

func (w *Watcher) splitedVersion(version string) (majorInt, minorInt int, err error) {
	splitted := strings.Split(strings.TrimPrefix(version, "v"), ".")
	major, minor := splitted[0], "0"
	if len(splitted) > 1 {
		minor = splitted[1]
	}

	minorInt, err = strconv.Atoi(minor)
	if err != nil {
		return
	}
	majorInt, err = strconv.Atoi(major)
	if err != nil {
		return
	}

	return majorInt, minorInt, nil
}

func (w *Watcher) podsOfIncorrectVersion(
	pods *v1.PodList,
	scheduler *models.Scheduler,
) ([]string, error) {
	podNames := []string{}
	for _, pod := range pods.Items {
		podMajorVersion, _, err := w.splitedVersion(pod.Labels["version"])
		if err != nil {
			return nil, err
		}

		schedulerMajorVersion, _, err := w.splitedVersion(scheduler.Version)
		if err != nil {
			return nil, err
		}

		if podMajorVersion != schedulerMajorVersion {
			podNames = append(podNames, pod.GetName())
		}
	}
	return podNames, nil
}

func hasTerminationState(status *v1.ContainerStatus) bool {
	state := status.LastTerminationState
	return state.Terminated != nil && state.Terminated.Reason != ""
}

// PodStatesCount sends metrics of pod states to statsd
func (w *Watcher) PodStatesCount() {
	if !reporters.HasReporters() {
		return
	}

	logger := w.Logger.WithField("method", "PodStatesCount")

	logger.Info("listing pods on namespace")
	pods, err := w.KubernetesClient.CoreV1().Pods(w.SchedulerName).List(metav1.ListOptions{})
	if err != nil {
		logger.WithError(err).Error("failed to list pods")
		return
	}

	restartCount := map[string]int{}

	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			logger.Debugf("termination state: %+v", status)
			if hasTerminationState(&status) {
				reason := status.LastTerminationState.Terminated.Reason
				restartCount[reason] = restartCount[reason] + 1
			}
		}
	}

	logger.Debug("reporting to statsd")

	for reason, count := range restartCount {
		logger.Debugf("sending result to statsd: {%s:%d}", reason, count)
		reporters.Report(reportersConstants.EventPodLastStatus, map[string]string{
			reportersConstants.TagGame:      w.GameName,
			reportersConstants.TagScheduler: w.SchedulerName,
			reportersConstants.TagReason:    reason,
			reportersConstants.ValueGauge:   fmt.Sprintf("%d", count),
		})
	}

	return
}
