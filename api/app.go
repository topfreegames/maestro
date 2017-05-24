// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	e "errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/extensions"
	"github.com/topfreegames/maestro/metadata"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"

	raven "github.com/getsentry/raven-go"
	newrelic "github.com/newrelic/go-agent"
)

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
}

//NewApp ctor
func NewApp(host string, port int, config *viper.Viper, logger logrus.FieldLogger, incluster bool, kubeconfigPath string, dbOrNil pginterfaces.DB, redisClientOrNil redisinterfaces.RedisClient, kubernetesClientOrNil kubernetes.Interface) (*App, error) {
	a := &App{
		Config:         config,
		Address:        fmt.Sprintf("%s:%d", host, port),
		Logger:         logger,
		InCluster:      incluster,
		KubeconfigPath: kubeconfigPath,
	}
	err := a.configureApp(dbOrNil, redisClientOrNil, kubernetesClientOrNil)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) getRouter() *mux.Router {
	r := mux.NewRouter()
	r.Handle("/healthcheck", Chain(
		NewHealthcheckHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
	)).Methods("GET").Name("healthcheck")

	r.HandleFunc("/scheduler", Chain(
		NewSchedulerCreateHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
		NewValidationMiddleware(func() interface{} { return &models.ConfigYAML{} }),
	).ServeHTTP).Methods("POST").Name("schedulerCreate")

	r.HandleFunc("/scheduler/{schedulerName}", Chain(
		NewSchedulerDeleteHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.SchedulerParams{} }),
	).ServeHTTP).Methods("DELETE").Name("schedulerDelete")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/ping", Chain(
		NewRoomPingHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomStatusPayload{} }),
	).ServeHTTP).Methods("PUT").Name("ping")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/address", Chain(
		NewRoomAddressHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
	).ServeHTTP).Methods("GET").Name("address")

	r.HandleFunc("/scheduler/{schedulerName}/rooms/{roomName}/status", Chain(
		NewRoomStatusHandler(a),
		NewMetricsReporterMiddleware(a),
		NewSentryMiddleware(),
		NewNewRelicMiddleware(a),
		NewLoggingMiddleware(a),
		NewVersionMiddleware(),
		NewParamMiddleware(func() interface{} { return &models.RoomParams{} }),
		NewValidationMiddleware(func() interface{} { return &models.RoomStatusPayload{} }),
	).ServeHTTP).Methods("PUT").Name("status")

	return r
}

func (a *App) configureApp(dbOrNil pginterfaces.DB, redisClientOrNil redisinterfaces.RedisClient, kubernetesClientOrNil kubernetes.Interface) error {
	a.loadConfigurationDefaults()
	a.configureLogger()

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

	a.configureSentry()

	a.configureServer()
	return nil
}

func (a *App) loadConfigurationDefaults() {
	a.Config.SetDefault("scaleUpTimeoutSeconds", 300)
	a.Config.SetDefault("scaleDownTimeoutSeconds", 300)
	a.Config.SetDefault("deleteTimeoutSeconds", 150)
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

func (a *App) configureServer() {
	a.Router = a.getRouter()
	a.Server = &http.Server{Addr: a.Address, Handler: wrapHandlerWithResponseWriter(a.Router)}
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
	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return nil, err
	}

	err = a.Server.Serve(listener)
	if err != nil {
		listener.Close()
		return nil, err
	}

	return listener, nil
}
