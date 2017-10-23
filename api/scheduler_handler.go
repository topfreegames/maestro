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
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/extensions/clock"
	"github.com/topfreegames/extensions/redis"
	yaml "gopkg.in/yaml.v2"

	"github.com/topfreegames/maestro/controller"
	maestroErrors "github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/eventforwarder"
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
	configs := configYamlFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "create",
	})

	for _, payload := range configs {
		logger.Debug("Creating scheduler...")

		timeoutSec := g.App.Config.GetInt("scaleUpTimeoutSeconds")
		err := mr.WithSegment(models.SegmentController, func() error {
			return controller.CreateScheduler(l, mr, g.App.DB, g.App.RedisClient, g.App.KubernetesClient, payload, timeoutSec)
		})

		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "already exists") {
				status = http.StatusConflict
			} else if err.Error() == "node without label error" {
				status = http.StatusUnprocessableEntity
			}
			logger.WithError(err).Error("Create scheduler failed.")
			g.App.HandleError(w, status, "Create scheduler failed", err)
			return
		}

		// this forwards the metadata configured for each enabled forwarder
		_, err = eventforwarder.ForwardRoomInfo(
			g.App.Forwarders,
			g.App.DB,
			g.App.KubernetesClient,
			payload.Name,
			g.App.SchedulerCache,
			g.App.Logger,
		)

		if err != nil {
			logger.WithError(err).Error("Room info forward failed.")
		}

		logger.Debug("Create scheduler succeeded.")
	}
	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusCreated, `{"success": true}`)
		return nil
	})
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
		"scheduler": params.SchedulerName,
	})

	logger.Info("deleting scheduler")

	timeoutSec := g.App.Config.GetInt("deleteTimeoutSeconds")
	err := mr.WithSegment(models.SegmentController, func() error {
		return controller.DeleteScheduler(l, mr, g.App.DB, g.App.RedisClient, g.App.KubernetesClient, params.SchedulerName, timeoutSec)
	})

	if err != nil {
		logger.WithError(err).Error("delete scheduler failed")
		status := http.StatusInternalServerError
		if _, ok := err.(*maestroErrors.ValidationFailedError); ok {
			status = http.StatusNotFound
		}
		g.App.HandleError(w, status, "delete scheduler failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Info("finished deleting scheduler")
}

// SchedulerUpdateHandler handler
type SchedulerUpdateHandler struct {
	App *App
}

// NewSchedulerUpdateHandler creates a new scheduler delete handler
func NewSchedulerUpdateHandler(a *App) *SchedulerUpdateHandler {
	m := &SchedulerUpdateHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())
	payload := configYamlFromCtx(r.Context())[0]
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "update",
		"scheduler": params.SchedulerName,
	})

	maxSurge, err := getMaxSurge(g.App, r)
	if err != nil {
		g.App.HandleError(w, http.StatusBadRequest, "invalid maxsurge parameter", err)
		return
	}

	logger.Info("updating scheduler")

	if params.SchedulerName != payload.Name {
		msg := fmt.Sprintf("url name %s doesn't match payload name %s", params.SchedulerName, payload.Name)
		err := maestroErrors.NewValidationFailedError(errors.New(msg))
		g.App.HandleError(w, http.StatusBadRequest, "Update scheduler failed", err)
		return
	}

	redisClient, err := redis.NewClient("extensions.redis", g.App.Config, g.App.RedisClient)
	if err != nil {
		logger.WithError(err).Error("error getting redisClient")
		g.App.HandleError(w, http.StatusInternalServerError, "Update scheduler failed", err)
		return
	}

	timeoutSec := g.App.Config.GetInt("updateTimeoutSeconds")
	logger.WithField("time", time.Now()).Info("Starting update")
	lockKey := fmt.Sprintf("%s-%s", g.App.Config.GetString("watcher.lockKey"), payload.Name)
	err = mr.WithSegment(models.SegmentController, func() error {
		return controller.UpdateSchedulerConfig(
			l,
			mr,
			g.App.DB,
			redisClient,
			g.App.KubernetesClient,
			payload,
			timeoutSec, g.App.Config.GetInt("watcher.lockTimeoutMs"), maxSurge,
			lockKey,
			&clock.Clock{},
			nil,
		)
	})
	logger.WithField("time", time.Now()).Info("finished update")

	if err != nil {
		status := http.StatusInternalServerError
		if _, ok := err.(*maestroErrors.ValidationFailedError); ok {
			status = http.StatusNotFound
		} else if err.Error() == "node without label error" {
			status = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "invalid parameter") {
			status = http.StatusBadRequest
		}
		logger.WithError(err).Error("Update scheduler failed.")
		g.App.HandleError(w, status, "Update scheduler failed", err)
		return
	}

	// this forwards the metadata configured for each enabled forwarder
	_, err = eventforwarder.ForwardRoomInfo(
		g.App.Forwarders,
		g.App.DB,
		g.App.KubernetesClient,
		payload.Name,
		nil, // intentionally omit SchedulerCache to force reload since it is an update
		g.App.Logger,
	)

	if err != nil {
		logger.WithError(err).Error("Room info forward failed.")
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Info("update scheduler succeeded")
}

