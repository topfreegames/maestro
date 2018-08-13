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

	maestroErrors "github.com/topfreegames/maestro/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/clock"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/controller"
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
	l := middleware.GetLogger(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	configs := configYamlFromCtx(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "create",
	})

	email := emailFromContext(r.Context())
	db := g.App.DBClient.WithContext(r.Context())
	for _, payload := range configs {
		logger.Debug("Creating scheduler...")

		if email != "" && !isAuth(email, payload.AuthorizedUsers) {
			if payload.AuthorizedUsers == nil {
				payload.AuthorizedUsers = []string{}
			}

			payload.AuthorizedUsers = append(payload.AuthorizedUsers, email)
		}

		timeoutSec := g.App.Config.GetInt("scaleUpTimeoutSeconds")
		err := controller.CreateScheduler(
			l,
			g.App.RoomManager,
			mr,
			db,
			g.App.RedisClient.Trace(r.Context()),
			g.App.KubernetesClient,
			payload,
			timeoutSec,
		)
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
			r.Context(),
			g.App.Forwarders,
			db,
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
	Write(w, http.StatusCreated, `{"success": true}`)
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
	l := middleware.GetLogger(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "delete",
		"scheduler": params.SchedulerName,
	})

	logger.Info("deleting scheduler")

	timeoutSec := g.App.Config.GetInt("deleteTimeoutSeconds")
	db := g.App.DBClient.WithContext(r.Context())
	err := controller.DeleteScheduler(
		l,
		mr,
		db,
		g.App.RedisClient.Trace(r.Context()),
		g.App.KubernetesClient,
		params.SchedulerName,
		timeoutSec,
	)
	if err != nil {
		logger.WithError(err).Error("delete scheduler failed")
		status := http.StatusInternalServerError
		if _, ok := err.(*maestroErrors.ValidationFailedError); ok {
			status = http.StatusNotFound
		}
		g.App.HandleError(w, status, "delete scheduler failed", err)
		return
	}

	Write(w, http.StatusOK, `{"success": true}`)
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

func (g *SchedulerUpdateHandler) update(
	r *http.Request,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	configYaml *models.ConfigYAML,
	operationManager *models.OperationManager,
) (status int, description string, err error) {
	g.App.gracefulShutdown.wg.Add(1)
	defer g.App.gracefulShutdown.wg.Done()

	status, description, err =
		updateSchedulerConfigCommon(r, g.App, logger, mr, configYaml, operationManager)
	if err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"description": description,
			"status":      status,
		}).Error("error updating scheduler config")
	}
	finishOpErr := mr.WithSegment(models.SegmentPipeExec, func() error {
		return operationManager.Finish(status, description, err)
	})
	if finishOpErr != nil {
		logger.WithError(err).Error("error saving the results on redis")
	}
	return
}

// ServeHTTP method
func (g *SchedulerUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.App.gracefulShutdown.wg.Add(1)
	defer g.App.gracefulShutdown.wg.Done()

	l := middleware.GetLogger(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	params := schedulerParamsFromContext(r.Context())
	payload := configYamlFromCtx(r.Context())[0]
	email := emailFromContext(r.Context())

	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "update",
		"scheduler": params.SchedulerName,
		"user":      email,
	})
	logger.Info("updating scheduler")

	if params.SchedulerName != payload.Name {
		msg := fmt.Sprintf("url name %s doesn't match payload name %s", params.SchedulerName, payload.Name)
		err := maestroErrors.NewValidationFailedError(errors.New(msg))
		g.App.HandleError(w, http.StatusBadRequest, "Update scheduler failed", err)
		return
	}

	operationManager, err := getOperationManager(r.Context(), g.App, payload.Name, "UpdateSchedulerConfig", logger, mr)
	if returnIfOperationManagerExists(g.App, w, err) {
		logger.WithError(err).Error("Update scheduler failed: error getting operation key")
		return
	}

	async := r.URL.Query().Get("async") == "true"
	if async {
		go g.update(r, logger, mr, payload, operationManager)
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":      true,
			"operationKey": operationManager.GetOperationKey(),
		})
		return
	}

	status, description, err := g.update(r, logger, mr, payload, operationManager)
	if err != nil {
		logger.WithError(err).Error("Update scheduler failed.")
		g.App.HandleError(w, status, description, err)
		return
	}

	Write(w, http.StatusOK, `{"success": true}`)
}

