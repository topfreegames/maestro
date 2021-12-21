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

//go:build unit
// +build unit

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
