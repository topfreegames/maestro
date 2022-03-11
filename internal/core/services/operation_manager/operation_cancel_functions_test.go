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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

func TestOperationCancelFunctions_putFunction(t *testing.T) {
	type fields struct {
		functions map[string]map[string]context.CancelFunc
	}
	type args struct {
		schedulerName string
		operationID   string
		cancelFunc    context.CancelFunc
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "when there is no entry in the functions map for the scheduler it creates a new one for scheduler an operation",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{},
			},
			args: args{
				schedulerName: "scheduler1",
				operationID:   "operation1",
				cancelFunc:    func() {},
			},
		},
		{
			name: "when the entry for scheduler in functions map already exists it creates a new entry for the operation",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{
					"scheduler1": {
						"operation2": func() {},
					},
				},
			},
			args: args{
				schedulerName: "scheduler1",
				operationID:   "operation1",
				cancelFunc:    func() {},
			},
		},
		{
			name: "when the entry for scheduler and operation in functions map already exists it overrides the entry",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{
					"scheduler1": {
						"operation1": func() {},
					},
				},
			},
			args: args{
				schedulerName: "scheduler1",
				operationID:   "operation1",
				cancelFunc:    func() {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			of := NewOperationCancelFunctions()
			populateFunctionsMap(tt.fields.functions, of)
			of.putFunction(tt.args.schedulerName, tt.args.operationID, tt.args.cancelFunc)
			got, err := of.getFunction(tt.args.schedulerName, tt.args.operationID)
			assert.NoError(t, err)
			assert.IsType(t, tt.args.cancelFunc, got)
		})
	}
}

func TestOperationCancelFunctions_removeFunction(t *testing.T) {
	cancelFunc := func() {}
	type fields struct {
		functions map[string]map[string]context.CancelFunc
	}
	type args struct {
		schedulerName string
		operationID   string
	}
	tests := []struct {
		name                  string
		fields                fields
		args                  args
		wantFuncBeforeRemoval context.CancelFunc
		wantErr               error
	}{
		{
			name: "when no entry exists for the scheduler and operation id in the functions map it does nothing",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{},
			},
			args: args{
				schedulerName: "scheduler-1",
				operationID:   "operation-1",
			},
			wantErr:               errors.NewErrNotFound("no cancel scheduler found for scheduler name: scheduler-1"),
			wantFuncBeforeRemoval: nil,
		},
		{
			name: "when no entry exists for the operation id in the functions map it does nothing",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{
					"scheduler-1": {
						"operation-2": cancelFunc,
					},
				},
			},
			args: args{
				schedulerName: "scheduler-1",
				operationID:   "operation-1",
			},
			wantErr:               errors.NewErrNotFound("no cancel function found for scheduler name: scheduler-1 and operation id: operation-1"),
			wantFuncBeforeRemoval: nil,
		},
		{
			name: "when the entry exists for the scheduler and operation id in the functions map it deletes the operation map entry",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{
					"scheduler-1": {
						"operation-1": func() {},
					},
				},
			},
			args: args{
				schedulerName: "scheduler-1",
				operationID:   "operation-1",
			},
			wantErr:               errors.NewErrNotFound("no cancel function found for scheduler name: scheduler-1 and operation id: operation-1"),
			wantFuncBeforeRemoval: cancelFunc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			of := NewOperationCancelFunctions()
			populateFunctionsMap(tt.fields.functions, of)
			got, _ := of.getFunction(tt.args.schedulerName, tt.args.operationID)
			if tt.wantFuncBeforeRemoval != nil {
				assert.NotNil(t, got)
			}
			of.removeFunction(tt.args.schedulerName, tt.args.operationID)
			_, err := of.getFunction(tt.args.schedulerName, tt.args.operationID)
			assert.Equal(t, err, tt.wantErr)
		})
	}
}

func TestOperationCancelFunctions_getFunction(t *testing.T) {
	cancelFunc := func() {}
	type fields struct {
		functions map[string]map[string]context.CancelFunc
	}
	type args struct {
		schedulerName string
		operationID   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    context.CancelFunc
		wantErr error
	}{
		{
			name: "when the function for the scheduler exists it returns the function",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{
					"scheduler-1": {
						"operation-1": cancelFunc,
					},
				},
			},
			args: args{
				schedulerName: "scheduler-1",
				operationID:   "operation-1",
			},
			want:    cancelFunc,
			wantErr: nil,
		},
		{
			name: "when no scheduler is found in functions map it returns error",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{},
			},
			args: args{
				schedulerName: "nonExistent",
				operationID:   "opId-1",
			},
			want:    nil,
			wantErr: errors.NewErrNotFound("no cancel scheduler found for scheduler name: nonExistent"),
		},
		{
			name: "when no function is found in functions map for scheduler it returns error",
			fields: fields{
				functions: map[string]map[string]context.CancelFunc{
					"scheduler-1": {
						"operation-2": cancelFunc,
					},
				},
			},
			args: args{
				schedulerName: "scheduler-1",
				operationID:   "inexistent-opId-1",
			},
			want:    nil,
			wantErr: errors.NewErrNotFound("no cancel function found for scheduler name: scheduler-1 and operation id: inexistent-opId-1"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			of := NewOperationCancelFunctions()
			populateFunctionsMap(tt.fields.functions, of)
			got, err := of.getFunction(tt.args.schedulerName, tt.args.operationID)
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.IsType(t, tt.want, got)
			}
		})
	}
}

func populateFunctionsMap(schedulersFunctionsMap map[string]map[string]context.CancelFunc, of *OperationCancelFunctions) {
	for scheduler, functionsMap := range schedulersFunctionsMap {
		for opId, cancelFunc := range functionsMap {
			of.putFunction(scheduler, opId, cancelFunc)
		}
	}
}
