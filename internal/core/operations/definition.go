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

type DefinitionConstructor func() Definition
type DefinitionConstructors map[string]DefinitionConstructor

func NewDefinitionConstructors() DefinitionConstructors {
	return DefinitionConstructors{}
}

// Definition is the operation parameters. It must be able to encode/decode
// itself.
type Definition interface {
	// ShouldExecute this function is called right before the operation is
	// started to check if the worker needs to execute it or not.
	ShouldExecute(ctx context.Context, currentOperations []*operation.Operation) bool
	// Marshal encodes the definition to be stored.
	Marshal() []byte
	// Unmarshal decodes the definition into itself.
	Unmarshal(raw []byte) error
	// Name returns the definition name. This is used to compare and identify it
	// among other definitions.
	Name() string
	// NoAction return a boolean showing when an operation does not change the runtime.
	NoAction() bool
}
