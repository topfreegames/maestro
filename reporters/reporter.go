// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

// Reporter implementations are responsible for reporting
// events to any sink that wants to consume them
type Reporter interface {
	Report(event string, opts map[string]string) error
}

// Reporters hold a map of structs that implement the Reporter interface
type Reporters struct {
	reporters map[string]Reporter
}

// SetReporter sets a Reporter in Reporters' map
func (r *Reporters) SetReporter(key string, value Reporter) {
	r.reporters[key] = value
}

// UnsetReporter deletes a Reporter from Reporters' map
func (r *Reporters) UnsetReporter(key string) {
	delete(r.reporters, key)
}

// GetReporter returns a reporter from Reporters' map
func (r *Reporters) GetReporter(key string) (Reporter, bool) {
	v, p := r.reporters[key]
	return v, p
}

// Report is Reporters' implementation of the Reporter interface
func (r *Reporters) Report(event string, opts map[string]string) error {
	for _, reporter := range r.reporters {
		reporter.Report(event, opts)
	}
	return nil
}

// HasReporters checks the length of Reporters' map and returns true if it's > 0
func HasReporters() bool {
	return len(GetInstance().reporters) > 0
}

// Report calls Report() in Reporters' singleton
func Report(event string, opts map[string]string) error {
	return GetInstance().Report(event, opts)
}

// MakeReporters creates Reporters' singleton from config/{}.yaml
func MakeReporters(config *viper.Viper, logger *logrus.Logger) {
	if config.IsSet("reporters.dogstatsd") {
		MakeDogStatsD(config, logger, GetInstance())
	}
}

// NewReporters ctor
func NewReporters() *Reporters {
	return &Reporters{
		reporters: make(map[string]Reporter),
	}
}

var instance *Reporters
var once sync.Once

// GetInstance returns Reporters' singleton
func GetInstance() *Reporters {
	once.Do(func() {
		instance = NewReporters()
	})
	return instance
}
