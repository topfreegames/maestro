// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"errors"
	"net/http"
)

//BasicAuthMiddleware guarantees that the user is logged
type BasicAuthMiddleware struct {
	App  *App
	next http.Handler
}

func NewBasicAuthMiddleware(a *App) *BasicAuthMiddleware {
	return &BasicAuthMiddleware{
		App: a,
	}
}

//ServeHTTP methods
func (m *BasicAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := loggerFromContext(r.Context())
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
			} else {
				m.App.HandleError(w, http.StatusUnauthorized, "authentication failed", errors.New("no basic auth sent"))
				return
			}
		}
	}
	// Call the next middleware/handler in chain
	m.next.ServeHTTP(w, r)
}

//SetNext middleware
func (m *BasicAuthMiddleware) SetNext(next http.Handler) {
	m.next = next
}
