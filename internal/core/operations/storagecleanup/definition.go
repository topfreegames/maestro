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

package storagecleanup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

// OperationName is the storage clean up operation name constant.
const OperationName = "storage_clean_up"

var _ operations.Definition = (*Definition)(nil)

// Definition is the definition struct to storage clean up operation.
type Definition struct{}

// ShouldExecute returns true since every time the operation was called it will execute its function.
func (d *Definition) ShouldExecute(_ context.Context, _ []*operation.Operation) bool {
	return true
}

// Name returns the name of the operation.
func (d *Definition) Name() string {
	return OperationName
}

// Marshal format the definition as json byte array.
func (d *Definition) Marshal() []byte {
	bytes, err := json.Marshal(d)
	if err != nil {
		zap.L().With(zap.Error(err)).Error("error marshalling update scheduler operation definition")
		return nil
	}

	return bytes
}

// Unmarshal fill an storage clean-up operation from a byte array JSON.
func (d *Definition) Unmarshal(raw []byte) error {
	err := json.Unmarshal(raw, d)
	if err != nil {
		return fmt.Errorf("error marshalling update scheduler operation definition: %w", err)
	}

	return nil
}

// HasNoAction returns if the operation has action to be logged on the operation history.
func (d *Definition) HasNoAction() bool {
	return true
}
