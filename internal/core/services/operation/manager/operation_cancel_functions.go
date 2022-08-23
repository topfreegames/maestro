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

package manager

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
	defer of.mutex.Unlock()
	schedulerCancelFunctions := of.functions[schedulerName]
	if schedulerCancelFunctions == nil {
		of.functions[schedulerName] = map[string]context.CancelFunc{}
	}
	of.functions[schedulerName][operationID] = cancelFunc
}

func (of *OperationCancelFunctions) removeFunction(schedulerName, operationID string) {
	of.mutex.Lock()
	defer of.mutex.Unlock()

	schedulerOperationCancellationFunctions := of.functions[schedulerName]
	if schedulerOperationCancellationFunctions == nil {
		return
	}

	delete(schedulerOperationCancellationFunctions, operationID)
}

func (of *OperationCancelFunctions) getFunction(schedulerName, operationID string) (context.CancelFunc, error) {
	of.mutex.RLock()
	defer of.mutex.RUnlock()
	schedulerOperationCancellationFunctions := of.functions[schedulerName]
	if schedulerOperationCancellationFunctions == nil {
		return nil, errors.NewErrNotFound("no cancel scheduler found for scheduler name: %s", schedulerName)
	}

	if schedulerOperationCancellationFunctions[operationID] == nil {
		return nil, errors.NewErrNotFound("no cancel function found for scheduler name: %s and operation id: %s", schedulerName, operationID)
	}

	function := schedulerOperationCancellationFunctions[operationID]

	return function, nil
}
