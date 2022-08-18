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

//go:build unit
// +build unit

package operation_manager

import (
	"context"
	"testing"
	"time"

	"errors"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"

	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"

	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
)

type testOperationDefinition struct {
	marshalResult   []byte
	unmarshalResult error
}

func (d *testOperationDefinition) Marshal() []byte            { return d.marshalResult }
func (d *testOperationDefinition) Unmarshal(raw []byte) error { return d.unmarshalResult }
func (d *testOperationDefinition) Name() string               { return "testOperationDefinition" }
func (d *testOperationDefinition) ShouldExecute(_ context.Context, _ []*operation.Operation) bool {
	return false
}

type opMatcher struct {
	status operation.Status
	def    operations.Definition
}

func (m *opMatcher) Matches(x interface{}) bool {
	op, _ := x.(*operation.Operation)
	_, err := uuid.Parse(op.ID)
	return err == nil && op.Status == m.status && m.def.Name() == op.DefinitionName
}

func (m *opMatcher) String() string {
	return fmt.Sprintf("a operation with definition \"%s\"", m.def.Name())
}

func TestCreateOperation(t *testing.T) {
	cases := map[string]struct {
		definition operations.Definition
		storageErr error
		flowErr    error
	}{
		"create without errors": {
			definition: &testOperationDefinition{marshalResult: []byte("test")},
		},
		"create with storage errors": {
			definition: &testOperationDefinition{},
			storageErr: porterrors.ErrUnexpected,
		},
		"create with flow errors": {
			definition: &testOperationDefinition{},
			flowErr:    porterrors.ErrUnexpected,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			schedulerName := "scheduler_name"
			operationFlow := mockports.NewMockOperationFlow(mockCtrl)
			operationStorage := mockports.NewMockOperationStorage(mockCtrl)
			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			definitionConstructors := operations.NewDefinitionConstructors()
			operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
			config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
			opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

			ctx := context.Background()
			operationStorage.EXPECT().CreateOperation(ctx, &opMatcher{operation.StatusPending, test.definition}).Return(test.storageErr)

			if test.storageErr == nil {
				operationFlow.EXPECT().InsertOperationID(ctx, schedulerName, gomock.Any()).Return(test.flowErr)
			}

			op, err := opManager.CreateOperation(ctx, schedulerName, test.definition)

			if test.storageErr != nil {
				require.ErrorIs(t, err, test.storageErr)
				require.Nil(t, op)
				return
			}

			if test.flowErr != nil {
				require.ErrorIs(t, err, test.flowErr)
				require.Nil(t, op)
				return
			}

			assert.NotEmpty(t, op.ID)
			assert.Equal(t, operation.StatusPending, op.Status)
			assert.Equal(t, test.definition.Name(), op.DefinitionName)
			assert.Equal(t, schedulerName, op.SchedulerName)
			assert.EqualValues(t, test.definition.Marshal(), op.Input)
		})
	}
}

func TestCreatePriorityOperation(t *testing.T) {
	cases := map[string]struct {
		definition operations.Definition
		storageErr error
		flowErr    error
	}{
		"create without errors": {
			definition: &testOperationDefinition{marshalResult: []byte("test")},
		},
		"create with storage errors": {
			definition: &testOperationDefinition{},
			storageErr: porterrors.ErrUnexpected,
		},
		"create with flow errors": {
			definition: &testOperationDefinition{},
			flowErr:    porterrors.ErrUnexpected,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			schedulerName := "scheduler_name"
			operationFlow := mockports.NewMockOperationFlow(mockCtrl)
			operationStorage := mockports.NewMockOperationStorage(mockCtrl)
			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			definitionConstructors := operations.NewDefinitionConstructors()
			operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
			config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
			opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

			ctx := context.Background()

			operationStorage.EXPECT().CreateOperation(ctx, &opMatcher{operation.StatusPending, test.definition}).Return(test.storageErr)

			if test.storageErr == nil {
				operationFlow.EXPECT().InsertPriorityOperationID(ctx, schedulerName, gomock.Any()).Return(test.flowErr)
			}

			op, err := opManager.CreatePriorityOperation(ctx, schedulerName, test.definition)

			if test.storageErr != nil {
				require.ErrorIs(t, err, test.storageErr)
				require.Nil(t, op)
				return
			}

			if test.flowErr != nil {
				require.ErrorIs(t, err, test.flowErr)
				require.Nil(t, op)
				return
			}

			assert.NotEmpty(t, op.ID)
			assert.Equal(t, operation.StatusPending, op.Status)
			assert.Equal(t, test.definition.Name(), op.DefinitionName)
			assert.Equal(t, schedulerName, op.SchedulerName)
			assert.EqualValues(t, test.definition.Marshal(), op.Input)
		})
	}
}

