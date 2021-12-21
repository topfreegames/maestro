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
