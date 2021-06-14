package operation_manager

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type OperationManager struct {
	storage ports.OperationStorage
}

func New(storage ports.OperationStorage) *OperationManager {
	return &OperationManager{storage}
}

func (o *OperationManager) CreateOperation(ctx context.Context, definition operation.Definition) (*operation.Operation, error) {
	op := &operation.Operation{
		ID:         uuid.NewString(),
		Status:     operation.StatusPending,
		Definition: definition,
	}

	err := o.storage.CreateOperation(ctx, op)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation: %w", err)
	}

	return op, nil
}
