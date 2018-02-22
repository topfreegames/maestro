package api

import (
	"errors"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/topfreegames/maestro/models"
)

// SchedulerRollbackHandler handler returns the scheduler config
type SchedulerRollbackHandler struct {
	App *App
}

// NewSchedulerRollbackHandler returns an instance of SchedulerConfigHandler
func NewSchedulerRollbackHandler(a *App) *SchedulerRollbackHandler {
	m := &SchedulerRollbackHandler{App: a}
	return m
}

func (g *SchedulerRollbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	schedulerName := mux.Vars(r)["schedulerName"]
	version := schedulerVersionParamsFromContext(r.Context()).Version

	l := loggerFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":                       "schedulerRollback",
		"operation":                    "scheduler rollback",
		"scheduler":                    schedulerName,
		"schedulerVersionToRollbackTo": version,
	})
	mr := metricsReporterFromCtx(r.Context())

	logger.Info("Starting scheduler rollback")

	yamlStr, err := models.LoadConfigWithVersion(g.App.DB, schedulerName, version)
	if err != nil {
		logger.WithError(err).Error("load scheduler with version error")
		g.App.HandleError(w, http.StatusInternalServerError, "config scheduler failed", err)
		return
	}
	if len(yamlStr) == 0 {
		logger.Error("config for scheduler and version not found")
		g.App.HandleError(w, http.StatusNotFound, "get config error", errors.New("config scheduler not found for version"))
		return
	}

	configYaml, err := models.NewConfigYAML(yamlStr)
	if err != nil {
		logger.WithError(err).Error("error unmarshalling yaml to config")
		g.App.HandleError(w, http.StatusInternalServerError, "config scheduler failed", err)
		return
	}

	updateSchedulerConfigCommon(g.App, w, r, logger, mr, configYaml)
	logger.Info("rollback scheduler release succeeded")
}
