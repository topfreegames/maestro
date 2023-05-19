package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
)

// SchedulerEventHandler returns the scheduler events
type SchedulerEventHandler struct {
	App *App
}

// NewSchedulerEventHandler returns an instance of SchedulerEventHandler
func NewSchedulerEventHandler(a *App) *SchedulerEventHandler {
	m := &SchedulerEventHandler{App: a}
	return m
}

func (g *SchedulerEventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	schedulerName := vars["schedulerName"]

	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "SchedulerEventHandler",
		"operation": "get scheduler events",
		"scheduler": schedulerName,
	})

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		errorMsg := "failed to convert page param to integer"
		logger.WithError(err).Error(errorMsg)
		g.App.HandleError(w, http.StatusBadRequest, errorMsg, err)
		return
	}

	logger.Info("Fetching scheduler events")

	events, err := g.App.SchedulerEventStorage.LoadSchedulerEvents(schedulerName, page)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success":     false,
			"description": err.Error(),
		})
		return
	}

	bts, _ := json.Marshal(events)
	WriteBytes(w, http.StatusOK, bts)
	logger.Info("Successfully wrote status response")
}
