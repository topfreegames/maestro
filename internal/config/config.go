package config

import "time"

// Config is used to fetch configurations using paths. The interface provides a
// a way to fetch configurations in specific types. Paths are set in strings and
// can have separated "scopes" using ".". An example of path would be:
// "api.metrics.enabled".
type Config interface {
	// GetString returns the configuration path as a string. Default: ""
	GetString(string) string
	// GetInt returns the configuration path as an int. Default: 0
	GetInt(string) int
	// GetFloat64 returns the configuration path as a float64. Default: 0.0
	GetFloat64(string) float64
	// GetBool returns the configuration path as a boolean. Default: false
	GetBool(string) bool
	// GetDuration returns a time.Duration of the config. Deafult: 0
	GetDuration(string) time.Duration
}
