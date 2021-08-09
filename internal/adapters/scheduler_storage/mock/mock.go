// Code generated by MockGen. DO NOT EDIT.
// Source: ../internal/core/ports/scheduler_storage.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	entities "github.com/topfreegames/maestro/internal/core/entities"
)

// MockSchedulerStorage is a mock of SchedulerStorage interface
type MockSchedulerStorage struct {
	ctrl     *gomock.Controller
	recorder *MockSchedulerStorageMockRecorder
}

// MockSchedulerStorageMockRecorder is the mock recorder for MockSchedulerStorage
type MockSchedulerStorageMockRecorder struct {
	mock *MockSchedulerStorage
}

// NewMockSchedulerStorage creates a new mock instance
func NewMockSchedulerStorage(ctrl *gomock.Controller) *MockSchedulerStorage {
	mock := &MockSchedulerStorage{ctrl: ctrl}
	mock.recorder = &MockSchedulerStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockSchedulerStorage) EXPECT() *MockSchedulerStorageMockRecorder {
	return m.recorder
}

// GetScheduler mocks base method
func (m *MockSchedulerStorage) GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetScheduler", ctx, name)
	ret0, _ := ret[0].(*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetScheduler indicates an expected call of GetScheduler
func (mr *MockSchedulerStorageMockRecorder) GetScheduler(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).GetScheduler), ctx, name)
}

// GetSchedulers mocks base method
func (m *MockSchedulerStorage) GetSchedulers(ctx context.Context, names []string) ([]*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSchedulers", ctx, names)
	ret0, _ := ret[0].([]*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSchedulers indicates an expected call of GetSchedulers
func (mr *MockSchedulerStorageMockRecorder) GetSchedulers(ctx, names interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSchedulers", reflect.TypeOf((*MockSchedulerStorage)(nil).GetSchedulers), ctx, names)
}

// GetAllSchedulers mocks base method
func (m *MockSchedulerStorage) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllSchedulers", ctx)
	ret0, _ := ret[0].([]*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllSchedulers indicates an expected call of GetAllSchedulers
func (mr *MockSchedulerStorageMockRecorder) GetAllSchedulers(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllSchedulers", reflect.TypeOf((*MockSchedulerStorage)(nil).GetAllSchedulers), ctx)
}

// CreateScheduler mocks base method
func (m *MockSchedulerStorage) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateScheduler indicates an expected call of CreateScheduler
func (mr *MockSchedulerStorageMockRecorder) CreateScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).CreateScheduler), ctx, scheduler)
}

// UpdateScheduler mocks base method
func (m *MockSchedulerStorage) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateScheduler indicates an expected call of UpdateScheduler
func (mr *MockSchedulerStorageMockRecorder) UpdateScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).UpdateScheduler), ctx, scheduler)
}

// DeleteScheduler mocks base method
func (m *MockSchedulerStorage) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteScheduler indicates an expected call of DeleteScheduler
func (mr *MockSchedulerStorageMockRecorder) DeleteScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).DeleteScheduler), ctx, scheduler)
}
