package api

import (
	"errors"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
)

// GetSchedulerConfigHandler handler returns the scheduler config
type GetSchedulerConfigHandler struct {
	App *App
}

// NewGetSchedulerConfigHandler returns an instance of SchedulerConfigHandler
func NewGetSchedulerConfigHandler(a *App) *GetSchedulerConfigHandler {
	m := &GetSchedulerConfigHandler{App: a}
	return m
}

func (g *GetSchedulerConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := schedulerParamsFromContext(r.Context())
	l := loggerFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "getSchedulerConfig",
		"operation": "get config",
		"scheduler": params.SchedulerName,
	})

	logger.Debug("Getting scheduler config")

	// version is in the format v<integer>, e.g., v1, v2, v3
	version := r.URL.Query().Get("version")

	var yamlStr string
	var err error
	yamlStr, err = models.LoadConfigWithVersion(g.App.DB, params.SchedulerName, version)
	if err != nil {
		logger.WithError(err).Error("config scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "config scheduler failed", err)
		return
	}
	if len(yamlStr) == 0 {
		logger.Error("config scheduler not found.")
		g.App.HandleError(w, http.StatusNotFound, "get config error", errors.New("config scheduler not found"))
		return
	}

	yamlConf, err := models.NewConfigYAML(yamlStr)
	if err != nil {
		logger.WithError(err).Error("config scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "config scheduler failed", err)
		return
	}

	bts := yamlConf.ToYAML()
	WriteBytes(w, http.StatusOK, bts)
	logger.Debug("config scheduler succeeded.")
}
