// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"github.com/DataDog/datadog-go/statsd"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

type DogStatsD struct {
	client *statsd.Client
}

func createTags(opts map[string]string) []string {
	var tags []string
	for _, value := range opts {
		tags = append(tags, value)
	}
	return tags
}

func (d *DogStatsD) Report(event string, opts map[string]string) error {
	tags := createTags(opts)
	d.client.Count(event, 1, tags, 1)
	return nil
}

func MakeDogStatsD(config *viper.Viper, logger *logrus.Logger) {
	r := GetInstance()
	dogstatsdR, err := NewDogStatsD(config, logger)

	if err == nil {
		r.SetReporter("dogstatsd", dogstatsdR)
	}
}

func NewDogStatsD(config *viper.Viper, logger *logrus.Logger) (*DogStatsD, error) {
	// handle non-existent host
	host := config.GetString("reporters.dogstatsd.host")
	c, err := statsd.New(host)
	if err != nil {
		return nil, err
	}
	prefix := config.GetString("reporters.dogstatsd.prefix")
	c.Namespace = prefix
	dogstatsdR := &DogStatsD{client: c}
	return dogstatsdR, nil
}
