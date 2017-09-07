// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"time"

	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	pg "gopkg.in/pg.v5"
	yaml "gopkg.in/yaml.v2"
)

// Scheduler is the struct that defines a maestro scheduler
type Scheduler struct {
	ID                 string      `db:"id"`
	Name               string      `db:"name" yaml:"name"`
	Game               string      `db:"game" yaml:"game"`
	YAML               string      `db:"yaml"`
	State              string      `db:"state"`
	StateLastChangedAt int64       `db:"state_last_changed_at"`
	LastScaleOpAt      int64       `db:"last_scale_op_at"`
	CreatedAt          pg.NullTime `db:"created_at"`
	UpdatedAt          pg.NullTime `db:"updated_at"`
}

// Resources the CPU and memory resources limits
type Resources struct {
	CPU    string `yaml:"cpu" json:"cpu" valid:"int64"`
	Memory string `yaml:"memory" json:"memory" valid:"int64"`
}

// EnvVar has name and value of an environment variable
// Obs.: ValueFrom must not be a pointer so it can compare at controller.go MustUpdate function
type EnvVar struct {
	Name      string    `yaml:"name" json:"name"`
	Value     string    `yaml:"value" json:"value"`
	ValueFrom ValueFrom `yaml:"valueFrom" json:"valueFrom"`
}

// ValueFrom has environment variables from secrets
// Obs.: ValueFrom must not be a pointer so it can compare at controller.go MustUpdate function
type ValueFrom struct {
	SecretKeyRef SecretKeyRef `yaml:"secretKeyRef" json:"secretKeyRef"`
}

// ValueFrom has environment variables from secrets
type SecretKeyRef struct {
	Name string `yaml:"name" json:"name"`
	Key  string `yaml:"key" json:"key"`
}

// ScalingPolicyTrigger has the configuration for a scaling policy trigger
// During 'Time' seconds with n measures, 'Usage'% of the machines needs
// to be occupied on 'Threshold'% of these n points.
// This will trigger a scale up or scale down.
type ScalingPolicyTrigger struct {
	Time      int `yaml:"time" json:"time" valid:"int64"`
	Usage     int `yaml:"usage" json:"usage" valid:"int64"`
	Threshold int `yaml:"threshold" json:"threshold" valid:"int64"`
}

// ScalingPolicy has the configuration for a scaling policy
type ScalingPolicy struct {
	Cooldown int                   `yaml:"cooldown" json:"cooldown" valid:"int64"`
	Delta    int                   `yaml:"delta" json:"delta" valid:"int64"`
	Trigger  *ScalingPolicyTrigger `yaml:"trigger" json:"trigger"`
}

// AutoScaling has the configuration for the GRU's auto scaling
type AutoScaling struct {
	Min  int            `yaml:"min" json:"min" valid:"int64"`
	Up   *ScalingPolicy `yaml:"up" json:"up" valid:"int64"`
	Down *ScalingPolicy `yaml:"down" json:"down" valid:"int64"`
}

// ConfigYAML is the struct for the config yaml
type ConfigYAML struct {
	Name            string       `yaml:"name" json:"name" valid:"required"`
	Game            string       `yaml:"game" json:"game" valid:"required"`
	Image           string       `yaml:"image" json:"image" valid:"required"`
	Ports           []*Port      `yaml:"ports" json:"ports"`
	Limits          *Resources   `yaml:"limits" json:"limits"`
	Requests        *Resources   `yaml:"requests" json:"requests"`
	ShutdownTimeout int          `yaml:"shutdownTimeout" json:"shutdownTimeout" valid:"int64"`
	AutoScaling     *AutoScaling `yaml:"autoscaling" json:"autoscaling" valid:"required"`
	Env             []*EnvVar    `yaml:"env" json:"env"`
	Cmd             []string     `yaml:"cmd" json:"cmd"`
	NodeAffinity    string       `yaml:"affinity" json:"affinity"`
	NodeToleration  string       `yaml:"toleration" json:"toleration"`
	OccupiedTimeout int64        `yaml:"occupiedTimeout" json:"occupiedTimeout"`
}

// NewScheduler is the scheduler constructor
func NewScheduler(name, game, yaml string) *Scheduler {
	return &Scheduler{
		Name:               name,
		Game:               game,
		YAML:               yaml,
		State:              StateCreating,
		StateLastChangedAt: time.Now().Unix(),
	}
}

// NewConfigYAML is the config yaml constructor
func NewConfigYAML(yamlString string) (*ConfigYAML, error) {
	configYAML := ConfigYAML{}
	err := yaml.Unmarshal([]byte(yamlString), &configYAML)
	if err != nil {
		return nil, errors.NewYamlError("parse yaml error", err)
	}
	return &configYAML, nil
}

// Load loads a scheduler from the database using the scheduler name
func (c *Scheduler) Load(db interfaces.DB) error {
	_, err := db.Query(c, "SELECT * FROM schedulers WHERE name = ?", c.Name)
	return err
}

// Create creates a scheduler in the database
func (c *Scheduler) Create(db interfaces.DB) error {
	_, err := db.Query(c, "INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id", c)
	return err
}

// Update updates a scheduler in the database
func (c *Scheduler) Update(db interfaces.DB) error {
	_, err := db.Query(c, "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", c)
	return err
}

// Delete deletes a scheduler from the database using the scheduler name
func (c *Scheduler) Delete(db interfaces.DB) error {
	_, err := db.Exec("DELETE FROM schedulers WHERE name = ?", c.Name)
	if err != nil && err.Error() != "pg: no rows in result set" {
		return err
	}
	return nil
}

// GetAutoScalingPolicy returns the scheduler auto scaling policy
func (c *Scheduler) GetAutoScalingPolicy() *AutoScaling {
	configYAML, _ := NewConfigYAML(c.YAML)
	return configYAML.AutoScaling
}

// ListSchedulersNames list all schedulers names
func ListSchedulersNames(db interfaces.DB) ([]string, error) {
	var schedulers []Scheduler
	_, err := db.Query(&schedulers, "SELECT name FROM schedulers")
	if err != nil && err.Error() != "pg: no rows in result set" {
		return []string{}, err
	}
	names := make([]string, len(schedulers))
	for idx, scheduler := range schedulers {
		names[idx] = scheduler.Name
	}
	return names, nil
}

func LoadConfig(db interfaces.DB, schedulerName string) (string, error) {
	c := new(Scheduler)
	_, err := db.Query(c, "SELECT yaml FROM schedulers WHERE name = ?", schedulerName)
	return c.YAML, err
}
