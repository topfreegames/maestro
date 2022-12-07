// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/dogstatsd"
	constants "github.com/topfreegames/maestro/reporters/constants"
	handlers "github.com/topfreegames/maestro/reporters/dogstatsd"
)

// DogStatsD reports metrics to a dogstatsd.Client
type DogStatsD struct {
	client         dogstatsd.Client
	logger         *logrus.Logger
	statsdClient   *statsd.Client
	region         string
	host           string
	prefix         string
}

func toMapStringString(o map[string]interface{}) map[string]string {
	n := map[string]string{}
	for k, v := range o {
		if str, ok := v.(string); ok {
			n[k] = str
		}
	}
	return n
}

// Report finds a matching handler to some 'event' metric and delegates
// further actions to it
func (d *DogStatsD) Report(event string, opts map[string]interface{}) error {
	handlerI, prs := handlers.Find(event)
	if prs == false {
		return fmt.Errorf("reportHandler for %s doesn't exist", event)
	}
	opts[constants.TagRegion] = d.region
	handler := handlerI.(func(dogstatsd.Client, string, map[string]string) error)
	err := handler(d.client, event, toMapStringString(opts))
	if err != nil {
		d.logger.Error(err)
	}
	return err
}

// MakeDogStatsD adds a DogStatsD struct to the Reporters' singleton
func MakeDogStatsD(config *viper.Viper, logger *logrus.Logger, r *Reporters) {
	dogstatsdR, err := NewDogStatsD(config, logger)

	if err == nil {
		r.SetReporter("dogstatsd", dogstatsdR)
	}
}

func loadDefaultDogStatsDConfigs(c *viper.Viper) {
	c.SetDefault("reporters.dogstatsd.host", "localhost:8125")
	c.SetDefault("reporters.dogstatsd.prefix", "test.")
	c.SetDefault("reporters.dogstatsd.region", "test")
	c.SetDefault("reporters.dogstatsd.restartTimeout", time.Duration(0))
}

// NewDogStatsD creates a DogStatsD struct using host and prefix from config
func NewDogStatsD(config *viper.Viper, logger *logrus.Logger) (*DogStatsD, error) {
	loadDefaultDogStatsDConfigs(config)
	host := config.GetString("reporters.dogstatsd.host")
	prefix := config.GetString("reporters.dogstatsd.prefix")
	c, err := dogstatsd.New(host, prefix)
	if err != nil {
		return nil, err
	}
	
	restartTimeout := config.GetDuration("reporters.dogstatsd.restartTimeout")
	r := config.GetString("reporters.dogstatsd.region")

	dogstatsdR := &DogStatsD{
		client:       c,
		statsdClient: c.Client.(*statsd.Client),
		logger:       logger,
		region:       r,
		host:         host,
		prefix:       prefix,
	}

	dogstatsdR.scheduleRestart(restartTimeout)
	return dogstatsdR, nil
}

// NewDogStatsDFromClient creates a DogStatsD struct with an already configured
// dogstatsd.Client -- or a mock client
func NewDogStatsDFromClient(c dogstatsd.Client, r string) *DogStatsD {
	return &DogStatsD{client: c, region: r}
}

func (d *DogStatsD) scheduleRestart(timeout time.Duration) {
	if timeout.Nanoseconds() > 0 {
		time.AfterFunc(timeout, func() {
			if d.client != nil {
				if err := d.statsdClient.Close(); err != nil {
					d.logger.Errorf("DogStatsD: failed to close statsd connection during restart: %s", err.Error())
				}
			}

			c, err := dogstatsd.New(d.host, d.prefix)
			if err == nil {
				d.statsdClient = c.Client.(*statsd.Client)
				d.client = c
			} else {
				d.logger.Errorf("DogStatsD: failed to recreate dogstatsd client during restart. %s", err.Error())
				d.client = nil
			}
			d.scheduleRestart(timeout)
		})
	}
}
