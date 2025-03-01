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
	"errors"
	"time"

	"github.com/Masterminds/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/port"
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

var (
	ErrNoPortRangeConfigured     = errors.New("must configure scheduler.PortRange or scheduler.Spec.Container.Ports.HostPortRange")
	ErrBothPortRangesConfigured  = errors.New("scheduler.PortRange and scheduler.Spec.Container.Ports.HostPortRange are mutually exclusive configurations; please choose only one")
	ErrMissingContainerPortRange = errors.New("must configure HostPortRange for all scheduler.Spec.Container.Ports")
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
	PortRange       *port.PortRange
	RoomsReplicas   int `validate:"min=0"`
	CreatedAt       time.Time
	LastDownscaleAt time.Time
	MaxSurge        string                 `validate:"required,max_surge"`
	Forwarders      []*forwarder.Forwarder `validate:"dive"`
	Annotations     map[string]string
	Labels          map[string]string
	MatchAllocation *allocation.MatchAllocation `validate:"required"`
}

// NewScheduler instantiate a new scheduler struct.
func NewScheduler(
	name string,
	game string,
	state string,
	maxSurge string,
	spec game_room.Spec,
	portRange *port.PortRange,
	roomsReplicas int,
	autoscaling *autoscaling.Autoscaling,
	forwarders []*forwarder.Forwarder,
	annotations map[string]string,
	labels map[string]string,
	matchAllocation *allocation.MatchAllocation,
) (*Scheduler, error) {
	scheduler := &Scheduler{
		Name:            name,
		Game:            game,
		State:           state,
		Spec:            spec,
		PortRange:       portRange,
		MaxSurge:        maxSurge,
		RoomsReplicas:   roomsReplicas,
		Autoscaling:     autoscaling,
		Forwarders:      forwarders,
		Annotations:     annotations,
		Labels:          labels,
		MatchAllocation: matchAllocation,
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
	err := validations.Validate.Struct(s)
	if err != nil {
		return err
	}

	err = s.HasValidPortRangeConfiguration()
	if err != nil {
		return err
	}

	return nil
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
			"LastDownscaleAt",
			"MaxSurge",
			"RoomsReplicas",
			"Autoscaling",
			"MatchAllocation",
		),
	)
}

// HasValidPortRangeConfiguration checks if the scheduler's port range configuration is valid.
// It is possible to configure PortRange in the scheduler, but also HostPortRange in each Spec.Container.Ports,
// both port ranges were made optional in the API, so we need to validate them here.
// The scheduler.PortRange was kept to avoid a breaking change to schedulers in older versions.
func (s *Scheduler) HasValidPortRangeConfiguration() error {
	hasSchedulerPortRange := false
	if s.PortRange != nil {
		hasSchedulerPortRange = true
	}

	hasContainerPortRange := false
	for _, c := range s.Spec.Containers {
		amountOfHostPortRanges := 0
		for _, p := range c.Ports {
			if p.HostPortRange != nil {
				amountOfHostPortRanges++
				hasContainerPortRange = true
			}
		}

		if !hasSchedulerPortRange && len(c.Ports) != amountOfHostPortRanges {
			return ErrMissingContainerPortRange
		}
	}

	if hasSchedulerPortRange && hasContainerPortRange {
		return ErrBothPortRangesConfigured
	}

	if !hasSchedulerPortRange && !hasContainerPortRange {
		return ErrNoPortRangeConfigured
	}

	return nil
}

func (s *Scheduler) IsSameMajorVersion(otherVersion string) bool {
	otherVer, err := semver.NewVersion(otherVersion)
	if err != nil {
		return false
	}

	schedulerVer, err := semver.NewVersion(s.Spec.Version)
	if err != nil {
		return false
	}

	return otherVer.Major() == schedulerVer.Major()
}
