// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"
	"strconv"

	"github.com/rs/cors"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

//Write to the response and with the status code
func Write(w http.ResponseWriter, status int, text string) {
	WriteBytes(w, status, []byte(text))
}

//WriteBytes to the response and with the status code
func WriteBytes(w http.ResponseWriter, status int, text []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(text)
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func wrapHandlerWithResponseWriter(wrappedHandler http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	return c.Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rw := newResponseWriter(w)
		wrappedHandler.ServeHTTP(rw, req)
	}))
}

func getStatusFromResponseWriter(w http.ResponseWriter) int {
	rw, ok := w.(*responseWriter)
	if ok {
		return rw.statusCode
	}
	return -1
}

func getMaxSurge(app *App, r *http.Request) (int, error) {
	var maxSurge int
	var err error
	maxSurgeStr := r.URL.Query().Get("maxsurge")
	if maxSurgeStr == "" {
		maxSurge = app.Config.GetInt("watcher.maxSurge")
	} else {
		maxSurge, err = strconv.Atoi(maxSurgeStr)
	}
	return maxSurge, err
}
