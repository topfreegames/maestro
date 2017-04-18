// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"net/http"

	"github.com/topfreegames/maestro/metadata"
)

// VersionMiddleware adds the version to the request
type VersionMiddleware struct {
	next http.Handler
}

// NewVersionMiddleware creates a new version middleware
func NewVersionMiddleware() *VersionMiddleware {
	m := &VersionMiddleware{}
	return m
}

//ServeHTTP method
func (m *VersionMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Maestro-Version", metadata.Version)
	m.next.ServeHTTP(w, r)
}

//SetNext handler
func (m *VersionMiddleware) SetNext(next http.Handler) {
	m.next = next
}
