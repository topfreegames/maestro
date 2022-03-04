// Code generated by MockGen. DO NOT EDIT.
// Source: ../internal/core/ports/scheduler_ports.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	entities "github.com/topfreegames/maestro/internal/core/entities"
	operation "github.com/topfreegames/maestro/internal/core/entities/operation"
	filters "github.com/topfreegames/maestro/internal/core/filters"
	ports "github.com/topfreegames/maestro/internal/core/ports"
)

// MockSchedulerManager is a mock of SchedulerManager interface.
type MockSchedulerManager struct {
	ctrl     *gomock.Controller
	recorder *MockSchedulerManagerMockRecorder
}

// MockSchedulerManagerMockRecorder is the mock recorder for MockSchedulerManager.
type MockSchedulerManagerMockRecorder struct {
	mock *MockSchedulerManager
}

// NewMockSchedulerManager creates a new mock instance.
func NewMockSchedulerManager(ctrl *gomock.Controller) *MockSchedulerManager {
	mock := &MockSchedulerManager{ctrl: ctrl}
	mock.recorder = &MockSchedulerManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSchedulerManager) EXPECT() *MockSchedulerManagerMockRecorder {
	return m.recorder
}

// CreateNewSchedulerVersion mocks base method.
func (m *MockSchedulerManager) CreateNewSchedulerVersion(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateNewSchedulerVersion", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateNewSchedulerVersion indicates an expected call of CreateNewSchedulerVersion.
func (mr *MockSchedulerManagerMockRecorder) CreateNewSchedulerVersion(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNewSchedulerVersion", reflect.TypeOf((*MockSchedulerManager)(nil).CreateNewSchedulerVersion), ctx, scheduler)
}

// CreateNewSchedulerVersionAndEnqueueSwitchVersion mocks base method.
func (m *MockSchedulerManager) CreateNewSchedulerVersionAndEnqueueSwitchVersion(ctx context.Context, scheduler *entities.Scheduler, replacePods bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateNewSchedulerVersionAndEnqueueSwitchVersion", ctx, scheduler, replacePods)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateNewSchedulerVersionAndEnqueueSwitchVersion indicates an expected call of CreateNewSchedulerVersionAndEnqueueSwitchVersion.
func (mr *MockSchedulerManagerMockRecorder) CreateNewSchedulerVersionAndEnqueueSwitchVersion(ctx, scheduler, replacePods interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNewSchedulerVersionAndEnqueueSwitchVersion", reflect.TypeOf((*MockSchedulerManager)(nil).CreateNewSchedulerVersionAndEnqueueSwitchVersion), ctx, scheduler, replacePods)
}

// DeleteScheduler mocks base method.
func (m *MockSchedulerManager) DeleteScheduler(ctx context.Context, schedulerName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteScheduler", ctx, schedulerName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteScheduler indicates an expected call of DeleteScheduler.
func (mr *MockSchedulerManagerMockRecorder) DeleteScheduler(ctx, schedulerName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteScheduler", reflect.TypeOf((*MockSchedulerManager)(nil).DeleteScheduler), ctx, schedulerName)
}

// EnqueueSwitchActiveVersionOperation mocks base method.
func (m *MockSchedulerManager) EnqueueSwitchActiveVersionOperation(ctx context.Context, newScheduler *entities.Scheduler, replacePods bool) (*operation.Operation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnqueueSwitchActiveVersionOperation", ctx, newScheduler, replacePods)
	ret0, _ := ret[0].(*operation.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EnqueueSwitchActiveVersionOperation indicates an expected call of EnqueueSwitchActiveVersionOperation.
func (mr *MockSchedulerManagerMockRecorder) EnqueueSwitchActiveVersionOperation(ctx, newScheduler, replacePods interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnqueueSwitchActiveVersionOperation", reflect.TypeOf((*MockSchedulerManager)(nil).EnqueueSwitchActiveVersionOperation), ctx, newScheduler, replacePods)
}

// GetActiveScheduler mocks base method.
func (m *MockSchedulerManager) GetActiveScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActiveScheduler", ctx, schedulerName)
	ret0, _ := ret[0].(*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActiveScheduler indicates an expected call of GetActiveScheduler.
func (mr *MockSchedulerManagerMockRecorder) GetActiveScheduler(ctx, schedulerName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActiveScheduler", reflect.TypeOf((*MockSchedulerManager)(nil).GetActiveScheduler), ctx, schedulerName)
}

// GetSchedulersInfo mocks base method.
func (m *MockSchedulerManager) GetSchedulersInfo(ctx context.Context, filter *filters.SchedulerFilter) ([]*entities.SchedulerInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSchedulersInfo", ctx, filter)
	ret0, _ := ret[0].([]*entities.SchedulerInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSchedulersInfo indicates an expected call of GetSchedulersInfo.
func (mr *MockSchedulerManagerMockRecorder) GetSchedulersInfo(ctx, filter interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSchedulersInfo", reflect.TypeOf((*MockSchedulerManager)(nil).GetSchedulersInfo), ctx, filter)
}

// SwitchActiveVersion mocks base method.
func (m *MockSchedulerManager) SwitchActiveVersion(ctx context.Context, schedulerName, targetVersion string) (*operation.Operation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SwitchActiveVersion", ctx, schedulerName, targetVersion)
	ret0, _ := ret[0].(*operation.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SwitchActiveVersion indicates an expected call of SwitchActiveVersion.
func (mr *MockSchedulerManagerMockRecorder) SwitchActiveVersion(ctx, schedulerName, targetVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SwitchActiveVersion", reflect.TypeOf((*MockSchedulerManager)(nil).SwitchActiveVersion), ctx, schedulerName, targetVersion)
}

// UpdateScheduler mocks base method.
func (m *MockSchedulerManager) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateScheduler indicates an expected call of UpdateScheduler.
func (mr *MockSchedulerManagerMockRecorder) UpdateScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateScheduler", reflect.TypeOf((*MockSchedulerManager)(nil).UpdateScheduler), ctx, scheduler)
}

// MockSchedulerStorage is a mock of SchedulerStorage interface.
type MockSchedulerStorage struct {
	ctrl     *gomock.Controller
	recorder *MockSchedulerStorageMockRecorder
}

// MockSchedulerStorageMockRecorder is the mock recorder for MockSchedulerStorage.
type MockSchedulerStorageMockRecorder struct {
	mock *MockSchedulerStorage
}

// NewMockSchedulerStorage creates a new mock instance.
func NewMockSchedulerStorage(ctrl *gomock.Controller) *MockSchedulerStorage {
	mock := &MockSchedulerStorage{ctrl: ctrl}
	mock.recorder = &MockSchedulerStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSchedulerStorage) EXPECT() *MockSchedulerStorageMockRecorder {
	return m.recorder
}

// CreateScheduler mocks base method.
func (m *MockSchedulerStorage) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateScheduler indicates an expected call of CreateScheduler.
func (mr *MockSchedulerStorageMockRecorder) CreateScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).CreateScheduler), ctx, scheduler)
}

// CreateSchedulerVersion mocks base method.
func (m *MockSchedulerStorage) CreateSchedulerVersion(ctx context.Context, transactionID ports.TransactionID, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateSchedulerVersion", ctx, transactionID, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateSchedulerVersion indicates an expected call of CreateSchedulerVersion.
func (mr *MockSchedulerStorageMockRecorder) CreateSchedulerVersion(ctx, transactionID, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSchedulerVersion", reflect.TypeOf((*MockSchedulerStorage)(nil).CreateSchedulerVersion), ctx, transactionID, scheduler)
}

// DeleteScheduler mocks base method.
func (m *MockSchedulerStorage) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteScheduler indicates an expected call of DeleteScheduler.
func (mr *MockSchedulerStorageMockRecorder) DeleteScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).DeleteScheduler), ctx, scheduler)
}

// GetAllSchedulers mocks base method.
func (m *MockSchedulerStorage) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllSchedulers", ctx)
	ret0, _ := ret[0].([]*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllSchedulers indicates an expected call of GetAllSchedulers.
func (mr *MockSchedulerStorageMockRecorder) GetAllSchedulers(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllSchedulers", reflect.TypeOf((*MockSchedulerStorage)(nil).GetAllSchedulers), ctx)
}

// GetScheduler mocks base method.
func (m *MockSchedulerStorage) GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetScheduler", ctx, name)
	ret0, _ := ret[0].(*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetScheduler indicates an expected call of GetScheduler.
func (mr *MockSchedulerStorageMockRecorder) GetScheduler(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).GetScheduler), ctx, name)
}

// GetSchedulerVersions mocks base method.
func (m *MockSchedulerStorage) GetSchedulerVersions(ctx context.Context, name string) ([]*entities.SchedulerVersion, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSchedulerVersions", ctx, name)
	ret0, _ := ret[0].([]*entities.SchedulerVersion)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSchedulerVersions indicates an expected call of GetSchedulerVersions.
func (mr *MockSchedulerStorageMockRecorder) GetSchedulerVersions(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSchedulerVersions", reflect.TypeOf((*MockSchedulerStorage)(nil).GetSchedulerVersions), ctx, name)
}

// GetSchedulerWithFilter mocks base method.
func (m *MockSchedulerStorage) GetSchedulerWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) (*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSchedulerWithFilter", ctx, schedulerFilter)
	ret0, _ := ret[0].(*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSchedulerWithFilter indicates an expected call of GetSchedulerWithFilter.
func (mr *MockSchedulerStorageMockRecorder) GetSchedulerWithFilter(ctx, schedulerFilter interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSchedulerWithFilter", reflect.TypeOf((*MockSchedulerStorage)(nil).GetSchedulerWithFilter), ctx, schedulerFilter)
}

// GetSchedulers mocks base method.
func (m *MockSchedulerStorage) GetSchedulers(ctx context.Context, names []string) ([]*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSchedulers", ctx, names)
	ret0, _ := ret[0].([]*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSchedulers indicates an expected call of GetSchedulers.
func (mr *MockSchedulerStorageMockRecorder) GetSchedulers(ctx, names interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSchedulers", reflect.TypeOf((*MockSchedulerStorage)(nil).GetSchedulers), ctx, names)
}

// GetSchedulersWithFilter mocks base method.
func (m *MockSchedulerStorage) GetSchedulersWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) ([]*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSchedulersWithFilter", ctx, schedulerFilter)
	ret0, _ := ret[0].([]*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSchedulersWithFilter indicates an expected call of GetSchedulersWithFilter.
func (mr *MockSchedulerStorageMockRecorder) GetSchedulersWithFilter(ctx, schedulerFilter interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSchedulersWithFilter", reflect.TypeOf((*MockSchedulerStorage)(nil).GetSchedulersWithFilter), ctx, schedulerFilter)
}

// RunWithTransaction mocks base method.
func (m *MockSchedulerStorage) RunWithTransaction(ctx context.Context, transactionFunc func(ports.TransactionID) error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunWithTransaction", ctx, transactionFunc)
	ret0, _ := ret[0].(error)
	return ret0
}

// RunWithTransaction indicates an expected call of RunWithTransaction.
func (mr *MockSchedulerStorageMockRecorder) RunWithTransaction(ctx, transactionFunc interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunWithTransaction", reflect.TypeOf((*MockSchedulerStorage)(nil).RunWithTransaction), ctx, transactionFunc)
}

// UpdateScheduler mocks base method.
func (m *MockSchedulerStorage) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateScheduler", ctx, scheduler)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateScheduler indicates an expected call of UpdateScheduler.
func (mr *MockSchedulerStorageMockRecorder) UpdateScheduler(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateScheduler", reflect.TypeOf((*MockSchedulerStorage)(nil).UpdateScheduler), ctx, scheduler)
}

// MockSchedulerCache is a mock of SchedulerCache interface.
type MockSchedulerCache struct {
	ctrl     *gomock.Controller
	recorder *MockSchedulerCacheMockRecorder
}

// MockSchedulerCacheMockRecorder is the mock recorder for MockSchedulerCache.
type MockSchedulerCacheMockRecorder struct {
	mock *MockSchedulerCache
}

// NewMockSchedulerCache creates a new mock instance.
func NewMockSchedulerCache(ctrl *gomock.Controller) *MockSchedulerCache {
	mock := &MockSchedulerCache{ctrl: ctrl}
	mock.recorder = &MockSchedulerCacheMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSchedulerCache) EXPECT() *MockSchedulerCacheMockRecorder {
	return m.recorder
}

// GetScheduler mocks base method.
func (m *MockSchedulerCache) GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetScheduler", ctx, name)
	ret0, _ := ret[0].(*entities.Scheduler)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetScheduler indicates an expected call of GetScheduler.
func (mr *MockSchedulerCacheMockRecorder) GetScheduler(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetScheduler", reflect.TypeOf((*MockSchedulerCache)(nil).GetScheduler), ctx, name)
}

// SetScheduler mocks base method.
func (m *MockSchedulerCache) SetScheduler(ctx context.Context, scheduler *entities.Scheduler, ttl time.Duration) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetScheduler", ctx, scheduler, ttl)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetScheduler indicates an expected call of SetScheduler.
func (mr *MockSchedulerCacheMockRecorder) SetScheduler(ctx, scheduler, ttl interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetScheduler", reflect.TypeOf((*MockSchedulerCache)(nil).SetScheduler), ctx, scheduler, ttl)
}
