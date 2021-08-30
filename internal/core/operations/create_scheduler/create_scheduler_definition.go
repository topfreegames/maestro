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

package create_scheduler

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

const OperationName = "create_scheduler"

type CreateSchedulerDefinition struct{}

// ShouldExecute we're going to always perform the scheduler creation.
func (d *CreateSchedulerDefinition) ShouldExecute(_ context.Context, _ []*operation.Operation) bool {
	return true
}

func (d *CreateSchedulerDefinition) Name() string {
	return OperationName
}

func (d *CreateSchedulerDefinition) Marshal() []byte {
	return make([]byte, 0)
}

func (d *CreateSchedulerDefinition) Unmarshal(raw []byte) error {
	return nil
}
