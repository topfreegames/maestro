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
	return payload.(*models.RoomStatusPayload)
}

func configYamlFromCtx(ctx context.Context) *models.ConfigYAML {
	payload := ctx.Value(payloadString)
	if payload == nil {
		return nil
	}
	return payload.(*models.ConfigYAML)
}

//ServeHTTP method
func (m *ValidationMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	payload := m.GetPayload()
	l := loggerFromContext(r.Context())

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(payload)

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

	c := newContextWithPayload(r.Context(), payload, r)

	m.next.ServeHTTP(w, r.WithContext(c))
}

//SetNext handler
func (m *ValidationMiddleware) SetNext(next http.Handler) {
	m.next = next
}
