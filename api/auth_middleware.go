// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"
	"strings"

	e "errors"

	"github.com/gorilla/mux"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
)

//AuthMiddleware ensure that this user has authorization to
//execute an operation on the scheduler
type AuthMiddleware struct {
	App     *App
	next    http.Handler
	admins  []string
	enabled bool
}

// NewAuthMiddleware returns an access middleware
// This middleware must come after BasicAuthMiddleware
// and AccessMiddleware, otherwise won't do anything
func NewAuthMiddleware(a *App) *AuthMiddleware {
	return &AuthMiddleware{
		App:     a,
		admins:  strings.Split(a.Config.GetString("users.admin"), ","),
		enabled: a.Config.GetBool("oauth.enabled"),
	}
}

//ServeHTTP methods
func (m *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := loggerFromContext(r.Context())

	if !m.enabled {
		logger.Debug("oauth disabled")
		m.next.ServeHTTP(w, r)
		return
	}

	logger.Debug("checking auth")

	isBasicAuthOK := isBasicAuthOkFromContext(r.Context())
	if isBasicAuthOK {
		logger.Debug("authorized user from basic auth")
		m.next.ServeHTTP(w, r)
		return
	}

	email := emailFromContext(r.Context())
	if email == "" {
		logger.Debug("user not sent")
		m.App.HandleError(w, http.StatusUnauthorized, "",
			errors.NewAccessError("user not sent",
				e.New("user is empty (not using basicauth nor oauth) and auth is required")))
		return
	}

	if !isAuth(email, m.admins) {
		schedulerName := mux.Vars(r)["schedulerName"]
		scheduler := models.NewScheduler(schedulerName, "", "")
		scheduler.Load(m.App.DB)
		configYaml, _ := models.NewConfigYAML(scheduler.YAML)
		if !isAuth(email, configYaml.AuthorizedUsers) {
			logger.Debug("not authorized user")
			m.App.HandleError(w, http.StatusUnauthorized, "",
				errors.NewAccessError("not authorized user",
					e.New("user is not admin and is not authorized to operate on this scheduler")))
			return
		}
	}

	logger.Debug("authorized user")
	m.next.ServeHTTP(w, r)
}

//SetNext handler
func (m *AuthMiddleware) SetNext(next http.Handler) {
	m.next = next
}
