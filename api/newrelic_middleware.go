// maestro
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	newrelic "github.com/newrelic/go-agent"
)

//NewRelicMiddleware handles logging
type NewRelicMiddleware struct {
	App  *App
	Next http.Handler
}

// NewNewRelicMiddleware creates a new newrelic middleware
func NewNewRelicMiddleware(a *App) *NewRelicMiddleware {
	m := &NewRelicMiddleware{App: a}
	return m
}

const newRelicTransactionKey = contextKey("newRelicTransaction")

func newContextWithNewRelicTransaction(ctx context.Context, txn newrelic.Transaction, r *http.Request) context.Context {
	c := context.WithValue(ctx, newRelicTransactionKey, txn)
	return c
}

func (m *NewRelicMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if m.App.NewRelic != nil {
		route, _ := mux.CurrentRoute(r).GetPathTemplate()
		txn := m.App.NewRelic.StartTransaction(fmt.Sprintf("%s %s", r.Method, route), w, r)
		defer txn.End()
		ctx = newContextWithNewRelicTransaction(r.Context(), txn, r)

		mr := metricsReporterFromCtx(ctx)
		if mr != nil {
			mr.AddReporter(&NewRelicMetricsReporter{
				App:         m.App,
				Transaction: txn,
			})
		}
	}

	// Call the next middleware/handler in chain
	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

//SetNext middleware
func (m *NewRelicMiddleware) SetNext(next http.Handler) {
	m.Next = next
}