// SchedulerListHandler handler
type SchedulerListHandler struct {
	App *App
}

// NewSchedulerListHandler lists schedulers
func NewSchedulerListHandler(a *App) *SchedulerListHandler {
	m := &SchedulerListHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "list",
	})
	names, err := controller.ListSchedulersNames(l, mr, g.App.DB)
	if err != nil {
		status := http.StatusInternalServerError
		logger.WithError(err).Error("List scheduler failed.")
		g.App.HandleError(w, status, "List scheduler failed", err)
		return
	}

	_, getInfo := r.URL.Query()["info"]
	if getInfo {
		g.returnInfo(w, r, l, mr, names)
		return
	}

	resp := map[string]interface{}{
		"schedulers": names,
	}

	bts, err := json.Marshal(resp)
	if err != nil {
		logger.WithError(err).Error("List scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "List scheduler failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		WriteBytes(w, http.StatusOK, bts)
		return nil
	})
	logger.Debug("List scheduler succeeded.")
}

// SchedulerStatusHandler handler
type SchedulerStatusHandler struct {
	App *App
}

// NewSchedulerStatusHandler get scheduler and its rooms status
func NewSchedulerStatusHandler(a *App) *SchedulerStatusHandler {
	m := &SchedulerStatusHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())

	_, getConfig := r.URL.Query()["config"]
	if getConfig {
		g.returnConfig(w, r, l, mr, params.SchedulerName)
		return
	}

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "status",
		"scheduler": params.SchedulerName,
	})

	logger.Debugf("Getting scheduler %s status", params.SchedulerName)

	scheduler, _, roomCountByStatus, err := controller.GetSchedulerScalingInfo(
		l,
		mr,
		g.App.DB,
		g.App.RedisClient,
		params.SchedulerName,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if _, ok := err.(*maestroErrors.ValidationFailedError); ok {
			status = http.StatusNotFound
		}
		logger.WithError(err).Error("Status scheduler failed.")
		g.App.HandleError(w, status, "Status scheduler failed", err)
		return
	}

	resp := map[string]interface{}{
		"game":               scheduler.Game,
		"state":              scheduler.State,
		"stateLastChangedAt": scheduler.StateLastChangedAt,
		"lastScaleOpAt":      scheduler.LastScaleOpAt,
		"roomsAtCreating":    roomCountByStatus.Creating,
		"roomsAtOccupied":    roomCountByStatus.Occupied,
		"roomsAtReady":       roomCountByStatus.Ready,
		"roomsAtTerminating": roomCountByStatus.Terminating,
	}
	bts, err := json.Marshal(resp)
	if err != nil {
		logger.WithError(err).Error("Status scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "Status scheduler failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		WriteBytes(w, http.StatusOK, bts)
		return nil
	})
	logger.Debug("Status scheduler succeeded.")
}

func (g *SchedulerStatusHandler) returnConfig(
	w http.ResponseWriter,
	r *http.Request,
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	schedulerName string,
) {
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "get config",
	})

	logger.Debugf("Getting scheduler %s config", schedulerName)

	var yamlStr string
	err := mr.WithSegment(models.SegmentSelect, func() error {
		var err error
		yamlStr, err = models.LoadConfig(g.App.DB, schedulerName)
		return err
	})
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
	resp := map[string]interface{}{
		"yaml": yamlStr,
	}

	bts, err := json.Marshal(resp)
	if err != nil {
		logger.WithError(err).Error("config scheduler failed.")
		g.App.HandleError(w, http.StatusInternalServerError, "config scheduler failed", err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		WriteBytes(w, http.StatusOK, bts)
		return nil
	})
	logger.Debug("config scheduler succeeded.")
}

