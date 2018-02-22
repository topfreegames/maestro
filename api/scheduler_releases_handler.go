package api

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
)

// GetSchedulerReleasesHandler handler returns the scheduler config
type GetSchedulerReleasesHandler struct {
	App *App
}

// NewGetSchedulerReleasesHandler returns an instance of SchedulerConfigHandler
func NewGetSchedulerReleasesHandler(a *App) *GetSchedulerReleasesHandler {
	m := &GetSchedulerReleasesHandler{App: a}
	return m
}

func (g *GetSchedulerReleasesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := schedulerParamsFromContext(r.Context())
	l := loggerFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "getSchedulerReleases",
		"operation": "get releases",
		"scheduler": params.SchedulerName,
	})

	logger.Debug("Getting scheduler releases")

	releases, err := models.ListSchedulerReleases(g.App.DB, params.SchedulerName)
	if err != nil {
		logger.WithError(err).Error("error listing scheduler releases")
		g.App.HandleError(w, http.StatusInternalServerError, "config releases failed", err)
		return
	}

	bytes, _ := json.Marshal(map[string]interface{}{
		"releases": releases,
	})

	WriteBytes(w, http.StatusOK, bytes)
}
