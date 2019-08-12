package api

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/models"
)

// SchedulerLocksListHandler handler returns the scheduler config
type SchedulerLocksListHandler struct {
	App *App
}

// NewSchedulerLocksListHandler returns an instance of SchedulerConfigHandler
func NewSchedulerLocksListHandler(a *App) *SchedulerLocksListHandler {
	h := &SchedulerLocksListHandler{App: a}
	return h
}

func (h *SchedulerLocksListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := schedulerParamsFromContext(r.Context())
	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerLocksList",
		"operation": "get locks",
		"scheduler": params.SchedulerName,
	})
	logger.Debug("Getting scheduler locks")
	locksPrefixes := []string{h.App.Config.GetString("watcher.lockKey")}
	locks, err := models.ListSchedulerLocks(
		h.App.RedisClient.Trace(r.Context()), params.SchedulerName, locksPrefixes,
	)
	if err != nil {
		logger.WithError(err).Error("error listing scheduler locks")
		h.App.HandleError(w, http.StatusInternalServerError, "list locks failed", err)
		return
	}
	bts, err := json.Marshal(locks)
	if err != nil {
		logger.WithError(err).Error("error marshaling scheduler locks")
		h.App.HandleError(w, http.StatusInternalServerError, "marshal locks failed", err)
		return
	}
	WriteBytes(w, http.StatusOK, bts)
}
