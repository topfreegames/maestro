package mock

import (
	"context"
)

type MockWorker struct {
	Run    bool
	StopCh chan struct{}
}

func (d *MockWorker) Start(_ context.Context) error {
	d.Run = true
	<-d.StopCh
	return nil
}

func (d *MockWorker) Stop(_ context.Context) {
	d.StopCh <- struct{}{}
	d.Run = false
}

func (d *MockWorker) IsRunning() bool {
	return d.Run
}
