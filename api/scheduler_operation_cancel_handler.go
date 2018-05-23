package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/topfreegames/maestro/models"
)

// SchedulerOperationCancelHandler returns the current status on scheduler operation
type SchedulerOperationCancelHandler struct {
	App *App
}

// NewSchedulerOperationCancelHandler returns an instance of SchedulerConfigHandler
func NewSchedulerOperationCancelHandler(a *App) *SchedulerOperationCancelHandler {
	m := &SchedulerOperationCancelHandler{App: a}
	return m
}

func (g *SchedulerOperationCancelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	schedulerName := vars["schedulerName"]
	operationKey := vars["operationKey"]
	mr := metricsReporterFromCtx(r.Context())

	l := loggerFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":       "SchedulerOperationCancelHandler",
		"operation":    "scheduler operation cancel",
		"scheduler":    schedulerName,
		"operationKey": operationKey,
	})

	logger.Info("Starting scheduler operation cancel")

	operationManager := models.NewOperationManager(schedulerName, g.App.RedisClient, logger)
	err := mr.WithSegment(models.SegmentPipeExec, func() error {
		return operationManager.Cancel(operationKey)
	})
	if err != nil {
		logger.WithError(err).Error("error deleting operation key on redis")
		g.App.HandleError(w, http.StatusInternalServerError, "error deleting operation key on redis", err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
	logger.Info("Successfully canceled operation")
}
