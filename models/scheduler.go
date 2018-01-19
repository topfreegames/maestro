// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"time"

	"github.com/go-pg/pg"
	"github.com/topfreegames/extensions/pg/interfaces"
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

// SecretKeyRef has environment variables from secrets
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
	Limit     int `yaml:"limit" json:"limit" valid:"int64"`
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

// Forwarder has the configuration for the event forwarders
type Forwarder struct {
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Metadata map[string]interface{} `yaml:"metadata" json:"metadata"`
}

// Container represents a container inside a pod
type Container struct {
	Name     string     `yaml:"name" json:"name" valid:"required"`
	Image    string     `yaml:"image" json:"image" valid:"required"`
	Ports    []*Port    `yaml:"ports" json:"ports"`
	Limits   *Resources `yaml:"limits" json:"limits"`
	Requests *Resources `yaml:"requests" json:"requests"`
	Env      []*EnvVar  `yaml:"env" json:"env"`
	Command  []string   `yaml:"cmd" json:"cmd"`
}

// NewWithCopiedEnvs copy all container properties and create new envs with same values as c
func (c *Container) NewWithCopiedEnvs() *Container {
	new := &Container{
		Name:     c.Name,
		Image:    c.Image,
		Ports:    c.Ports,
		Limits:   c.Limits,
		Requests: c.Requests,
		Command:  c.Command,
		Env:      make([]*EnvVar, len(c.Env)),
	}

	for i, env := range c.Env {
		new.Env[i] = &EnvVar{
			Name:      env.Name,
			Value:     env.Value,
			ValueFrom: env.ValueFrom,
		}
	}

	return new
}

//GetImage returns the container Image
func (c *Container) GetImage() string {
	return c.Image
}

//GetName returns the container Image
func (c *Container) GetName() string {
	return c.Name
}

//GetPorts returns the container Ports
func (c *Container) GetPorts() []*Port {
	return c.Ports
}

//GetLimits returns the container Limits
func (c *Container) GetLimits() *Resources {
	return c.Limits
}

//GetRequests returns the container Requests
func (c *Container) GetRequests() *Resources {
	return c.Requests
}

//GetCmd returns the container Cmd
func (c *Container) GetCmd() []string {
	return c.Command
}

//GetEnv returns the container Env
func (c *Container) GetEnv() []*EnvVar {
	return c.Env
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

// Load loads a scheduler from the database using the scheduler name
func (c *Scheduler) Load(db interfaces.DB) error {
	_, err := db.Query(c, "SELECT * FROM schedulers WHERE name = ?", c.Name)
	return err
}

// LoadSchedulers loads a slice of schedulers from database by names
func LoadSchedulers(db interfaces.DB, names []string) ([]Scheduler, error) {
	var schedulers []Scheduler
	_, err := db.Query(
		&schedulers,
		"SELECT * FROM schedulers WHERE name IN (?)",
		pg.In(names),
	)
	return schedulers, err
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
	if configYAML.AutoScaling.Up.Trigger.Limit <= 0 {
		configYAML.AutoScaling.Up.Trigger.Limit = 90
	}
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

// LoadConfig loads the scheduler config from the database
func LoadConfig(db interfaces.DB, schedulerName string) (string, error) {
	c := new(Scheduler)
	_, err := db.Query(c, "SELECT yaml FROM schedulers WHERE name = ?", schedulerName)
	return c.YAML, err
}
