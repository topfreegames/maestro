// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"github.com/Sirupsen/logrus"
	"github.com/ooyala/go-dogstatsd"
	"github.com/spf13/viper"
)

type DogStatsD struct {
	client *dogstatsd.Client
}

func appendTag(key string, tags []string, opts map[string]string) []string {
	tag, prs := opts[key]
	if prs {
		return append(tags, tag)
	}
	return tags
}

func (d *DogStatsD) Report(event string, opts map[string]string) error {
	var tags []string
	tags = appendTag("game", tags, opts)
	tags = appendTag("scheduler", tags, opts)
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
	c, err := dogstatsd.New(host)
	if err != nil {
		return nil, err
	}
	prefix := config.GetString("reporters.dogstatsd.prefix")
	c.Namespace = prefix
	dogstatsdR := &DogStatsD{client: c}
	return dogstatsdR, nil
}
