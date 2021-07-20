package operations

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

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
	// amond other definitions.
	Name() string
}
