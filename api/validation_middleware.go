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

func statusPayloadFromCtx(ctx context.Context) *models.RoomStatusPayload {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	return payload.([]interface{})[0].(*models.RoomStatusPayload)
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
	return payload.([]interface{})[0].(*models.SchedulerImageParams)
}

func schedulerMinParamsFromCtx(ctx context.Context) *models.SchedulerMinParams {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	return payload.([]interface{})[0].(*models.SchedulerMinParams)
}

//ServeHTTP method
func (m *ValidationMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	l := loggerFromContext(r.Context())

	btsBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		l.WithError(err).Error("Payload could not be decoded.")
		vErr := errors.NewValidationFailedError(err)
		WriteBytes(w, http.StatusUnprocessableEntity, vErr.Serialize())
		return
	}

	listBytes := bytes.Split(btsBody, []byte("---"))
	listParams := []interface{}{}

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
	c := newContextWithPayload(r.Context(), listParams, r)
	m.next.ServeHTTP(w, r.WithContext(c))
}

//SetNext handler
func (m *ValidationMiddleware) SetNext(next http.Handler) {
	m.next = next
}
