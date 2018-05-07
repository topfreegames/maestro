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
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	raven "github.com/getsentry/raven-go"
	newrelic "github.com/newrelic/go-agent"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	logininterfaces "github.com/topfreegames/maestro/login/interfaces"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/extensions"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

type gracefulShutdown struct {
	wg      *sync.WaitGroup
	timeout time.Duration
}

//App is our API application
type App struct {
	Address          string
	Config           *viper.Viper
	DB               pginterfaces.DB
	RedisClient      redisinterfaces.RedisClient
	KubernetesClient kubernetes.Interface
	Logger           logrus.FieldLogger
	NewRelic         newrelic.Application
	Router           *mux.Router
	Server           *http.Server
	InCluster        bool
	KubeconfigPath   string
	Login            logininterfaces.Login
	EmailDomains     []string
	Forwarders       []*eventforwarder.Info
	SchedulerCache   *models.SchedulerCache
	gracefulShutdown *gracefulShutdown
}

//NewApp ctor
func NewApp(
	host string,
	port int,
	config *viper.Viper,
	logger logrus.FieldLogger,
	incluster, showProfile bool,
	kubeconfigPath string,
	dbOrNil pginterfaces.DB,
	redisClientOrNil redisinterfaces.RedisClient,
	kubernetesClientOrNil kubernetes.Interface,
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

	err := a.configureApp(dbOrNil, redisClientOrNil, kubernetesClientOrNil, showProfile)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) getRouter(showProfile bool) *mux.Router {
	r := mux.NewRouter()
	r.Handle("/healthcheck", Chain(
		NewHealthcheckHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
	)).Methods("GET").Name("healthcheck")

	if showProfile {
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	r.Handle("/login", Chain(
		NewLoginUrlHandler(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
	)).Methods("GET").Name("oauth")

	r.Handle("/access", Chain(
		NewLoginAccessHandler(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
	)).Methods("GET").Name("oauth")

	r.HandleFunc("/scheduler", Chain(
		NewSchedulerListHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
	).ServeHTTP).Methods("GET").Name("schedulerList")

	r.HandleFunc("/scheduler", Chain(
		NewSchedulerCreateHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewValidationMiddleware(func() interface{} { return &models.ConfigYAML{} }),
	).ServeHTTP).Methods("POST").Name("schedulerCreate")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerUpdateHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewValidationMiddleware(func() interface{} { return &models.ConfigYAML{} }),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerUpdate")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerDeleteHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("DELETE").Name("schedulerDelete")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerStatusHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerStatus")

	r.HandleFunc("/scheduler/{schedulerName}/config", Chain(
		NewGetSchedulerConfigHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerConfigs")

	r.HandleFunc("/scheduler/{schedulerName}/releases", Chain(
		NewGetSchedulerReleasesHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("GET").Name("schedulerConfigs")

	r.HandleFunc("/scheduler/{schedulerName}/diff", Chain(
		NewSchedulerDiffHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewVersionMiddleware(),
		NewValidationMiddleware(func() interface{} { return &models.SchedulersDiff{} }),
	).ServeHTTP).Methods("GET").Name("schedulersDiff")

	r.HandleFunc("/scheduler/{schedulerName}/rollback", Chain(
		NewSchedulerRollbackHandler(a),
		NewLoggingMiddleware(a),
		NewAccessMiddleware(a),
		NewAuthMiddleware(a),
		NewVersionMiddleware(),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerVersion{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerRollback")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerScaleHandler(a),
		NewLoggingMiddleware(a),
		NewBasicAuthMiddleware(a),
		NewAuthMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerScaleParams{} }),
	).ServeHTTP).Methods("POST").Name("schedulerScale")

	r.HandleFunc("/scheduler/{schedulerName}/image", Chain(
		NewSchedulerImageHandler(a),
		NewLoggingMiddleware(a),
		NewBasicAuthMiddleware(a),
		NewAuthMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerImageParams{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerImage")

	r.HandleFunc("/scheduler/{schedulerName}/min", Chain(
		NewSchedulerUpdateMinHandler(a),
		NewLoggingMiddleware(a),
		NewBasicAuthMiddleware(a),
		NewAuthMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.SchedulerMinParams{} }),
	).ServeHTTP).Methods("PUT").Name("schedulerMin")

	r.HandleFunc("/scheduler/{schedulerName}/operations/{operationKey}/status", Chain(
		NewSchedulerOperationHandler(a),
		NewLoggingMiddleware(a),
		NewBasicAuthMiddleware(a),
		NewAuthMiddleware(a),
		NewVersionMiddleware(),
	).ServeHTTP).Methods("GET").Name("schedulersOperationStatus")

	r.HandleFunc("/scheduler/{schedulerName}/operations/{operationKey}/cancel", Chain(
		NewSchedulerOperationCancelHandler(a),
		NewLoggingMiddleware(a),
		NewBasicAuthMiddleware(a),
		NewAuthMiddleware(a),
		NewVersionMiddleware(),
	).ServeHTTP).Methods("PUT").Name("schedulersOperationStatus")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/ping", Chain(
		NewRoomPingHandler(a),
		NewLoggingMiddleware(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomStatusPayload{} }),
	).ServeHTTP).Methods("PUT").Name("ping")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/address", Chain(
		NewRoomAddressHandler(a),
		NewLoggingMiddleware(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
	).ServeHTTP).Methods("GET").Name("address")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/status", Chain(
		NewRoomStatusHandler(a),
		NewLoggingMiddleware(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomStatusPayload{} }),
	).ServeHTTP).Methods("PUT").Name("status")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/roomevent", Chain(
		NewRoomEventHandler(a),
		NewLoggingMiddleware(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomEventPayload{} }),
	).ServeHTTP).Methods("POST").Name("roomEvent")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/playerevent", Chain(
		NewPlayerEventHandler(a),
		NewLoggingMiddleware(a),
		NewResponseTimeMiddleware(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.PlayerEventPayload{} }),
	).ServeHTTP).Methods("POST").Name("playerEvent")

	return r
}

