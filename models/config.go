// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"github.com/topfreegames/extensions/interfaces"
	"github.com/topfreegames/maestro/errors"
	yaml "gopkg.in/yaml.v2"
)

// Config is the struct that defines a config for running maestro
type Config struct {
	Name string `yaml:"name"`
	Game string `yaml:"game"`
	YAML string
}

// Port has the port container port and protocol
type Port struct {
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol"`
}

// Limits has the CPU and memory resources limits
type Limits struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

// EnvVar has name and value of an enviroment variable
type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// ScalingPolicyTrigger has the configuration for a scaling policy trigger
type ScalingPolicyTrigger struct {
	Time  int `yaml:"time"`
	Usage int `yaml:"usage"`
}

// ScalingPolicy has the configuration for a scaling policy
type ScalingPolicy struct {
	Cooldown int                   `yaml:"cooldown"`
	Delta    int                   `yaml:"delta"`
	Trigger  *ScalingPolicyTrigger `yaml:"trigger"`
}

// AutoScaling has the configuration for the GRU's auto scaling
type AutoScaling struct {
	Min  int            `yaml:"min"`
	Up   *ScalingPolicy `yaml:"up"`
	Down *ScalingPolicy `yaml:"down"`
}

// ConfigYAML is the struct for the config yaml
type ConfigYAML struct {
	Name            string       `yaml:"name"`
	Game            string       `yaml:"game"`
	Image           string       `yaml:"image"`
	Ports           []*Port      `yaml:"ports"`
	Limits          *Limits      `yaml:"limits"`
	ShutdownTimeout int          `yaml:"shutdownTimeout"`
	AutoScaling     *AutoScaling `yaml:"autoscaling"`
	Env             []*EnvVar    `yaml:"env"`
	Cmd             []string     `yaml:"cmd"`
}

// NewConfig is the config constructor
func NewConfig(name, game, yaml string) *Config {
	return &Config{
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

// Load loads a config from the database using the config name
func (c *Config) Load(db interfaces.DB) error {
	_, err := db.Query(c, `SELECT * FROM configs WHERE name = ?`, c.Name)
	return err
}

// Create creates a config in the database
func (c *Config) Create(db interfaces.DB) error {
	_, err := db.Query(c, `
		INSERT INTO configs (name, game, yaml) VALUES (?name, ?game, ?yaml)
		RETURNING id
	`, c)
	return err
}
