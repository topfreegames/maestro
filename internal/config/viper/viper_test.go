//+build unit

package viper

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewViperConfig(t *testing.T) {
	t.Run("load configuration properly", func(t *testing.T) {
		configPath := fmt.Sprintf("%s/config.yaml", t.TempDir())
		err := os.WriteFile(configPath, []byte(""), 0666)
		require.NoError(t, err)

		_, err = NewViperConfig(configPath)
		require.NoError(t, err)
	})

	t.Run("fails to load config", func(t *testing.T) {
		_, err := NewViperConfig("")
		require.Error(t, err)
	})
}

func TestGetConfiguration(t *testing.T) {
	t.Run("from file", func(t *testing.T) {
		configPath := fmt.Sprintf("%s/config.yaml", t.TempDir())
		err := os.WriteFile(configPath, []byte("key: \"value\""), 0666)
		require.NoError(t, err)

		config, err := NewViperConfig(configPath)
		require.NoError(t, err)
		require.Equal(t, "value", config.GetString("key"))
	})

	t.Run("from environment variable", func(t *testing.T) {
		err := os.Setenv("MAESTRO_RANDOMCONFIG_KEY", "value")
		defer os.Unsetenv("MAESTRO_RANDOMCONFIG_KEY")
		require.NoError(t, err)

		configPath := fmt.Sprintf("%s/config.yaml", t.TempDir())
		err = os.WriteFile(configPath, []byte("key: \"value\""), 0666)
		require.NoError(t, err)

		config, err := NewViperConfig(configPath)
		require.NoError(t, err)
		require.Equal(t, "value", config.GetString("randomConfig.key"))
	})
}
