package models

import (
	"encoding/json"
	"fmt"

	"github.com/topfreegames/maestro/errors"
	yaml "gopkg.in/yaml.v2"
)

// ConfigYAMLv1 is the ConfigYAML before refactor to n containers per pod
type ConfigYAMLv1 struct {
	Name            string                           `yaml:"name"`
	Game            string                           `yaml:"game"`
	ShutdownTimeout int                              `yaml:"shutdownTimeout"`
	AutoScaling     *AutoScaling                     `yaml:"autoscaling"`
	NodeAffinity    string                           `yaml:"affinity"`
	NodeToleration  string                           `yaml:"toleration"`
	OccupiedTimeout int64                            `yaml:"occupiedTimeout"`
	Forwarders      map[string]map[string]*Forwarder `yaml:"forwarders"`

	// Container level, to keep compatibility
	Image    string     `yaml:"image" json:"image"`
	Ports    []*Port    `yaml:"ports" json:"ports"`
	Limits   *Resources `yaml:"limits" json:"limits"`
	Requests *Resources `yaml:"requests" json:"requests"`
	Env      []*EnvVar  `yaml:"env" json:"env"`
	Cmd      []string   `yaml:"cmd" json:"cmd"`
}

// ConfigYAMLv2 is the ConfigYAML after refactor to n containers per pod
type ConfigYAMLv2 struct {
	Name            string                           `yaml:"name"`
	Game            string                           `yaml:"game"`
	ShutdownTimeout int                              `yaml:"shutdownTimeout"`
	AutoScaling     *AutoScaling                     `yaml:"autoscaling"`
	NodeAffinity    string                           `yaml:"affinity"`
	NodeToleration  string                           `yaml:"toleration"`
	OccupiedTimeout int64                            `yaml:"occupiedTimeout"`
	Forwarders      map[string]map[string]*Forwarder `yaml:"forwarders"`
	Containers      []*Container                     `yaml:"containers"`
}

