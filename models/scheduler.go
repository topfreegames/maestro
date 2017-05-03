// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	pg "gopkg.in/pg.v5"
	yaml "gopkg.in/yaml.v2"
)

// Scheduler is the struct that defines a maestro scheduler
type Scheduler struct {
	ID        string
	Name      string `yaml:"name"`
	Game      string `yaml:"game"`
	YAML      string
	CreatedAt pg.NullTime
	UpdatedAt pg.NullTime
}

// Port has the port container port and protocol
type Port struct {
	ContainerPort int    `yaml:"containerPort" json:"containerPort" valid:"int64,required"`
	Protocol      string `yaml:"protocol" json:"protocol" valid:"required"`
	Name          string `yaml:"name" json:"name" valid:"required"`
}

// Limits has the CPU and memory resources limits
type Limits struct {
	CPU    string `yaml:"cpu" json:"cpu" valid:"int64"`
	Memory string `yaml:"memory" json:"memory" valid:"int64"`
}

// EnvVar has name and value of an enviroment variable
type EnvVar struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// ScalingPolicyTrigger has the configuration for a scaling policy trigger
type ScalingPolicyTrigger struct {
	Time  int `yaml:"time" json:"time" valid:"int64"`
	Usage int `yaml:"usage" json:"usage" valid:"int64"`
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
	Limits          *Limits      `yaml:"limits" json:"limits"`
	ShutdownTimeout int          `yaml:"shutdownTimeout" json:"shutdownTimeout" valid:"int64"`
	AutoScaling     *AutoScaling `yaml:"autoscaling" json:"autoscaling" valid:"required"`
	Env             []*EnvVar    `yaml:"env" json:"env"`
	Cmd             []string     `yaml:"cmd" json:"cmd"`
}

// NewScheduler is the scheduler constructor
func NewScheduler(name, game, yaml string) *Scheduler {
	return &Scheduler{
		Name: name,
		Game: game,
		YAML: yaml,
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
	_, err := db.Query(c, "INSERT INTO schedulers (name, game, yaml) VALUES (?name, ?game, ?yaml) RETURNING id", c)
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
