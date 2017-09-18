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

type Reporters struct {
	reporters map[string]Reporter
}

func (r *Reporters) SetReporter(key string, value Reporter) {
	r.reporters[key] = value
}

func (r *Reporters) UnsetReporter(key string) {
	delete(r.reporters, key)
}

func (r *Reporters) GetReporter(key string) (Reporter, bool) {
	v, p := r.reporters[key]
	return v, p
}

func (r *Reporters) Report(event string, opts map[string]string) error {
	for _, reporter := range r.reporters {
		reporter.Report(event, opts)
	}
	return nil
}

func MakeReporters(config *viper.Viper, logger *logrus.Logger) {
	if config.IsSet("reporters.dogstatsd") {
		MakeDogStatsD(config, logger)
	}
}

var instance *Reporters
var once sync.Once

func GetInstance() *Reporters {
	once.Do(func() {
		instance = &Reporters{
			reporters: make(map[string]Reporter),
		}
	})
	return instance
}
