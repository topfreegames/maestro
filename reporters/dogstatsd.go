// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"fmt"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/dogstatsd"
	handlers "github.com/topfreegames/maestro/reporters/dogstatsd"
)

var reportHandlers = map[string]interface{}{
	"gru.new":    handlers.GruIncrHandler,
	"gru.delete": handlers.GruIncrHandler,
}

type DogStatsD struct {
	client dogstatsd.Client
}

func (d *DogStatsD) Report(event string, opts map[string]string) error {
	handlerI, prs := reportHandlers[event]

	if prs == false {
		return fmt.Errorf("reportHandler for %s doesn't exist", event)
	}
	handler := handlerI.(func(dogstatsd.Client, string, map[string]string) error)
	return handler(d.client, event, opts)
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

func NewDogStatsDFromClient(client dogstatsd.Client) *DogStatsD {
	return &DogStatsD{client: client}
}
