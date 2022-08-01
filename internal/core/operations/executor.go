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

package operations

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

// Executor is where the actual operation logic is implemented, and it will
// receive as input its correlated definition.
type Executor interface {
	// Execute is where the operation logic will live; it will receive a context
	// that will be used for deadline and cancellation. This function has only
	// one return which is the operation error (if any);
	Execute(ctx context.Context, op *operation.Operation, definition Definition) *ExecutionError
	// Rollback is called if Execute returns an error. This will be used
	// for operations that need to do some cleanup or any process if it fails.
	Rollback(ctx context.Context, op *operation.Operation, definition Definition, executeErr *ExecutionError) error
	// Name returns the executor name. This is used to identify a executor for a
	// definition.
	Name() string
}
