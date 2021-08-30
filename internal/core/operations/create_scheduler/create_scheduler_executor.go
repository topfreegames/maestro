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

package create_scheduler

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type CreateSchedulerExecutor struct {
	runtime ports.Runtime
	storage ports.SchedulerStorage
}

func NewExecutor(runtime ports.Runtime, storage ports.SchedulerStorage) *CreateSchedulerExecutor {
	return &CreateSchedulerExecutor{
		runtime: runtime,
		storage: storage,
	}
}

func (e *CreateSchedulerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {

	err := e.runtime.CreateScheduler(ctx, &entities.Scheduler{Name: op.SchedulerName})
	if err != nil {
		return fmt.Errorf("failed to create scheduler in runtime: %w", err)
	}

	return nil
}

func (e *CreateSchedulerExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	scheduler, err := e.storage.GetScheduler(ctx, op.SchedulerName)
	if err != nil {
		return fmt.Errorf("failed to find scheduler by id: %w", err)
	}

	scheduler.State = entities.StateOnError

	return e.storage.UpdateScheduler(ctx, scheduler)
}

func (e *CreateSchedulerExecutor) Name() string {
	return OperationName
}
