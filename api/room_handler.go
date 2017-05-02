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
	"github.com/topfreegames/maestro/models"
)

// RoomPingHandler handler
type RoomPingHandler struct {
	App *App
}

// NewRoomPingHandler creates a new ping handler
func NewRoomPingHandler(a *App) *RoomPingHandler {
	m := &RoomPingHandler{App: a}
	return m
}

// ServeHTTP method
func (g *RoomPingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := roomParamsFromContext(r.Context())
	payload := pingPayloadFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "ping",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing ping...")

	room := models.NewRoom(params.Name, params.Scheduler)
	err := mr.WithSegment(models.SegmentUpdate, func() error {
		return room.Ping(g.App.RedisClient, payload.Status)
	})

	if err != nil {
		logger.WithError(err).Error("Ping failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Ping failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Debug("Ping successful.")
}

// RoomStatusHandler handler
type RoomStatusHandler struct {
	App *App
}

// NewRoomStatusHandler creates a new status handler
func NewRoomStatusHandler(a *App) *RoomStatusHandler {
	m := &RoomStatusHandler{App: a}
	return m
}

// ServeHTTP method
func (g *RoomStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := roomParamsFromContext(r.Context())
	payload := statusPayloadFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "statusHandler",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing status update...")

	room := models.NewRoom(params.Name, params.Scheduler)
	err := mr.WithSegment(models.SegmentUpdate, func() error {
		return room.SetStatus(g.App.RedisClient, payload.LastStatus, payload.Status)
	})

	if err != nil {
		logger.WithError(err).Error("Status update failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Status update failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Debug("Performed status update.")
}
