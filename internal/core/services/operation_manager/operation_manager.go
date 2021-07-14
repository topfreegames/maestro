package operation_manager

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operations_registry"
)

type OperationManager struct {
	flow            ports.OperationFlow
	storage            ports.OperationStorage
	operationsRegistry operations_registry.Registry
}

func NewWithRegistry(flow ports.OperationFlow, storage ports.OperationStorage, operationsRegistry operations_registry.Registry) *OperationManager {
	return &OperationManager{
		flow:               flow,
		storage:            storage,
		operationsRegistry: operationsRegistry,
	}
}

func New(flow ports.OperationFlow, storage ports.OperationStorage) *OperationManager {
	return &OperationManager{
		flow:               flow,
		storage:            storage,
		operationsRegistry: operations_registry.DefaultRegistry,
	}
}

func (o *OperationManager) CreateOperation(ctx context.Context, schedulerName string, definition operations.Definition) (*operation.Operation, error) {
	op := &operation.Operation{
		ID:             uuid.NewString(),
		Status:         operation.StatusPending,
		DefinitionName: definition.Name(),
		SchedulerName: schedulerName,
	}

	err := o.storage.CreateOperation(ctx, op, definition.Marshal())
	if err != nil {
		return nil, fmt.Errorf("failed to create operation: %w", err)
	}

	err = o.flow.InsertOperationID(ctx, op.SchedulerName, op.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert operation on flow: %w", err)
	}

	return op, nil
}

func (o *OperationManager) GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, operations.Definition, error) {
	op, definitionContents, err := o.storage.GetOperation(ctx, schedulerName, operationID)
	if err != nil {
		return nil, nil, err
	}

	definition, err := o.operationsRegistry.Get(op.DefinitionName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get definition: %s", err)
	}

	err = definition.Unmarshal(definitionContents)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal definition: %s", err)
	}

	return op, definition, nil
}

// NextSchedulerOperation returns the next scheduler operation to be processed.
func (o *OperationManager) NextSchedulerOperation(ctx context.Context, schedulerName string) (*operation.Operation, operations.Definition, error) {
	operationID, err := o.flow.NextOperationID(ctx, schedulerName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve the next operation: %w", err)
	}

	return o.GetOperation(ctx, schedulerName, operationID)
}

// StartOperation used when an operation will start executing.
func (o *OperationManager) StartOperation(ctx context.Context, op *operation.Operation) error {
	op.Status = operation.StatusInProgress

	err := o.storage.SetOperationActive(ctx, op)
	if err != nil {
		return fmt.Errorf("failed to start operation: %w", err)
	}

	return nil
}

func (o *OperationManager) ListActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {
	operations, err := o.storage.ListSchedulerActiveOperations(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("failed to list all active operations: %w", err)
	}

	return operations, nil
}

func (o *OperationManager) FinishOperation(ctx context.Context, op *operation.Operation) error {
	return nil
}