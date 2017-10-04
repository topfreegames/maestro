// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
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
	payload := statusPayloadFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "ping",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing ping...")

	room := models.NewRoom(params.Name, params.Scheduler)
	err := controller.SetRoomStatus(
		g.App.Logger,
		g.App.RedisClient,
		g.App.DB,
		mr,
		g.App.KubernetesClient,
		payload.Status,
		g.App.Config,
		room,
		g.App.SchedulerCache,
	)

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

// PlayerEventHandler handler
type PlayerEventHandler struct {
	App *App
}

// NewPlayerEventHandler creates a new player event handler
func NewPlayerEventHandler(a *App) *PlayerEventHandler {
	p := &PlayerEventHandler{App: a}
	return p
}

// ServeHTTP method
func (g *PlayerEventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := roomParamsFromContext(r.Context())
	payload := playerEventPayloadFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "playerEventHandler",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing player event handler...")

	room := models.NewRoom(params.Name, params.Scheduler)

	err := eventforwarder.ForwardPlayerEvent(g.App.Forwarders, g.App.DB, room.ID, payload.Event, payload.Metadata)

	if err != nil {
		logger.WithError(err).Error("Player event forward failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Player event forward failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})

	logger.Debug("Performed player event forward.")
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
	err := controller.SetRoomStatus(
		g.App.Logger,
		g.App.RedisClient,
		g.App.DB,
		mr,
		g.App.KubernetesClient,
		payload.Status,
		g.App.Config,
		room,
		g.App.SchedulerCache,
	)
	if err != nil {
		logger.WithError(err).Error("Status update failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Status update failed", err)
		return
	}
	eventforwarder.ForwardRoomEvent(
		g.App.Forwarders,
		g.App.DB,
		g.App.KubernetesClient,
		room, payload.Status,
		payload.Metadata,
		g.App.SchedulerCache,
	)
	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Debug("Performed status update.")
}

// RoomAddressHandler handler
type RoomAddressHandler struct {
	App *App
}

// NewRoomAddressHandler creates a new address handler
func NewRoomAddressHandler(a *App) *RoomAddressHandler {
	m := &RoomAddressHandler{App: a}
	return m
}

// ServerHTTP method
func (h *RoomAddressHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := roomParamsFromContext(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "roomHandler",
		"operation": "addressHandler",
	})

	logger.Debug("Address handler called")

	room := models.NewRoom(params.Name, params.Scheduler)
	roomAddresses, err := room.GetAddresses(h.App.KubernetesClient)

	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusUnprocessableEntity
		}
		logger.WithError(err).Error("Address handler failed.")
		h.App.HandleError(w, status, "Address handler error", err)
		return
	}

	bytes, err := json.Marshal(roomAddresses)
	if err != nil {
		logger.WithError(err).Error("Address handler failed.")
		h.App.HandleError(w, http.StatusInternalServerError, "Address handler error", err)
		return
	}
	mr.WithSegment(models.SegmentSerialization, func() error {
		WriteBytes(w, http.StatusOK, bytes)
		return nil
	})
	logger.Debug("Performed address handler.")
}
