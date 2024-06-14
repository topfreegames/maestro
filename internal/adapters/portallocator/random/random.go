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

package random

import (
	"math/rand"

	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

var _ ports.PortAllocator = (*RandomPortAllocator)(nil)

type RandomPortAllocator struct {
	defaultPortRange *port.PortRange
}

func NewRandomPortAllocator(portRange *port.PortRange) *RandomPortAllocator {
	return &RandomPortAllocator{
		defaultPortRange: portRange,
	}
}

func (r *RandomPortAllocator) Allocate(portRange *port.PortRange, quantity int) ([]int32, error) {
	currentRange := r.defaultPortRange
	if portRange != nil {
		currentRange = portRange
	}

	ports := []int32{}
	alreadyAllocated := make(map[int32]struct{})

	for len(ports) < quantity {
		port := currentRange.Start + rand.Int31n(currentRange.Total())
		if _, ok := alreadyAllocated[port]; ok {
			if len(alreadyAllocated) == int(currentRange.Total()) {
				return ports, errors.NewErrInvalidArgument("not enough ports to allocate")
			}

			continue
		}

		alreadyAllocated[port] = struct{}{}
		ports = append(ports, port)
	}

	return ports, nil
}
