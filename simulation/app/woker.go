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

package app

import (
	"context"
	"log"
	"time"

	"git.topfreegames.com/maestro/test-service/internal/core"
)

type Worker struct {
	Worker *core.Worker

	ticker *time.Ticker
	quit   chan bool
	cancel context.CancelFunc
}

func (w *Worker) Start() {
	w.ticker = time.NewTicker(1 * time.Second)
	w.quit = make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	go func() {
		for {
			select {
			case <-w.quit:
				return
			case <-w.ticker.C:
				go w.Worker.Sync(ctx)
			}
		}
	}()
}

func (w *Worker) Stop(_ context.Context) {
	defer w.cancel()
	w.ticker.Stop()

	log.Println("Stopping worker...")
	time.Sleep(2 * time.Second) // wait ongoing sync
	w.quit <- true
	log.Println("Worker stopped")
}
