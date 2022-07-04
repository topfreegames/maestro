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

package ports

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/filters"
)

// Primary ports (input, driving ports)

type SchedulerManager interface {
	UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	GetActiveScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error)
	GetSchedulerByVersion(ctx context.Context, schedulerName, schedulerVersion string) (*entities.Scheduler, error)
	CreateNewSchedulerVersionAndEnqueueSwitchVersion(ctx context.Context, scheduler *entities.Scheduler) (string, error)
	CreateNewSchedulerVersion(ctx context.Context, scheduler *entities.Scheduler) error
	EnqueueSwitchActiveVersionOperation(ctx context.Context, schedulerName, newVersion string) (*operation.Operation, error)
	EnqueueDeleteSchedulerOperation(ctx context.Context, schedulerName string) (*operation.Operation, error)
	GetSchedulersInfo(ctx context.Context, filter *filters.SchedulerFilter) ([]*entities.SchedulerInfo, error)
	GetSchedulerVersions(ctx context.Context, schedulerName string) ([]*entities.SchedulerVersion, error)
	DeleteScheduler(ctx context.Context, schedulerName string) error
	PatchSchedulerAndCreateNewSchedulerVersionOperation(ctx context.Context, schedulerName string, patchMap map[string]interface{}) (*operation.Operation, error)
	AddRooms(ctx context.Context, schedulerName string, amount int32) (*operation.Operation, error)
	RemoveRooms(ctx context.Context, schedulerName string, amount int) (*operation.Operation, error)
	GetSchedulersWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) ([]*entities.Scheduler, error)
	GetScheduler(ctx context.Context, schedulerName, version string) (*entities.Scheduler, error)
	EnqueueNewSchedulerVersionOperation(ctx context.Context, scheduler *entities.Scheduler) (*operation.Operation, error)
	CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) (*entities.Scheduler, error)
}

// Secondary ports (output, driven ports)

type SchedulerStorage interface {
	GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error)
	GetSchedulerWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) (*entities.Scheduler, error)
	GetSchedulerVersions(ctx context.Context, name string) ([]*entities.SchedulerVersion, error)
	GetSchedulers(ctx context.Context, names []string) ([]*entities.Scheduler, error)
	GetSchedulersWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) ([]*entities.Scheduler, error)
	GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error)
	CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	DeleteScheduler(ctx context.Context, transactionID TransactionID, scheduler *entities.Scheduler) error
	CreateSchedulerVersion(ctx context.Context, transactionID TransactionID, scheduler *entities.Scheduler) error
	RunWithTransaction(ctx context.Context, transactionFunc func(transactionId TransactionID) error) error
}

type SchedulerCache interface {
	GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error)
	SetScheduler(ctx context.Context, scheduler *entities.Scheduler, ttl time.Duration) error
	DeleteScheduler(ctx context.Context, schedulerName string) error
}

type TransactionID string
