// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
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
	l := middleware.GetLogger(r.Context())
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
		g.App.RoomManager,
		g.App.RedisClient.Trace(r.Context()),
		g.App.DBClient.WithContext(r.Context()),
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

	// TODO: consider sampling requests by scheduler name and only forwarding a few pings
	eventforwarder.ForwardRoomEvent(
		r.Context(),
		g.App.Forwarders,
		g.App.DBClient.WithContext(r.Context()),
		g.App.KubernetesClient,
		room, fmt.Sprintf("ping%s", strings.Title(payload.Status)),
		payload.Metadata,
		g.App.SchedulerCache,
		g.App.Logger,
	)

	Write(w, http.StatusOK, `{"success": true}`)
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
	l := middleware.GetLogger(r.Context())
	params := roomParamsFromContext(r.Context())
	payload := playerEventPayloadFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "playerEventHandler",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing player event handler...")

	room := models.NewRoom(params.Name, params.Scheduler)

	resp, err := eventforwarder.ForwardPlayerEvent(
		r.Context(),
		g.App.Forwarders,
		g.App.DBClient.WithContext(r.Context()),
		g.App.KubernetesClient,
		room,
		payload.Event,
		payload.Metadata,
		g.App.SchedulerCache,
		g.App.Logger,
	)

	if err != nil {
		logger.WithError(err).Error("Player event forward failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Player event forward failed", err)
		return
	}

	if resp.Code != 200 {
		err := errors.New(resp.Message)
		logger.WithError(err).Error("Player event forward failed.")
		g.App.HandleError(w, resp.Code, "player event forward failed", err)
		return
	}

	resBytes, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": resp.Message,
	})
	Write(w, http.StatusOK, string(resBytes))
	logger.Debug("Performed player event forward.")
}

// RoomEventHandler handler
type RoomEventHandler struct {
	App *App
}

// NewRoomEventHandler creates a new room event handler
func NewRoomEventHandler(a *App) *RoomEventHandler {
	r := &RoomEventHandler{App: a}
	return r
}

// ServeHTTP method
func (g *RoomEventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := middleware.GetLogger(r.Context())
	params := roomParamsFromContext(r.Context())
	payload := roomEventPayloadFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "roomEventHandler",
		"payloadTimestamp": payload.Timestamp,
		"event":            payload.Event,
	})

	logger.Debug("Performing room event handler...")

	room := models.NewRoom(params.Name, params.Scheduler)

	if payload.Metadata != nil {
		payload.Metadata["eventType"] = payload.Event
	} else {
		payload.Metadata = map[string]interface{}{
			"eventType": payload.Event,
		}
	}

	resp, err := eventforwarder.ForwardRoomEvent(
		r.Context(),
		g.App.Forwarders,
		g.App.DBClient.WithContext(r.Context()),
		g.App.KubernetesClient,
		room,
		"roomEvent",
		payload.Metadata,
		g.App.SchedulerCache,
		g.App.Logger,
	)

	if err != nil {
		logger.WithError(err).Error("Room event forward failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Room event forward failed", err)
		return
	}

	if resp.Code != 200 {
		err := errors.New(resp.Message)
		logger.WithError(err).Error("Room event forward failed.")
		g.App.HandleError(w, resp.Code, "room event forward failed", err)
		return
	}

	resBytes, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": resp.Message,
	})
	Write(w, http.StatusOK, string(resBytes))
	logger.Debug("Performed room event forward.")
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
	l := middleware.GetLogger(r.Context())
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
		g.App.RoomManager,
		g.App.RedisClient.Trace(r.Context()),
		g.App.DBClient.WithContext(r.Context()),
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
		r.Context(),
		g.App.Forwarders,
		g.App.DBClient.WithContext(r.Context()),
		g.App.KubernetesClient,
		room, payload.Status,
		payload.Metadata,
		g.App.SchedulerCache,
		g.App.Logger,
	)
	Write(w, http.StatusOK, `{"success": true}`)
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
	l := middleware.GetLogger(r.Context())
	params := roomParamsFromContext(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "roomHandler",
		"operation": "addressHandler",
	})

	logger.Debug("Address handler called")

	room := models.NewRoom(params.Name, params.Scheduler)
	roomAddresses, err := h.App.RoomAddrGetter.Get(room, h.App.KubernetesClient)

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
	WriteBytes(w, http.StatusOK, bytes)
	logger.Debug("Performed address handler.")
}
