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

package game_room_test

import (
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"testing"
)

func TestIsImagePullPolicySupported(t *testing.T) {
	t.Run("with success when policy is supported by maestro", func(t *testing.T) {
		supported := game_room.IsImagePullPolicySupported("Always")
		require.True(t, supported)
	})

	t.Run("fails when policy is not supported by maestro", func(t *testing.T) {
		wrongPolicy := "unsupported"
		supported := game_room.IsImagePullPolicySupported(wrongPolicy)
		require.False(t, supported)
	})
}

func TestIsProtocolSupported(t *testing.T) {
	t.Run("with success when port protocol is supported by maestro", func(t *testing.T) {
		supported := game_room.IsProtocolSupported("tcp")
		require.True(t, supported)
	})

	t.Run("fails when port protocol is not supported by maestro", func(t *testing.T) {
		wrongProtocol := "unsupported"
		supported := game_room.IsProtocolSupported(wrongProtocol)
		require.False(t, supported)
	})
}
