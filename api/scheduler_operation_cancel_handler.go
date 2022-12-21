package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/v9/middleware"
	"github.com/topfreegames/maestro/models"
)

func cancelOperation(
	ctx context.Context, app *App, logger logrus.FieldLogger,
	schedulerName, operationKey string,
) (string, error) {
	operationManager := models.NewOperationManager(
		schedulerName, app.RedisClient.Trace(ctx), logger,
	)
	mr := metricsReporterFromCtx(ctx)
	err := mr.WithSegment(models.SegmentPipeExec, func() error {
		return operationManager.Cancel(operationKey)
	})
	if err != nil {
		return "error deleting operation key on redis", err
	}

	return "", nil
}

// SchedulerOperationCancelHandler returns the current status on scheduler operation
type SchedulerOperationCancelHandler struct {
	App *App
}

// NewSchedulerOperationCancelHandler returns an instance of SchedulerConfigHandler
func NewSchedulerOperationCancelHandler(a *App) *SchedulerOperationCancelHandler {
	m := &SchedulerOperationCancelHandler{App: a}
	return m
}

func (g *SchedulerOperationCancelHandler) ServeHTTP(
	w http.ResponseWriter, r *http.Request,
) {
	vars := mux.Vars(r)
	schedulerName := vars["schedulerName"]
	operationKey := vars["operationKey"]

	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":       "SchedulerOperationCancelHandler",
		"operation":    "scheduler operation cancel",
		"scheduler":    schedulerName,
		"operationKey": operationKey,
	})

	logger.Info("Starting scheduler operation cancel")

	errorMsg, err := cancelOperation(
		r.Context(), g.App, logger, schedulerName, operationKey,
	)
	if err != nil {
		logger.WithError(err).Error(errorMsg)
		g.App.HandleError(
			w, http.StatusInternalServerError, errorMsg, err,
		)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
	logger.Info("Successfully canceled operation")
}

// SchedulerOperationCancelCurrentHandler returns the current status on
// scheduler operation
type SchedulerOperationCancelCurrentHandler struct {
	App *App
}

// NewSchedulerOperationCancelCurrentHandler returns an instance of
// SchedulerConfigHandler
func NewSchedulerOperationCancelCurrentHandler(
	a *App,
) *SchedulerOperationCancelCurrentHandler {
	m := &SchedulerOperationCancelCurrentHandler{App: a}
	return m
}

func (g *SchedulerOperationCancelCurrentHandler) ServeHTTP(
	w http.ResponseWriter, r *http.Request,
) {
	vars := mux.Vars(r)
	schedulerName := vars["schedulerName"]

	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "SchedulerOperationCancelCurrentHandler",
		"operation": "scheduler operation cancel",
		"scheduler": schedulerName,
	})

	logger.Info("Starting scheduler operation cancel current")

	operationManager := models.NewOperationManager(
		schedulerName, g.App.RedisClient.Trace(r.Context()), logger,
	)
	currOperation, err := operationManager.CurrentOperation()
	if err != nil {
		logger.WithError(err).Error("error getting current operation")
		g.App.HandleError(
			w, http.StatusInternalServerError, "error getting current operation", err,
		)
		return
	}
	if currOperation == "" {
		logger.Info(fmt.Sprintf("No current operation over %s", schedulerName))
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"operating": "false",
		})
		return
	}

	errorMsg, err := cancelOperation(
		r.Context(), g.App, logger, schedulerName, currOperation,
	)
	if err != nil {
		logger.WithError(err).Error(errorMsg)
		g.App.HandleError(
			w, http.StatusInternalServerError, errorMsg, err,
		)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
	logger.Info("Successfully canceled operation")
}
