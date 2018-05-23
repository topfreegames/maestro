package api

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
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

func (g *SchedulerRollbackHandler) update(
	r *http.Request,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	configYaml *models.ConfigYAML,
	operationManager *models.OperationManager,
) (status int, description string, err error) {
	status, description, err =
		updateSchedulerConfigCommon(r, g.App, logger, mr, configYaml, operationManager)
	if err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"description": description,
			"status":      status,
		}).Error("error updating scheduler config")
	}
	finishOpErr := mr.WithSegment(models.SegmentPipeExec, func() error {
		return operationManager.Finish(status, description, err)
	})
	if finishOpErr != nil {
		logger.WithError(err).Error("error saving the results on redis")
	}
	return
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

	yamlStr, err := models.LoadConfigWithVersion(
		g.App.DBClient.WithContext(r.Context()),
		schedulerName,
		version,
	)
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

	async := r.URL.Query().Get("async") == "true"
	operationManager, err := getOperationManager(r.Context(), g.App, schedulerName, "SchedulerRollback", logger, mr)
	if returnIfOperationManagerExists(g.App, w, err) {
		logger.WithError(err).Error("Rollback scheduler failed")
		return
	}

	if async {
		go g.update(r, logger, mr, configYaml, operationManager)
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":      true,
			"operationKey": operationManager.GetOperationKey(),
		})
		return
	}

	logger.Info("rollback scheduler release succeeded")
	status, description, err := g.update(r, logger, mr, configYaml, operationManager)
	if err != nil {
		g.App.HandleError(w, status, description, err)
		return
	}
	Write(w, http.StatusOK, `{"success": true}`)
	logger.Info("successfully updated scheduler min")
}
