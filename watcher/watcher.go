// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher

import (
	"context"
	e "errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/extensions/clock"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redis "github.com/topfreegames/extensions/redis"
	kubernetesExtensions "github.com/topfreegames/go-extensions-k8s-client-go/kubernetes"
	"github.com/topfreegames/maestro/constants"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/autoscaler"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/extensions"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsClient "k8s.io/metrics/pkg/client/clientset/versioned"
)

func createRoomUsages(pods *v1.PodList) ([]*models.RoomUsage, map[string]int) {
	roomUsages := make([]*models.RoomUsage, len(pods.Items))
	roomUsagesIdxMap := make(map[string]int, len(pods.Items))
	for i, pod := range pods.Items {
		roomUsages[i] = &models.RoomUsage{Name: pod.Name, Usage: float64(math.MaxInt64)}
		roomUsagesIdxMap[pod.Name] = i
	}

	return roomUsages, roomUsagesIdxMap
}

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
	KubernetesMetricsClient   metricsClient.Interface
	Logger                    logrus.FieldLogger
	RoomManager               models.RoomManager
	RoomAddrGetter            models.AddrGetter
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
	ScaleInfo                 *models.ScaleInfo
	AutoScaler                *autoscaler.AutoScaler
}

type scaling struct {
	ChangedState, InSync bool
	Delta                int
}

func reportUsage(game, scheduler, metric string, requests, usage int64) error {
	if requests == 0 {
		return e.New("cannot divide by zero")
	}
	gauge := fmt.Sprintf("%.2f", float64(usage)/float64(requests))
	return reporters.Report(reportersConstants.EventGruMetricUsage, map[string]interface{}{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: scheduler,
		reportersConstants.TagMetric:    metric,
		reportersConstants.ValueGauge:   gauge,
	})
}

// NewWatcher is the watcher constructor
func NewWatcher(
	config *viper.Viper,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	metricsClientset metricsClient.Interface,
	schedulerName, gameName string,
	occupiedTimeout int64,
	eventForwarders []*eventforwarder.Info,
) *Watcher {
	w := &Watcher{
		Config:                  config,
		Logger:                  logger,
		DB:                      db,
		RedisClient:             redisClient,
		KubernetesClient:        clientset,
		KubernetesMetricsClient: metricsClientset,
		MetricsReporter:         mr,
		SchedulerName:           schedulerName,
		GameName:                gameName,
		OccupiedTimeout:         occupiedTimeout,
		EventForwarders:         eventForwarders,
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
	w.Config.SetDefault(constants.EnvironmentConfig, constants.ProdEnvironment)
}

func (w *Watcher) configure() error {
	w.AutoScalingPeriod = w.Config.GetInt("watcher.autoScalingPeriod")
	w.RoomsStatusesReportPeriod = w.Config.GetInt("watcher.roomsStatusesReportPeriod")
	w.EnsureCorrectRoomsPeriod = w.Config.GetDuration("watcher.ensureCorrectRoomsPeriod")
	w.PodStatesCountPeriod = w.Config.GetDuration("watcher.podStatesCountPeriod")
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
	w.configureEnvironment()

	w.MetricsReporter.AddReporter(&models.DogStatsdMetricsReporter{
		Scheduler: w.SchedulerName,
		Route:     "watcher",
	})
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
	w.ScaleInfo = models.NewScaleInfo(w.RedisClient.Client)
	w.AutoScaler = autoscaler.NewAutoScaler(w.SchedulerName, w.KubernetesClient, w.KubernetesMetricsClient)
}

func (w *Watcher) configureEnvironment() {
	w.RoomAddrGetter = &models.RoomAddressesFromHostPort{}
	w.RoomManager = &models.GameRoom{}

	if w.Config.GetString(constants.EnvironmentConfig) == constants.DevEnvironment {
		w.RoomAddrGetter = &models.RoomAddressesFromNodePort{}
		w.RoomManager = &models.GameRoomWithService{}
		w.Logger.Info("development environment")
		return
	}

	w.Logger.Info("production environment")
}

// Start starts the watcher
func (w *Watcher) Start() {
	l := w.Logger.WithFields(logrus.Fields{
		"operation": "watcher.Start",
	})
	l.Info("starting watcher")
	w.Run = true
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(w.AutoScalingPeriod) * time.Second)
	defer ticker.Stop()
	tickerEnsure := time.NewTicker(w.EnsureCorrectRoomsPeriod)
	defer tickerEnsure.Stop()
	tickerStateCount := time.NewTicker(w.PodStatesCountPeriod)
	defer tickerStateCount.Stop()

	go w.reportRoomsStatusesRoutine()

	for w.Run == true {
		l = w.Logger.WithFields(logrus.Fields{
			"operation": "watcher.Start",
		})
		select {
		case <-ticker.C:
			w.watchRooms()
		case <-tickerStateCount.C:
			w.PodStatesCount()
		case sig := <-sigchan:
			l.Warnf("caught signal %v: terminating\n", sig)
			w.Run = false
		}
	}
	extensions.GracefulShutdown(l, w.gracefulShutdown.wg, w.gracefulShutdown.timeout)
}

