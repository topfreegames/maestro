package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayerEvent(t *testing.T) {
	t.Run("with success when converting to RoomEventType", func(t *testing.T) {

		converted, err := ConvertToPlayerEventType("playerLeft")
		require.NoError(t, err)
		require.IsType(t, PlayerEventType("playerLeft"), converted)

		converted, err = ConvertToPlayerEventType("playerJoin")
		require.NoError(t, err)
		require.IsType(t, PlayerEventType("playerJoin"), converted)
	})

	t.Run("with error when converting to RoomPingEventType", func(t *testing.T) {

		_, err := ConvertToPlayerEventType("INVALID")
		require.Error(t, err)
	})
}