func (g *SchedulerListHandler) returnInfo(
	w http.ResponseWriter,
	r *http.Request,
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	schedulersNames []string,
) {
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "get infos",
	})

	logger.Debugf("Getting schedulers infos")

	var schedulers []models.Scheduler
	err := mr.WithSegment(models.SegmentSelect, func() error {
		var err error
		schedulers, err = models.LoadSchedulers(g.App.DB, schedulersNames)
		return err
	})
	if err != nil {
		errStr := "failed to get schedulers from DB"
		logger.WithError(err).Error(errStr)
		g.App.HandleError(w, http.StatusInternalServerError, errStr, err)
		return
	}

	redisClient, err := redis.NewClient("extensions.redis", g.App.Config, g.App.RedisClient)
	if err != nil {
		errStr := "failed to create redis client"
		logger.WithError(err).Error(errStr)
		g.App.HandleError(w, http.StatusInternalServerError, errStr, err)
		return
	}
	roomsCounts, err := models.GetRoomsCountByStatusForSchedulers(redisClient.Client, schedulersNames)
	if err != nil {
		errStr := "failed to get rooms counts for schedulers"
		logger.WithError(err).Error(errStr)
		g.App.HandleError(w, http.StatusInternalServerError, errStr, err)
		return
	}

	resp := make([]map[string]interface{}, len(schedulersNames))
	for i, s := range schedulers {
		configYaml := models.ConfigYAML{}
		err = yaml.Unmarshal([]byte(s.YAML), &configYaml)
		if err != nil {
			errStr := fmt.Sprintf("parse yaml error for scheduler %s", s.Name)
			logger.WithError(err).Error(errStr)
			g.App.HandleError(w, http.StatusInternalServerError, errStr, err)
			return
		}

		resp[i] = map[string]interface{}{
			"name":                        s.Name,
			"game":                        s.Game,
			"state":                       s.State,
			"roomsReady":                  roomsCounts[i].Ready,
			"roomsOccupied":               roomsCounts[i].Occupied,
			"roomsCreating":               roomsCounts[i].Creating,
			"roomsTerminating":            roomsCounts[i].Terminating,
			"autoscalingMin":              configYaml.AutoScaling.Min,
			"autoscalingUpTriggerUsage":   configYaml.AutoScaling.Up.Trigger.Usage,
			"autoscalingDownTriggerUsage": configYaml.AutoScaling.Down.Trigger.Usage,
		}
	}

	bts, err := json.Marshal(resp)
	if err != nil {
		errStr := "failed to marshal schedulers infos"
		logger.WithError(err).Error(errStr)
		g.App.HandleError(w, http.StatusInternalServerError, errStr, err)
		return
	}

	mr.WithSegment(models.SegmentSerialization, func() error {
		WriteBytes(w, http.StatusOK, bts)
		return nil
	})
	logger.Debug("get schedulers infos succeeded")
}

// SchedulerScaleHandler handler
type SchedulerScaleHandler struct {
	App *App
}

// NewSchedulerScaleHandler get scheduler and its rooms status
func NewSchedulerScaleHandler(a *App) *SchedulerScaleHandler {
	m := &SchedulerScaleHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerScaleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())
	scaleParams := schedulerScaleParamsFromCtx(r.Context())
	if scaleParams == nil {
		g.App.HandleError(w, http.StatusBadRequest, "ValidationFailedError", errors.New("empty body sent"))
		return
	}

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerScaleHandler",
		"operation": "scale",
		"scheduler": params.SchedulerName,
	})
	logger.Info("scaling scheduler")

	err := controller.ScaleScheduler(
		logger,
		mr,
		g.App.DB,
		g.App.RedisClient,
		g.App.KubernetesClient,
		g.App.Config.GetInt("scaleUpTimeoutSeconds"), g.App.Config.GetInt("scaleDownTimeoutSeconds"),
		scaleParams.ScaleUp, scaleParams.ScaleDown, scaleParams.Replicas,
		params.SchedulerName,
	)

	if err != nil {
		logger.WithError(err).Error("scheduler scale failed")
		status := http.StatusInternalServerError

		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "empty") {
			status = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
		}

		g.App.HandleError(w, status, "scale scheduler failed", err)
		return
	}
	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
	logger.Info("scheduler successfully scaled")
}

