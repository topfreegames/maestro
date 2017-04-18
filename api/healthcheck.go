// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"

	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
)

//HealthcheckHandler handler
type HealthcheckHandler struct {
	App *App
}

// NewHealthcheckHandler creates a new healthcheck handler
func NewHealthcheckHandler(a *App) *HealthcheckHandler {
	m := &HealthcheckHandler{App: a}
	return m
}

//ServeHTTP method
func (h *HealthcheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())

	l.Debug("Performing healthcheck...")

	err := mr.WithDatastoreSegment("select 1", "select", func() error {
		_, err := h.App.DB.Exec("select 1")
		return err
	})
	if err != nil {
		l.WithError(err).Error("Database is offline")
		vErr := errors.NewDatabaseError(err)
		WriteBytes(w, http.StatusInternalServerError, vErr.Serialize())
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"healthy": true}`)
		return nil
	})
	l.Debug("Healthcheck done.")
}
