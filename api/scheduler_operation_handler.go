package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/models"
)

// OperationNotFoundError happens when opManager.Get(key) doesn't
// find an associated operation
type OperationNotFoundError struct{}

// NewOperationNotFoundError ctor
func NewOperationNotFoundError() *OperationNotFoundError {
	return &OperationNotFoundError{}
}

func (e *OperationNotFoundError) Error() string {
	return "scheduler with operation key not found"
}

func getOperationStatus(
	ctx context.Context, app *App, logger *logrus.Entry,
	mr *models.MixedMetricsReporter, schedulerName, operationKey string,
) (map[string]string, string, error) {
	var empty map[string]string

	operationManager := models.NewOperationManager(
		schedulerName, app.RedisClient.Trace(ctx), logger,
	)
	operationManager.SetOperationKey(operationKey)

	scheduler := models.NewScheduler(schedulerName, "", "")
	err := scheduler.Load(app.DBClient.WithContext(ctx))
	if err != nil {
		return empty, "error getting scheduler for progress information", err
	}

	status, err := operationManager.GetOperationStatus(mr, *scheduler)
	if err != nil {
		return status, "error getting operation status", err
	}

	if status == nil {
		status = map[string]string{"progress": "0.00"}
	}

	status["progress"] = fmt.Sprintf("%s%%", status["progress"])
	return status, "", nil
}

// SchedulerOperationHandler returns the current status on scheduler operation
type SchedulerOperationHandler struct {
	App *App
}

// NewSchedulerOperationHandler returns an instance of SchedulerConfigHandler
func NewSchedulerOperationHandler(a *App) *SchedulerOperationHandler {
	m := &SchedulerOperationHandler{App: a}
	return m
}

func (g *SchedulerOperationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mr := metricsReporterFromCtx(r.Context())
	schedulerName := vars["schedulerName"]
	operationKey := vars["operationKey"]

	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":       "SchedulerOperationHandler",
		"operation":    "scheduler operation status",
		"scheduler":    schedulerName,
		"operationKey": operationKey,
	})

	logger.Info("Starting scheduler operation status")

	status, errorMsg, err := getOperationStatus(
		r.Context(), g.App, logger, mr, schedulerName, operationKey,
	)
	if err != nil {
		if _, ok := err.(*OperationNotFoundError); ok {
			logger.Error(errorMsg)
			WriteJSON(w, http.StatusNotFound, map[string]interface{}{
				"success":     false,
				"description": errorMsg,
			})
		} else {
			logger.WithError(err).Error(errorMsg)
			g.App.HandleError(
				w, http.StatusInternalServerError, errorMsg, err,
			)
		}
		return
	}

	bts, _ := json.Marshal(status)
	WriteBytes(w, http.StatusOK, bts)
	logger.Info("Successfully wrote status response")
}

// SchedulerOperationCurrentStatusHandler returns the current status
// on scheduler operation
type SchedulerOperationCurrentStatusHandler struct {
	App *App
}

// NewSchedulerOperationCurrentStatusHandler returns an instance of
// SchedulerConfigHandler
func NewSchedulerOperationCurrentStatusHandler(
	a *App,
) *SchedulerOperationCurrentStatusHandler {
	m := &SchedulerOperationCurrentStatusHandler{App: a}
	return m
}

func (g *SchedulerOperationCurrentStatusHandler) ServeHTTP(
	w http.ResponseWriter, r *http.Request,
) {
	vars := mux.Vars(r)
	mr := metricsReporterFromCtx(r.Context())
	schedulerName := vars["schedulerName"]

	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "SchedulerOperationCurrentStatusHandler",
		"operation": "scheduler operation current status",
		"scheduler": schedulerName,
	})

	logger.Info("Starting scheduler operation status")

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

	status, errorMsg, err := getOperationStatus(
		r.Context(), g.App, logger, mr, schedulerName, currOperation,
	)
	if err != nil {
		if _, ok := err.(*OperationNotFoundError); ok {
			logger.Error(errorMsg)
			WriteJSON(w, http.StatusNotFound, map[string]interface{}{
				"success":     false,
				"description": errorMsg,
			})
		} else {
			logger.WithError(err).Error(errorMsg)
			g.App.HandleError(
				w, http.StatusInternalServerError, errorMsg, err,
			)
		}
		return
	}

	bts, _ := json.Marshal(status)
	WriteBytes(w, http.StatusOK, bts)
	logger.Info("Successfully wrote status response")
}
