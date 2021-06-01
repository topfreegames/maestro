package ports

import "github.com/topfreegames/maestro/internal/core/entities"

// PortAllocator is responsbile for allocating ports for the game rooms.
type PortAllocator interface {
	// Allocate allocates some port numbers of any type. If the allocation fails
	// for any reason, it returns an error.
	Allocate(portRange *entities.PortRange, quantity int) ([]int32, error)
}
