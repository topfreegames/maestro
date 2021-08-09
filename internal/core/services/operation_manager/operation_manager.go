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

package operation_manager

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type OperationManager struct {
	flow                            ports.OperationFlow
	storage                         ports.OperationStorage
	operationDefinitionConstructors map[string]operations.DefinitionConstructor
}

func New(flow ports.OperationFlow, storage ports.OperationStorage, operationDefinitionConstructors map[string]operations.DefinitionConstructor) *OperationManager {
	return &OperationManager{
		flow:                            flow,
		storage:                         storage,
		operationDefinitionConstructors: operationDefinitionConstructors,
	}
}

func (o *OperationManager) CreateOperation(ctx context.Context, schedulerName string, definition operations.Definition) (*operation.Operation, error) {
	op := &operation.Operation{
		ID:             uuid.NewString(),
		Status:         operation.StatusPending,
		DefinitionName: definition.Name(),
		SchedulerName:  schedulerName,
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

	definitionConstructor := o.operationDefinitionConstructors[op.DefinitionName]
	if definitionConstructor == nil {
		return nil, nil, fmt.Errorf("no definition constructor implemented for %s: %s", op.DefinitionName, err)
	}

	definition := definitionConstructor()
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
	err := o.storage.UpdateOperationStatus(ctx, op.SchedulerName, op.ID, operation.StatusInProgress)
	if err != nil {
		return fmt.Errorf("failed to start operation: %w", err)
	}

	op.Status = operation.StatusInProgress
	return nil
}

func (o *OperationManager) ListSchedulerPendingOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {

	pendingOperationIDs, err := o.flow.ListSchedulerPendingOperationIDs(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("failed to list all pending operations: %w", err)
	}
	pendingOperations := make([]*operation.Operation, len(pendingOperationIDs))
	for i, operationID := range pendingOperationIDs {
		op, _, err := o.storage.GetOperation(ctx, schedulerName, operationID)
		if err != nil {
			return nil, err
		}
		pendingOperations[i] = op
	}

	return pendingOperations, nil
}

func (o *OperationManager) ListSchedulerActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {

	return o.storage.ListSchedulerActiveOperations(ctx, schedulerName)
}

func (o *OperationManager) ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {

	return o.storage.ListSchedulerFinishedOperations(ctx, schedulerName)
}

func (o *OperationManager) FinishOperation(ctx context.Context, op *operation.Operation) error {
	err := o.storage.UpdateOperationStatus(ctx, op.SchedulerName, op.ID, op.Status)
	if err != nil {
		return fmt.Errorf("failed to finish operation: %w", err)
	}

	return nil
}
