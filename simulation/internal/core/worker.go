package core

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Worker struct {
	room   *Room // No problem with concurrent reads, next loop will have updated values
	http   *http.Client
	config *Config
}

func NewWorker(room *Room, config *Config) *Worker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.IdleConnTimeout = 10 * time.Second

	return &Worker{
		config: config,
		room:   room,
		http: &http.Client{
			Timeout:   1 * time.Second,
			Transport: transport,
		},
	}
}

func (w *Worker) Sync(ctx context.Context) {
	jbody := []byte(fmt.Sprintf(`{"status": "%s", "runningMatches": %d, "timestamp": %d}`, w.room.Status.String(), w.room.RunningMatches, time.Now().Unix()))
	body := bytes.NewReader(jbody)

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/scheduler/%s/rooms/%s/ping", w.config.URL, w.room.Scheduler, w.room.ID), body)
	if err != nil {
		log.Printf("failed to create request: %s", err.Error())
		return
	}

	_, err = w.http.Do(request)
	if err != nil {
		log.Printf("failed to send request: %s", err.Error())
		return
	}
}
