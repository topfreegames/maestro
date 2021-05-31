//+build !integration

package random

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/services/port_allocator"
)

func TestRandomPortAllocator(t *testing.T) {
	cases := map[string]struct {
		portRange     *port_allocator.PortRange
		quantity      int
		expectedPorts []int32
		withError     error
	}{
		"correct number of ports": {
			portRange:     &port_allocator.PortRange{Start: 1000, End: 1004, Total: 5},
			quantity:      5,
			expectedPorts: []int32{1000, 1001, 1002, 1003, 1004},
		},
		"not enough ports": {
			portRange: &port_allocator.PortRange{Start: 1000, End: 1004, Total: 5},
			quantity:  7,
			withError: port_allocator.ErrNotEnoughPorts,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := NewRandomPortAllocator(test.portRange).Allocate(test.quantity)
			if test.withError != nil {
				require.Error(t, err)
				require.ErrorIs(t, test.withError, err)
				return
			}

			require.NoError(t, err)
			require.ElementsMatch(t, test.expectedPorts, res)
		})
	}
}