func (w *Watcher) reportRoomsStatusesRoutine() {
	w.gracefulShutdown.wg.Add(1)
	defer w.gracefulShutdown.wg.Done()

	tickerRs := time.NewTicker(time.Duration(w.RoomsStatusesReportPeriod) * time.Second)
	defer tickerRs.Stop()

	for w.Run == true {
		select {
		case <-tickerRs.C:
			w.ReportRoomsStatuses()
		}
	}
}

func (w *Watcher) watchRooms() error {
	l := w.Logger.WithFields(logrus.Fields{
		"operation": "watcher.watchRooms.EnsureCorrectRooms",
	})
	w.WithDownscalingLock(l, w.EnsureCorrectRooms)

	l = w.Logger.WithFields(logrus.Fields{
		"operation": "watcher.watchRooms.RemoveDeadRooms",
	})
	w.WithDownscalingLock(l, w.RemoveDeadRooms)

	l = w.Logger.WithFields(logrus.Fields{
		"operation": "watcher.watchRooms.AutoScale",
	})
	w.WithTerminationLock(l, w.AutoScale)
	w.AddUtilizationMetricsToRedis()
	return nil
}

// AddUtilizationMetricsToRedis store the pods usage metrics (cpu and mem) in a redis sorted set
func (w *Watcher) AddUtilizationMetricsToRedis() error {
	logger := w.Logger.WithFields(logrus.Fields{
		"executionID": uuid.NewV4().String(),
		"operation":   "addUtilizationMetricsToRedis",
		"scheduler":   w.SchedulerName,
	})
	logger.Info("starting to add utilization metrics to redis")

	scheduler, _, _, err := controller.GetSchedulerScalingInfo(
		logger,
		w.MetricsReporter,
		w.DB,
		w.RedisClient.Client,
		w.SchedulerName,
	)

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			w.Run = false
			return err
		}
		logger.WithError(err).Error("failed to get scheduler scaling info")
		return err
	}

	requests := scheduler.GetResourcesRequests()
	sp := scheduler.GetAutoScalingPolicy()
	metricsMap := map[models.AutoScalingPolicyType]bool{}
	metricsTriggers := append(sp.Up.MetricsTrigger, sp.Down.MetricsTrigger...)
	for _, trigger := range metricsTriggers {
		if models.ResourcePolicyType(trigger.Type) {
			metricsMap[trigger.Type] = true
		}
	}

	// If it does not use metricsTriggers we dont need to save metrics
	if len(metricsMap) == 0 {
		return nil
	}

	// Load pods and set their usage to MaxInt64 for all resources
	var pods *v1.PodList
	err = w.MetricsReporter.WithSegment(models.SegmentPod, func() error {
		var err error
		pods, err = w.KubernetesClient.CoreV1().Pods(w.SchedulerName).List(metav1.ListOptions{})
		return err
	})
	if err != nil {
		logger.WithError(err).Error("failed to list pods on namespace")
		return err
	} else if len(pods.Items) == 0 {
		logger.Warn("empty list of pods on namespace")
		return err
	}

	// Load pods metricses
	var pmetricsList *v1beta1.PodMetricsList
	err = w.MetricsReporter.WithSegment(models.SegmentMetrics, func() error {
		var err error
		pmetricsList, err = w.KubernetesMetricsClient.Metrics().PodMetricses(w.SchedulerName).List(metav1.ListOptions{})
		return err
	})
	if err != nil {
		logger.WithError(err).Error("failed to list pods metricses")
	}

	for metric := range metricsMap {
		roomUsages, roomUsagesIdxMap := createRoomUsages(pods)
		if pmetricsList != nil && len(pmetricsList.Items) > 0 {
			for _, pmetrics := range pmetricsList.Items {
				usage := int64(0)
				for _, container := range pmetrics.Containers {
					usage += models.GetResourceUsage(container.Usage, metric)
				}
				roomUsages[roomUsagesIdxMap[pmetrics.Name]].Usage = float64(usage)
				l := logger.WithFields(logrus.Fields{
					"game":         scheduler.Game,
					"name":         scheduler.Name,
					"metric":       string(metric),
					"requests":     requests[metric],
					"usage":        usage,
					"HasReporters": reporters.HasReporters(),
				})
				l.Debug("will report usage")
				err := reportUsage(scheduler.Game, scheduler.Name, string(metric), requests[metric], usage)
				if err != nil {
					l.WithError(err).Debug("failed to report usage")
				}
			}
		}

		scheduler.SavePodsMetricsUtilizationPipeAndExec(
			w.RedisClient.Client,
			w.KubernetesMetricsClient,
			w.MetricsReporter,
			metric,
			roomUsages,
		)

	}
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
		reporters.Report(reportersConstants.EventGruStatus, map[string]interface{}{
			reportersConstants.TagGame:      w.GameName,
			reportersConstants.TagScheduler: w.SchedulerName,
			"status":                        r.Status,
			"gauge":                         r.Gauge,
		})
	}

	return nil
}

