package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/go-extensions-k8s-client-go/kubernetes"
	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func getOperationRollingProgress(
	ctx context.Context, app *App, schedulerName string,
) (float64, string, error) {
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := scheduler.Load(app.DBClient.WithContext(ctx))
	if err != nil {
		return 0, "error getting scheduler for getting progress", err
	}

	scheduler.NextMinorVersion()
	minor := scheduler.Version
	scheduler.NextMajorVersion()
	major := scheduler.Version

	k := kubernetes.TryWithContext(app.KubernetesClient, ctx)
	totalPods, err := k.CoreV1().Pods(schedulerName).List(
		metav1.ListOptions{},
	)
	if err != nil {
		return 0, "error getting getting pods from kubernetes", err
	}
	total := float64(len(totalPods.Items))

	var new float64

	podsMinorVersion, err := k.CoreV1().Pods(
		schedulerName,
	).List(metav1.ListOptions{
		LabelSelector: labels.Set{"version": minor}.String(),
	})
	if err != nil {
		return 0, "error getting getting pods from kubernetes", err
	}
	for _, pod := range podsMinorVersion.Items {
		if models.IsPodReady(&pod) {
			new = new + 1.0
		}
	}

	podsMajorVersion, err := k.CoreV1().Pods(
		schedulerName,
	).List(metav1.ListOptions{
		LabelSelector: labels.Set{"version": major}.String(),
	})
	if err != nil {
		return 0, "error getting getting pods from kubernetes", err
	}
	for _, pod := range podsMajorVersion.Items {
		if models.IsPodReady(&pod) {
			new = new + 1.0
		}
	}

	return 100.0 * new / total, "", nil
}

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
	ctx context.Context, app *App, logger logrus.FieldLogger,
	schedulerName, operationKey string,
) (map[string]string, string, error) {
	var empty map[string]string

	operationManager := models.NewOperationManager(
		schedulerName, app.RedisClient.Trace(ctx), logger,
	)
	status, err := operationManager.Get(operationKey)
	if err != nil {
		return empty, "error accesssing operation key on redis", err
	}

	if status == nil || len(status) == 0 {
		return empty, "scheduler with operation key not found",
			NewOperationNotFoundError()
	}

	if _, ok := status["status"]; ok {
		return status, "", nil
	}

	progress, errorMsg, err := getOperationRollingProgress(ctx, app, schedulerName)
	if err != nil {
		return empty, errorMsg, err
	}
	status["progress"] = fmt.Sprintf("%.2f%%", progress)
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
		r.Context(), g.App, logger, schedulerName, operationKey,
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
		r.Context(), g.App, logger, schedulerName, currOperation,
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
