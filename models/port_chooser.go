package models

import (
	"math/rand"
	"sync"
	"time"
)

// PortChooser has a Choose method that returns
// an array of ports
type PortChooser interface {
	Choose(start, end, quantity int, excluding []int) []int
}

var (
	random *rand.Rand
	once   sync.Once
)

// RandomPortChooser implements PortChooser
type RandomPortChooser struct{}

// Choose initialized an seed and chooses quantity random ports between [start, end]
func (r *RandomPortChooser) Choose(start, end, quantity int, excluding []int) []int {
	once.Do(func() {
		source := rand.NewSource(time.Now().Unix())
		random = rand.New(source)
	})

	usedPortMap := map[int]bool{}
	for _, port := range excluding {
		usedPortMap[port] = true
	}

	ports := make([]int, quantity)
	for i := 0; i < quantity; i++ {
		port := start + random.Intn(end-start)
		for usedPortMap[port] {
			port = start + random.Intn(end-start)
		}
		usedPortMap[port] = true
		ports[i] = port
	}

	return ports
}