// WithLock is a helper function that runs a block of code
// that needs to hold a type of lock to redis
func (w *Watcher) WithLock(l *logrus.Entry, lockKey string, f func() error) (lockErr, err error) {
	lock, err := controller.AcquireLockOnce(
		context.Background(),
		l,
		w.RedisClient,
		w.Config,
		lockKey,
		w.SchedulerName,
	)
	defer controller.ReleaseLock(
		l,
		w.RedisClient,
		lock,
		w.SchedulerName,
	)
	if err != nil {
		return err, nil
	}

	return nil, f()
}

// WithDownscalingLock is a sugar for WithLock for downscaling
func (w *Watcher) WithDownscalingLock(l *logrus.Entry, f func() error) (lockErr, err error) {
	downScalingLockKey := models.GetSchedulerDownScalingLockKey(w.Config.GetString("watcher.lockKey"), w.SchedulerName)
	return w.WithLock(l, downScalingLockKey, f)
}

// WithTerminationLock is a sugar for WithLock for scheduler termination
func (w *Watcher) WithTerminationLock(l *logrus.Entry, f func() error) (lockErr, err error) {
	terminationLockKey := models.GetSchedulerTerminationLockKey(w.Config.GetString("watcher.lockKey"), w.SchedulerName)
	return w.WithLock(l, terminationLockKey, f)
}

// List names of rooms with no ping
func (w *Watcher) roomsWithNoPing(logger *logrus.Entry) ([]string, error) {
	var roomsNoPingSince []string
	since := time.Now().Unix() - w.Config.GetInt64("pingTimeout")
	l := logger.WithFields(logrus.Fields{
		"since": since,
	})

	err := w.MetricsReporter.WithSegment(models.SegmentZRangeBy, func() error {
		var err error
		roomsNoPingSince, err = models.GetRoomsNoPingSince(w.RedisClient.Client, w.SchedulerName, since, w.MetricsReporter)
		return err
	})

	if err != nil {
		l.WithError(err).Error("error listing rooms with no ping since")
	}

	if len(roomsNoPingSince) > 0 {
		l.WithFields(logrus.Fields{
			"quantity": len(roomsNoPingSince),
		}).Info("replacing rooms that are not pinging Maestro")

		err = w.forwardRemovalRoomEvent(logger, roomsNoPingSince)
		if err != nil {
			return nil, err
		}
	}

	return roomsNoPingSince, nil
}

// List names of rooms that expired occupation timeout
func (w *Watcher) roomsWithOccupationTimeout(logger *logrus.Entry) ([]string, error) {
	var roomsOnOccupiedTimeout []string
	since := time.Now().Unix() - w.OccupiedTimeout
	l := logger.WithFields(logrus.Fields{
		"since": since,
	})

	if w.OccupiedTimeout <= 0 {
		return nil, nil
	}

	err := w.MetricsReporter.WithSegment(models.SegmentZRangeBy, func() error {
		var err error
		roomsOnOccupiedTimeout, err = models.GetRoomsOccupiedTimeout(w.RedisClient.Client, w.SchedulerName, since, w.MetricsReporter)
		return err
	})

	if err != nil {
		l.WithError(err).Error("error listing rooms with no occupied timeout")
	}

	if len(roomsOnOccupiedTimeout) > 0 {
		l.WithFields(logrus.Fields{
			"quantity": len(roomsOnOccupiedTimeout),
		}).Info("replacing rooms that expired occupied timeout")

		err = w.forwardRemovalRoomEvent(logger, roomsOnOccupiedTimeout)
		if err != nil {
			return nil, err
		}
	}

	return roomsOnOccupiedTimeout, nil
}

func (w *Watcher) forwardRemovalRoomEvent(logger *logrus.Entry, rooms []string) (err error) {
	// get metadata
	metadatas, err := models.GetRoomsMetadatas(
		w.RedisClient.Client,
		w.SchedulerName, rooms)
	if err != nil {
		logger.WithError(err).Error("failed to get rooms metadata")
		return err
	}

	// forward
	for _, roomName := range rooms {
		room := &models.Room{
			ID:            roomName,
			SchedulerName: w.SchedulerName,
		}

		_, err := eventforwarder.ForwardRoomEvent(
			context.Background(),
			w.EventForwarders, w.RedisClient.Client, w.DB, w.KubernetesClient,
			room, models.RoomTerminated, eventforwarder.PingTimeoutEvent,
			metadatas[roomName], nil, w.Logger, w.RoomAddrGetter)
		if err != nil {
			logger.WithError(err).Error("pingTimeout event forward failed")
		}
	}
	return nil
}

