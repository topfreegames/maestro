// maestro
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

//DogStatsdMiddleware handles logging
type DogStatsdMiddleware struct {
	App  *App
	Next http.Handler
}

// NewDogStatsdMiddleware creates a new newrelic middleware
func NewDogStatsdMiddleware(a *App) *DogStatsdMiddleware {
	return &DogStatsdMiddleware{App: a}
}

func (m *DogStatsdMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mr := metricsReporterFromCtx(ctx)
	if mr != nil {
		schedulerName := mux.Vars(r)["schedulerName"]
		routeName, _ := mux.CurrentRoute(r).GetPathTemplate()
		mr.AddReporter(&DogStatsdMetricsReporter{
			App:       m.App,
			Scheduler: schedulerName,
			Route:     routeName,
		})
	}

	// Call the next middleware/handler in chain
	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

//SetNext middleware
func (m *DogStatsdMiddleware) SetNext(next http.Handler) {
	m.Next = next
}