// SchedulerImageHandler handler
type SchedulerImageHandler struct {
	App *App
}

// NewSchedulerImageHandler creates a new scheduler create handler
func NewSchedulerImageHandler(a *App) *SchedulerImageHandler {
	m := &SchedulerImageHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "setSchedulerImage",
		"scheduler": params.SchedulerName,
	})

	maxSurge, err := getMaxSurge(g.App, r)
	if err != nil {
		g.App.HandleError(w, http.StatusBadRequest, "invalid maxsurge parameter", err)
		return
	}

	schedulerImage := schedulerImageParamsFromCtx(r.Context())
	if schedulerImage == nil {
		g.App.HandleError(w, http.StatusBadRequest, "image name not sent on body", errors.New("image name not sent on body"))
		return
	}

	logger.Info("updating scheduler's image")

	redisClient, err := redis.NewClient("extensions.redis", g.App.Config, g.App.RedisClient)
	if err != nil {
		logger.WithError(err).Error("error getting redisClient")
		g.App.HandleError(w, http.StatusInternalServerError, "Update scheduler failed", err)
		return
	}

	timeoutSec := g.App.Config.GetInt("scaleUpTimeoutSeconds")
	lockKey := fmt.Sprintf("%s-%s", g.App.Config.GetString("watcher.lockKey"), params.SchedulerName)
	err = mr.WithSegment(models.SegmentController, func() error {
		return controller.UpdateSchedulerImage(
			l,
			mr,
			g.App.DB,
			redisClient,
			g.App.KubernetesClient,
			params.SchedulerName, schedulerImage.Image, lockKey,
			timeoutSec, g.App.Config.GetInt("watcher.lockTimeoutMs"), maxSurge,
			&clock.Clock{},
		)
	})

	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "invalid parameter") {
			status = http.StatusBadRequest
		}
		logger.WithError(err).Error("failed to update scheduler image")
		g.App.HandleError(w, status, "failed to update scheduler image", err)
		return
	}

	logger.Info("successfully updated scheduler image")

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
}

// SchedulerUpdateMinHandler handler
type SchedulerUpdateMinHandler struct {
	App *App
}

// NewSchedulerUpdateMinHandler creates a new scheduler create handler
func NewSchedulerUpdateMinHandler(a *App) *SchedulerUpdateMinHandler {
	m := &SchedulerUpdateMinHandler{App: a}
	return m
}

// ServeHTTP method
func (g *SchedulerUpdateMinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "updateSchedulerMin",
		"scheduler": params.SchedulerName,
	})

	schedulerMin := schedulerMinParamsFromCtx(r.Context())
	if schedulerMin == nil {
		g.App.HandleError(w, http.StatusBadRequest, "min not sent on body", errors.New("min not sent on body"))
		return
	}

	logger.Info("updating scheduler's min")

	redisClient, err := redis.NewClient("extensions.redis", g.App.Config, g.App.RedisClient)
	if err != nil {
		logger.WithError(err).Error("error getting redisClient")
		g.App.HandleError(w, http.StatusInternalServerError, "Update scheduler failed", err)
		return
	}
	lockKey := fmt.Sprintf("%s-%s", g.App.Config.GetString("watcher.lockKey"), params.SchedulerName)
	timeoutSec := g.App.Config.GetInt("updateTimeoutSeconds")

	err = mr.WithSegment(models.SegmentController, func() error {
		return controller.UpdateSchedulerMin(
			l,
			mr,
			g.App.DB,
			redisClient,
			params.SchedulerName, lockKey,
			timeoutSec, g.App.Config.GetInt("watcher.lockTimeoutMs"), schedulerMin.Min,
			&clock.Clock{},
		)
	})

	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		logger.WithError(err).Error("failed to update scheduler min")
		g.App.HandleError(w, status, "failed to update scheduler min", err)
		return
	}

	logger.Info("successfully updated scheduler min")

	mr.WithSegment(models.SegmentSerialization, func() error {
		Write(w, http.StatusOK, `{"success": true}`)
		return nil
	})
}
