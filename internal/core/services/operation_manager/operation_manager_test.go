package operation_manager

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operations_registry"
)

type testOperationDefinition struct {
	marshalResult   []byte
	unmarshalResult error
}

func (d *testOperationDefinition) Marshal() []byte            { return d.marshalResult }
func (d *testOperationDefinition) Unmarshal(raw []byte) error { return d.unmarshalResult }
func (d *testOperationDefinition) Name() string               { return "testOperationDefinition" }

type opMatcher struct {
	status operation.Status
	def    operation.Definition
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
		definition operation.Definition
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
			defer mockCtrl.Finish()

			schedulerName := "scheduler_name"
			operationFlow := opflow.NewMockOperationFlow(mockCtrl)
			operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
			opManager := New(operationFlow, operationStorage)

			ctx := context.Background()
			testDefinition, _ := test.definition.(*testOperationDefinition)
			operationStorage.EXPECT().CreateOperation(ctx, &opMatcher{operation.StatusPending, test.definition}, testDefinition.marshalResult).Return(test.storageErr)

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

			require.NotNil(t, op)
			require.Equal(t, operation.StatusPending, op.Status)
		})
	}
}

func TestGetOperation(t *testing.T) {
	t.Run("find operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{} }
		registry := operations_registry.NewRegistry()
		registry.Register(defFunc().Name(), defFunc)

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			[]byte{},
			nil,
		)

		op, definition, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.Equal(t, operationID, op.ID)
		require.Equal(t, schedulerName, op.SchedulerName)
		require.IsType(t, defFunc(), definition)
	})

	t.Run("defition not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{} }
		registry := operations_registry.NewRegistry()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			[]byte{},
			nil,
		)

		_, _, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.Error(t, err)
	})

	t.Run("operation not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{} }
		registry := operations_registry.NewRegistry()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			[]byte{},
			porterrors.ErrNotFound,
		)

		_, _, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.Error(t, err)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{unmarshalResult: errors.New("invalid")} }
		registry := operations_registry.NewRegistry()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			[]byte{},
			porterrors.ErrNotFound,
		)

		_, _, err := opManager.GetOperation(ctx, schedulerName, operationID)
		require.Error(t, err)
	})
}

func TestNextSchedulerOperation(t *testing.T) {
	t.Run("fetch operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{} }
		registry := operations_registry.NewRegistry()
		registry.Register(defFunc().Name(), defFunc)

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"

		operationFlow.EXPECT().NextOperationID(ctx, schedulerName).Return(operationID, nil)
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			&operation.Operation{ID: operationID, SchedulerName: schedulerName, DefinitionName: defFunc().Name()},
			[]byte{},
			nil,
		)

		op, definition, err := opManager.NextSchedulerOperation(ctx, schedulerName)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.Equal(t, operationID, op.ID)
		require.Equal(t, schedulerName, op.SchedulerName)
		require.IsType(t, defFunc(), definition)
	})

	t.Run("no next operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{} }
		registry := operations_registry.NewRegistry()
		registry.Register(defFunc().Name(), defFunc)

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationFlow.EXPECT().NextOperationID(ctx, schedulerName).Return("", porterrors.ErrUnexpected)

		_, _, err := opManager.NextSchedulerOperation(ctx, schedulerName)
		require.Error(t, err)
	})

	t.Run("operation not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		defFunc := func() operation.Definition { return &testOperationDefinition{} }
		registry := operations_registry.NewRegistry()
		registry.Register(defFunc().Name(), defFunc)

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, registry)

		ctx := context.Background()
		schedulerName := "test-scheduler"
		operationID := "some-op-id"

		operationFlow.EXPECT().NextOperationID(ctx, schedulerName).Return(operationID, nil)
		operationStorage.EXPECT().GetOperation(ctx, schedulerName, operationID).Return(
			nil,
			[]byte{},
			porterrors.ErrNotFound,
		)

		_, _, err := opManager.NextSchedulerOperation(ctx, schedulerName)
		require.Error(t, err)
	})
}

func TestStartOperation(t *testing.T) {
	t.Run("starts operation with succeess", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, operations_registry.NewRegistry())

		ctx := context.Background()
		op := &operation.Operation{ID: uuid.NewString(), DefinitionName: (&testOperationDefinition{}).Name()}

		operationStorage.EXPECT().UpdateOperationStatus(ctx, op.SchedulerName, op.ID, operation.StatusInProgress).Return(nil)
		err := opManager.StartOperation(ctx, op)
		require.NoError(t, err)
	})
}

func TestFinishOperation(t *testing.T) {
	t.Run("finishes operation with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, operations_registry.NewRegistry())

		ctx := context.Background()
		expectedStatus := operation.StatusError
		op := &operation.Operation{ID: uuid.NewString(), DefinitionName: (&testOperationDefinition{}).Name(), Status: expectedStatus}

		operationStorage.EXPECT().UpdateOperationStatus(ctx, op.SchedulerName, op.ID, expectedStatus).Return(nil)
		err := opManager.FinishOperation(ctx, op)
		require.NoError(t, err)
	})
}

func TestListActiveOperations(t *testing.T) {
	t.Run("lists", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		opManager := NewWithRegistry(operationFlow, operationStorage, operations_registry.NewRegistry())

		ctx := context.Background()
		operationsResult := []*operation.Operation{
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
			{ID: uuid.NewString()},
		}

		schedulerName := "test-scheduler"
		operationStorage.EXPECT().ListSchedulerActiveOperations(ctx, schedulerName).Return(operationsResult, nil)
		operations, err := opManager.ListActiveOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.ElementsMatch(t, operationsResult, operations)
	})
}
