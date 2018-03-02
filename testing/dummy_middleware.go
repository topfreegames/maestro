package testing

import "net/http"

// DummyMiddleware implements http.Handler but does nothing
type DummyMiddleware struct{}

// ServeHTTP does nothing
func (*DummyMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
