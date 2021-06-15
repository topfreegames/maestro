package operations_registry

import (
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

// OperationDefinitionConstructor defines a function that constructs a new
// definition.
type OperationDefinitionConstructor func() operation.Definition

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
func (r Registry) Get(name string) (operation.Definition, error) {
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
func Get(name string) (operation.Definition, error) {
	return DefaultRegistry.Get(name)
}