func TestGetOperation(t *testing.T) {
	t.Run("find operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name(), Status: operation.StatusPending},
			nil,
		)

		op, definition, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.NoError(t, err)

		assert.Equal(t, op.ID, operationID)
		assert.Equal(t, operation.StatusPending, op.Status)
		assert.Equal(t, schedulerName, op.SchedulerName)
		assert.Equal(t, defFunc().Name(), op.DefinitionName)
		assert.EqualValues(t, defFunc().Marshal(), op.Input)

		require.IsType(t, defFunc(), definition)
	})

	t.Run("definition not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			nil,
		)

		_, _, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.Error(t, err)
	})

	t.Run("operation not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			porterrors.ErrNotFound,
		)

		_, _, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.Error(t, err)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{unmarshalResult: errors.New("invalid")} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			porterrors.ErrNotFound,
		)

		_, _, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.Error(t, err)
	})
}

func TestPendingOperationsChan(t *testing.T) {
	type args struct {
		ctx           context.Context
		schedulerName string
	}
	type mockPreparation struct {
		mockSetup func(opFlow *mockports.MockOperationFlow, opStorage *mockports.MockOperationStorage)
	}
	type expectedResult struct {
		operationIDs []string
	}
	tests := []struct {
		name            string
		mockPreparation mockPreparation
		args            args
		expectedResult  expectedResult
		expectErr       bool
	}{
		{
			"returns a channel that keeps sending pending operations if no error occurs",
			mockPreparation{
				mockSetup: func(opFlow *mockports.MockOperationFlow, opStorage *mockports.MockOperationStorage) {
					opFlow.EXPECT().NextOperationID(gomock.Any(), "test-scheduler").Return("some-op-id", nil).MinTimes(2)
					opFlow.EXPECT().RemoveNextOperation(gomock.Any(), "test-scheduler").MaxTimes(2)
				},
			},
			args{
				ctx:           context.Background(),
				schedulerName: "test-scheduler",
			},
			expectedResult{
				operationIDs: []string{"some-op-id", "some-op-id"},
			},
			false,
		},
		{
			"returns a channel that is closed if an error occurs while getting next operation ID",
			mockPreparation{
				mockSetup: func(opFlow *mockports.MockOperationFlow, opStorage *mockports.MockOperationStorage) {
					opFlow.EXPECT().NextOperationID(gomock.Any(), "test-scheduler").Return("", errors.New("some error"))
				},
			},
			args{
				ctx:           context.Background(),
				schedulerName: "test-scheduler",
			},
			expectedResult{},
			true,
		},
		{
			"returns a channel that keeps sending pending operations if some error occurs in removing operations from the flow",
			mockPreparation{
				mockSetup: func(opFlow *mockports.MockOperationFlow, opStorage *mockports.MockOperationStorage) {
					opFlow.EXPECT().NextOperationID(gomock.Any(), "test-scheduler").Return("some-op-id", nil).MinTimes(2)
					opFlow.EXPECT().RemoveNextOperation(gomock.Any(), "test-scheduler").Return(errors.New("some error")).MinTimes(1)
				},
			},
			args{
				ctx:           context.Background(),
				schedulerName: "test-scheduler",
			},
			expectedResult{
				operationIDs: []string{"some-op-id", "some-op-id"},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			defFunc := func() operations.Definition { return &testOperationDefinition{} }
			definitionConstructors := operations.NewDefinitionConstructors()
			definitionConstructors[defFunc().Name()] = defFunc

			operationFlow := mockports.NewMockOperationFlow(mockCtrl)
			operationStorage := mockports.NewMockOperationStorage(mockCtrl)
			operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
			opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

			ctx := tt.args.ctx
			schedulerName := tt.args.schedulerName

			tt.mockPreparation.mockSetup(operationFlow, operationStorage)

			pendingOpChan := opManager.PendingOperationsChan(ctx, schedulerName)

			if tt.expectErr {
				_, ok := <-pendingOpChan
				require.False(t, ok)
			} else {

				for _, expectedOpID := range tt.expectedResult.operationIDs {
					opID, ok := <-pendingOpChan
					require.True(t, ok)
					assert.Equal(t, expectedOpID, opID)
				}
			}
		})
	}
}