func (a *App) configureApp(dbOrNil pginterfaces.DB, redisClientOrNil redisinterfaces.RedisClient, kubernetesClientOrNil kubernetes.Interface, showProfile bool) error {
	a.loadConfigurationDefaults()
	a.configureLogger()

	a.configureForwarders()
	err := a.configureDatabase(dbOrNil)
	if err != nil {
		return err
	}

	err = a.configureRedisClient(redisClientOrNil)
	if err != nil {
		return err
	}

	err = a.configureKubernetesClient(kubernetesClientOrNil)
	if err != nil {
		return err
	}

	err = a.configureNewRelic()
	if err != nil {
		return err
	}

	a.configureCache()

	a.configureSentry()

	a.configureLogin()

	a.configureServer(showProfile)

	return nil
}

func (a *App) loadConfigurationDefaults() {
	a.Config.SetDefault("scaleUpTimeoutSeconds", 600)
	a.Config.SetDefault("scaleDownTimeoutSeconds", 300)
	a.Config.SetDefault("deleteTimeoutSeconds", 150)
	a.Config.SetDefault("deleteTimeoutSeconds", 150)
	a.Config.SetDefault("watcher.maxSurge", 25)
	a.Config.SetDefault("watcher.maxSurge", 25)
	a.Config.SetDefault("schedulerCache.defaultExpiration", "5m")
	a.Config.SetDefault("schedulerCache.cleanupInterval", "10m")
	a.Config.SetDefault("schedulers.versions.toKeep", 100)
	a.Config.SetDefault("oauth.enabled", true)
	a.Config.SetDefault("forwarders.grpc.matchmaking.timeout", 1*time.Second)
}

func (a *App) configureCache() {
	expirationTime := a.Config.GetDuration("schedulerCache.defaultExpiration")
	cleanupInterval := a.Config.GetDuration("schedulerCache.cleanupInterval")
	a.SchedulerCache = models.NewSchedulerCache(expirationTime, cleanupInterval, a.Logger)
}

func (a *App) configureForwarders() {
	a.Forwarders = eventforwarder.LoadEventForwardersFromConfig(a.Config, a.Logger)
}

func (a *App) configureKubernetesClient(kubernetesClientOrNil kubernetes.Interface) error {
	if kubernetesClientOrNil != nil {
		a.KubernetesClient = kubernetesClientOrNil
		return nil
	}
	clientset, err := extensions.GetKubernetesClient(a.Logger, a.InCluster, a.KubeconfigPath)
	if err != nil {
		return err
	}
	a.KubernetesClient = clientset
	return nil
}

func (a *App) configureDatabase(dbOrNil pginterfaces.DB) error {
	if dbOrNil != nil {
		a.DB = dbOrNil
		return nil
	}
	db, err := extensions.GetDB(a.Logger, a.Config)
	if err != nil {
		return err
	}

	a.DB = db
	return nil
}

func (a *App) configureRedisClient(redisClientOrNil redisinterfaces.RedisClient) error {
	if redisClientOrNil != nil {
		a.RedisClient = redisClientOrNil
		return nil
	}
	redisClient, err := extensions.GetRedisClient(a.Logger, a.Config)
	if err != nil {
		return err
	}
	a.RedisClient = redisClient.Client
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

func (a *App) configureServer(showProfile bool) {
	a.Router = a.getRouter(showProfile)
	a.Server = &http.Server{Addr: a.Address, Handler: wrapHandlerWithResponseWriter(a.Router)}
}

func (a *App) configureLogin() {
	a.Login = login.NewLogin()
	a.Login.Setup()
}

//HandleError writes an error response with message and status
func (a *App) HandleError(w http.ResponseWriter, status int, msg string, err interface{}) {
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

	logger.Info("listening on tcp address")
	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return nil, err
	}

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
