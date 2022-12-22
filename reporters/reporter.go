// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"strings"
	"sync"

	"github.com/getlantern/deepcopy"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Reporter implementations are responsible for reporting
// events to any sink that wants to consume them
type Reporter interface {
	Report(event string, opts map[string]interface{}) error
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

func copyOpts(src map[string]interface{}) map[string]interface{} {
	var dst map[string]interface{}
	deepcopy.Copy(&dst, src)
	return dst
}

// Report is Reporters' implementation of the Reporter interface
func (r *Reporters) Report(event string, opts map[string]interface{}) error {
	for _, reporter := range r.reporters {
		// We ignore the reporter errors explicitly here for the following reason:
		// if we return these errors, it could bring issues in the ping mechanism,
		// and we would not be able to find any room.
		_ = reporter.Report(event, copyOpts(opts))
	}
	return nil
}

// HasReporters checks the length of Reporters' map and returns true if it's > 0
func HasReporters() bool {
	return len(GetInstance().reporters) > 0
}

// Report calls Report() in Reporters' singleton
func Report(event string, opts map[string]interface{}) error {
	return GetInstance().Report(event, opts)
}

// MakeReporters creates Reporters' singleton from config/{}.yaml
func MakeReporters(config *viper.Viper, logger logrus.FieldLogger) {
	if config.IsSet("reporters.dogstatsd") {
		MakeDogStatsD(config, logger, GetInstance())
	}
	if config.IsSet("reporters.http") {
		MakeHTTP(config, logger, GetInstance())
	}
	correctlySet := []string{}
	for k := range GetInstance().reporters {
		correctlySet = append(correctlySet, k)
	}
	logger.Infof("Active reporters: %s", strings.Join(correctlySet, ", "))
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