func (w *Watcher) listPods() (pods []v1.Pod, err error) {
	k := w.KubernetesClient
	podList, err := k.CoreV1().Pods(w.SchedulerName).List(
		metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

// filterPodsByName returns a []v1.Pod with pods which names are in podNames
func (w *Watcher) filterPodsByName(logger *logrus.Entry, pods []v1.Pod, podNames []string) (filteredPods []v1.Pod) {
	podNameMap := map[string]bool{}
	for _, name := range podNames {
		podNameMap[name] = true
	}

	for _, pod := range pods {
		if podNameMap[pod.GetName()] {
			filteredPods = append(filteredPods, pod)
		}
	}

	return filteredPods
}

// zombie rooms are the ones that are in terminating state but the pods doesn't exist
func (w *Watcher) removeZombies(pods []v1.Pod, rooms []string) ([]string, error) {
	zombieRooms := []*models.Room{}
	zombieRoomsNames := []string{}

	liveKubePods := map[string]bool{}
	for _, pod := range pods {
		liveKubePods[pod.GetName()] = true
	}
	for _, room := range rooms {
		if _, ok := liveKubePods[room]; !ok {
			zombieRooms = append(zombieRooms, models.NewRoom(room, w.SchedulerName))
			zombieRoomsNames = append(zombieRoomsNames, room)
		}
	}

	return zombieRoomsNames, models.ClearAllMultipleRooms(w.RedisClient.Client, w.MetricsReporter, zombieRooms)
}

// RemoveDeadRooms remove rooms that have not sent ping requests for a while
func (w *Watcher) RemoveDeadRooms() error {
	w.gracefulShutdown.wg.Add(1)
	defer w.gracefulShutdown.wg.Done()

	logger := w.Logger.WithFields(logrus.Fields{
		"executionID": uuid.NewV4().String(),
		"operation":   "watcher.RemoveDeadRooms",
	})

	pods := []v1.Pod{}

	// get rooms with no ping
	roomsNoPingSince, err := w.roomsWithNoPing(logger)
	if err != nil {
		logger.WithError(err).Error("failed to list rooms that are not pinging")
		return err
	}

	// get rooms with occupation timeout
	roomsOnOccupiedTimeout, err := w.roomsWithOccupationTimeout(logger)
	if err != nil {
		logger.WithError(err).Error("failed to list rooms that are on occupied timeout state")
		return err
	}

	// get rooms registered
	rooms, err := models.GetRooms(w.RedisClient.Client, w.SchedulerName, w.MetricsReporter)
	if err != nil {
		logger.WithError(err).Error("error listing registered rooms")
	}

	// append rooms with no ping and on occupation timeout
	// to make sure these keys don't leak
	noPingAndOccupied := append(roomsNoPingSince, roomsOnOccupiedTimeout...)
	rooms = append(rooms, noPingAndOccupied...)

	if len(rooms) > 0 {
		pods, err = w.listPods()
		if err != nil {
			logger.WithError(err).Error("failed to list pods on namespace")
			return err
		}

		// zombie rooms are the ones that are registered but the pods doesn't exist
		roomsRemoved, err := w.removeZombies(pods, rooms)
		if err != nil {
			logger.WithError(err).Error("failed to remove zombie rooms")
			return err
		}

		l := logger.WithFields(logrus.Fields{
			"rooms": fmt.Sprintf("%v", roomsRemoved),
		})
		l.Info("successfully deleted zombie rooms")
	}

	if len(roomsNoPingSince) > 0 || len(roomsOnOccupiedTimeout) > 0 {
		if len(pods) <= 0 {
			pods, err = w.listPods()
			if err != nil {
				logger.WithError(err).Error("failed to list pods on namespace")
				return err
			}
		}
		podsToReplace := w.filterPodsByName(logger, pods, append(roomsNoPingSince, roomsOnOccupiedTimeout...))

		// load scheduler from database
		scheduler := models.NewScheduler(w.SchedulerName, "", "")
		err = scheduler.Load(w.DB)
		if err != nil {
			logger.WithError(err).Error("error accessing db while removing dead rooms")
			return err
		}

		timeoutSec := w.Config.GetInt("updateTimeoutSeconds")
		timeoutDur := time.Duration(timeoutSec) * time.Second
		willTimeoutAt := time.Now().Add(timeoutDur)

		configYAML, err := models.NewConfigYAML(scheduler.YAML)
		if err != nil {
			logger.WithError(err).Error("failed to unmarshal config yaml")
			return err
		}

		timeoutErr, _, err := controller.SegmentAndReplacePods(
			context.Background(),
			logger,
			w.RoomManager,
			w.MetricsReporter,
			w.KubernetesClient,
			w.DB,
			w.RedisClient.Client,
			willTimeoutAt,
			configYAML,
			podsToReplace,
			scheduler,
			nil,
			w.Config.GetInt("watcher.maxSurge"),
			&clock.Clock{},
		)

		if timeoutErr != nil {
			logger.WithError(err).Error("timeout replacing pods on RemoveDeadRooms")
		}

		if err != nil {
			logger.WithError(err).Error("replacing pods returned error on RemoveDeadRooms")
		}

		if timeoutErr == nil && err == nil {
			l := logger.WithFields(logrus.Fields{
				"rooms": fmt.Sprintf("%v", roomsNoPingSince),
			})
			l.Info("successfully deleted rooms that were not pinging")

			l = logger.WithFields(logrus.Fields{
				"rooms": fmt.Sprintf("%v", roomsOnOccupiedTimeout),
			})
			l.Info("successfully deleted rooms that expired occupied timeout")
		}
	}
	logger.Info("finish check of dead rooms")
	return nil
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
func (w *Watcher) AutoScale() error {
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
			return err
		}
		logger.WithError(err).Error("failed to get scheduler scaling info")
		return err
	}

	err = w.updateOccupiedTimeout(scheduler)
	if err != nil {
		logger.WithError(err).Error("failed to update scheduler occupied timeout")
		return err
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
		return err
	}

	nowTimestamp := time.Now().Unix()

	scaling, err := w.checkState(
		autoScalingInfo,
		roomCountByStatus,
		scheduler,
		nowTimestamp,
	)
	if err != nil {
		logger.WithError(err).Error("failed to get scheduler occupancy info")
		return err
	}

	if scaling.Delta > 0 {
		l.Info("scheduler is subdimensioned, scaling up")
		timeoutSec := w.Config.GetInt("scaleUpTimeoutSeconds")

		err = controller.ScaleUp(
			logger,
			w.RoomManager,
			w.MetricsReporter,
			w.DB,
			w.RedisClient.Client,
			w.KubernetesClient,
			scheduler,
			scaling.Delta,
			timeoutSec,
			false,
		)
		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		scaling.ChangedState = true
		if err == nil {
			scheduler.LastScaleOpAt = nowTimestamp
		}
	} else if scaling.Delta < 0 {
		l.Info("scheduler is overdimensioned, should scale down")
		timeoutSec := w.Config.GetInt("scaleDownTimeoutSeconds")

		scaleDown := func() error {
			return controller.ScaleDown(
				logger,
				w.RoomManager,
				w.MetricsReporter,
				w.DB,
				w.RedisClient.Client,
				w.KubernetesClient,
				scheduler,
				-scaling.Delta,
				timeoutSec,
			)
		}
		lockErr, err := w.WithDownscalingLock(l, scaleDown)
		if lockErr != nil {
			l.WithError(err).Info("not able to acquire downScalingLock. Not scaling down")
			return nil
		}

		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		scaling.ChangedState = true
		if err == nil {
			scheduler.LastScaleOpAt = nowTimestamp
		}
	} else {
		l.Infof("scheduler '%s': state is as expected", scheduler.Name)
	}

	if err != nil {
		logger.WithError(err).Error("error scaling scheduler")
	}

	if scaling.ChangedState {
		err = controller.UpdateSchedulerState(logger, w.MetricsReporter, w.DB, scheduler)
		if err != nil {
			logger.WithError(err).Error("failed to update scheduler info")
		}
	}
	return nil
}

func (w *Watcher) checkState(
	autoScalingInfo *models.AutoScaling,
	roomCount *models.RoomsStatusCount,
	scheduler *models.Scheduler,
	nowTimestamp int64,
) (*scaling, error) {
	var err error
	scaling := &scaling{
		Delta:        0,
		ChangedState: false,
		InSync:       true,
	}

	w.transformLegacyInMetricsTrigger(autoScalingInfo)
	w.sendUsages(roomCount, autoScalingInfo)

	// Creating or Terminating state
	if scheduler.State == models.StateCreating || scheduler.State == models.StateTerminating {
		return scaling, nil
	}
	// Rooms below Min
	if roomCount.Available() < autoScalingInfo.Min {
		scaling.Delta = autoScalingInfo.Min - roomCount.Available()
		return scaling, nil
	}
	// Rooms above Max
	if autoScalingInfo.Max > 0 && roomCount.Available() > autoScalingInfo.Max {
		scaling.Delta = autoScalingInfo.Max - roomCount.Available()
		return scaling, nil
	}

	// Up
	if (roomCount.Available() < autoScalingInfo.Max && autoScalingInfo.Max > 0) || autoScalingInfo.Max == 0 {
		scaling, err = w.checkMetricsTrigger(autoScalingInfo, autoScalingInfo.Up, roomCount, scheduler, nowTimestamp)
	}

	// Down
	if scaling.Delta == 0 && scaling.InSync && roomCount.Available() > autoScalingInfo.Min {
		scaling, err = w.checkMetricsTrigger(autoScalingInfo, autoScalingInfo.Down, roomCount, scheduler, nowTimestamp)
	}

	if scaling.InSync && scheduler.State != models.StateInSync {
		scheduler.State = models.StateInSync
		scheduler.StateLastChangedAt = nowTimestamp
		scaling.ChangedState = true
	}

	return scaling, err
}

func (w *Watcher) transformLegacyInMetricsTrigger(autoScalingInfo *models.AutoScaling) {
	// Up
	if len(autoScalingInfo.Up.MetricsTrigger) == 0 {
		autoScalingInfo.Up.MetricsTrigger = append(
			autoScalingInfo.Up.MetricsTrigger,
			&models.ScalingPolicyMetricsTrigger{
				Type:      models.LegacyAutoScalingPolicyType,
				Usage:     autoScalingInfo.Up.Trigger.Usage,
				Limit:     autoScalingInfo.Up.Trigger.Limit,
				Threshold: autoScalingInfo.Up.Trigger.Threshold,
				Time:      autoScalingInfo.Up.Trigger.Time,
				Delta:     autoScalingInfo.Up.Delta,
			},
		)
	}
	// Down
	if len(autoScalingInfo.Down.MetricsTrigger) == 0 {
		autoScalingInfo.Down.MetricsTrigger = append(
			autoScalingInfo.Down.MetricsTrigger,
			&models.ScalingPolicyMetricsTrigger{
				Type:      models.LegacyAutoScalingPolicyType,
				Usage:     autoScalingInfo.Down.Trigger.Usage,
				Limit:     autoScalingInfo.Down.Trigger.Limit,
				Threshold: autoScalingInfo.Down.Trigger.Threshold,
				Time:      autoScalingInfo.Down.Trigger.Time,
				Delta:     -autoScalingInfo.Down.Delta,
			},
		)
	}
}

func (w *Watcher) sendUsages(
	roomCount *models.RoomsStatusCount,
	autoScalingInfo *models.AutoScaling,
) {
	metricTypeMap := map[models.AutoScalingPolicyType]*models.ScalingPolicyMetricsTrigger{}

	// populate metricTypeMap to send usage only one time when the same type is on both up and down
	// and send the trigger with the larger time to store the sufficient amount of points for both triggers

	// up
	for _, trigger := range autoScalingInfo.Up.MetricsTrigger {
		metricTypeMap[trigger.Type] = trigger
	}
	// down
	for _, trigger := range autoScalingInfo.Down.MetricsTrigger {
		if t, ok := metricTypeMap[trigger.Type]; ok {
			if trigger.Time > t.Time {
				metricTypeMap[trigger.Type] = trigger
				continue
			}
		}
		metricTypeMap[trigger.Type] = trigger
	}

	for _, trigger := range metricTypeMap {
		w.ScaleInfo.SendUsage(
			w.SchedulerName, trigger.Type,
			w.AutoScaler.CurrentUtilization(trigger, roomCount),
			int64(w.ScaleInfo.Capacity(trigger.Time, w.AutoScalingPeriod)),
		)
	}
}

func (w *Watcher) checkMetricsTrigger(
	autoScalingInfo *models.AutoScaling,
	scalingPolicy *models.ScalingPolicy,
	roomCount *models.RoomsStatusCount,
	scheduler *models.Scheduler,
	nowTimestamp int64,
) (*scaling, error) {
	isDown := autoScalingInfo.Down == scalingPolicy
	unbalancedState := models.StateSubdimensioned
	scaleType := models.ScaleTypeUp
	scaling := &scaling{
		Delta:        0,
		ChangedState: false,
		InSync:       true,
	}

	if isDown {
		scaleType = models.ScaleTypeDown
		unbalancedState = models.StateOverdimensioned
	}

	for _, trigger := range scalingPolicy.MetricsTrigger {
		threshold := trigger.Threshold
		usage := float32(trigger.Usage) / 100
		capacity := w.ScaleInfo.Capacity(trigger.Time, w.AutoScalingPeriod)
		currentUsage := w.AutoScaler.CurrentUtilization(trigger, roomCount)

		// Limit define a threshold that if surpassed should trigger scale up no matter what.
		if !isDown {
			isAboveLimit := w.checkIfUsageIsAboveLimit(trigger, roomCount, scheduler, scaling, currentUsage, unbalancedState, trigger.Limit, nowTimestamp)

			if isAboveLimit {
				l := w.Logger.WithFields(logrus.Fields{
					"scheduler":    scheduler.Name,
					"currentUsage": currentUsage,
					"targetUsage":  usage,
					"triggerType":  trigger.Type,
					"delta":        scaling.Delta,
				})
				l.Info("Usage is above limit")
				return scaling, nil
			}
		}

		isAboveThreshold, err := w.ScaleInfo.ReturnStatus(
			w.SchedulerName,
			trigger.Type, // distinct
			scaleType,
			capacity, roomCount.Available(),
			threshold, usage,
		)
		if err != nil {
			return scaling, err
		}

		if isAboveThreshold {
			delta := w.AutoScaler.Delta(trigger, roomCount)

			// As Delta() is generic (works for both up and down scheduling)
			// it is necessary to check if the direction of scaling is coherent with
			// the delta signal, as it may occur that values lower than the up trigger usage
			// give us a negative delta or vice-versa.
			//
			// Example:
			// Current usage = 70%
			// Up trigger usage = 75%
			// Down trigger usage = 60%
			//
			// resulting in delta = -1 when checking up trigger
			//
			if (isDown && delta < 0) || (!isDown && delta > 0) {
				scaling.InSync = false
				if scheduler.State != unbalancedState {
					scaling.ChangedState = true
					scheduler.State = unbalancedState
					scheduler.StateLastChangedAt = nowTimestamp
				}

				// not in cooldown window
				if nowTimestamp-scheduler.LastScaleOpAt > int64(scalingPolicy.Cooldown) {
					scaling.Delta = delta
					l := w.Logger.WithFields(logrus.Fields{
						"scheduler":    scheduler.Name,
						"currentUsage": currentUsage,
						"targetUsage":  usage,
						"triggerType":  trigger.Type,
						"delta":        scaling.Delta,
					})
					l.Info("Usage is above threshold")

					return scaling, err
				}
				l := w.Logger.WithFields(logrus.Fields{
					"scheduler":    scheduler.Name,
					"currentUsage": currentUsage,
					"targetUsage":  usage,
					"triggerType":  trigger.Type,
					"delta":        delta,
				})
				l.Info("Still in cooldown period")
			}
		}
	}

	return scaling, nil
}

func (w *Watcher) getInvalidPodsAndPodNames(
	podList *v1.PodList,
	scheduler *models.Scheduler,
) (invalidPods []v1.Pod, invalidPodNames []string, err error) {
	concat := func(pods []v1.Pod, err error) error {
		if err != nil {
			return err
		}
		invalidPods = append(invalidPods, pods...)
		return nil
	}

	err = concat(w.podsOfIncorrectVersion(podList, scheduler))
	if err != nil {
		return nil, nil, err
	}

	err = concat(w.podsNotRegistered(podList))
	if err != nil {
		return nil, nil, err
	}

	if len(invalidPods) > 0 {
		for _, pod := range invalidPods {
			invalidPodNames = append(invalidPodNames, pod.GetName())
		}
	}

	return invalidPods, invalidPodNames, err
}

func (w *Watcher) getOperation(ctx context.Context, logger logrus.FieldLogger) (operationManager *models.OperationManager, err error) {
	operationManager = models.NewOperationManager(
		w.SchedulerName, w.RedisClient.Trace(ctx), logger,
	)

	currentOpKey, _ := operationManager.CurrentOperation()
	if err != nil {
		return nil, err
	}

	operationManager.SetOperationKey(currentOpKey)

	if currentOpKey == "" {
		operationManager = nil
	}

	return operationManager, err
}

func (w *Watcher) rollbackSchedulerVersion(
	ctx context.Context,
	logger logrus.FieldLogger,
	scheduler *models.Scheduler,
	configYAML *models.ConfigYAML,
) {
	oldScheduler, err := models.PreviousVersion(w.DB, scheduler.Name, scheduler.Version)
	if err != nil {
		logger.WithError(err).Error("error to load previous scheduler version to rollback on EnsureCorrectRooms")
	}
	oldConfig, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		logger.WithError(err).Error("failed to unmarshal config yaml to rollback on EnsureCorrectRooms")
	}
	dbRollbackErr := controller.DBRollback(
		ctx,
		logger,
		w.MetricsReporter,
		w.DB,
		w.RedisClient,
		configYAML,
		oldConfig,
		&clock.Clock{},
		scheduler,
		w.Config,
		oldScheduler.Version,
	)
	if dbRollbackErr != nil {
		logger.WithError(dbRollbackErr).Error("error during scheduler database roll back on EnsureCorrectRooms")
	}
}

