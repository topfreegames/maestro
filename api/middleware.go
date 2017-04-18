// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import "net/http"

//Middleware contract
type Middleware interface {
	SetNext(http.Handler)
	http.Handler
}

// Chain applies middlewares to a http.HandlerFunc
func Chain(f http.Handler, middlewares ...Middleware) http.Handler {
	if len(middlewares) == 0 {
		return f
	}

	var last Middleware
	for i, m := range middlewares {
		if i == len(middlewares)-1 {
			m.SetNext(f)
		}

		if i > 0 {
			last.SetNext(m)
		}

		last = m
	}

	return middlewares[0]
}
