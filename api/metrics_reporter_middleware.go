// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	"net/http"

	"github.com/topfreegames/maestro/models"
)

//MetricsReporterMiddleware handles logging
type MetricsReporterMiddleware struct {
	App  *App
	Next http.Handler
}

// NewMetricsReporterMiddleware creates a new metrics reporter middleware
func NewMetricsReporterMiddleware(a *App) *MetricsReporterMiddleware {
	m := &MetricsReporterMiddleware{App: a}
	return m
}

const metricsReporterKey = contextKey("metricsReporter")

func newContextWithMetricsReporter(ctx context.Context, mr *models.MixedMetricsReporter) context.Context {
	c := context.WithValue(ctx, metricsReporterKey, mr)
	return c
}

func metricsReporterFromCtx(ctx context.Context) *models.MixedMetricsReporter {
	mr := ctx.Value(metricsReporterKey)
	if mr == nil {
		return nil
	}
	return mr.(*models.MixedMetricsReporter)
}

func (m *MetricsReporterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newContextWithMetricsReporter(r.Context(), models.NewMixedMetricsReporter())

	// Call the next middleware/handler in chain
	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

//SetNext middleware
func (m *MetricsReporterMiddleware) SetNext(next http.Handler) {
	m.Next = next
}
