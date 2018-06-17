// maestro api
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/login"
)

//LoginUrlHandler handles login url requests
type LoginUrlHandler struct {
	App *App
}

//LoginUrlHandler handles login url requests
func NewLoginUrlHandler(a *App) *LoginUrlHandler {
	return &LoginUrlHandler{
		App: a,
	}
}

//ServeHTTP method
func (l *LoginUrlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	logger.Debug("Generating log in URL")

	oauthState := r.FormValue("state")
	if len(oauthState) == 0 {
		l.App.HandleError(w, http.StatusBadRequest, "state must not be empty", fmt.Errorf("state must not be empty"))
		return
	}

	url, err := l.App.Login.GenerateLoginURL(oauthState)
	if err != nil {
		logger.WithError(err).Errorln("undefined env vars")
		l.App.HandleError(w, http.StatusInternalServerError, "undefined env vars", err)
		return
	}

	bodyResponse := map[string]string{
		"url": url,
	}
	bts, err := json.Marshal(bodyResponse)
	if err != nil {
		logger.WithError(err).Errorln("error parsing map")
		l.App.HandleError(w, http.StatusInternalServerError, "error parsing map", err)
		return
	}

	WriteBytes(w, http.StatusOK, bts)
	logger.Debug("Login URL generated")
}

//LoginAccessHandler handles login url requests
type LoginAccessHandler struct {
	App *App
}

//LoginAccessHandler handles login url requests
func NewLoginAccessHandler(a *App) *LoginAccessHandler {
	return &LoginAccessHandler{
		App: a,
	}
}

//ServeHTTP method
func (l *LoginAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	logger.Debug("Getting access token")

	authCode := r.FormValue("code")
	if len(authCode) == 0 {
		l.App.HandleError(w, http.StatusBadRequest, "code must not be empty", fmt.Errorf("code must not be empty"))
		return
	}
	redirectURI := r.FormValue("redirect_uri")

	token, err := l.App.Login.GetAccessToken(authCode, redirectURI)
	if err != nil {
		l.App.HandleError(w, http.StatusBadRequest, "failed to get access token", fmt.Errorf("failed to get access token"))
		return
	}

	db := l.App.DBClient.WithContext(r.Context())
	//If the last error didn't occur, then the error from Authenticate method won't happen
	email, _, _ := l.App.Login.Authenticate(token, db)
	if !verifyEmailDomain(email, l.App.EmailDomains) {
		logger.WithError(err).Error("Invalid email")
		err := errors.NewAccessError(
			"authorization access error",
			fmt.Errorf("the email on OAuth authorization is not from domain %s", l.App.EmailDomains),
		)
		l.App.HandleError(w, http.StatusUnauthorized, "error validating access token", err)
		return
	}

	err = login.SaveToken(token, email, token.AccessToken, db)
	if err != nil {
		l.App.HandleError(w, http.StatusBadRequest, "", err)
		return
	}

	keyAccessToken, err := login.GetKeyAccessToken(email, db)

	body := fmt.Sprintf(`{"token": "%s"}`, keyAccessToken)

	Write(w, http.StatusOK, body)
	logger.Debug("Returning access token")
}
