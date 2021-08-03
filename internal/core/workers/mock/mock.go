package mock

import (
	"context"
	"time"
)

type MockWorker struct {
	Run           bool
	SleepDuration time.Duration
}

func (d *MockWorker) Start(_ context.Context) error {
	d.Run = true
	time.Sleep(d.SleepDuration)
	return nil
}

func (d *MockWorker) Stop(_ context.Context) {
	d.Run = false
}

func (d *MockWorker) IsRunning() bool {
	return d.Run
}
