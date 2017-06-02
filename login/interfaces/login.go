// maestro api
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package interfaces

import (
	"github.com/topfreegames/extensions/pg/interfaces"
	"golang.org/x/oauth2"
)

type Login interface {
	Setup()
	GenerateLoginURL(string) (string, error)
	GetAccessToken(string) (*oauth2.Token, error)
	Authenticate(*oauth2.Token, interfaces.DB) (string, int, error)
}
