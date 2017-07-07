package extensions

import (
	"fmt"
	"plugin"

	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/eventforwarder"
)

// LoadPlugin loads a plugin
func LoadPlugin(pluginName string, path string) (*plugin.Plugin, error) {
	return plugin.Open(fmt.Sprintf("%s/%s.so", path, pluginName))
}

// LoadForwarder loads a forwarder from a plugin
func LoadForwarder(p *plugin.Plugin, config *viper.Viper) (eventforwarder.EventForwarder, error) {
	f, err := p.Lookup("NewForwarder")
	if err != nil {
		return nil, err
	}
	ff, ok := f.(func(*viper.Viper) (eventforwarder.EventForwarder, error))
	if !ok {
		return nil, err
	}
	return ff(config)
}
