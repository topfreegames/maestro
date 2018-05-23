// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package eventforwarder

import (
	"fmt"
	"plugin"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	ex "github.com/topfreegames/maestro/extensions"
)

// Info is the struct that defines the information of a event forwarder
type Info struct {
	Plugin    string
	Name      string
	Forwarder EventForwarder
}

// EventForwarder interface
type EventForwarder interface {
	Forward(event string, infos map[string]interface{}, fwdMetadata map[string]interface{}) (int32, string, error)
}

// LoadEventForwardersFromConfig returns a slice of configured eventforwarders
func LoadEventForwardersFromConfig(config *viper.Viper, logger logrus.FieldLogger) []*Info {
	forwarders := []*Info{}
	if runtime.GOOS == "linux" {
		forwardersConfig := config.GetStringMap("forwarders")
		if len(forwardersConfig) > 0 {
			for plugin, v := range forwardersConfig {
				logger.Infof("loading plugin: %s", plugin)
				p, err := ex.LoadPlugin(plugin, "./bin")
				if err != nil {
					logger.Errorf("error loading plugin %s: %s", plugin, err.Error())
					continue
				}
				forwarderConfigMap, ok := v.(map[string]interface{})
				if ok {
					for name := range forwarderConfigMap {
						logger.Infof("loading forwarder %s.%s", plugin, name)
						cfg := config.Sub(fmt.Sprintf("forwarders.%s.%s", plugin, name))
						forwarder, err := LoadForwarder(p, cfg, logger)
						if err != nil {
							logger.Error(err)
							continue
						}
						info := &Info{
							Plugin:    plugin,
							Name:      name,
							Forwarder: forwarder,
						}
						forwarders = append(forwarders, info)
					}
				}
			}
		}
	} else {
		logger.Warn("not loading any forwarder plugin because not running on linux")
	}
	return forwarders
}

// LoadForwarder loads a forwarder from a plugin
func LoadForwarder(p *plugin.Plugin, config *viper.Viper, logger logrus.FieldLogger) (EventForwarder, error) {
	f, err := p.Lookup("NewForwarder")
	if err != nil {
		return nil, err
	}
	ff, ok := f.(func(*viper.Viper, logrus.FieldLogger) (EventForwarder, error))
	if !ok {
		return nil, err
	}
	return ff(config, logger)
}
