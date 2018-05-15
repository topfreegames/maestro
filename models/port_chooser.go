package models

import (
	"math/rand"
	"sync"
	"time"
)

// PortChooser has a Choose method that returns
// an array of ports
type PortChooser interface {
	Choose(start, end, quantity int) []int
}

var (
	random *rand.Rand
	once   sync.Once
)

// RandomPortChooser implements PortChooser
type RandomPortChooser struct{}

// Choose initialized an seed and chooses quantity random ports between [start, end]
func (r *RandomPortChooser) Choose(start, end, quantity int) []int {
	once.Do(func() {
		source := rand.NewSource(time.Now().Unix())
		random = rand.New(source)
	})

	ports := make([]int, quantity)
	for i := 0; i < quantity; i++ {
		port := start + random.Intn(end-start)
		ports[i] = port
	}

	return ports
}
