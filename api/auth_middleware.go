// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	e "errors"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/api/auth"
	"github.com/topfreegames/maestro/errors"
	"net/http"
	"strings"
)

//AuthMiddleware ensure that this user has authorization to
//execute an operation on the scheduler
type AuthMiddleware struct {
	App      *App
	next     http.Handler
	admins   []string
	resolver auth.PermissionResolver
}

// NewAuthMiddleware returns an access middleware
// This middleware must come after BasicAuthMiddleware
// and AccessMiddleware, otherwise won't do anything
func NewAuthMiddleware(a *App, resolver auth.PermissionResolver) *AuthMiddleware {
	return &AuthMiddleware{
		App:      a,
		admins:   strings.Split(a.Config.GetString("users.admin"), ","),
		resolver: resolver,
	}
}

//ServeHTTP methods
func (m *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if auth.IsBasicAuthOkFromContext(ctx) {
		m.next.ServeHTTP(w, r)
		return
	}
	if m.App.Config.GetBool("william.enabled") {
		db := m.App.DBClient.WithContext(ctx)
		logger := middleware.GetLogger(ctx)
		authorized, err := auth.CheckWilliamPermission(db, logger, m.App.William, r, m.resolver)
		if err != nil {
			m.App.HandleError(w, http.StatusInternalServerError, "internal server error", err)
			return
		}
		if !authorized {
			m.App.HandleError(w,
				http.StatusForbidden,
				"forbidden",
				errors.NewAccessError("not authorized user",
					e.New("user is not authorized to operate on this resource")),
			)
			return
		}
	} else if m.App.Config.GetBool("oauth.enabled") {
		db := m.App.DBClient.WithContext(ctx)
		logger := middleware.GetLogger(ctx)
		authorized, err := auth.CheckAuthorization(db, logger, r, m.admins)
		if err != nil {
			m.App.HandleError(w, http.StatusInternalServerError, "internal server error", err)
			return
		}
		if !authorized {
			m.App.HandleError(w,
				http.StatusUnauthorized,
				"unauthorized",
				errors.NewAccessError("not authorized user",
					e.New("user is not admin and is not authorized to operate on this scheduler")),
			)
			return
		}
	}

	m.next.ServeHTTP(w, r)
}

//SetNext handler
func (m *AuthMiddleware) SetNext(next http.Handler) {
	m.next = next
}