// EnsureCorrectRooms walks through the pods on the namespace and
// delete those that have incorrect version and those pods that
// are not registered on Maestro
func (w *Watcher) EnsureCorrectRooms() error {
	ctx := context.Background()
	logger := w.Logger.WithField("operation", "EnsureCorrectRooms")
	logger.Info("loading scheduler from database")

	w.gracefulShutdown.wg.Add(1)
	defer w.gracefulShutdown.wg.Done()

	// load scheduler
	scheduler, configYAML, err := controller.LoadScheduler(w.MetricsReporter, w.DB, nil, w.SchedulerName)
	if err != nil {
		logger.WithError(err).Error("failed to load scheduler from database")
		return err
	}

	// list current pods
	k := kubernetesExtensions.TryWithContext(w.KubernetesClient, ctx)
	pods, err := controller.ListCurrentPods(w.MetricsReporter, k, w.SchedulerName)
	if err != nil {
		logger.WithError(err).Error("failed to list pods on namespace")
		return err
	}

	// get invalid pods (wrong versions and pods not registered in redis)
	logger.Info("searching for invalid pods")
	invalidPods, invalidPodNames, err := w.getInvalidPodsAndPodNames(pods, scheduler)
	if err != nil {
		logger.WithError(err).Error("failed to get invalid pods")
		return err
	}

	if len(invalidPods) <= 0 {
		logger.Debug("no invalid pods to replace")
		return nil
	}

	logger.WithField("invalidPods", invalidPodNames).Info("replacing invalid pods")

	// get operation manager if it exists.
	// It won't exist if not in a UpdateSchedulerConfig operation
	operationManager, err := w.getOperation(ctx, logger)
	if err != nil {
		logger.WithError(err).Error("error trying to get current operation from operationManager")
		return err
	}

	// save invalid pods in redis to track rolling update progress
	if operationManager != nil {
		err = models.SetInvalidRooms(w.RedisClient.Trace(ctx), w.MetricsReporter, w.SchedulerName, invalidPodNames)
		if err != nil {
			logger.WithError(err).Error("error trying to save invalid rooms to track progress")
			return err
		}
		err = operationManager.SetDescription(models.OpManagerRollingUpdate)
		if err != nil {
			logger.WithError(err).Error("error trying to set opmanager to rolling update status")
			return err
		}
	}

	// replace invalid pods using rolling strategy
	timeoutSec := w.Config.GetInt("updateTimeoutSeconds")
	timeoutDur := time.Duration(timeoutSec) * time.Second
	willTimeoutAt := time.Now().Add(timeoutDur)
	timeoutErr, _, err := controller.SegmentAndReplacePods(
		ctx,
		logger,
		w.RoomManager,
		w.MetricsReporter,
		w.KubernetesClient,
		w.DB,
		w.RedisClient.Client,
		willTimeoutAt,
		&configYAML,
		invalidPods,
		scheduler,
		operationManager,
		w.Config.GetInt("watcher.maxSurge"),
		&clock.Clock{},
	)

	if err != nil {
		// if operationManager != nil there is a scheduler update in place
		if operationManager != nil {
			logger.WithError(err).Error("replacing pods returned error on EnsureCorrectRooms during update scheduler config")
			operationManager.SetError(err.Error())
		} else {
			logger.WithError(err).Error("replacing pods returned error on EnsureCorrectRooms. Rolling back")
			w.rollbackSchedulerVersion(ctx, logger, scheduler, &configYAML)
		}
	}

	if timeoutErr != nil {
		logger.WithError(err).Error("timeout replacing pods on EnsureCorrectRooms")
		if operationManager != nil {
			operationManager.SetDescription(models.OpManagerTimedout)
		}
	}

	return nil
}

