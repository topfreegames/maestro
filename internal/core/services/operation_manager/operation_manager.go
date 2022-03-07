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
	goerrors "errors"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/google/uuid"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
)

type OperationManager struct {
	OperationCancelFunctions        *OperationCancelFunctions
	Flow                            ports.OperationFlow
	Storage                         ports.OperationStorage
	LeaseStorage                    ports.OperationLeaseStorage
	Config                          OperationManagerConfig
	OperationDefinitionConstructors map[string]operations.DefinitionConstructor
	SchedulerStorage                ports.SchedulerStorage
	Logger                          *zap.Logger
}

func New(flow ports.OperationFlow, storage ports.OperationStorage, operationDefinitionConstructors map[string]operations.DefinitionConstructor, leaseStorage ports.OperationLeaseStorage, config OperationManagerConfig, schedulerStorage ports.SchedulerStorage) *OperationManager {
	return &OperationManager{
		Flow:                            flow,
		Storage:                         storage,
		OperationDefinitionConstructors: operationDefinitionConstructors,
		OperationCancelFunctions:        NewOperationCancelFunctions(),
		LeaseStorage:                    leaseStorage,
		Config:                          config,
		SchedulerStorage:                schedulerStorage,
		Logger:                          zap.L().With(zap.String("component", "service"), zap.String("service", "operation_manager")),
	}
}

func (om *OperationManager) CreateOperation(ctx context.Context, schedulerName string, definition operations.Definition) (*operation.Operation, error) {
	op := &operation.Operation{
		ID:             uuid.NewString(),
		Status:         operation.StatusPending,
		DefinitionName: definition.Name(),
		SchedulerName:  schedulerName,
		CreatedAt:      time.Now(),
	}

	err := om.Storage.CreateOperation(ctx, op, definition.Marshal())
	if err != nil {
		return nil, fmt.Errorf("failed to create operation: %w", err)
	}

	err = om.Flow.InsertOperationID(ctx, op.SchedulerName, op.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert operation on flow: %w", err)
	}

	return op, nil
}

