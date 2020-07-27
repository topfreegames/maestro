// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rs/cors"
)

//Write to the response and with the status code
func Write(w http.ResponseWriter, status int, text string) {
	WriteBytes(w, status, []byte(text))
}

//WriteJSON to the response and with the status code
func WriteJSON(w http.ResponseWriter, status int, body map[string]interface{}) {
	bts, _ := json.Marshal(body)
	WriteBytes(w, status, bts)
}

//WriteBytes to the response and with the status code
func WriteBytes(w http.ResponseWriter, status int, text []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(text)
}

func corsAllowedFallback(envvar, fallback string) []string {
	val, prs := os.LookupEnv(envvar)
	if prs == false {
		val = fallback
	}
	return strings.Split(val, " ")
}

func corsAllowedOrigins() []string {
	return corsAllowedFallback("CORS_ALLOWED_ORIGINS", "*")
}

func corsAllowedMethods() []string {
	return corsAllowedFallback("CORS_ALLOWED_METHODS", "GET PUT POST DELETE")
}

func corsAllowedHeaders() []string {
	return corsAllowedFallback("CORS_ALLOWED_HEADERS", "authorization")
}

func wrapHandlerWithCors(wrappedHandler http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins: corsAllowedOrigins(),
		AllowedMethods: corsAllowedMethods(),
		AllowedHeaders: corsAllowedHeaders(),
	})

	return c.Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrappedHandler.ServeHTTP(w, req)
	}))
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
