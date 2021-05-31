package port_allocator

// PortAllocator is responsbile for allocating ports for the game rooms.
type PortAllocator interface {
	// Allocate allocates some port numbers of any type. If the allocation fails
	// for any reason, it returns an error.
	Allocate(quantity int) ([]int32, error)
}
