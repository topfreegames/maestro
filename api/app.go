// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	e "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	goredis "github.com/go-redis/redis"
	"github.com/topfreegames/maestro/api/auth"
	"github.com/topfreegames/maestro/storage"

	raven "github.com/getsentry/raven-go"
	newrelic "github.com/newrelic/go-agent"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/extensions/pg"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/extensions/router"
	logininterfaces "github.com/topfreegames/maestro/login/interfaces"
	storageredis "github.com/topfreegames/maestro/storage/redis"

	"github.com/gorilla/mux"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/extensions"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/william"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"k8s.io/client-go/kubernetes"
	metricsClient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type gracefulShutdown struct {
	wg      *sync.WaitGroup
	timeout time.Duration
}

//App is our API application
type App struct {
	Address                 string
	Config                  *viper.Viper
	DBClient                *pg.Client
	RedisClient             *redis.Client
	KubernetesClient        kubernetes.Interface
	KubernetesMetricsClient metricsClient.Interface
	RoomAddrGetter          models.AddrGetter
	RoomManager             models.RoomManager
	Logger                  logrus.FieldLogger
	NewRelic                newrelic.Application
	Router                  *mux.Router
	Server                  *http.Server
	InCluster               bool
	KubeconfigPath          string
	Login                   logininterfaces.Login
	EmailDomains            []string
	Forwarders              []*eventforwarder.Info
	SchedulerCache          *models.SchedulerCache
	SchedulerEventStorage   storage.SchedulerEventStorage
	William                 *william.WilliamAuth
	gracefulShutdown        *gracefulShutdown
}

