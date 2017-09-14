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

type Reporter interface {
	Report(str string) error
}

type Reporters struct {
	reporters map[string]Reporter
}

func (r *Reporters) SetReporter(key string, value Reporter) {
	r.reporters[key] = value
}

func (r *Reporters) GetReporter(key string) (Reporter, bool) {
	v, p := r.reporters[key]
	return v, p
}

func (r *Reporters) Report(str string) error {
	for _, reporter := range r.reporters {
		reporter.Report(str)
	}
	return nil
}

func MakeReporters(config *viper.Viper, logger *logrus.Logger) {
	if config.IsSet("reporters.statsd") {
		MakeStatsD(config, logger)
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