func TestStartOperation(t *testing.T) {
	t.Run("starts operation with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		op := &operation.Operation{ID: uuid.NewString(), DefinitionName: (&testOperationDefinition{}).Name()}

		operationStorage.EXPECT().UpdateOperationStatus(ctx, op.SchedulerName, op.ID, operation.StatusInProgress).Return(nil)
		err := opManager.StartOperation(ctx, op, func() {})
		require.NoError(t, err)
	})
}

func TestFinishOperation(t *testing.T) {
	t.Run("finishes operation with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		op := &operation.Operation{
			Status:         operation.StatusInProgress,
			SchedulerName:  uuid.NewString(),
			ID:             uuid.NewString(),
			DefinitionName: (&testOperationDefinition{}).Name(),
		}
		definition := &testOperationDefinition{}

		operationStorage.EXPECT().UpdateOperationStatus(ctx, op.SchedulerName, op.ID, operation.StatusInProgress).Return(nil)
		operationStorage.EXPECT().UpdateOperationDefinition(ctx, op.SchedulerName, op.ID, definition).Return(nil)
		err := opManager.FinishOperation(ctx, op, definition)
		require.NoError(t, err)
	})

	t.Run("return error when fails to update operation definition", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		op := &operation.Operation{
			SchedulerName:  uuid.NewString(),
			ID:             uuid.NewString(),
			DefinitionName: (&testOperationDefinition{}).Name(),
		}
		definition := &testOperationDefinition{}

		operationStorage.EXPECT().UpdateOperationDefinition(ctx, op.SchedulerName, op.ID, definition).Return(errors.New("some error"))
		err := opManager.FinishOperation(ctx, op, definition)
		require.ErrorContains(t, err, "failed to update operation definition: some error")
	})

	t.Run("return error when fails to update operation status", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		op := &operation.Operation{
			Status:         operation.StatusInProgress,
			SchedulerName:  uuid.NewString(),
			ID:             uuid.NewString(),
			DefinitionName: (&testOperationDefinition{}).Name(),
		}
		definition := &testOperationDefinition{}

		operationStorage.EXPECT().UpdateOperationDefinition(ctx, op.SchedulerName, op.ID, definition).Return(nil)
		operationStorage.EXPECT().UpdateOperationStatus(ctx, op.SchedulerName, op.ID, operation.StatusInProgress).Return(errors.New("some error"))
		err := opManager.FinishOperation(ctx, op, definition)
		require.ErrorContains(t, err, "failed to update operation status: some error")
	})
}

