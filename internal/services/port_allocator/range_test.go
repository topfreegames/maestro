//+build !integration

package port_allocator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePortRange(t *testing.T) {
	cases := map[string]struct {
		rangeStr          string
		expectedPortRange *PortRange
		withError         error
	}{
		"valid format": {
			rangeStr:          "1000-2000",
			expectedPortRange: &PortRange{Start: 1000, End: 2000, Total: 1000},
		},
		"invalid format": {
			rangeStr:          "abc",
			expectedPortRange: nil,
			withError:         ErrPortRangeInvalidFormat,
		},
		"missing end": {
			rangeStr:          "1000",
			expectedPortRange: nil,
			withError:         ErrPortRangeInvalidFormat,
		},
		"invalid start value": {
			rangeStr:          "abc-2000",
			expectedPortRange: nil,
			withError:         ErrPortRangeInvalidValue,
		},
		"invalid end value": {
			rangeStr:          "abc-2000",
			expectedPortRange: nil,
			withError:         ErrPortRangeInvalidValue,
		},
		"empty end": {
			rangeStr:          "1000-",
			expectedPortRange: nil,
			withError:         ErrPortRangeInvalidValue,
		},
		"end lower than start": {
			rangeStr:          "1000-999",
			expectedPortRange: nil,
			withError:         ErrPortRangeEndLowerThanStart,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			portRange, err := ParsePortRange(test.rangeStr)
			if test.withError != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, test.withError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, portRange)
			require.Equal(t, test.expectedPortRange.Start, portRange.Start)
			require.Equal(t, test.expectedPortRange.End, portRange.End)
			require.Equal(t, test.expectedPortRange.Total, portRange.Total)
		})
	}
}
