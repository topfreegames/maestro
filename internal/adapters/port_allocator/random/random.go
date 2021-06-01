package random

import (
	"math/rand"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

var _ ports.PortAllocator = (*RandomPortAllocator)(nil)

type RandomPortAllocator struct {
	random           *rand.Rand
	defaultPortRange *entities.PortRange
}

func NewRandomPortAllocator(portRange *entities.PortRange) *RandomPortAllocator {
	return &RandomPortAllocator{
		defaultPortRange: portRange,
		random:           rand.New(rand.NewSource(time.Now().Unix())),
	}
}

func (r *RandomPortAllocator) Allocate(portRange *entities.PortRange, quantity int) ([]int32, error) {
	currentRange := r.defaultPortRange
	if portRange != nil {
		currentRange = portRange
	}

	ports := []int32{}
	alreadyAllocated := make(map[int32]struct{})

	for len(ports) < quantity {
		port := currentRange.Start + r.random.Int31n(currentRange.Total())
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
