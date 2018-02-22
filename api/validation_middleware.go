// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/asaskevich/govalidator"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
)

//ValidationMiddleware adds the version to the request
type ValidationMiddleware struct {
	GetPayload func() interface{}
	next       http.Handler
}

type contextKey string

const payloadString = contextKey("payload")

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(f func() interface{}) *ValidationMiddleware {
	m := &ValidationMiddleware{GetPayload: f}
	return m
}

func newContextWithPayload(ctx context.Context, payload interface{}, r *http.Request) context.Context {
	c := context.WithValue(ctx, payloadString, payload)
	return c
}

func playerEventPayloadFromCtx(ctx context.Context) *models.PlayerEventPayload {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.PlayerEventPayload)
}

func roomEventPayloadFromCtx(ctx context.Context) *models.RoomEventPayload {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.RoomEventPayload)
}

func statusPayloadFromCtx(ctx context.Context) *models.RoomStatusPayload {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.RoomStatusPayload)
}

func configYamlFromCtx(ctx context.Context) []*models.ConfigYAML {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	listParam := []*models.ConfigYAML{}
	for _, param := range payload.([]interface{}) {
		listParam = append(listParam, param.(*models.ConfigYAML))
	}
	return listParam
}

func schedulerImageParamsFromCtx(ctx context.Context) *models.SchedulerImageParams {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.SchedulerImageParams)
}

func schedulerMinParamsFromCtx(ctx context.Context) *models.SchedulerMinParams {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.SchedulerMinParams)
}

func schedulerScaleParamsFromCtx(ctx context.Context) *models.SchedulerScaleParams {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.SchedulerScaleParams)
}

func schedulerVersionParamsFromContext(ctx context.Context) *models.SchedulerVersion {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	arr := payload.([]interface{})
	if len(arr) == 0 || arr[0] == nil {
		return nil
	}
	return arr[0].(*models.SchedulerVersion)
}

//ServeHTTP method
func (m *ValidationMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := loggerFromContext(r.Context())
	listParams := []interface{}{}

	if r.Body != nil {
		defer r.Body.Close()

		btsBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			l.WithError(err).Error("Payload could not be decoded.")
			vErr := errors.NewValidationFailedError(err)
			WriteBytes(w, http.StatusUnprocessableEntity, vErr.Serialize())
			return
		}

		listBytes := bytes.Split(btsBody, []byte("---"))

		for _, bts := range listBytes {
			if strings.TrimSpace(string(bts)) == "" {
				continue
			}

			payload := m.GetPayload()
			err = yaml.Unmarshal(bts, payload)
			if err != nil {
				l.WithError(err).Error("Payload could not be decoded.")
				vErr := errors.NewValidationFailedError(err)
				WriteBytes(w, http.StatusUnprocessableEntity, vErr.Serialize())
				return
			}
			_, err = govalidator.ValidateStruct(payload)
			if err != nil {
				l.WithError(err).Error("Payload is invalid.")
				vErr := errors.NewValidationFailedError(err)
				WriteBytes(w, http.StatusUnprocessableEntity, vErr.Serialize())
				return
			}
			listParams = append(listParams, payload)
		}
	}

	c := newContextWithPayload(r.Context(), listParams, r)
	m.next.ServeHTTP(w, r.WithContext(c))
}

//SetNext handler
func (m *ValidationMiddleware) SetNext(next http.Handler) {
	m.next = next
}
