package entities

import "time"

type GameRoomSpec struct {
	Version                string
	TerminationGracePeriod time.Duration
	Containers             []GameRoomContainer

	// NOTE: consider moving it to a kubernetes-specific option?
	Toleration string
	// NOTE: consider moving it to a kubernetes-specific option?
	Affinity string
}
