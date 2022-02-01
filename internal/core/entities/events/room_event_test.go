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

package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoomEvent(t *testing.T) {
	t.Run("with success when converting to RoomEventType", func(t *testing.T) {

		converted, err := ConvertToRoomEventType("resync")
		require.NoError(t, err)
		require.IsType(t, RoomEventType("resync"), converted)

		converted, err = ConvertToRoomEventType("roomEvent")
		require.NoError(t, err)
		require.IsType(t, RoomEventType("roomEvent"), converted)
	})

	t.Run("with success when converting to RoomPingEventType", func(t *testing.T) {
		converted, err := ConvertToRoomPingEventType("ready")
		require.NoError(t, err)
		require.IsType(t, RoomPingEventType("ready"), converted)

		converted, err = ConvertToRoomPingEventType("occupied")
		require.NoError(t, err)
		require.IsType(t, RoomPingEventType("occupied"), converted)

		converted, err = ConvertToRoomPingEventType("terminating")
		require.NoError(t, err)
		require.IsType(t, RoomPingEventType("terminating"), converted)

		converted, err = ConvertToRoomPingEventType("terminated")
		require.NoError(t, err)
		require.IsType(t, RoomPingEventType("terminated"), converted)
	})

	t.Run("with error when converting to RoomEventType", func(t *testing.T) {

		_, err := ConvertToRoomEventType("INVALID")
		require.Error(t, err)
	})

	t.Run("with error when converting to RoomPingEventType", func(t *testing.T) {

		_, err := ConvertToRoomPingEventType("INVALID")
		require.Error(t, err)
	})
}