func (w *Watcher) podsNotRegistered(
	pods *v1.PodList,
) ([]v1.Pod, error) {
	registered, err := models.GetAllRegisteredRooms(w.RedisClient.Client,
		w.SchedulerName)
	if err != nil {
		return nil, err
	}

	notRegistered := []v1.Pod{}
	for _, pod := range pods.Items {
		if _, ok := registered[pod.Name]; !ok {
			notRegistered = append(notRegistered, pod)
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
) ([]v1.Pod, error) {
	incorrectPods := []v1.Pod{}

	schedulerMajorVersion, _, err := w.splitedVersion(scheduler.Version)
	if err != nil {
		return nil, err
	}

	schedulerMajorRollbackVersion := -1
	if scheduler.RollbackVersion != "" {
		schedulerMajorRollbackVersion, _, err = w.splitedVersion(scheduler.RollbackVersion)
		if err != nil {
			return nil, err
		}
	}

	for _, pod := range pods.Items {
		podMajorVersion, _, err := w.splitedVersion(pod.Labels["version"])
		if err != nil {
			return nil, err
		}

		// if a rollback happened consider 2 versions as correct:
		// actual version(after DB rollback) and the version the rolled-back-to version (the one before corrupt version)
		if podMajorVersion != schedulerMajorVersion && podMajorVersion != schedulerMajorRollbackVersion {
			incorrectPods = append(incorrectPods, pod)
		}
	}
	return incorrectPods, nil
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
	k := kubernetesExtensions.TryWithContext(w.KubernetesClient, context.Background())
	pods, err := k.CoreV1().Pods(w.SchedulerName).List(metav1.ListOptions{})
	if err != nil {
		logger.WithError(err).Error("failed to list pods")
		return
	}

	restartCount := map[string]int{}
	stateCount := map[v1.PodPhase]int{
		v1.PodPending:   0,
		v1.PodRunning:   0,
		v1.PodSucceeded: 0,
		v1.PodFailed:    0,
		v1.PodUnknown:   0,
	}

	for _, pod := range pods.Items {
		stateCount[pod.Status.Phase]++
		for _, status := range pod.Status.ContainerStatuses {
			logger.Debugf("termination state: %+v", status)
			if hasTerminationState(&status) {
				reason := status.LastTerminationState.Terminated.Reason
				restartCount[reason] = restartCount[reason] + 1
			}
		}
	}

	logger.Debug("reporting to statsd")

	for state, count := range stateCount {
		logger.Debugf("sending pods status to statsd: {%s:%d}", state, count)
		reporters.Report(reportersConstants.EventPodStatus, map[string]interface{}{
			reportersConstants.TagPodStatus: state,
			reportersConstants.TagGame:      w.GameName,
			reportersConstants.TagScheduler: w.SchedulerName,
			reportersConstants.ValueGauge:   fmt.Sprintf("%d", count),
		})
	}

	for reason, count := range restartCount {
		logger.Debugf("sending result to statsd: {%s:%d}", reason, count)
		reporters.Report(reportersConstants.EventPodLastStatus, map[string]interface{}{
			reportersConstants.TagGame:      w.GameName,
			reportersConstants.TagScheduler: w.SchedulerName,
			reportersConstants.TagReason:    reason,
			reportersConstants.ValueGauge:   fmt.Sprintf("%d", count),
		})
	}

	return
}

func (w *Watcher) checkIfUsageIsAboveLimit(
	trigger interface{},
	roomCount *models.RoomsStatusCount,
	scheduler *models.Scheduler,
	scaling *scaling,
	currentUsage float32,
	unbalancedState string,
	limit int,
	nowTimestamp int64,
) bool { // isAboveLimit
	var limitUsage float32
	triggerObj := trigger.(*models.ScalingPolicyMetricsTrigger)
	limitUsage = float32(limit) / 100

	if currentUsage >= limitUsage {
		w.Logger.Debug("Usage is above limit. Should scale up")
		scheduler.State = unbalancedState
		scheduler.StateLastChangedAt = nowTimestamp
		scaling.InSync = false
		scaling.ChangedState = true
		scaling.Delta = w.AutoScaler.Delta(triggerObj, roomCount)
		return true
	}
	return false
}
