package api

import (
	"fmt"
	"net/http"
	"runtime/trace"
	"strconv"
	"time"
)

// TraceHandlerFunc is an http HandlerFunc
func TraceHandlerFunc(w http.ResponseWriter, r *http.Request) {
	sec, _ := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec == 0 {
		sec = 30
	}

	if durationExceedsWriteTimeout(r, float64(sec)) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Go-Trace", "1")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "trace duration exceeds server's WriteTimeout")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	if err := trace.Start(w); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Go-Trace", "1")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Could not trace: %s\n", err)
		return
	}
	sleep(w, time.Duration(sec)*time.Second)
	trace.Stop()
}

func sleep(w http.ResponseWriter, d time.Duration) {
	var clientGone <-chan bool
	if cn, ok := w.(http.CloseNotifier); ok {
		clientGone = cn.CloseNotify()
	}
	select {
	case <-time.After(d):
	case <-clientGone:
	}
}

func durationExceedsWriteTimeout(r *http.Request, seconds float64) bool {
	srv, ok := r.Context().Value(http.ServerContextKey).(*http.Server)
	return ok && srv.WriteTimeout != 0 && seconds >= srv.WriteTimeout.Seconds()
}
