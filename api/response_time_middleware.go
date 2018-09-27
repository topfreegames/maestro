// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/topfreegames/extensions/middleware"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
)

// ResponseTimeMiddleware sends to a statsd the route response time
type ResponseTimeMiddleware struct {
	App  *App
	next http.Handler
}

// NewResponseTimeMiddleware returns an instance of ResponseTimeMiddleware
func NewResponseTimeMiddleware(a *App) *ResponseTimeMiddleware {
	return &ResponseTimeMiddleware{
		App: a,
	}
}

//ServeHTTP methods
func (m *ResponseTimeMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	logger.Debug("response time middleware")

	start := time.Now()

	schedulerName := mux.Vars(r)["schedulerName"]

	writerWrapper := models.NewWriterWrapper(w)
	m.next.ServeHTTP(writerWrapper, r)

	routeName, _ := mux.CurrentRoute(r).GetPathTemplate()
	reporters.Report(reportersConstants.EventHTTPResponseTime, map[string]interface{}{
		reportersConstants.TagRoute:        routeName,
		reportersConstants.TagResponseTime: time.Now().Sub(start).String(),
		reportersConstants.TagHTTPStatus:   writerWrapper.Status(),
		reportersConstants.TagScheduler:    schedulerName,
		reportersConstants.TagHostname:     eventforwarder.Hostname(),
	})
}

//SetNext handler
func (m *ResponseTimeMiddleware) SetNext(next http.Handler) {
	m.next = next
}
