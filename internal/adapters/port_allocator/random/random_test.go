//+build unit

package random

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
)

func TestRandomPortAllocator(t *testing.T) {
	cases := map[string]struct {
		defaultPortRange *entities.PortRange
		portRange        *entities.PortRange
		quantity         int
		expectedPorts    []int32
		withError        bool
	}{
		"correct number of ports": {
			portRange:     &entities.PortRange{Start: 1000, End: 1004},
			quantity:      5,
			expectedPorts: []int32{1000, 1001, 1002, 1003, 1004},
		},
		"not enough ports": {
			portRange: &entities.PortRange{Start: 1000, End: 1004},
			quantity:  7,
			withError: true,
		},
		"use default portRange": {
			defaultPortRange: &entities.PortRange{Start: 2000, End: 2004},
			quantity:  5,
			expectedPorts: []int32{2000, 2001, 2002, 2003, 2004},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := NewRandomPortAllocator(test.defaultPortRange).Allocate(test.portRange, test.quantity)
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.ElementsMatch(t, test.expectedPorts, res)
		})
	}
}