func TestListSchedulerActiveOperations(t *testing.T) {
	t.Run("it returns an operation list with pending status", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		operationsResult := []*operation.Operation{
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
		}
		operationsLease := []*operation.OperationLease{
			{OperationID: operationsResult[0].ID, Ttl: time.Unix(1641306511, 0)},
			{OperationID: operationsResult[1].ID, Ttl: time.Unix(1641306522, 0)},
			{OperationID: operationsResult[2].ID, Ttl: time.Unix(1641306533, 0)},
		}
		operationsResult[0].Lease = operationsLease[0]
		operationsResult[1].Lease = operationsLease[1]
		operationsResult[2].Lease = operationsLease[2]

		schedulerName := "test-scheduler"
		operationStorage.EXPECT().ListSchedulerActiveOperations(ctx, schedulerName).Return(operationsResult, nil)
		operationLeaseStorage.EXPECT().FetchOperationsLease(ctx, schedulerName, operationsResult[0].ID, operationsResult[1].ID, operationsResult[2].ID).Return(operationsLease, nil)
		operations, err := opManager.ListSchedulerActiveOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.ElementsMatch(t, operationsResult, operations)
	})
	t.Run("it returns an empty list when there is no operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		var operationsResult []*operation.Operation

		schedulerName := "test-scheduler"
		operationStorage.EXPECT().ListSchedulerActiveOperations(ctx, schedulerName).Return(operationsResult, nil)
		operations, err := opManager.ListSchedulerActiveOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.Empty(t, operations)
	})

	t.Run("it returns error when some error occurs in operation storage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManagerConfig := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, operationManagerConfig, schedulerStorage)

		ctx := context.Background()

		schedulerName := "test-scheduler"
		operationStorage.EXPECT().ListSchedulerActiveOperations(ctx, schedulerName).Return(nil, errors.New("some error"))
		_, err := opManager.ListSchedulerActiveOperations(ctx, schedulerName)
		require.Error(t, err, fmt.Errorf("failed get active operations list fort scheduler %s : %w", schedulerName, errors.New("some error")))
	})

	t.Run("it returns error when some error occurs in operation lease storage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManagerConfig := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, operationManagerConfig, schedulerStorage)

		ctx := context.Background()
		operationsResult := []*operation.Operation{
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
		}
		schedulerName := "test-scheduler"
		operationStorage.EXPECT().ListSchedulerActiveOperations(ctx, schedulerName).Return(operationsResult, nil)
		operationLeaseStorage.EXPECT().FetchOperationsLease(ctx, schedulerName, operationsResult[0].ID, operationsResult[1].ID, operationsResult[2].ID).Return([]*operation.OperationLease{}, errors.New("some error"))
		_, err := opManager.ListSchedulerActiveOperations(ctx, schedulerName)
		require.Error(t, err, fmt.Errorf("failed to fetch operations lease for scheduler %s: %w", schedulerName, errors.New("some error")))
	})

}

func TestListSchedulerFinishedOperations(t *testing.T) {
	t.Run("it returns an operation list with finished status", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		operationsResult := []*operation.Operation{
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
		}

		schedulerName := "test-scheduler"

		operationStorage.EXPECT().ListSchedulerFinishedOperations(ctx, schedulerName, int64(0), int64(-1)).Return(operationsResult, int64(0), nil)
		operations, err := opManager.ListSchedulerFinishedOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.ElementsMatch(t, operationsResult, operations)
	})
}

func TestListSchedulerPendingOperations(t *testing.T) {
	t.Run("it returns an operation list with pending status", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		operationsResult := []*operation.Operation{
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
		}

		schedulerName := "test-scheduler"
		operationFlow.EXPECT().ListSchedulerPendingOperationIDs(ctx, schedulerName).Return([]string{"1", "2", "3"}, nil)
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, "1").Return(operationsResult[0], nil)
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, "2").Return(operationsResult[1], nil)
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, "3").Return(operationsResult[2], nil)

		operations, err := opManager.ListSchedulerPendingOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.ElementsMatch(t, operationsResult, operations)
	})
}

func TestWatchOperationCancellationRequests(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("cancels a operation successfully", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, nil, operationLeaseStorage, config, schedulerStorage)

		cancelableContext, cancelFunction := context.WithCancel(context.Background())
		opManager.OperationCancelFunctions.putFunction(schedulerName, operationID, cancelFunction)

		requestChannel := make(chan ports.OperationCancellationRequest, 1000)
		operationFlow.EXPECT().WatchOperationCancellationRequests(gomock.Any()).Return(requestChannel)

		ctx, ctxCancelFunction := context.WithCancel(context.Background())
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(&operation.Operation{
			SchedulerName: schedulerName,
			ID:            operationID,
			Status:        operation.StatusInProgress,
		}, nil)

		go func() {
			err := opManager.WatchOperationCancellationRequests(ctx)
			require.NoError(t, err)
		}()

		requestChannel <- ports.OperationCancellationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		}

		require.Eventually(t, func() bool {
			if cancelableContext.Err() != nil {
				require.ErrorIs(t, cancelableContext.Err(), context.Canceled)
				return true
			}

			return false
		}, time.Second, 100*time.Millisecond)

		ctxCancelFunction()
	})
}

