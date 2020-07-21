// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"github.com/topfreegames/extensions/middleware"
	"net/http"
	"strings"
)

type WilliamMiddleware struct {
	App      *App
	Resolver PermissionResolver
	next     http.Handler
	enabled  bool
}

func NewWilliamMiddleware(app *App, resolver PermissionResolver) *WilliamMiddleware {
	return &WilliamMiddleware{
		App:      app,
		Resolver: resolver,
		enabled:  app.Config.GetBool("william.enabled"),
	}
}

func (m *WilliamMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	if !m.enabled {
		logger.Debug("william disabled")
		m.next.ServeHTTP(w, r)
		return
	}

	token := r.Header.Get("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	if len(token) == 0 {
		m.App.HandleError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	permission, resource, err := m.Resolver.ResolvePermission(m.App, r)
	if err != nil {
		logger.WithError(err).Error("error resolving permission")
		m.App.HandleError(w, http.StatusInternalServerError, "internal server error", nil)
		return
	}

	hasPermission, err := m.App.William.Check(token, permission, resource)
	if err != nil {
		logger.WithError(err).Error("error checking permission")
		m.App.HandleError(w, http.StatusInternalServerError, "internal server error", nil)
		return
	}

	if !hasPermission {
		m.App.HandleError(w, http.StatusForbidden, "forbidden", nil)
		return
	}
	m.next.ServeHTTP(w, r)
}

//SetNext handler
func (m *WilliamMiddleware) SetNext(next http.Handler) {
	m.next = next
}
