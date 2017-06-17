// maestro api
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package login

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	logininterfaces "github.com/topfreegames/maestro/login/interfaces"
	"golang.org/x/oauth2"
)

type Login struct {
	GoogleOauthConfig  logininterfaces.GoogleOauthConfig
	clientIDEnvVar     string
	clientSecretEnvVar string
}

func NewLogin() *Login {
	return &Login{}
}

func (l *Login) Setup() {
	l.clientIDEnvVar = "MAESTRO_GOOGLE_CLIENT_ID"
	l.clientSecretEnvVar = "MAESTRO_GOOGLE_CLIENT_SECRET"
	googleOauthConfig := NewGoogleOauthConfig()
	googleOauthConfig.Setup()
	l.GoogleOauthConfig = googleOauthConfig
}

func (l *Login) getClientCredentials() error {
	if len(os.Getenv(l.clientIDEnvVar)) == 0 {
		return errors.NewAccessError(
			fmt.Sprintf("Undefined environment variable %s", l.clientIDEnvVar),
			fmt.Errorf(
				"Define your app's OAuth2 Client ID on %s environment variable and run again",
				l.clientIDEnvVar,
			),
		)
	}
	l.GoogleOauthConfig.SetClientID(os.Getenv(l.clientIDEnvVar))
	if len(os.Getenv(l.clientSecretEnvVar)) == 0 {
		return errors.NewAccessError(
			fmt.Sprintf("Undefined environment variable %s", l.clientSecretEnvVar),
			fmt.Errorf(
				"Define your app's OAuth2 Client Secret on %s environment variable and run again",
				l.clientSecretEnvVar,
			),
		)
	}
	l.GoogleOauthConfig.SetClientSecret(os.Getenv(l.clientSecretEnvVar))
	return nil
}

//GenerateLoginURL generates the login url using googleapis OAuth2 Client Secret and OAuth2 Client ID
func (l *Login) GenerateLoginURL(oauthState string) (string, error) {
	err := l.getClientCredentials()
	if err != nil {
		return "", err
	}
	url := l.GoogleOauthConfig.AuthCodeURL(oauthState, oauth2.AccessTypeOffline)
	return url, nil
}

//GetAccessToken exchange authorization code with access token
func (l *Login) GetAccessToken(code string) (*oauth2.Token, error) {
	token, err := l.GoogleOauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		err := errors.NewAccessError("GoogleCallback: Code exchange failed", err)
		return nil, err
	}
	if !token.Valid() {
		err := errors.NewAccessError("GoogleCallback", fmt.Errorf("Invalid token received from Authorization Server"))
		return nil, err
	}
	return token, nil
}

//Authenticate authenticates an access token or gets a new one with the refresh token
//The returned string is either the error message or the user email
func (l *Login) Authenticate(token *oauth2.Token, db interfaces.DB) (string, int, error) {
	var email string
	var status int
	err := l.getClientCredentials()
	if err != nil {
		return email, status, errors.NewAccessError("error getting access token", err)
	}
	newToken := new(oauth2.Token)
	expired := time.Now().UTC().After(token.Expiry)
	if expired {
		var err error
		newToken, err = l.GoogleOauthConfig.TokenSource(oauth2.NoContext, token).Token()
		if err != nil {
			return email, status, errors.NewAccessError("error getting access token", err)
		}
		newToken.RefreshToken = ""
	} else {
		*newToken = *token
	}
	client := l.GoogleOauthConfig.Client(oauth2.NoContext, newToken)
	url := fmt.Sprintf("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=%s", newToken.AccessToken)
	resp, err := client.Get(url)
	if err != nil {
		return email, status, errors.NewGenericError("error during get request", err)
	}
	defer resp.Body.Close()

	status = resp.StatusCode
	bts, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return email, status, errors.NewGenericError("error reading google response body", err)
	}
	if status != http.StatusOK {
		return string(bts), status, nil
	}
	var bodyObj map[string]interface{}
	json.Unmarshal(bts, &bodyObj)
	email = bodyObj["email"].(string)
	if expired {
		err = SaveToken(newToken, email, token.AccessToken, db)
		if err != nil {
			return email, http.StatusInternalServerError, errors.NewDatabaseError(err)
		}
	}
	return email, status, nil
}
