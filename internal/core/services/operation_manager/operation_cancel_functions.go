package operation_manager

import (
	"context"
	"sync"

	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

type OperationCancelFunctions struct {
	mutex     sync.RWMutex
	functions map[string]map[string]context.CancelFunc
}

func NewOperationCancelFunctions() *OperationCancelFunctions {
	return &OperationCancelFunctions{
		functions: map[string]map[string]context.CancelFunc{},
	}
}

func (of *OperationCancelFunctions) putFunction(schedulerName, operationID string, cancelFunc context.CancelFunc) {
	of.mutex.Lock()
	schedulerCancelFunctions := of.functions[schedulerName]
	if schedulerCancelFunctions == nil {
		of.functions[schedulerName] = map[string]context.CancelFunc{}
	}
	of.functions[schedulerName][operationID] = cancelFunc
	of.mutex.Unlock()
}

func (of *OperationCancelFunctions) removeFunction(schedulerName, operationID string) {
	of.mutex.Lock()

	schedulerOperationCancellationFunctions := of.functions[schedulerName]
	if schedulerOperationCancellationFunctions == nil {
		return
	}

	delete(schedulerOperationCancellationFunctions, operationID)

	of.mutex.Unlock()
}

func (of *OperationCancelFunctions) getFunction(schedulerName, operationID string) (context.CancelFunc, error) {
	of.mutex.RLock()
	schedulerOperationCancellationFunctions := of.functions[schedulerName]
	if schedulerOperationCancellationFunctions == nil {
		return nil, errors.NewErrNotFound("no cancel scheduler found for scheduler name: %s", schedulerName)
	}

	if schedulerOperationCancellationFunctions[operationID] == nil {
		return nil, errors.NewErrNotFound("no cancel function found for scheduler name: %s and operation id: %s", schedulerName, operationID)
	}

	function := schedulerOperationCancellationFunctions[operationID]
	of.mutex.RUnlock()

	return function, nil
}