//NewApp ctor
func NewApp(
	host string,
	port int,
	config *viper.Viper,
	logger logrus.FieldLogger,
	incluster bool,
	kubeconfigPath string,
	dbOrNil pginterfaces.DB,
	dbCtxOrNil pginterfaces.CtxWrapper,
	redisClientOrNil redisinterfaces.RedisClient,
	redisTraceWrapperOrNil redisinterfaces.TraceWrapper,
	kubernetesClientOrNil kubernetes.Interface,
	metricsClientsetOrNil metricsClient.Interface,
	schedulerEventStorageOrNil storage.SchedulerEventStorage,
) (*App, error) {
	a := &App{
		Config:         config,
		Address:        fmt.Sprintf("%s:%d", host, port),
		Logger:         logger,
		InCluster:      incluster,
		KubeconfigPath: kubeconfigPath,
		EmailDomains:   config.GetStringSlice("oauth.acceptedDomains"),
		Forwarders:     []*eventforwarder.Info{},
	}

	var wg sync.WaitGroup
	a.gracefulShutdown = &gracefulShutdown{
		wg:      &wg,
		timeout: a.Config.GetDuration("api.gracefulShutdownTimeout"),
	}

	err := a.configureApp(
		dbOrNil, dbCtxOrNil,
		redisClientOrNil, redisTraceWrapperOrNil,
		kubernetesClientOrNil,
		metricsClientsetOrNil,
		schedulerEventStorageOrNil,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) getRouter() *mux.Router {
	r := router.NewRouter()
	r.Use(middleware.Version(metadata.Version))
	r.Use(middleware.Logging(a.Logger))

	r.Handle("/healthcheck", Chain(
		NewHealthcheckHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
	)).Methods("GET").Name("healthcheck")

	r.Handle("/login", Chain(
		NewLoginUrlHandler(a),
	)).Methods("GET").Name("oauth")

	r.Handle("/access", Chain(
		NewLoginAccessHandler(a),
	)).Methods("GET").Name("oauth")

	if a.Config.GetBool("william.enabled") {
		r.Handle("/am", NewWilliamHandler(a)).
			Methods("GET").
			Name("william")
	}

	r.HandleFunc("/scheduler", Chain(
		NewSchedulerListHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.GameQueryResolver("ListSchedulers", "game")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
	).ServeHTTP).Methods("GET").Name("schedulerList")

	r.HandleFunc("/scheduler", Chain(
		NewSchedulerCreateHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.ActionResolver("CreateScheduler")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewValidationMiddleware(func() interface{} { return &models.ConfigYAML{} }),
	).ServeHTTP).Methods("POST").Name("schedulerCreate")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerUpdateHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewValidationMiddleware(func() interface{} { return &models.ConfigYAML{} }),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerUpdate")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerDeleteHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("DeleteScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("DELETE").Name("schedulerDelete")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerStatusHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerStatus")

	r.HandleFunc("/scheduler/{schedulerName}/events", Chain(
		NewSchedulerEventHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerEventsParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerEvents")

	r.HandleFunc("/scheduler/{schedulerName}/config", Chain(
		NewGetSchedulerConfigHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerConfigs")

	r.HandleFunc("/scheduler/{schedulerName}/releases", Chain(
		NewGetSchedulerReleasesHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerConfigs")

	r.HandleFunc("/scheduler/{schedulerName}/diff", Chain(
		NewSchedulerDiffHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewValidationMiddleware(func() interface{} { return &models.SchedulersDiff{} }),
	).ServeHTTP).Methods("GET").Name("schedulersDiff")

	r.HandleFunc("/scheduler/{schedulerName}/rollback", Chain(
		NewSchedulerRollbackHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerVersion{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerRollback")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerScaleHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("ScaleScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerScaleParams{} }),
	).ServeHTTP).Methods("POST").Name("schedulerScale")

	r.HandleFunc("/scheduler/{schedulerName}/image", Chain(
		NewSchedulerImageHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerImageParams{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerImage")

	r.HandleFunc("/scheduler/{schedulerName}/min", Chain(
		NewSchedulerUpdateMinHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerMinParams{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerMin")

	r.HandleFunc("/scheduler/{schedulerName}/operations/current/status", Chain(
		NewSchedulerOperationCurrentStatusHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
	).ServeHTTP).Methods("GET").Name("schedulersOperationCurrentStatus")

	r.HandleFunc("/scheduler/{schedulerName}/operations/{operationKey}/status", Chain(
		NewSchedulerOperationHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("GetScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
	).ServeHTTP).Methods("GET").Name("schedulersOperationStatus")

	r.HandleFunc("/scheduler/{schedulerName}/operations/current/cancel", Chain(
		NewSchedulerOperationCancelCurrentHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
	).ServeHTTP).Methods("PUT").Name("schedulersOperationCancelCurrent")

	r.HandleFunc("/scheduler/{schedulerName}/operations/{operationKey}/cancel", Chain(
		NewSchedulerOperationCancelHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
	).ServeHTTP).Methods("PUT").Name("schedulersOperationCancel")

	r.HandleFunc("/scheduler/{schedulerName}/rooms", Chain(
		NewRoomListByMetricHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("roomsByMetric")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomId}", Chain(
		NewRoomDetailsHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerRoomDetailsParams{} }),
	).ServeHTTP).Methods("GET").Name("roomDetails")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/status/{status}", Chain(
		NewRoomListBySchedulerAndStatusHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerRoomsParams{} }),
	).ServeHTTP).Methods("GET").Name("roomsByStatus")

	r.HandleFunc("/scheduler/{schedulerName}/locks", Chain(
		NewSchedulerLocksListHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerLockParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerLocksList")

	r.HandleFunc("/scheduler/{schedulerName}/locks/{lockKey}", Chain(
		NewSchedulerLockDeleteHandler(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a, auth.SchedulerPathResolver("UpdateScheduler", "schedulerName")),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.SchedulerLockParams{} }),
	).ServeHTTP).Methods("DELETE").Name("schedulerLockDelete")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/ping", Chain(
		NewRoomPingHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomStatusPayload{} }),
	).ServeHTTP).Methods("PUT").Name("ping")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/address", Chain(
		NewRoomAddressHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
	).ServeHTTP).Methods("GET").Name("address")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/status", Chain(
		NewRoomStatusHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomStatusPayload{} }),
	).ServeHTTP).Methods("PUT").Name("status")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/roomevent", Chain(
		NewRoomEventHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomEventPayload{} }),
	).ServeHTTP).Methods("POST").Name("roomEvent")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/playerevent", Chain(
		NewPlayerEventHandler(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewDogStatsdMiddleware(a),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.PlayerEventPayload{} }),
	).ServeHTTP).Methods("POST").Name("playerEvent")

	return r
}

func (a *App) configureApp(
	dbOrNil pginterfaces.DB,
	dbCtxOrNil pginterfaces.CtxWrapper,
	redisClientOrNil redisinterfaces.RedisClient,
	redisTraceWrapperOrNil redisinterfaces.TraceWrapper,
	kubernetesClientOrNil kubernetes.Interface,
	metricsClientsetOrNil metricsClient.Interface,
	schedulerEventStorageOrNil storage.SchedulerEventStorage,
) error {
	a.loadConfigurationDefaults()
	a.configureLogger()
	a.configureJaeger()
	a.configureForwarders()
	if err := a.configureDatabase(dbOrNil, dbCtxOrNil); err != nil {
		return err
	}
	if err := a.configureRedisClient(redisClientOrNil, redisTraceWrapperOrNil); err != nil {
		return err
	}
	if err := a.configureKubernetesClient(kubernetesClientOrNil, metricsClientsetOrNil); err != nil {
		return err
	}
	if err := a.configureNewRelic(); err != nil {
		return err
	}
	a.configureCache()
	a.configureSentry()
	a.configureLogin()
	a.configureWilliam()
	a.configureServer()
	a.configureEnvironment()
	a.configureEventStorage(schedulerEventStorageOrNil)
	return nil
}

