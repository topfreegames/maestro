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

package operations_registry

import (
	"fmt"

	"github.com/topfreegames/maestro/internal/core/operations"
)

// OperationDefinitionConstructor defines a function that constructs a new
// definition.
type OperationDefinitionConstructor func() operations.Definition

// Registry contains all the operations definitions.
type Registry map[string]OperationDefinitionConstructor

// NewRegistry initialises a new registry.
func NewRegistry() Registry {
	return Registry(map[string]OperationDefinitionConstructor{})
}

// Register register a new operation definition.
func (r Registry) Register(name string, constructor OperationDefinitionConstructor) {
	r[name] = constructor
}

// Get fetches an operation definition.
func (r Registry) Get(name string) (operations.Definition, error) {
	if constructor, ok := r[name]; ok {
		return constructor(), nil
	}

	return nil, fmt.Errorf("definition \"%s\" not found", name)
}

// DefaultRegistry is a shared registry on the package-level.
var DefaultRegistry Registry = NewRegistry()

// Register register a new definition on the the default registry.
func Register(name string, constructor OperationDefinitionConstructor) {
	DefaultRegistry.Register(name, constructor)
}

// Get fetches a definition on the the default registry.
func Get(name string) (operations.Definition, error) {
	return DefaultRegistry.Get(name)
}
