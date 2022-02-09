// Code generated by MockGen. DO NOT EDIT.
// Source: ../internal/core/services/interfaces/room_manager.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"
	sync "sync"

	gomock "github.com/golang/mock/gomock"
	entities "github.com/topfreegames/maestro/internal/core/entities"
	game_room "github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// MockRoomManager is a mock of RoomManager interface.
type MockRoomManager struct {
	ctrl     *gomock.Controller
	recorder *MockRoomManagerMockRecorder
}

// MockRoomManagerMockRecorder is the mock recorder for MockRoomManager.
type MockRoomManagerMockRecorder struct {
	mock *MockRoomManager
}

// NewMockRoomManager creates a new mock instance.
func NewMockRoomManager(ctrl *gomock.Controller) *MockRoomManager {
	mock := &MockRoomManager{ctrl: ctrl}
	mock.recorder = &MockRoomManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRoomManager) EXPECT() *MockRoomManagerMockRecorder {
	return m.recorder
}

// CleanRoomState mocks base method.
func (m *MockRoomManager) CleanRoomState(ctx context.Context, schedulerName, roomId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CleanRoomState", ctx, schedulerName, roomId)
	ret0, _ := ret[0].(error)
	return ret0
}

// CleanRoomState indicates an expected call of CleanRoomState.
func (mr *MockRoomManagerMockRecorder) CleanRoomState(ctx, schedulerName, roomId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CleanRoomState", reflect.TypeOf((*MockRoomManager)(nil).CleanRoomState), ctx, schedulerName, roomId)
}

// CreateRoomAndWaitForReadiness mocks base method.
func (m *MockRoomManager) CreateRoomAndWaitForReadiness(ctx context.Context, scheduler entities.Scheduler) (*game_room.GameRoom, *game_room.Instance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRoomAndWaitForReadiness", ctx, scheduler)
	ret0, _ := ret[0].(*game_room.GameRoom)
	ret1, _ := ret[1].(*game_room.Instance)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateRoomAndWaitForReadiness indicates an expected call of CreateRoomAndWaitForReadiness.
func (mr *MockRoomManagerMockRecorder) CreateRoomAndWaitForReadiness(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRoomAndWaitForReadiness", reflect.TypeOf((*MockRoomManager)(nil).CreateRoomAndWaitForReadiness), ctx, scheduler)
}

// DeleteRoomAndWaitForRoomTerminated mocks base method.
func (m *MockRoomManager) DeleteRoomAndWaitForRoomTerminated(ctx context.Context, gameRoom *game_room.GameRoom) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteRoomAndWaitForRoomTerminated", ctx, gameRoom)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteRoomAndWaitForRoomTerminated indicates an expected call of DeleteRoomAndWaitForRoomTerminated.
func (mr *MockRoomManagerMockRecorder) DeleteRoomAndWaitForRoomTerminated(ctx, gameRoom interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteRoomAndWaitForRoomTerminated", reflect.TypeOf((*MockRoomManager)(nil).DeleteRoomAndWaitForRoomTerminated), ctx, gameRoom)
}

// ListRoomsWithDeletionPriority mocks base method.
func (m *MockRoomManager) ListRoomsWithDeletionPriority(ctx context.Context, schedulerName, ignoredVersion string, amount int, roomsBeingReplaced *sync.Map) ([]*game_room.GameRoom, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListRoomsWithDeletionPriority", ctx, schedulerName, ignoredVersion, amount, roomsBeingReplaced)
	ret0, _ := ret[0].([]*game_room.GameRoom)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListRoomsWithDeletionPriority indicates an expected call of ListRoomsWithDeletionPriority.
func (mr *MockRoomManagerMockRecorder) ListRoomsWithDeletionPriority(ctx, schedulerName, ignoredVersion, amount, roomsBeingReplaced interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListRoomsWithDeletionPriority", reflect.TypeOf((*MockRoomManager)(nil).ListRoomsWithDeletionPriority), ctx, schedulerName, ignoredVersion, amount, roomsBeingReplaced)
}

// SchedulerMaxSurge mocks base method.
func (m *MockRoomManager) SchedulerMaxSurge(ctx context.Context, scheduler *entities.Scheduler) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SchedulerMaxSurge", ctx, scheduler)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SchedulerMaxSurge indicates an expected call of SchedulerMaxSurge.
func (mr *MockRoomManagerMockRecorder) SchedulerMaxSurge(ctx, scheduler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SchedulerMaxSurge", reflect.TypeOf((*MockRoomManager)(nil).SchedulerMaxSurge), ctx, scheduler)
}

// UpdateRoom mocks base method.
func (m *MockRoomManager) UpdateRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateRoom", ctx, gameRoom)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateRoom indicates an expected call of UpdateRoom.
func (mr *MockRoomManagerMockRecorder) UpdateRoom(ctx, gameRoom interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateRoom", reflect.TypeOf((*MockRoomManager)(nil).UpdateRoom), ctx, gameRoom)
}

// UpdateRoomInstance mocks base method.
func (m *MockRoomManager) UpdateRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateRoomInstance", ctx, gameRoomInstance)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateRoomInstance indicates an expected call of UpdateRoomInstance.
func (mr *MockRoomManagerMockRecorder) UpdateRoomInstance(ctx, gameRoomInstance interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateRoomInstance", reflect.TypeOf((*MockRoomManager)(nil).UpdateRoomInstance), ctx, gameRoomInstance)
}