func (a *App) loadConfigurationDefaults() {
	a.Config.SetDefault("scaleUpTimeoutSeconds", 600)
	a.Config.SetDefault("scaleDownTimeoutSeconds", 300)
	a.Config.SetDefault("deleteTimeoutSeconds", 150)
	a.Config.SetDefault("deleteTimeoutSeconds", 150)
	a.Config.SetDefault("watcher.maxSurge", 25)
	a.Config.SetDefault("schedulerCache.defaultExpiration", "5m")
	a.Config.SetDefault("schedulerCache.cleanupInterval", "10m")
	a.Config.SetDefault("schedulers.versions.toKeep", 100)
	a.Config.SetDefault("basicauth.enabled", true)
	a.Config.SetDefault("oauth.enabled", true)
	a.Config.SetDefault("william.enabled", false)
	a.Config.SetDefault("forwarders.grpc.matchmaking.timeout", 1*time.Second)
	a.Config.SetDefault("api.limitManager.keyTimeout", 1*time.Minute)
	a.Config.SetDefault("jaeger.disabled", false)
	a.Config.SetDefault("jaeger.samplingProbability", 1.0)
	a.Config.SetDefault("addrGetter.cache.use", true)
	a.Config.SetDefault("addrGetter.cache.expirationInterval", "10m")
	a.Config.SetDefault("extensions.kubernetesClient.timeout", "1s")
	a.Config.SetDefault("extensions.kubernetesClient.burst", 300)
	a.Config.SetDefault("extensions.kubernetesClient.qps", 300)
	a.Config.SetDefault(EnvironmentConfig, ProdEnvironment)
}

func (a *App) configureJaeger() {

	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		a.Logger.Error("Could not parse Jaeger env vars: %s", err.Error())
		return
	}

	tracer, _, err := cfg.NewTracer()
	if err != nil {
		a.Logger.Error("Could not initialize jaeger tracer: %s", err.Error())
		return
	}

	opentracing.SetGlobalTracer(tracer)
}

func (a *App) configureCache() {
	expirationTime := a.Config.GetDuration("schedulerCache.defaultExpiration")
	cleanupInterval := a.Config.GetDuration("schedulerCache.cleanupInterval")
	a.SchedulerCache = models.NewSchedulerCache(expirationTime, cleanupInterval, a.Logger)
}

func (a *App) configureEventStorage(schedulerEventStorageOrNil storage.SchedulerEventStorage) {
	if schedulerEventStorageOrNil != nil {
		a.SchedulerEventStorage = schedulerEventStorageOrNil
		return
	}

	a.SchedulerEventStorage = storageredis.NewRedisSchedulerEventStorage(a.RedisClient.Client)
}

func (a *App) configureForwarders() {
	a.Forwarders = eventforwarder.LoadEventForwardersFromConfig(a.Config, a.Logger)
}

func (a *App) configureKubernetesClient(kubernetesClientOrNil kubernetes.Interface, metricsClientsetOrNil metricsClient.Interface) error {
	if kubernetesClientOrNil != nil {
		a.KubernetesClient = kubernetesClientOrNil
		if metricsClientsetOrNil != nil {
			a.KubernetesMetricsClient = metricsClientsetOrNil
		}
		return nil
	}
	clientset, metricsClientset, err := extensions.GetKubernetesClient(a.Logger, a.Config, a.InCluster, a.KubeconfigPath)
	if err != nil {
		return err
	}
	a.KubernetesClient = clientset
	a.KubernetesMetricsClient = metricsClientset
	return nil
}

func (a *App) configureDatabase(dbOrNil pginterfaces.DB, dbCtxOrNil pginterfaces.CtxWrapper) error {
	dbClient, err := extensions.GetDB(a.Logger, a.Config, dbOrNil, dbCtxOrNil)
	if err != nil {
		return err
	}

	a.DBClient = dbClient
	return nil
}

func (a *App) configureRedisClient(redisClientOrNil redisinterfaces.RedisClient, redisTraceWrapperOrNil redisinterfaces.TraceWrapper) error {
	if redisClientOrNil == nil {
		options, err := goredis.ParseURL(a.Config.GetString("extensions.redis.url"))
		if err != nil {
			return err
		}
		options.ReadTimeout = a.Config.GetDuration("extensions.redis.readTimeout")
		options.WriteTimeout = a.Config.GetDuration("extensions.redis.writeTimeout")
		redisClientOrNil = goredis.NewClient(options)
	}

	redisClient, err := extensions.GetRedisClient(a.Logger, a.Config, redisClientOrNil, redisTraceWrapperOrNil)
	if err != nil {
		return err
	}
	a.RedisClient = redisClient
	return nil
}

func (a *App) configureLogger() {
	a.Logger = a.Logger.WithFields(logrus.Fields{
		"source":    "maestro",
		"operation": "initializeApp",
		"version":   metadata.Version,
	})
}

