// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/reporters/dogstatsd"

	godogstatsd "github.com/ooyala/go-dogstatsd"
)

var reportHandlers = map[string]interface{}{
	"gru.new":    dogstatsd.GruIncrementHandler,
	"gru.delete": dogstatsd.GruIncrementHandler,
}

type DogStatsD struct {
	client *godogstatsd.Client
}

func (d *DogStatsD) Report(event string, opts map[string]string) error {
	handlerI, prs := reportHandlers[event]

	if prs == false {
		return fmt.Errorf("reportHandler for %s doesn't exist", event)
	}
	handler := handlerI.(func(*godogstatsd.Client, string, map[string]string) error)
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
	c, err := godogstatsd.New(host)
	if err != nil {
		return nil, err
	}
	prefix := config.GetString("reporters.dogstatsd.prefix")
	c.Namespace = prefix
	dogstatsdR := &DogStatsD{client: c}
	return dogstatsdR, nil
}
