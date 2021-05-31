package random

import (
	"math/rand"
	"time"

	"github.com/topfreegames/maestro/internal/services/port_allocator"
)

type RandomPortAllocator struct {
	random    *rand.Rand
	portRange *port_allocator.PortRange
}

func NewRandomPortAllocator(portRange *port_allocator.PortRange) *RandomPortAllocator {
	return &RandomPortAllocator{
		portRange: portRange,
		random:    rand.New(rand.NewSource(time.Now().Unix())),
	}
}

func (r *RandomPortAllocator) Allocate(quantity int) ([]int32, error) {
	ports := []int32{}
	alreadyAllocated := make(map[int32]struct{})

	for len(ports) < quantity {
		port := r.portRange.Start + r.random.Int31n(r.portRange.Total)
		if _, ok := alreadyAllocated[port]; ok {
			if len(alreadyAllocated) == int(r.portRange.Total) {
				return ports, port_allocator.ErrNotEnoughPorts
			}

			continue
		}

		alreadyAllocated[port] = struct{}{}
		ports = append(ports, port)
	}

	return ports, nil
}
