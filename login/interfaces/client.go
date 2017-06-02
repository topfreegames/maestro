// maestro api
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package interfaces

import "net/http"

type Client interface {
	Get(url string) (resp *http.Response, err error)
}
