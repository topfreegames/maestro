// Code generated by MockGen. DO NOT EDIT.
// Source: storage/scheduler_event_storage.go

package mock

import (
	gomock "github.com/golang/mock/gomock"
	models "github.com/topfreegames/maestro/models"
	reflect "reflect"
)

// MockSchedulerEventStorage is a mock of SchedulerEventStorage interface
type MockSchedulerEventStorage struct {
	ctrl     *gomock.Controller
	recorder *MockSchedulerEventStorageMockRecorder
}

// MockSchedulerEventStorageMockRecorder is the mock recorder for MockSchedulerEventStorage
type MockSchedulerEventStorageMockRecorder struct {
	mock *MockSchedulerEventStorage
}

// NewMockSchedulerEventStorage creates a new mock instance
func NewMockSchedulerEventStorage(ctrl *gomock.Controller) *MockSchedulerEventStorage {
	mock := &MockSchedulerEventStorage{ctrl: ctrl}
	mock.recorder = &MockSchedulerEventStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockSchedulerEventStorage) EXPECT() *MockSchedulerEventStorageMockRecorder {
	return _m.recorder
}

// PersistSchedulerEvent mocks base method
func (_m *MockSchedulerEventStorage) PersistSchedulerEvent(event *models.SchedulerEvent) error {
	ret := _m.ctrl.Call(_m, "PersistSchedulerEvent", event)
	ret0, _ := ret[0].(error)
	return ret0
}

// PersistSchedulerEvent indicates an expected call of PersistSchedulerEvent
func (_mr *MockSchedulerEventStorageMockRecorder) PersistSchedulerEvent(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "PersistSchedulerEvent", reflect.TypeOf((*MockSchedulerEventStorage)(nil).PersistSchedulerEvent), arg0)
}

// LoadSchedulerEvents mocks base method
func (_m *MockSchedulerEventStorage) LoadSchedulerEvents(schedulerName string, page int) ([]*models.SchedulerEvent, error) {
	ret := _m.ctrl.Call(_m, "LoadSchedulerEvents", schedulerName, page)
	ret0, _ := ret[0].([]*models.SchedulerEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadSchedulerEvents indicates an expected call of LoadSchedulerEvents
func (_mr *MockSchedulerEventStorageMockRecorder) LoadSchedulerEvents(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "LoadSchedulerEvents", reflect.TypeOf((*MockSchedulerEventStorage)(nil).LoadSchedulerEvents), arg0, arg1)
}