package game_room

import "time"

type Spec struct {
	Version                string
	TerminationGracePeriod time.Duration
	Containers             []Container

	// NOTE: consider moving it to a kubernetes-specific option?
	Toleration string
	// NOTE: consider moving it to a kubernetes-specific option?
	Affinity string
}
