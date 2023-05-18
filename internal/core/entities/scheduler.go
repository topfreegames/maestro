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

package entities

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/validations"
)

const (
	//StateCreating represents a cluster state
	StateCreating = "creating"

	//StateTerminating represents a cluster state
	StateTerminating = "terminating"

	//StateInSync represents a cluster state
	StateInSync = "in-sync"

	//StateOnError represents a cluster state
	StateOnError = "on-error"
)

// Scheduler represents one of the basic maestro structs.
// It holds GameRooms specifications, as well as optional events forwarders.
type Scheduler struct {
	Name            string `validate:"required,kube_resource_name"`
	Game            string `validate:"required"`
	State           string `validate:"required"`
	RollbackVersion string
	Spec            game_room.Spec
	Autoscaling     *autoscaling.Autoscaling
	PortRange       *PortRange
	RoomsReplicas   int `validate:"min=0"`
	CreatedAt       time.Time
	MaxSurge        string                 `validate:"required,max_surge"`
	Forwarders      []*forwarder.Forwarder `validate:"dive"`
	Annotations     []*Annotation
}

type Annotation struct {
	Name string
	Opt  string
}

func NewAnnotation(name string, opt string) *Annotation {
	return &Annotation{
		Name: name,
		Opt:  opt,
	}
}

// NewScheduler instantiate a new scheduler struct.
func NewScheduler(
	name string,
	game string,
	state string,
	maxSurge string,
	spec game_room.Spec,
	portRange *PortRange,
	roomsReplicas int,
	autoscaling *autoscaling.Autoscaling,
	forwarders []*forwarder.Forwarder,
	annotations []*Annotation,
) (*Scheduler, error) {
	scheduler := &Scheduler{
		Name:          name,
		Game:          game,
		State:         state,
		Spec:          spec,
		PortRange:     portRange,
		MaxSurge:      maxSurge,
		RoomsReplicas: roomsReplicas,
		Autoscaling:   autoscaling,
		Forwarders:    forwarders,
		Annotations:   annotations,
	}
	return scheduler, scheduler.Validate()
}

func (s *Scheduler) SetSchedulerVersion(version string) {
	s.Spec.Version = version
}

func (s *Scheduler) SetSchedulerRollbackVersion(version string) {
	s.RollbackVersion = version
}

func (s *Scheduler) Validate() error {
	return validations.Validate.Struct(s)
}

// IsMajorVersion checks if the scheduler changes are major or not.
// We consider major changes if the Instances need to be recreated, in this case
// the following fields require it: `Spec` and `PortRange`. Any other field
// change is considered minor (we don't need to recreate instances).
func (s *Scheduler) IsMajorVersion(newScheduler *Scheduler) bool {
	schedulerContainerPorts := map[string]game_room.ContainerPort{}

	for _, container := range s.Spec.Containers {
		for _, port := range container.Ports {
			schedulerContainerPorts[port.Name] = port
		}
	}

	return !cmp.Equal(
		s,
		newScheduler,
		cmpopts.IgnoreSliceElements(func(container game_room.ContainerPort) bool {
			return cmp.Equal(container, schedulerContainerPorts[container.Name], cmpopts.IgnoreFields(container, "HostPort"))
		}),
		cmpopts.IgnoreFields(
			Scheduler{},
			"Name",
			"Spec.Version",
			"Game",
			"State",
			"RollbackVersion",
			"CreatedAt",
			"MaxSurge",
			"RoomsReplicas",
			"Autoscaling",
		),
	)
}
