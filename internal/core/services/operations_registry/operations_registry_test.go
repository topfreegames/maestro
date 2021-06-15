package operations_registry

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

func TestRegistry(t *testing.T) {
	t.Run("register and get", func(t *testing.T) {
		registry := NewRegistry()

		var def operation.Definition
		registry.Register("some", func() operation.Definition { return def })

		defFromRegistry, err := registry.Get("some")
		require.NoError(t, err)
		require.Equal(t, def, defFromRegistry)
	})

	t.Run("not found", func(t *testing.T) {
		registry := NewRegistry()

		defFromRegistry, err := registry.Get("some")
		require.Error(t, err)
		require.Nil(t, defFromRegistry)
	})
}
