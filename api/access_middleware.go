// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/login"
)

//AccessMiddleware guarantees that the user is logged
type AccessMiddleware struct {
	App     *App
	next    http.Handler
	enabled bool
}

// NewAccessMiddleware returns an access middleware
func NewAccessMiddleware(a *App) *AccessMiddleware {
	enabled := a.Config.GetBool("oauth.enabled")
	return &AccessMiddleware{
		App:     a,
		enabled: enabled,
	}
}

const emailKey = contextKey("emailKey")

func emailFromContext(ctx context.Context) string {
	payload := ctx.Value(emailKey)
	if payload == nil {
		return ""
	}
	return payload.(string)
}

// NewContextWithEmail adds the email from oauth into context
func NewContextWithEmail(ctx context.Context, email string) context.Context {
	c := context.WithValue(ctx, emailKey, email)
	return c
}

func basicAuthWithXForwardedUserEmail(
	basicAuthUser, basicAuthPass string, r *http.Request,
) (string, bool) {
	if basicAuthUser == "" && basicAuthPass == "" {
		return "", false
	}
	user, pass, ok := r.BasicAuth()
	if !ok || user != basicAuthUser || pass != basicAuthPass {
		return "", false
	}
	email := r.Header.Get("x-forwarded-user-email")
	if email == "" {
		return "", false
	}
	return email, true
}

//ServeHTTP methods
func (m *AccessMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.enabled {
		m.next.ServeHTTP(w, r)
		return
	}

	// checking basic auth in case of x-forwarded-user-email
	basicAuthUser := m.App.Config.GetString("basicauth.username")
	basicAuthPass := m.App.Config.GetString("basicauth.password")
	if email, ok := basicAuthWithXForwardedUserEmail(
		basicAuthUser, basicAuthPass, r,
	); ok {
		ctx := NewContextWithEmail(r.Context(), email)
		m.next.ServeHTTP(w, r.WithContext(ctx))
		return
	}

	logger := middleware.GetLogger(r.Context())
	logger.Debug("Checking access token")

	accessToken := r.Header.Get("Authorization")
	accessToken = strings.TrimPrefix(accessToken, "Bearer ")

	token, err := login.GetToken(accessToken, m.App.DBClient.WithContext(r.Context()))
	if err != nil {
		m.App.HandleError(w, http.StatusInternalServerError, "", err)
		return
	}
	if token.RefreshToken == "" {
		m.App.HandleError(
			w,
			http.StatusUnauthorized,
			"",
			errors.NewAccessError("access token was not found on db", fmt.Errorf("access token error")),
		)
		return
	}

	msg, status, err := m.App.Login.Authenticate(token, m.App.DBClient.WithContext(r.Context()))
	if err != nil {
		logger.WithError(err).Error("error fetching googleapis")
		m.App.HandleError(w, http.StatusInternalServerError, "Error fetching googleapis", err)
		return
	}

	if status == http.StatusBadRequest {
		logger.WithError(err).Error("error validating access token")
		err := errors.NewAccessError("Unauthorized access token", fmt.Errorf(msg))
		m.App.HandleError(w, http.StatusUnauthorized, "Unauthorized access token", err)
		return
	}

	if status != http.StatusOK {
		logger.WithError(err).Error("invalid access token")
		err := errors.NewAccessError("invalid access token", fmt.Errorf(msg))
		m.App.HandleError(w, status, "error validating access token", err)
		return
	}

	email := msg
	if !verifyEmailDomain(email, m.App.EmailDomains) {
		logger.WithError(err).Error("Invalid email")
		err := errors.NewAccessError(
			"authorization access error",
			fmt.Errorf("the email on OAuth authorization is not from domain %s", m.App.EmailDomains),
		)
		m.App.HandleError(w, http.StatusUnauthorized, "error validating access token", err)
		return
	}

	ctx := NewContextWithEmail(r.Context(), email)

	logger.Debug("Access token checked")
	m.next.ServeHTTP(w, r.WithContext(ctx))
}

func verifyEmailDomain(email string, emailDomains []string) bool {
	for _, domain := range emailDomains {
		if strings.HasSuffix(email, domain) {
			return true
		}
	}
	return false
}

//SetNext handler
func (m *AccessMiddleware) SetNext(next http.Handler) {
	m.next = next
}
