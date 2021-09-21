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

package runtime_watcher_worker

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/workers"
)

var _ workers.Worker = (*runtimeWatcherWorker)(nil)

// runtimeWatcherWorker is a work that will watch for changes on the Runtime
// and apply to keep the Maestro state up-to-date. It is not expected to perform
// any action over the game rooms or the scheduler.
type runtimeWatcherWorker struct{}

func NewRuntimeWatcherWorker(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
	return &runtimeWatcherWorker{}
}

func (w *runtimeWatcherWorker) Start(_ context.Context) error {
	return nil
}

func (w *runtimeWatcherWorker) Stop(_ context.Context) {
}

func (w *runtimeWatcherWorker) IsRunning() bool {
	return true
}
