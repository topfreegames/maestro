// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"fmt"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/api/auth"
	"net/http"
	"strings"

	"github.com/topfreegames/maestro/errors"
)

//AccessMiddleware guarantees that the user is logged
type AccessMiddleware struct {
	App  *App
	next http.Handler
}

// NewAccessMiddleware returns an access middleware
func NewAccessMiddleware(a *App) *AccessMiddleware {
	return &AccessMiddleware{
		App: a,
	}
}

var emptyErr = fmt.Errorf("")

//ServeHTTP methods
func (m *AccessMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.GetLogger(ctx)
	if m.App.Config.GetBool("basicauth.enabled") {
		logger.Debug("checking basic auth")
		result, email := auth.CheckBasicAuth(m.App.Config, r)

		// Basic Auth is valid, proceed to next handler
		if result == auth.AuthenticationOk {
			logger.Debug("basic auth ok")
			ctx = auth.NewContextWithEmail(ctx, email)
			ctx = auth.NewContextWithBasicAuthOK(ctx)
			m.next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Basic Auth is invalid, return unauthorized
		if result == auth.AuthenticationInvalid {
			logger.Debug("basic auth invalid")
			m.App.HandleError(w, http.StatusUnauthorized, "authentication failed", fmt.Errorf("invalid basic auth"))
			return
		}

		logger.Debug("basic auth missing")
		// If basic auth is missing and `basicAuth.tryOauthIfUnset` is false return unauthorized
		if !m.App.Config.GetBool("basicAuth.tryOauthIfUnset") {
			logger.Debug("basic auth tryOauthIfUnset is false so unauthorized")
			m.App.HandleError(w, http.StatusUnauthorized, "authentication failed", fmt.Errorf("no basic auth sent"))
			return
		}
	}

	if m.App.Config.GetBool("william.enabled") {
		logger.Debug("william enabled, checking token")
		token := r.Header.Get("Authorization")
		if len(token) == 0 {
			logger.Debug("token empty")
			m.App.HandleError(w, http.StatusUnauthorized, "", errors.NewAccessError("missing access token", emptyErr))
			return
		}

		token = strings.TrimPrefix(token, "Bearer ")

		if len(token) == 0 {
			logger.Debug("no bearer token")
			m.App.HandleError(w, http.StatusUnauthorized, "", errors.NewAccessError("Unauthorized access token", emptyErr))
			return
		}

		logger.Debug("token received")
	} else if m.App.Config.GetBool("oauth.enabled") {
		logger.Debug("oauth enabled, checking token")

		result, email, err := auth.CheckOauthToken(m.App.Login, m.App.DBClient.WithContext(ctx), logger, r, m.App.EmailDomains)
		if err != nil {
			if result == auth.AuthenticationError {
				logger.Debug("authentication error")
				m.App.HandleError(w, http.StatusInternalServerError, "", err)
			} else {
				logger.Debug("authentication invalid")
				m.App.HandleError(w, http.StatusUnauthorized, "", err)
			}
			return
		}

		logger.Debug("token authenticated, putting email on context", email)

		ctx = auth.NewContextWithEmail(r.Context(), email)
		m.next.ServeHTTP(w, r.WithContext(ctx))
		return
	}

	m.next.ServeHTTP(w, r)
}

//SetNext handler
func (m *AccessMiddleware) SetNext(next http.Handler) {
	m.next = next
}