// ConfigYAML is the struct for the config yaml
type ConfigYAML struct {
	// Scheduler level
	Name            string                           `yaml:"name" json:"name" valid:"required"`
	Game            string                           `yaml:"game" json:"game" valid:"required"`
	ShutdownTimeout int                              `yaml:"shutdownTimeout" json:"shutdownTimeout" valid:"int64"`
	AutoScaling     *AutoScaling                     `yaml:"autoscaling" json:"autoscaling" valid:"required"`
	NodeAffinity    string                           `yaml:"affinity" json:"affinity"`
	NodeToleration  string                           `yaml:"toleration" json:"toleration"`
	OccupiedTimeout int64                            `yaml:"occupiedTimeout" json:"occupiedTimeout"`
	Forwarders      map[string]map[string]*Forwarder `yaml:"forwarders" json:"forwarders"`

	// Container level, to keep compatibility
	Image    string     `yaml:"image" json:"image"`
	Ports    []*Port    `yaml:"ports" json:"ports"`
	Limits   *Resources `yaml:"limits" json:"limits"`
	Requests *Resources `yaml:"requests" json:"requests"`
	Env      []*EnvVar  `yaml:"env" json:"env"`
	Cmd      []string   `yaml:"cmd" json:"cmd"`

	// Container level, for schedulers with more than one container per pod
	Containers []*Container `yaml:"containers" json:"containers"`
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

// ToYAML returns configYAML marshaled to yaml
func (c *ConfigYAML) ToYAML() []byte {
	var config interface{}

	if c.Version() == "v1" {
		config = &ConfigYAMLv1{
			Name:            c.Name,
			Game:            c.Game,
			ShutdownTimeout: c.ShutdownTimeout,
			AutoScaling:     c.AutoScaling,
			NodeAffinity:    c.NodeAffinity,
			NodeToleration:  c.NodeToleration,
			OccupiedTimeout: c.OccupiedTimeout,
			Forwarders:      c.Forwarders,
			Image:           c.Image,
			Ports:           c.Ports,
			Limits:          c.Limits,
			Requests:        c.Requests,
			Env:             c.Env,
			Cmd:             c.Cmd,
		}
	} else if c.Version() == "v2" {
		config = &ConfigYAMLv2{
			Name:            c.Name,
			Game:            c.Game,
			ShutdownTimeout: c.ShutdownTimeout,
			AutoScaling:     c.AutoScaling,
			NodeAffinity:    c.NodeAffinity,
			NodeToleration:  c.NodeToleration,
			OccupiedTimeout: c.OccupiedTimeout,
			Forwarders:      c.Forwarders,
			Containers:      c.Containers,
		}
	}

	configBytes, _ := yaml.Marshal(config)
	bytes, _ := json.Marshal(map[string]interface{}{
		"yaml": string(configBytes),
	})

	return bytes
}

// EnsureDefaultValues check if specific fields are empty and
// fill them with default values
func (c *ConfigYAML) EnsureDefaultValues() {
	if c == nil {
		return
	}

	defaultPolicyTrigger := &ScalingPolicyTrigger{
		Time:      600,
		Usage:     80,
		Threshold: 80,
		Limit:     90,
	}

	defaultScalingPolicy := &ScalingPolicy{
		Cooldown: 600,
		Delta:    1,
		Trigger:  defaultPolicyTrigger,
	}

	if c.AutoScaling == nil {
		c.AutoScaling = &AutoScaling{
			Up:   defaultScalingPolicy,
			Down: defaultScalingPolicy,
		}
	}

	if c.AutoScaling.Up == nil {
		c.AutoScaling.Up = defaultScalingPolicy
	}

	if c.AutoScaling.Down == nil {
		c.AutoScaling.Down = defaultScalingPolicy
	}

	if c.AutoScaling.Up.Trigger == nil {
		c.AutoScaling.Up.Trigger = defaultPolicyTrigger
	}

	if c.AutoScaling.Down.Trigger == nil {
		c.AutoScaling.Down.Trigger = defaultPolicyTrigger
	}

	if c.AutoScaling.Up.Trigger.Limit == 0 {
		c.AutoScaling.Up.Trigger.Limit = 90
	}
}

// Version returns the config version
func (c *ConfigYAML) Version() string {
	if c.Containers != nil && len(c.Containers) > 0 {
		return "v2"
	}

	return "v1"
}

// UpdateImage updates the image of the configYaml
// Returns true if the image was updated
// Returns false if the the image was the same
// Returns error if version v2 and there is no container with that name
func (c *ConfigYAML) UpdateImage(imageParams *SchedulerImageParams) (bool, error) {
	if c.Version() == "v1" {
		if c.Image == imageParams.Image {
			return false, nil
		}

		c.Image = imageParams.Image
		return true, nil
	} else if c.Version() == "v2" {
		if imageParams.Container == "" {
			return false, errors.NewValidationFailedError(
				fmt.Errorf("need to specify container name"))
		}

		for _, container := range c.Containers {
			if container.Name == imageParams.Container {
				if container.Image == imageParams.Image {
					return false, nil
				}

				container.Image = imageParams.Image
				return true, nil
			}
		}

		return false, errors.NewValidationFailedError(
			fmt.Errorf("no container with name %s", imageParams.Container))
	}

	return false, errors.NewValidationFailedError(
		fmt.Errorf("no update function for version %s", c.Version()))
}

//GetImage returns the container Image
func (c *ConfigYAML) GetImage() string {
	return c.Image
}

//GetName returns the container Image
func (c *ConfigYAML) GetName() string {
	return c.Name
}

//GetPorts returns the container Ports
func (c *ConfigYAML) GetPorts() []*Port {
	return c.Ports
}

//GetLimits returns the container Limits
func (c *ConfigYAML) GetLimits() *Resources {
	return c.Limits
}

//GetRequests returns the container Requests
func (c *ConfigYAML) GetRequests() *Resources {
	return c.Requests
}

//GetCmd returns the container Cmd
func (c *ConfigYAML) GetCmd() []string {
	return c.Cmd
}

//GetEnv returns the container Env
func (c *ConfigYAML) GetEnv() []*EnvVar {
	return c.Env
}