func TestGrantLease(t *testing.T) {
	t.Run("Grant Lease success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		operationLeaseStorage.EXPECT().GrantLease(ctx, schedulerName, operationID, config.OperationLeaseTtl).Return(nil)
		err := opManager.GrantLease(ctx, op)
		require.NoError(t, err)

	})

	t.Run("Grant Lease error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		operationLeaseStorage.EXPECT().GrantLease(ctx, schedulerName, operationID, config.OperationLeaseTtl).Return(errors.New("error"))
		err := opManager.GrantLease(ctx, op)
		require.Error(t, err)
		require.Equal(t, err.Error(), "failed to grant lease to operation: error")
	})
}

func TestRevokeLease(t *testing.T) {
	t.Run("Revoke Lease success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		operationLeaseStorage.EXPECT().RevokeLease(ctx, schedulerName, operationID).Return(nil)
		err := opManager.RevokeLease(ctx, op)
		require.NoError(t, err)
	})

	t.Run("Revoke Lease error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		operationLeaseStorage.EXPECT().RevokeLease(ctx, schedulerName, operationID).Return(errors.New("error"))
		err := opManager.RevokeLease(ctx, op)
		require.Error(t, err)
		require.Equal(t, err.Error(), "failed to revoke lease to operation: error")
	})
}

func TestStartLeaseRenewGoRoutine(t *testing.T) {
	t.Run("Renews lease not executed since op.status = finished", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
			Status:        operation.StatusFinished,
		}

		operationLeaseStorage.EXPECT().RenewLease(ctx, schedulerName, operationID, config.OperationLeaseTtl).MaxTimes(0)
		opManager.StartLeaseRenewGoRoutine(ctx, op)
		time.Sleep(time.Second * 1)
	})
	t.Run("changed op.status = finished after starting renew lease go routine", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
			Status:        operation.StatusInProgress,
		}

		operationLeaseStorage.EXPECT().RenewLease(ctx, schedulerName, operationID, config.OperationLeaseTtl).MaxTimes(1)
		opManager.StartLeaseRenewGoRoutine(ctx, op)
		time.Sleep(time.Second * 1)
		op.Status = operation.StatusFinished
		time.Sleep(time.Second * 2)
	})

	t.Run("Renews lease error does not panic", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx, cancelFunction := context.WithCancel(context.Background())
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		operationLeaseStorage.EXPECT().RenewLease(ctx, schedulerName, operationID, config.OperationLeaseTtl).Return(errors.New("error")).MinTimes(1).MaxTimes(2)
		opManager.StartLeaseRenewGoRoutine(ctx, op)
		time.Sleep(time.Second * 2)
		cancelFunction()
	})

	t.Run("Renews lease being called correct number of times", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx, cancelFunction := context.WithCancel(context.Background())
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		operationLeaseStorage.EXPECT().RenewLease(ctx, schedulerName, operationID, config.OperationLeaseTtl).MaxTimes(3).MinTimes(2)
		opManager.StartLeaseRenewGoRoutine(ctx, op)
		time.Sleep(time.Second * 3)
		cancelFunction()
	})

	t.Run("Context canceled breaks StartLeaseRenewGoRoutine execution", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx, cancelFunction := context.WithCancel(context.Background())
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName: schedulerName,
		}

		opManager.StartLeaseRenewGoRoutine(ctx, op)
		cancelFunction()
		require.Eventually(t, func() bool {
			return ctx.Err() == context.Canceled
		}, time.Second, time.Millisecond*100)
	})
}