func updateSchedulerConfigCommon(
	r *http.Request,
	app *App,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	configYaml *models.ConfigYAML,
	operationManager *models.OperationManager,
) (status int, description string, err error) {
	maxSurge, err := getMaxSurge(app, r)
	if err != nil {
		return http.StatusBadRequest, "invalid maxsurge parameter", err
	}

	db := app.DBClient.WithContext(r.Context())
	logger.WithField("time", time.Now()).Info("Starting update")
	err = controller.UpdateSchedulerConfig(
		r.Context(),
		logger,
		app.RoomManager,
		mr,
		db,
		app.RedisClient,
		app.KubernetesClient,
		configYaml,
		maxSurge,
		&clock.Clock{},
		nil,
		app.Config,
		operationManager,
	)
	logger.WithField("time", time.Now()).Info("finished update")

	if err != nil {
		status := http.StatusInternalServerError
		description = "update scheduler failed"
		if _, ok := err.(*maestroErrors.ValidationFailedError); ok {
			status = http.StatusNotFound
			description = "maestro validation error"
		} else if err.Error() == "node without label error" {
			status = http.StatusUnprocessableEntity
			description = "no nodes with label"
		} else if strings.Contains(err.Error(), "invalid parameter") {
			status = http.StatusBadRequest
			description = "invalid parameters"
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
			description = "scheduler not found"
		}
		logger.WithError(err).Error("Update scheduler failed.")

		return status, description, err
	}

	// this forwards the metadata configured for each enabled forwarder
	_, err = eventforwarder.ForwardRoomInfo(
		r.Context(),
		app.Forwarders,
		db,
		app.KubernetesClient,
		configYaml.Name,
		nil, // intentionally omit SchedulerCache to force reload since it is an update
		app.Logger,
	)

	if err != nil {
		logger.WithError(err).Error("Room info forward failed.")
	}

	logger.Info("update scheduler succeeded")
	return http.StatusOK, "", nil
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
	l := middleware.GetLogger(r.Context())
	mr := metricsReporterFromCtx(r.Context())
	logger := l.WithFields(logrus.Fields{
		"source":    "schedulerHandler",
		"operation": "list",
	})
	db := g.App.DBClient.WithContext(r.Context())
	names, err := controller.ListSchedulersNames(l, mr, db)
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

	WriteBytes(w, http.StatusOK, bts)
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
	l := middleware.GetLogger(r.Context())
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

	db := g.App.DBClient.WithContext(r.Context())
	logger.Debugf("Getting scheduler %s status", params.SchedulerName)
	scheduler, _, roomCountByStatus, err := controller.GetSchedulerScalingInfo(
		l,
		mr,
		db,
		g.App.RedisClient.Trace(r.Context()),
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

	WriteBytes(w, http.StatusOK, bts)
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

	db := g.App.DBClient.WithContext(r.Context())
	var yamlStr string
	err := mr.WithSegment(models.SegmentSelect, func() error {
		var err error
		yamlStr, err = models.LoadConfig(db, schedulerName)
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

	if len(schedulersNames) == 0 {
		bts, _ := json.Marshal([]map[string]interface{}{})
		WriteBytes(w, http.StatusOK, bts)
		logger.Debug("get schedulers infos succeeded")
		return
	}

	db := g.App.DBClient.WithContext(r.Context())
	var schedulers []models.Scheduler
	err := mr.WithSegment(models.SegmentSelect, func() error {
		var err error
		schedulers, err = models.LoadSchedulers(db, schedulersNames)
		return err
	})
	if err != nil {
		errStr := "failed to get schedulers from DB"
		logger.WithError(err).Error(errStr)
		g.App.HandleError(w, http.StatusInternalServerError, errStr, err)
		return
	}

	roomsCounts, err := models.GetRoomsCountByStatusForSchedulers(
		g.App.RedisClient.Trace(r.Context()),
		schedulersNames,
	)
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
			"roomsReady":                  roomsCounts[s.Name].Ready,
			"roomsOccupied":               roomsCounts[s.Name].Occupied,
			"roomsCreating":               roomsCounts[s.Name].Creating,
			"roomsTerminating":            roomsCounts[s.Name].Terminating,
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

	WriteBytes(w, http.StatusOK, bts)
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
	l := middleware.GetLogger(r.Context())
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

	db := g.App.DBClient.WithContext(r.Context())
	err := controller.ScaleScheduler(
		logger,
		g.App.RoomManager,
		mr,
		db,
		g.App.RedisClient.Trace(r.Context()),
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
	Write(w, http.StatusOK, `{"success": true}`)
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

func (g *SchedulerImageHandler) update(
	r *http.Request,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	schedulerName string,
	imageParams *models.SchedulerImageParams,
	maxSurge int,
	operationManager *models.OperationManager,
) (status int, description string, err error) {
	g.App.gracefulShutdown.wg.Add(1)
	defer g.App.gracefulShutdown.wg.Done()

	status = http.StatusOK

	db := g.App.DBClient.WithContext(r.Context())
	err = controller.UpdateSchedulerImage(r.Context(), logger, g.App.RoomManager, mr, db, g.App.RedisClient, g.App.KubernetesClient,
		schedulerName, imageParams, maxSurge, &clock.Clock{}, g.App.Config, operationManager)
	if err != nil {
		status = http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "invalid parameter") {
			status = http.StatusBadRequest
		}

		description = "failed to update scheduler image"
		logger.WithError(err).Error(description)
	}
	finishOpErr := mr.WithSegment(models.SegmentPipeExec, func() error {
		return operationManager.Finish(status, description, err)
	})
	if finishOpErr != nil {
		logger.WithError(err).Error("error saving the results on redis")
	}

	return
}

// ServeHTTP method
func (g *SchedulerImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.App.gracefulShutdown.wg.Add(1)
	defer g.App.gracefulShutdown.wg.Done()

	l := middleware.GetLogger(r.Context())
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

	async := r.URL.Query().Get("async") == "true"
	operationManager, err := getOperationManager(r.Context(), g.App, params.SchedulerName, "UpdateSchedulerImage", logger, mr)
	if returnIfOperationManagerExists(g.App, w, err) {
		logger.WithError(err).Error("Update scheduler image failed")
		return
	}

	if async {
		go g.update(r, logger, mr, params.SchedulerName, schedulerImage, maxSurge, operationManager)
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":      true,
			"operationKey": operationManager.GetOperationKey(),
		})
		return
	}

	status, description, err := g.update(r, logger, mr, params.SchedulerName, schedulerImage, maxSurge, operationManager)
	if err != nil {
		g.App.HandleError(w, status, description, err)
		return
	}
	Write(w, http.StatusOK, `{"success": true}`)
	logger.Info("successfully updated scheduler image")
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

func (g *SchedulerUpdateMinHandler) update(
	r *http.Request,
	w http.ResponseWriter,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	schedulerName string,
	schedulerMin int,
	operationManager *models.OperationManager,
) (status int, description string, err error) {
	g.App.gracefulShutdown.wg.Add(1)
	defer g.App.gracefulShutdown.wg.Done()

	status = http.StatusOK
	description = ""

	logger.Info("starting controllers update min")
	db := g.App.DBClient.WithContext(r.Context())
	err = controller.UpdateSchedulerMin(r.Context(), logger, g.App.RoomManager, mr, db, g.App.RedisClient, schedulerName,
		schedulerMin, &clock.Clock{}, g.App.Config, operationManager)
	if err != nil {
		status = http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			description = "scheduler not found"
			status = http.StatusNotFound
		}
		logger.WithError(err).Error("error updating scheduler min")
	} else {
		logger.Info("finished with success controllers update min")
	}
	finishOpErr := mr.WithSegment(models.SegmentPipeExec, func() error {
		return operationManager.Finish(status, description, err)
	})
	if finishOpErr != nil {
		logger.WithError(err).Error("error saving the results on redis")
	}

	return status, description, err
}

// ServeHTTP method
func (g *SchedulerUpdateMinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.App.gracefulShutdown.wg.Add(1)
	defer g.App.gracefulShutdown.wg.Done()

	l := middleware.GetLogger(r.Context())
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

	async := r.URL.Query().Get("async") == "true"

	operationManager, err := getOperationManager(r.Context(), g.App, params.SchedulerName, "UpdateSchedulerMin", logger, mr)
	if returnIfOperationManagerExists(g.App, w, err) {
		logger.WithError(err).Error("Update scheduler min failed")
		return
	}

	if async {
		go g.update(r, w, logger, mr, params.SchedulerName, schedulerMin.Min, operationManager)
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":      true,
			"operationKey": operationManager.GetOperationKey(),
		})
		return
	}

	status, description, err := g.update(r, w, logger, mr, params.SchedulerName, schedulerMin.Min, operationManager)
	if err != nil {
		g.App.HandleError(w, status, description, err)
		return
	}
	Write(w, http.StatusOK, `{"success": true}`)
	logger.Info("successfully updated scheduler min")
}
