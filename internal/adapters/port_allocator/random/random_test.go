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
			quantity:         5,
			expectedPorts:    []int32{2000, 2001, 2002, 2003, 2004},
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
