// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package interfaces

import (
	"context"

	"golang.org/x/oauth2"
)

type GoogleOauthConfig interface {
	SetClientID(clientID string)
	SetClientSecret(clientSecret string)
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource
	Client(ctx context.Context, t *oauth2.Token) Client
}
