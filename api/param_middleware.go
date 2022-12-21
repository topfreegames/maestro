// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/asaskevich/govalidator"
	"github.com/gorilla/mux"
	"github.com/topfreegames/extensions/v9/middleware"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
)

//ParamMiddleware add into the model the parameters that came in the URI
type ParamMiddleware struct {
	GetParams func() interface{}
	next      http.Handler
}

const paramKey = contextKey("param")

//NewParamMiddleware constructs a new param middleware
func NewParamMiddleware(f func() interface{}) *ParamMiddleware {
	m := &ParamMiddleware{GetParams: f}
	return m
}

func newContextWithParams(ctx context.Context, params interface{}) context.Context {
	return context.WithValue(ctx, paramKey, params)
}

func schedulerParamsFromContext(ctx context.Context) *models.SchedulerParams {
	param := ctx.Value(paramKey)
	if param == nil {
		return nil
	}
	return param.(*models.SchedulerParams)
}

func schedulerRoomsParamsFromContext(ctx context.Context) *models.SchedulerRoomsParams {
	param := ctx.Value(paramKey)
	if param == nil {
		return nil
	}
	return param.(*models.SchedulerRoomsParams)
}

func schedulerRoomDetailsParamsFromContext(ctx context.Context) *models.SchedulerRoomDetailsParams {
	param := ctx.Value(paramKey)
	if param == nil {
		return nil
	}
	return param.(*models.SchedulerRoomDetailsParams)
}

func schedulerLockParamsFromContext(ctx context.Context) *models.SchedulerLockParams {
	param := ctx.Value(paramKey)
	if param == nil {
		return nil
	}
	return param.(*models.SchedulerLockParams)
}

func roomParamsFromContext(ctx context.Context) *models.RoomParams {
	param := ctx.Value(paramKey)
	if param == nil {
		return nil
	}
	return param.(*models.RoomParams)
}

func (m *ParamMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := m.GetParams()
	l := middleware.GetLogger(r.Context())
	b, err := json.Marshal(mux.Vars(r))
	if err != nil {
		l.WithError(err).Error("Params could not be decoded.")
		vErr := errors.NewValidationFailedError(err)
		WriteBytes(w, http.StatusBadRequest, vErr.Serialize())
		return
	}

	err = json.Unmarshal(b, &params)
	if err != nil {
		l.WithError(err).Error("Params could not be decoded.")
		vErr := errors.NewValidationFailedError(err)
		WriteBytes(w, http.StatusBadRequest, vErr.Serialize())
		return
	}

	_, err = govalidator.ValidateStruct(params)

	if err != nil {
		l.WithError(err).Error("Params are invalid.")
		vErr := errors.NewValidationFailedError(err)
		WriteBytes(w, http.StatusUnprocessableEntity, vErr.Serialize())
		return
	}

	c := newContextWithParams(r.Context(), params)

	m.next.ServeHTTP(w, r.WithContext(c))
}

//SetNext handler
func (m *ParamMiddleware) SetNext(next http.Handler) {
	m.next = next
}
