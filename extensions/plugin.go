package extensions

import (
	"fmt"
	"plugin"
)

// LoadPlugin loads a plugin
func LoadPlugin(pluginName string, path string) (*plugin.Plugin, error) {
	return plugin.Open(fmt.Sprintf("%s/%s.so", path, pluginName))
}
