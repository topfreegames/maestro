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
