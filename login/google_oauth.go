// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package login

import (
	"context"
	"os"

	"github.com/topfreegames/maestro/login/interfaces"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOauthConfig struct {
	googleOauthConfig *oauth2.Config
}

func NewGoogleOauthConfig() *GoogleOauthConfig {
	return &GoogleOauthConfig{}
}

func (g *GoogleOauthConfig) Setup() {
	g.googleOauthConfig = &oauth2.Config{
		RedirectURL: "http://localhost:57460/google-callback",
		Endpoint:    google.Endpoint,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
	}
}

func (g *GoogleOauthConfig) SetClientID(clientID string) {
	g.googleOauthConfig.ClientID = clientID
}

func (g *GoogleOauthConfig) SetClientSecret(clientSecret string) {
	g.googleOauthConfig.ClientSecret = clientSecret
}

func (g *GoogleOauthConfig) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return g.googleOauthConfig.AuthCodeURL(state, opts...)
}

func (g *GoogleOauthConfig) Exchange(ctx context.Context, code, redirectURI string) (*oauth2.Token, error) {
	var token *oauth2.Token
	var err error

	if redirectURI == "" {
		token, err = g.googleOauthConfig.Exchange(ctx, code)
	} else {
		c := &oauth2.Config{
			ClientID:     os.Getenv(clientIDEnvVar),
			ClientSecret: os.Getenv(clientSecretEnvVar),
			RedirectURL:  redirectURI,
			Endpoint:     google.Endpoint,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/userinfo.email",
			},
		}
		token, err = c.Exchange(ctx, code)
	}
	return token, err
}

func (g *GoogleOauthConfig) TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
	return g.googleOauthConfig.TokenSource(ctx, t)
}

func (g *GoogleOauthConfig) Client(ctx context.Context, t *oauth2.Token) interfaces.Client {
	return g.googleOauthConfig.Client(ctx, t)
}
