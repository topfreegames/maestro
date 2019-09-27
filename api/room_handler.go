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
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/go-extensions-k8s-client-go/kubernetes"
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
	ctx := r.Context()
	l := middleware.GetLogger(ctx)
	mr := metricsReporterFromCtx(ctx)
	params := roomParamsFromContext(ctx)
	payload := statusPayloadFromCtx(ctx)

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "ping",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing ping...")
	var err error
	room := models.NewRoom(params.Name, params.Scheduler)

	if len(g.App.Forwarders) > 0 && len(payload.Metadata) == 0 {
		payload.Metadata, err = models.GetRoomMetadata(
			g.App.RedisClient.Trace(ctx),
			room.SchedulerName, room.ID)
		if err != nil {
			logger.WithError(err).Error("failed to get room metadata from redis")
			g.App.HandleError(w, http.StatusInternalServerError, "Redis failed", err)
			return
		}
	}

	kubernetesClient := kubernetes.TryWithContext(g.App.KubernetesClient, ctx)
	err = controller.SetRoomStatus(
		g.App.Logger,
		g.App.RoomManager,
		g.App.RedisClient.Trace(ctx),
		g.App.DBClient.WithContext(ctx),
		mr,
		kubernetesClient,
		payload,
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
	_, err = eventforwarder.ForwardRoomEvent(
		ctx,
		g.App.Forwarders,
		g.App.RedisClient.Trace(ctx),
		g.App.DBClient.WithContext(ctx),
		kubernetesClient,
		mr,
		room, fmt.Sprintf("ping%s", strings.Title(payload.Status)),
		"",
		payload.Metadata,
		g.App.SchedulerCache,
		logger,
		g.App.RoomAddrGetter,
	)

	if err != nil {
		logger.WithError(err).Error("ping event forward failed")
		g.App.HandleError(w, http.StatusInternalServerError, "event forward failed", err)
		return
	}

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
	ctx := r.Context()
	l := middleware.GetLogger(ctx)
	params := roomParamsFromContext(ctx)
	payload := playerEventPayloadFromCtx(ctx)

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "playerEventHandler",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing player event handler...")

	room := models.NewRoom(params.Name, params.Scheduler)

	kubernetesClient := kubernetes.TryWithContext(g.App.KubernetesClient, ctx)
	resp, err := eventforwarder.ForwardPlayerEvent(
		r.Context(),
		g.App.Forwarders,
		g.App.DBClient.WithContext(ctx),
		kubernetesClient,
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
	ctx := r.Context()
	mr := metricsReporterFromCtx(ctx)
	l := middleware.GetLogger(ctx)
	params := roomParamsFromContext(ctx)
	payload := roomEventPayloadFromCtx(ctx)

	logger := l.WithFields(logrus.Fields{
		"operation": "roomEventHandler",
		"scheduler": params.Scheduler,
		"roomId":    params.Name,
		"event":     payload.Event,
	})

	logger.
		WithField("metadata", payload.Metadata).
		Debug("performing room event handler")

	room := models.NewRoom(params.Name, params.Scheduler)

	if payload.Metadata != nil {
		payload.Metadata["eventType"] = payload.Event
	} else {
		payload.Metadata = map[string]interface{}{
			"eventType": payload.Event,
		}
	}

	kubernetesClient := kubernetes.TryWithContext(g.App.KubernetesClient, ctx)
	resp, err := eventforwarder.ForwardRoomEvent(
		r.Context(),
		g.App.Forwarders,
		g.App.RedisClient.Trace(ctx),
		g.App.DBClient.WithContext(ctx),
		kubernetesClient,
		mr,
		room,
		"roomEvent",
		"",
		payload.Metadata,
		g.App.SchedulerCache,
		g.App.Logger,
		g.App.RoomAddrGetter,
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
	ctx := r.Context()
	l := middleware.GetLogger(ctx)
	mr := metricsReporterFromCtx(ctx)
	params := roomParamsFromContext(ctx)
	payload := statusPayloadFromCtx(ctx)

	logger := l.WithFields(logrus.Fields{
		"source":           "roomHandler",
		"operation":        "statusHandler",
		"payloadTimestamp": payload.Timestamp,
	})

	logger.Debug("Performing status update...")

	kubernetesClient := kubernetes.TryWithContext(g.App.KubernetesClient, ctx)
	room := models.NewRoom(params.Name, params.Scheduler)
	err := controller.SetRoomStatus(
		g.App.Logger,
		g.App.RoomManager,
		g.App.RedisClient.Trace(ctx),
		g.App.DBClient.WithContext(ctx),
		mr,
		kubernetesClient,
		payload,
		g.App.Config,
		room,
		g.App.SchedulerCache,
	)
	if err != nil {
		logger.WithError(err).Error("Status update failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Status update failed", err)
		return
	}
	_, err = eventforwarder.ForwardRoomEvent(
		ctx,
		g.App.Forwarders,
		g.App.RedisClient.Trace(ctx),
		g.App.DBClient.WithContext(ctx),
		kubernetesClient,
		mr,
		room, payload.Status, "",
		payload.Metadata,
		g.App.SchedulerCache,
		g.App.Logger,
		g.App.RoomAddrGetter,
	)
	if err != nil {
		logger.WithError(err).Error("status update event forward failed")
		g.App.HandleError(w, http.StatusInternalServerError, "event forward failed", err)
		return
	}

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
	ctx := r.Context()
	mr := metricsReporterFromCtx(ctx)
	l := middleware.GetLogger(ctx)
	params := roomParamsFromContext(ctx)

	logger := l.WithFields(logrus.Fields{
		"source":    "roomHandler",
		"operation": "addressHandler",
	})

	logger.Debug("Address handler called")

	room := models.NewRoom(params.Name, params.Scheduler)
	kubernetesClient := kubernetes.TryWithContext(h.App.KubernetesClient, ctx)
	roomAddresses, err := h.App.RoomAddrGetter.Get(room, kubernetesClient, h.App.RedisClient.Trace(ctx), mr)

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

// RoomListByMetricHandler handler
type RoomListByMetricHandler struct {
	App *App
}

// NewRoomListByMetricHandler creates a new address handler
func NewRoomListByMetricHandler(a *App) *RoomListByMetricHandler {
	m := &RoomListByMetricHandler{App: a}
	return m
}

// ServerHTTP method
func (h *RoomListByMetricHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := middleware.GetLogger(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "roomHandler",
		"operation": "listByMetricHandler",
	})

	logger.Debug("list by metric handler called")

	metric := "room"
	if metricArray, ok := r.URL.Query()["metric"]; ok && len(metricArray) > 0 {
		metric = metricArray[0]
	}

	if !models.ValidPolicyType(metric) {
		err := fmt.Errorf("invalid metric %s", metric)
		logger.WithError(err).Error("room list by metric failed")
		h.App.HandleError(w, http.StatusBadRequest, "room list by metric failed", err)
		return
	}

	limit := 5
	if limitArray, ok := r.URL.Query()["limit"]; ok && len(limitArray) > 0 {
		var err error
		limit, err = strconv.Atoi(limitArray[0])
		if err != nil {
			logger.WithError(err).Error("room list by limit failed")
			h.App.HandleError(w, http.StatusBadRequest, "room list by limit failed", err)
			return
		}
	}

	rooms, err := models.GetRoomsByMetric(
		h.App.RedisClient.Trace(r.Context()),
		params.SchedulerName,
		metric,
		limit,
		mr,
	)

	if err != nil {
		status := http.StatusInternalServerError
		logger.WithError(err).Error("list by metrics handler failed")
		h.App.HandleError(w, status, "list by metrics handler error", err)
		return
	}

	bytes, err := json.Marshal(map[string]interface{}{"rooms": rooms})
	if err != nil {
		logger.WithError(err).Error("list by metrics handler failed")
		h.App.HandleError(w, http.StatusInternalServerError, "list by metrics handler error", err)
		return
	}
	WriteBytes(w, http.StatusOK, bytes)
	logger.Debug("performed list by metrics handler")
}
