package api

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/models"
)

// SchedulerLocksListHandler handler returns schedulers locks
type SchedulerLocksListHandler struct {
	App *App
}

// NewSchedulerLocksListHandler returns an instance of SchedulerLocksListHandler
func NewSchedulerLocksListHandler(a *App) *SchedulerLocksListHandler {
	h := &SchedulerLocksListHandler{App: a}
	return h
}

func (h *SchedulerLocksListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := schedulerLockParamsFromContext(r.Context())
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

// SchedulerLockDeleteHandler handler deletes a scheduler lock
type SchedulerLockDeleteHandler struct {
	App *App
}

// NewSchedulerLockDeleteHandler returns an instance of SchedulerLocksDeleteHandler
func NewSchedulerLockDeleteHandler(a *App) *SchedulerLockDeleteHandler {
	h := &SchedulerLockDeleteHandler{App: a}
	return h
}

func (h *SchedulerLockDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := schedulerLockParamsFromContext(r.Context())
	l := middleware.GetLogger(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerLocksDelete",
		"operation": "delete lock",
		"scheduler": params.SchedulerName,
		"lock":      params.LockKey,
	})
	logger.Debug("Getting scheduler locks")
	locksPrefixes := []string{h.App.Config.GetString("watcher.lockKey")}
	isLockValid := false
	validLocksKeys := models.ListSchedulerLocksKeys(params.SchedulerName, locksPrefixes)
	for _, possible := range validLocksKeys {
		if possible == params.LockKey {
			isLockValid = true
			break
		}
	}
	if !isLockValid {
		logger.Info("trying to delete invaldi lock key")
		Write(w, http.StatusBadRequest, "invalid lock key")
		return
	}
	err := models.DeleteSchedulerLock(
		h.App.RedisClient.Trace(r.Context()), params.SchedulerName, params.LockKey,
	)
	if err != nil {
		logger.WithError(err).Error("error deleting scheduler lock")
		h.App.HandleError(w, http.StatusInternalServerError, "delete lock failed", err)
		return
	}
	Write(w, http.StatusNoContent, "")
}
