package pg

import (
	"time"

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
}

func NewDBScheduler(scheduler *entities.Scheduler) *Scheduler {
	info := schedulerInfo{
		TerminationGracePeriod: scheduler.Spec.TerminationGracePeriod,
		Toleration:             scheduler.Spec.Toleration,
		Affinity:               scheduler.Spec.Affinity,
		Containers:             scheduler.Spec.Containers,
		PortRange:              scheduler.PortRange,
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
	}, nil
}
