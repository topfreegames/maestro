// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/topfreegames/maestro/models"
)

// SchedulerDiffHandler handler returns the scheduler config
type SchedulerDiffHandler struct {
	App *App
}

// NewSchedulerDiffHandler returns an instance of SchedulerConfigHandler
func NewSchedulerDiffHandler(a *App) *SchedulerDiffHandler {
	m := &SchedulerDiffHandler{App: a}
	return m
}

func (g *SchedulerDiffHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var scheduler *models.Scheduler
	schedulerName := mux.Vars(r)["schedulerName"]
	schedulersVersion := schedulersDiffParamsFromContext(r.Context())

	l := loggerFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulersDiff",
		"operation": "schedulers diff",
		"scheduler": schedulerName,
	})
	logger.Info("Executing schedulers diff")

	var err error
	var yamlStr1, yamlStr2 string

	if schedulersVersion.Version1 == "" {
		scheduler = models.NewScheduler(schedulerName, "", "")
		err = scheduler.Load(g.App.DB)
		if err != nil {
			logger.WithError(err).Error("error accessing database")
			g.App.HandleError(w, http.StatusInternalServerError, "schedulers diff failed", err)
			return
		}

		schedulersVersion.Version1 = fmt.Sprintf("v%d", scheduler.Version)
		yamlStr1 = scheduler.YAML
	}

	if schedulersVersion.Version2 == "" {
		versionInt, err := strconv.Atoi(strings.TrimPrefix(schedulersVersion.Version1, "v"))
		if err != nil {
			logger.WithError(err).Errorf("invalid version was sent: %s", schedulersVersion.Version1)
			g.App.HandleError(w, http.StatusBadRequest, "schedulers diff failed", err)
			return
		}

		schedulersVersion.Version2 = fmt.Sprintf("v%d", versionInt-1)
	}

	if yamlStr1 == "" {
		yamlStr1, err = models.LoadConfigWithVersion(g.App.DB, schedulerName, schedulersVersion.Version1)
		if err != nil {
			logger.WithError(err).Error("load scheduler with version error")
			g.App.HandleError(w, http.StatusInternalServerError, "schedulers diff failed", err)
			return
		}
		if len(yamlStr1) == 0 {
			logger.Error("config for scheduler and version not found")
			g.App.HandleError(w, http.StatusNotFound, "schedulers diff failed",
				fmt.Errorf("config scheduler not found: %s:%s", schedulerName, schedulersVersion.Version1))
			return
		}
	}

	yamlStr2, err = models.LoadConfigWithVersion(g.App.DB, schedulerName, schedulersVersion.Version2)
	if err != nil {
		logger.WithError(err).Error("load scheduler with version error")
		g.App.HandleError(w, http.StatusInternalServerError, "schedulers diff failed", err)
		return
	}
	if len(yamlStr2) == 0 {
		logger.Error("config for scheduler and version not found")
		g.App.HandleError(w, http.StatusNotFound, "schedulers diff failed",
			fmt.Errorf("config scheduler not found: %s:%s", schedulerName, schedulersVersion.Version2))
		return
	}

	configYaml1, err := models.NewConfigYAML(yamlStr1)
	if err != nil {
		logger.WithError(err).Error("error unmarshalling yaml to config")
		g.App.HandleError(w, http.StatusInternalServerError, "schedulers diff failed", err)
		return
	}

	configYaml2, err := models.NewConfigYAML(yamlStr2)
	if err != nil {
		logger.WithError(err).Error("error unmarshalling yaml to config")
		g.App.HandleError(w, http.StatusInternalServerError, "schedulers diff failed", err)
		return
	}

	schedulersVersion.Diff = configYaml1.Diff(configYaml2)

	bts, err := json.Marshal(schedulersVersion)
	if err != nil {
		logger.WithError(err).Error("error marshalling struct to bytes")
		g.App.HandleError(w, http.StatusInternalServerError, "schedulers diff failed", err)
		return
	}

	WriteBytes(w, http.StatusOK, bts)
	logger.Info("successfully done yamls diff")
}
