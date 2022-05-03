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

package scheduler

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/ghodss/yaml"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/go-pg/pg"
	"github.com/topfreegames/maestro/internal/core/entities"
)

type Scheduler struct {
	ID                  string      `db:"id"`
	Name                string      `db:"name" yaml:"name"`
	Game                string      `db:"game" yaml:"game"`
	Yaml                string      `db:"yaml"`
	State               string      `db:"state"`
	StateLastChangedAt  int64       `db:"state_last_changed_at"`
	LastScaleOpAt       int64       `db:"last_scale_op_at"`
	CreatedAt           pg.NullTime `db:"created_at"`
	UpdatedAt           pg.NullTime `db:"updated_at"`
	Version             string      `db:"version"`
	RollbackVersion     string      `db:"rollback_version"`
	RollingUpdateStatus string      `db:"rolling_update_status"`
}

type schedulerInfo struct {
	TerminationGracePeriod time.Duration
	Toleration             string
	Affinity               string
	Containers             []game_room.Container
	PortRange              *entities.PortRange
	MaxSurge               string
	RoomsReplicas          int
	Forwarders             []*forwarder.Forwarder
}

func NewDBScheduler(scheduler *entities.Scheduler) *Scheduler {
	info := schedulerInfo{
		TerminationGracePeriod: scheduler.Spec.TerminationGracePeriod,
		Toleration:             scheduler.Spec.Toleration,
		Affinity:               scheduler.Spec.Affinity,
		Containers:             scheduler.Spec.Containers,
		PortRange:              scheduler.PortRange,
		MaxSurge:               scheduler.MaxSurge,
		RoomsReplicas:          scheduler.RoomsReplicas,
		Forwarders:             scheduler.Forwarders,
	}
	yamlBytes, _ := yaml.Marshal(info)
	return &Scheduler{
		Name:            scheduler.Name,
		Game:            scheduler.Game,
		Version:         scheduler.Spec.Version,
		RollbackVersion: scheduler.RollbackVersion,
		State:           scheduler.State,
		Yaml:            string(yamlBytes),
	}
}

func (s *Scheduler) ToScheduler() (*entities.Scheduler, error) {
	var info schedulerInfo
	err := yaml.Unmarshal([]byte(s.Yaml), &info)
	if err != nil {
		return nil, err
	}
	return &entities.Scheduler{
		Name:  s.Name,
		Game:  s.Game,
		State: s.State,
		Spec: game_room.Spec{
			Version:                s.Version,
			TerminationGracePeriod: info.TerminationGracePeriod,
			Toleration:             info.Toleration,
			Affinity:               info.Affinity,
			Containers:             info.Containers,
		},
		PortRange:       info.PortRange,
		RollbackVersion: s.RollbackVersion,
		CreatedAt:       s.CreatedAt.Time,
		MaxSurge:        info.MaxSurge,
		RoomsReplicas:   info.RoomsReplicas,
		Forwarders:      info.Forwarders,
	}, nil
}
