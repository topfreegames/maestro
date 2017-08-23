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

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	ex "github.com/topfreegames/maestro/extensions"
)

// EventForwarder interface
type EventForwarder interface {
	Forward(event string, infos map[string]interface{}) (int32, error)
}

// LoadEventForwardersFromConfig returns a slice of configured eventforwarders
func LoadEventForwardersFromConfig(config *viper.Viper, logger logrus.FieldLogger) []EventForwarder {
	forwarders := []EventForwarder{}
	if runtime.GOOS == "linux" {
		forwardersConfig := config.GetStringMap("forwarders")
		if len(forwardersConfig) > 0 {
			for k, v := range forwardersConfig {
				logger.Infof("loading plugin: %s", k)
				p, err := ex.LoadPlugin(k, "./bin")
				if err != nil {
					logger.Errorf("error loading plugin %s: %s", k, err.Error())
					continue
				}
				forwarderConfigMap, ok := v.(map[string]interface{})
				if ok {
					for kk := range forwarderConfigMap {
						logger.Infof("loading forwarder %s.%s", k, kk)
						cfg := config.Sub(fmt.Sprintf("forwarders.%s.%s", k, kk))
						forwarder, err := LoadForwarder(p, cfg, logger)
						if err != nil {
							logger.Error(err)
							continue
						}
						forwarders = append(forwarders, forwarder)
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
