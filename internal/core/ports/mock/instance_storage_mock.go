// Code generated by MockGen. DO NOT EDIT.
// Source: ../internal/core/ports/instance_storage.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	game_room "github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// MockGameRoomInstanceStorage is a mock of GameRoomInstanceStorage interface.
type MockGameRoomInstanceStorage struct {
	ctrl     *gomock.Controller
	recorder *MockGameRoomInstanceStorageMockRecorder
}

// MockGameRoomInstanceStorageMockRecorder is the mock recorder for MockGameRoomInstanceStorage.
type MockGameRoomInstanceStorageMockRecorder struct {
	mock *MockGameRoomInstanceStorage
}

// NewMockGameRoomInstanceStorage creates a new mock instance.
func NewMockGameRoomInstanceStorage(ctrl *gomock.Controller) *MockGameRoomInstanceStorage {
	mock := &MockGameRoomInstanceStorage{ctrl: ctrl}
	mock.recorder = &MockGameRoomInstanceStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGameRoomInstanceStorage) EXPECT() *MockGameRoomInstanceStorageMockRecorder {
	return m.recorder
}

// DeleteInstance mocks base method.
func (m *MockGameRoomInstanceStorage) DeleteInstance(ctx context.Context, scheduler, roomId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteInstance", ctx, scheduler, roomId)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteInstance indicates an expected call of DeleteInstance.
func (mr *MockGameRoomInstanceStorageMockRecorder) DeleteInstance(ctx, scheduler, roomId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteInstance", reflect.TypeOf((*MockGameRoomInstanceStorage)(nil).DeleteInstance), ctx, scheduler, roomId)
}

// GetAllInstances mocks base method.
func (m *MockGameRoomInstanceStorage) GetAllInstances(ctx context.Context, scheduler string) ([]*game_room.Instance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllInstances", ctx, scheduler)
	ret0, _ := ret[0].([]*game_room.Instance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllInstances indicates an expected call of GetAllInstances.
func (mr *MockGameRoomInstanceStorageMockRecorder) GetAllInstances(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllInstances", reflect.TypeOf((*MockGameRoomInstanceStorage)(nil).GetAllInstances), ctx, scheduler)
}

// GetInstance mocks base method.
func (m *MockGameRoomInstanceStorage) GetInstance(ctx context.Context, scheduler, roomId string) (*game_room.Instance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstance", ctx, scheduler, roomId)
	ret0, _ := ret[0].(*game_room.Instance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInstance indicates an expected call of GetInstance.
func (mr *MockGameRoomInstanceStorageMockRecorder) GetInstance(ctx, scheduler, roomId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstance", reflect.TypeOf((*MockGameRoomInstanceStorage)(nil).GetInstance), ctx, scheduler, roomId)
}

// GetInstanceCount mocks base method.
func (m *MockGameRoomInstanceStorage) GetInstanceCount(ctx context.Context, scheduler string) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstanceCount", ctx, scheduler)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInstanceCount indicates an expected call of GetInstanceCount.
func (mr *MockGameRoomInstanceStorageMockRecorder) GetInstanceCount(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstanceCount", reflect.TypeOf((*MockGameRoomInstanceStorage)(nil).GetInstanceCount), ctx, scheduler)
}

// UpsertInstance mocks base method.
func (m *MockGameRoomInstanceStorage) UpsertInstance(ctx context.Context, instance *game_room.Instance) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpsertInstance", ctx, instance)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpsertInstance indicates an expected call of UpsertInstance.
func (mr *MockGameRoomInstanceStorageMockRecorder) UpsertInstance(ctx, instance interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpsertInstance", reflect.TypeOf((*MockGameRoomInstanceStorage)(nil).UpsertInstance), ctx, instance)
}
