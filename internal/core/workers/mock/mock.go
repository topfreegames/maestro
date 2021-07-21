package mock

import "context"

type MockWorker struct {
	Run bool
}

func (d *MockWorker) Start(_ context.Context) error {
	d.Run = true
	return nil
}

func (d *MockWorker) Stop(_ context.Context) {
	d.Run = false
	return
}

func (d *MockWorker) IsRunning(_ context.Context) bool {
	return d.Run
}