func (a *App) configureSentry() {
	l := a.Logger.WithFields(logrus.Fields{
		"operation": "configureSentry",
	})
	sentryURL := a.Config.GetString("sentry.url")
	l.Debug("Configuring sentry...")
	raven.SetDSN(sentryURL)
	raven.SetRelease(metadata.Version)
}

func (a *App) configureNewRelic() error {
	appName := a.Config.GetString("newrelic.app")
	key := a.Config.GetString("newrelic.key")

	l := a.Logger.WithFields(logrus.Fields{
		"appName":   appName,
		"operation": "configureNewRelic",
	})

	if key == "" {
		l.Warning("New Relic key not found. No data will be sent to New Relic.")
		return nil
	}

	l.Debug("Configuring new relic...")
	config := newrelic.NewConfig(appName, key)
	app, err := newrelic.NewApplication(config)
	if err != nil {
		l.WithError(err).Error("Failed to configure new relic.")
		return err
	}

	l.WithFields(logrus.Fields{
		"key": key,
	}).Info("New Relic configured successfully.")
	a.NewRelic = app
	return nil
}

func (a *App) configureServer() {
	a.Router = a.getRouter()
	a.Server = &http.Server{
		Addr:    a.Address,
		Handler: wrapHandlerWithCors(middleware.UseResponseWriter(a.Router)),
	}
}

func (a *App) configureEnvironment() {
	a.RoomAddrGetter = models.NewRoomAddressesFromHostPort(
		a.Logger,
		a.Config.GetString(Ipv6KubernetesLabelKey),
		a.Config.GetBool("addrGetter.cache.use"),
		a.Config.GetDuration("addrGetter.cache.expirationInterval"),
	)
	a.RoomManager = &models.GameRoom{}

	if a.Config.GetString(EnvironmentConfig) == DevEnvironment {
		a.RoomAddrGetter = models.NewRoomAddressesFromNodePort(
			a.Logger,
			a.Config.GetString(Ipv6KubernetesLabelKey),
			a.Config.GetBool("addrGetter.cache.use"),
			a.Config.GetDuration("addrGetter.cache.expirationInterval"),
		)
		a.RoomManager = &models.GameRoomWithService{}
		a.Logger.Info("development environment")
		return
	}

	a.Logger.Info("production environment")
}

func (a *App) configureLogin() {
	a.Login = login.NewLogin()
	a.Login.Setup()
}

func (a *App) configureWilliam() {
	permissions := []william.Permission{
		{Action: "ListSchedulers", IncludeGame: true, IncludeScheduler: false},
		{Action: "GetScheduler", IncludeGame: true, IncludeScheduler: true},
		{Action: "CreateScheduler", IncludeGame: false, IncludeScheduler: false},
		{Action: "UpdateScheduler", IncludeGame: true, IncludeScheduler: true},
		{Action: "ScaleScheduler", IncludeGame: true, IncludeScheduler: true},
		{Action: "DeleteScheduler", IncludeGame: true, IncludeScheduler: true},
	}
	a.William = william.NewWilliamAuth(a.Config, permissions)
}

//HandleError writes an error response with message and status
func (a *App) HandleError(
	w http.ResponseWriter, status int, msg string, err interface{},
) {
	w.WriteHeader(status)
	var sErr errors.SerializableError
	val, ok := err.(errors.SerializableError)
	if ok {
		sErr = val
	} else {
		sErr = errors.NewGenericError(msg, err.(error))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(sErr.Serialize())

	var rErr error
	errVal, ok := err.(error)
	if ok {
		rErr = errVal
	} else {
		rErr = e.New(msg)
	}
	raven.CaptureError(rErr, nil)
}

//ListenAndServe requests
func (a *App) ListenAndServe() (io.Closer, error) {
	logger := a.Logger.WithField("operation", "ListenAndServe")

	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		logger.WithError(err).Error("failed to listen on tcp address")
		return nil, err
	}
	logger.Info("listening on tcp address")

	idleConnsClosed := make(chan struct{})
	go func() {
		logger.Info("capturing SIGINT and SIGTERM")
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig

		logger.Info("captured termination signal")

		logger.Info("waiting for updates to finish")
		extensions.GracefulShutdown(logger, a.gracefulShutdown.wg, a.gracefulShutdown.timeout)

		if err := a.Server.Shutdown(context.Background()); err != nil {
			logger.Infof("HTTP server Shutdown: %v", err)
		}

		close(idleConnsClosed)
	}()

	err = a.Server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		listener.Close()
		return nil, err
	}

	logger.Info("waiting to gracefully shutdown api")

	logger.Info("waiting for connections to close")
	<-idleConnsClosed

	logger.Info("all connections are closed, shutting down api")

	return listener, nil
}
