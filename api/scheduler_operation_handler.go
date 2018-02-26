package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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

	l := loggerFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":       "SchedulerOperationHandler",
		"operation":    "scheduler operation status",
		"scheduler":    schedulerName,
		"operationKey": operationKey,
	})

	logger.Info("Starting scheduler operation status")

	operationManager := models.NewOperationManager(schedulerName, g.App.RedisClient, logger)
	status, err := operationManager.Get(operationKey)
	if err != nil {
		logger.WithError(err).Error("error accesssing operation key on redis")
		g.App.HandleError(w, http.StatusInternalServerError, "error accesssing operation key on redis", err)
		return
	}

	if status == nil || len(status) == 0 {
		logger.Error("scheduler with operation key not found")
		WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success":     false,
			"description": "scheduler with operation key not found",
		})
		return
	}

	scheduler := models.NewScheduler(schedulerName, "", "")
	err = scheduler.Load(g.App.DB)
	if err != nil {
		g.App.HandleError(w, http.StatusInternalServerError, "error getting scheduler for getting progress", err)
		return
	}

	scheduler.NextMinorVersion()
	minor := scheduler.Version
	scheduler.NextMajorVersion()
	major := scheduler.Version

	totalPods, err := g.App.KubernetesClient.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
	if err != nil {
		g.App.HandleError(w, http.StatusInternalServerError, "error getting getting pods from kubernetes", err)
		return
	}
	total := float64(len(totalPods.Items))

	var news float64

	podsMinorVersion, err := g.App.KubernetesClient.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
		LabelSelector: labels.Set{"version": minor}.String(),
	})
	if err != nil {
		g.App.HandleError(w, http.StatusInternalServerError, "error getting getting pods from kubernetes", err)
		return
	}
	news = news + float64(len(podsMinorVersion.Items))

	podsMajorVersion, err := g.App.KubernetesClient.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
		LabelSelector: labels.Set{"version": major}.String(),
	})
	if err != nil {
		g.App.HandleError(w, http.StatusInternalServerError, "error getting getting pods from kubernetes", err)
		return
	}
	news = news + float64(len(podsMajorVersion.Items))

	status["progress"] = fmt.Sprintf("%.2f%%", 100.0*news/total)

	bts, _ := json.Marshal(status)
	WriteBytes(w, http.StatusOK, bts)
	logger.Info("Successfully wrote status response")
}
