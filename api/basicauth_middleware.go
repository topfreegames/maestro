// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/topfreegames/extensions/middleware"
)

//BasicAuthMiddleware guarantees that the user is logged
type BasicAuthMiddleware struct {
	App  *App
	next http.Handler
}

// NewBasicAuthMiddleware returns an instance of basicauth
func NewBasicAuthMiddleware(a *App) *BasicAuthMiddleware {
	return &BasicAuthMiddleware{
		App: a,
	}
}

const basicPayloadString = contextKey("basicPayload")

// NewContextWithBasicAuthOK gets if basic auth was sent and is ok
func NewContextWithBasicAuthOK(ctx context.Context) context.Context {
	c := context.WithValue(ctx, basicPayloadString, true)
	return c
}

func isBasicAuthOkFromContext(ctx context.Context) bool {
	payload := ctx.Value(basicPayloadString)
	if payload == nil {
		return false
	}

	return payload.(bool)
}

//ServeHTTP methods
func (m *BasicAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	logger.Debug("checking basic auth")

	basicAuthUser := m.App.Config.GetString("basicauth.username")
	basicAuthPass := m.App.Config.GetString("basicauth.password")
	if basicAuthUser != "" && basicAuthPass != "" {
		user, pass, ok := r.BasicAuth()
		if ok {
			if user != basicAuthUser || pass != basicAuthPass {
				m.App.HandleError(w, http.StatusUnauthorized, "authentication failed", errors.New("invalid basic auth"))
				return
			}
		} else {
			if m.App.Config.GetBool("basicauth.tryOauthIfUnset") {
				nextMiddleware := NewAccessMiddleware(m.App)
				nextMiddleware.SetNext(m.next)
				nextMiddleware.ServeHTTP(w, r)
				return
			}

			m.App.HandleError(w, http.StatusUnauthorized, "authentication failed", errors.New("no basic auth sent"))
			return
		}
	}

	email := r.Header.Get("x-forwarded-user-email")
	ctxWithEmail := NewContextWithEmail(r.Context(), email)
	ctxWithBasicAuth := NewContextWithBasicAuthOK(ctxWithEmail)

	// Call the next middleware/handler in chain
	m.next.ServeHTTP(w, r.WithContext(ctxWithBasicAuth))
}

//SetNext middleware
func (m *BasicAuthMiddleware) SetNext(next http.Handler) {
	m.next = next
}
