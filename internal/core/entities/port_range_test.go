//+build unit

package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePortRange(t *testing.T) {
	cases := map[string]struct {
		rangeStr          string
		expectedPortRange *PortRange
		total             int32
		withError         bool
	}{
		"valid format": {
			rangeStr:          "1000-2000",
			total:             1001,
			expectedPortRange: &PortRange{Start: 1000, End: 2000},
		},
		"invalid format": {
			rangeStr:          "abc",
			expectedPortRange: nil,
			withError:         true,
		},
		"missing end": {
			rangeStr:          "1000",
			expectedPortRange: nil,
			withError:         true,
		},
		"invalid start value": {
			rangeStr:          "abc-2000",
			expectedPortRange: nil,
			withError:         true,
		},
		"invalid end value": {
			rangeStr:          "abc-2000",
			expectedPortRange: nil,
			withError:         true,
		},
		"empty end": {
			rangeStr:          "1000-",
			expectedPortRange: nil,
			withError:         true,
		},
		"end lower than start": {
			rangeStr:          "1000-999",
			expectedPortRange: nil,
			withError:         true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			portRange, err := ParsePortRange(test.rangeStr)
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, portRange)
			require.Equal(t, test.expectedPortRange.Start, portRange.Start)
			require.Equal(t, test.expectedPortRange.End, portRange.End)
			require.Equal(t, test.total, portRange.Total())
		})
	}
}