func TestEnqueueOperationCancellationRequest(t *testing.T) {
	schedulerName := "scheduler-name"
	operationID := "123"
	defFunc := func() operations.Definition { return &testOperationDefinition{} }
	op := &operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()}

	t.Run("it returns nil when cancelled op with success", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(newValidScheduler(), nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(
			op,
			nil,
		)
		operationFlow.EXPECT().EnqueueOperationCancellationRequest(gomock.Any(), gomock.Any()).Return(nil)

		err := opManager.EnqueueOperationCancellationRequest(context.Background(), schedulerName, operationID)

		assert.NoError(t, err)
	})

	t.Run("it returns error type conflict when op have final state", func(t *testing.T) {
		t.Parallel()
		finalStatuses := []operation.Status{operation.StatusFinished, operation.StatusCanceled, operation.StatusEvicted, operation.StatusError}
		for _, status := range finalStatuses {

			operationFinished := &operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name(), Status: status}
			mockCtrl := gomock.NewController(t)
			operationFlow := mockports.NewMockOperationFlow(mockCtrl)
			operationStorage := mockports.NewMockOperationStorage(mockCtrl)
			definitionConstructors := operations.NewDefinitionConstructors()
			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
			definitionConstructors[defFunc().Name()] = defFunc
			config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
			opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)
			schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(newValidScheduler(), nil)
			operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(
				operationFinished,
				nil,
			)

			err := opManager.EnqueueOperationCancellationRequest(context.Background(), schedulerName, operationID)

			assert.Error(t, err)
			assert.ErrorIs(t, err, porterrors.ErrConflict)
		}
	})

	t.Run("it returns error when op have invalid state", func(t *testing.T) {
		t.Parallel()
		operationInvalid := &operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name(), Status: 9999}
		mockCtrl := gomock.NewController(t)
		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(newValidScheduler(), nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(
			operationInvalid,
			nil,
		)

		err := opManager.EnqueueOperationCancellationRequest(context.Background(), schedulerName, operationID)

		assert.Error(t, err)
	})

	t.Run("it returns error when scheduler was not found", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defFunc := func() operations.Definition { return &testOperationDefinition{} }
		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(nil, errors.New("err"))

		err := opManager.EnqueueOperationCancellationRequest(context.Background(), schedulerName, operationID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch scheduler from storage")
	})

	t.Run("it returns error when op was not found", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defFunc := func() operations.Definition { return &testOperationDefinition{} }
		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(newValidScheduler(), nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(
			nil,
			errors.New("err"),
		)

		err := opManager.EnqueueOperationCancellationRequest(context.Background(), schedulerName, operationID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch operation from storage")
	})

	t.Run("it returns error when op couldn't enqueued", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(newValidScheduler(), nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(
			op,
			nil,
		)
		operationFlow.EXPECT().EnqueueOperationCancellationRequest(gomock.Any(), gomock.Any()).Return(errors.New("err"))

		err := opManager.EnqueueOperationCancellationRequest(context.Background(), schedulerName, operationID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to enqueue operation cancellation request:")
	})
}

func TestAppendOperationEventToExecutionHistory(t *testing.T) {
	t.Run("success case", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		defFunc := func() operations.Definition { return &testOperationDefinition{} }

		operationFlow := mockports.NewMockOperationFlow(mockCtrl)
		operationStorage := mockports.NewMockOperationStorage(mockCtrl)
		definitionConstructors := operations.NewDefinitionConstructors()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
		definitionConstructors[defFunc().Name()] = defFunc
		config := OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		opManager := New(operationFlow, operationStorage, definitionConstructors, operationLeaseStorage, config, schedulerStorage)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := uuid.NewString()
		op := &operation.Operation{
			ID: operationID, DefinitionName: (&testOperationDefinition{}).Name(),
			SchedulerName:    schedulerName,
			ExecutionHistory: []operation.OperationEvent{},
		}

		eventMessage := "some-event-message"
		operationStorage.EXPECT().UpdateOperationExecutionHistory(ctx, op).Do(func(ctx context.Context, op *operation.Operation) {
			for _, event := range op.ExecutionHistory {
				if event.Event == eventMessage {
					return
				}

				assert.Fail(t, "ExecutionHistory does not have new event")
			}
		}).Return(nil)
		opManager.AppendOperationEventToExecutionHistory(ctx, op, eventMessage)
	})
}

// newValidScheduler generates a valid scheduler with the required fields.
func newValidScheduler() *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"hello"},
					Ports: []game_room.ContainerPort{
						{Name: "tcp", Protocol: "tcp", Port: 80},
					},
					Requests: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
					Limits: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
				},
			},
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
}
