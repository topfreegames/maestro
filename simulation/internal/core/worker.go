// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
