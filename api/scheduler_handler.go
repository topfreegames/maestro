// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
)

// SchedulerCreateHandler handler
type SchedulerCreateHandler struct {
	App *App
}

// NewSchedulerCreateHandler creates a new scheduler create handler
func NewSchedulerCreateHandler(a *App) *SchedulerCreateHandler {
	m := &SchedulerCreateHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	payload := configYamlFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "create",
	})

	logger.Debug("Creating scheduler...")

	err := mr.WithSegment(models.SegmentController, func() error {
		timeoutSec := g.App.Config.GetInt("scaleUpTimeout")
		return controller.CreateScheduler(l, mr, g.App.DB, g.App.RedisClient, g.App.KubernetesClient, payload, timeoutSec)
	})

	if err != nil {
		logger.WithError(err).Error("Create scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Create scheduler failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusCreated, `{"success": true}`)
		return nil
	})
	logger.Debug("Create scheduler succeeded.")
}

// SchedulerDeleteHandler handler
type SchedulerDeleteHandler struct {
	App *App
}

// NewSchedulerDeleteHandler creates a new scheduler delete handler
func NewSchedulerDeleteHandler(a *App) *SchedulerDeleteHandler {
	m := &SchedulerDeleteHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "delete",
	})

	logger.Debug("Deleting scheduler...")

	err := mr.WithSegment(models.SegmentController, func() error {
		return controller.DeleteScheduler(l, mr, g.App.DB, g.App.KubernetesClient, params.SchedulerName)
	})

	if err != nil {
		logger.WithError(err).Error("Delete scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Delete scheduler failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Debug("Delete scheduler succeeded.")
}