func (om *OperationManager) GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, operations.Definition, error) {
	op, definitionContents, err := om.Storage.GetOperation(ctx, schedulerName, operationID)
	if err != nil {
		return nil, nil, err
	}

	definitionConstructor := om.OperationDefinitionConstructors[op.DefinitionName]
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
func (om *OperationManager) NextSchedulerOperation(ctx context.Context, schedulerName string) (*operation.Operation, operations.Definition, error) {
	operationID, err := om.Flow.NextOperationID(ctx, schedulerName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve the next operation: %w", err)
	}

	return om.GetOperation(ctx, schedulerName, operationID)
}

// StartOperation used when an operation will start executing.
func (om *OperationManager) StartOperation(ctx context.Context, op *operation.Operation, cancelFunction context.CancelFunc) error {
	err := om.Storage.UpdateOperationStatus(ctx, op.SchedulerName, op.ID, operation.StatusInProgress)
	if err != nil {
		return fmt.Errorf("failed to start operation: %w", err)
	}

	op.Status = operation.StatusInProgress

	om.OperationCancelFunctions.putFunction(op.SchedulerName, op.ID, cancelFunction)
	return nil
}

func (om *OperationManager) ListSchedulerPendingOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {

	pendingOperationIDs, err := om.Flow.ListSchedulerPendingOperationIDs(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("failed to list all pending operations: %w", err)
	}
	pendingOperations := make([]*operation.Operation, len(pendingOperationIDs))
	for i, operationID := range pendingOperationIDs {
		op, _, err := om.Storage.GetOperation(ctx, schedulerName, operationID)
		if err != nil {
			return nil, err
		}
		pendingOperations[i] = op
	}

	return pendingOperations, nil
}

func (om *OperationManager) ListSchedulerActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {
	ops, err := om.Storage.ListSchedulerActiveOperations(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("failed get active operations list fort scheduler %s : %w", schedulerName, err)
	}
	if len(ops) == 0 {
		return []*operation.Operation{}, err
	}
	err = om.addOperationsLeaseData(ctx, schedulerName, ops)
	if err != nil {
		return nil, err
	}
	return ops, nil
}

func (om *OperationManager) ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {

	return om.Storage.ListSchedulerFinishedOperations(ctx, schedulerName)
}

func (om *OperationManager) FinishOperation(ctx context.Context, op *operation.Operation) error {
	err := om.Storage.UpdateOperationStatus(ctx, op.SchedulerName, op.ID, op.Status)
	if err != nil {
		return fmt.Errorf("failed to finish operation: %w", err)
	}

	om.OperationCancelFunctions.removeFunction(op.SchedulerName, op.ID)

	return nil
}

func (om *OperationManager) EnqueueOperationCancellationRequest(ctx context.Context, schedulerName, operationID string) error {
	_, err := om.SchedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		return fmt.Errorf("failed to fetch scheduler from storage: %w", err)
	}

	_, _, err = om.Storage.GetOperation(ctx, schedulerName, operationID)
	if err != nil {
		return fmt.Errorf("failed to fetch operation from storage: %w", err)
	}

	err = om.Flow.EnqueueOperationCancellationRequest(ctx, ports.OperationCancellationRequest{
		SchedulerName: schedulerName,
		OperationID:   operationID,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue operation cancellation request: %w", err)
	}

	return nil
}

func (om *OperationManager) WatchOperationCancellationRequests(ctx context.Context) error {
	requestChannel := om.Flow.WatchOperationCancellationRequests(ctx)

	for {
		select {
		case request, ok := <-requestChannel:
			if !ok {
				return errors.NewErrUnexpected("operation cancellation request channel closed")
			}

			err := om.cancelOperation(ctx, request.SchedulerName, request.OperationID)
			if err != nil {
				om.Logger.
					With(zap.String("schedulerName", request.SchedulerName)).
					With(zap.String("operationID", request.OperationID)).
					With(zap.Error(err)).
					Error("failed to cancel operation")
			}

		case <-ctx.Done():
			if goerrors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return fmt.Errorf("loop to consume cancel operation requests received an error context event: %w", ctx.Err())
		}
	}
}

func (om *OperationManager) GrantLease(ctx context.Context, operation *operation.Operation) error {
	err := om.LeaseStorage.GrantLease(ctx, operation.SchedulerName, operation.ID, om.Config.OperationLeaseTtl)
	if err != nil {
		return fmt.Errorf("failed to grant lease to operation: %w", err)
	}

	return nil
}

func (om *OperationManager) RevokeLease(ctx context.Context, operation *operation.Operation) error {
	err := om.LeaseStorage.RevokeLease(ctx, operation.SchedulerName, operation.ID)
	if err != nil {
		return fmt.Errorf("failed to revoke lease to operation: %w", err)
	}

	return nil
}

func (om *OperationManager) StartLeaseRenewGoRoutine(operationCtx context.Context, op *operation.Operation) {
	go func() {
		om.Logger.Info("starting operation lease renew go routine")

		ticker := time.NewTicker(om.Config.OperationLeaseTtl)
		defer ticker.Stop()

	renewLeaseLoop:
		for {
			select {
			case <-ticker.C:
				if op.Status == operation.StatusFinished || op.Status == operation.StatusError {
					status, _ := op.Status.String()
					om.Logger.Sugar().Infof("finish operation lease renew go routine since operation got status %v", status)
					break renewLeaseLoop
				}

				err := om.LeaseStorage.RenewLease(operationCtx, op.SchedulerName, op.ID, om.Config.OperationLeaseTtl)
				if err != nil {
					om.Logger.
						With(zap.String("schedulerName", op.SchedulerName)).
						With(zap.String("operationID", op.ID)).
						With(zap.Error(err)).
						Error("failed to renew operation lease")
				}
			case <-operationCtx.Done():
				if goerrors.Is(operationCtx.Err(), context.Canceled) {
					om.Logger.Info("finish operation lease renew go routine since operation is canceled")
				} else {
					om.Logger.
						With(zap.String("schedulerName", op.SchedulerName)).
						With(zap.String("operationID", op.ID)).
						With(zap.Error(operationCtx.Err())).
						Error("loop to renew operation lease received an error context event")
				}
				break renewLeaseLoop
			}
		}
	}()
}

func (om *OperationManager) addOperationsLeaseData(ctx context.Context, schedulerName string, ops []*operation.Operation) error {
	opMap := make(map[string]*operation.Operation)
	opIds := make([]string, 0, len(ops))
	for _, op := range ops {
		opMap[op.ID] = op
		opIds = append(opIds, op.ID)
	}

	leases, err := om.LeaseStorage.FetchOperationsLease(ctx, schedulerName, opIds...)
	if err != nil {
		return fmt.Errorf("failed to fetch operations lease for scheduler %s: %w", schedulerName, err)
	}

	for _, lease := range leases {
		opMap[lease.OperationID].SetLease(lease)
	}

	return nil
}

func (om OperationManager) cancelOperation(ctx context.Context, schedulerName, operationID string) error {

	op, _, err := om.Storage.GetOperation(ctx, schedulerName, operationID)
	if err != nil {
		return fmt.Errorf("failed to fetch operation from storage: %w", err)
	}

	if op.Status == operation.StatusPending {
		err = om.Storage.UpdateOperationStatus(ctx, schedulerName, operationID, operation.StatusCanceled)
		if err != nil {
			return fmt.Errorf("failed update operation as canceled: %w", err)
		}
	} else {
		cancelFn, err := om.OperationCancelFunctions.getFunction(schedulerName, operationID)
		if err != nil {
			return fmt.Errorf("failed to fetch cancel function from operation: %w", err)
		}
		cancelFn()
	}

	return nil
}
