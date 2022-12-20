// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters

import (
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/reporters/constants"
	handlers "github.com/topfreegames/maestro/reporters/dogstatsd"
)

// DogStatsD reports metrics to a dogstatsd.Client
type DogStatsD struct {
	client       statsd.ClientInterface
	logger       logrus.FieldLogger
	mutex        sync.RWMutex
	ticker       *time.Ticker
	region       string
	host         string
	prefix       string
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
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	handlerI, prs := handlers.Find(event)
	if !prs {
		return fmt.Errorf("reportHandler for %s doesn't exist", event)
	}
	opts[constants.TagRegion] = d.region
	handler := handlerI.(func(statsd.ClientInterface, string, map[string]string) error)
	err := handler(d.client, event, toMapStringString(opts))
	if err != nil {
		return fmt.Errorf("failed to report event '%s': %w", event, err)
	}
	return nil
}

// MakeDogStatsD adds a DogStatsD struct to the Reporters' singleton
func MakeDogStatsD(config *viper.Viper, logger logrus.FieldLogger, r *Reporters) {
	dogstatsdR, err := NewDogStatsD(config, logger.WithField("reporter", "dogstatsd"))

	if err == nil {
		r.SetReporter("dogstatsd", dogstatsdR)
	}
}

func loadDefaultDogStatsDConfigs(c *viper.Viper) {
	c.SetDefault("reporters.dogstatsd.host", "localhost:8125")
	c.SetDefault("reporters.dogstatsd.prefix", "test.")
	c.SetDefault("reporters.dogstatsd.region", "test")
	c.SetDefault("reporters.dogstatsd.restartTimeout", "0s")
}

// NewDogStatsD creates a DogStatsD struct using host and prefix from config
func NewDogStatsD(config *viper.Viper, logger logrus.FieldLogger) (*DogStatsD, error) {
	loadDefaultDogStatsDConfigs(config)
	host := config.GetString("reporters.dogstatsd.host")
	prefix := config.GetString("reporters.dogstatsd.prefix")
	c, err := statsd.New(host, statsd.WithNamespace(prefix))
	if err != nil {
		return nil, err
	}

	restartTimeout := config.GetDuration("reporters.dogstatsd.restartTimeout")
	r := config.GetString("reporters.dogstatsd.region")

	dogstatsdR := &DogStatsD{
		client:       c,
		logger:       logger,
		region:       r,
		host:         host,
		prefix:       prefix,
	}

	if restartTimeout.Nanoseconds() > 0 {
		logger.Info("Starting DogStatsD restart ticker routine")
		dogstatsdR.ticker = time.NewTicker(restartTimeout)
		go dogstatsdR.restartTicker()
	}

	return dogstatsdR, nil
}

// NewDogStatsDFromClient creates a DogStatsD struct with an already configured
// dogstatsd.Client -- or a mock client
func NewDogStatsDFromClient(c statsd.ClientInterface, r string) *DogStatsD {
	return &DogStatsD{client: c, region: r}
}

func (d *DogStatsD) restartTicker() {
	for range d.ticker.C {
		d.logger.Info("Trying to restart the dogstatsd client")
		d.mutex.Lock()
		if err := d.restartDogStatsdClient(); err != nil {
			d.logger.WithError(err).Errorf("failed to close statsd connection during restart")
		}
		d.mutex.Unlock()
	}
}

func (d *DogStatsD) restartDogStatsdClient() error {
	err := d.client.Close()
	if err != nil {
		return fmt.Errorf("failed to close statsd connection: %w", err)
	}

	c, err := statsd.New(d.host, statsd.WithNamespace(d.prefix))
	if err != nil {
		return fmt.Errorf("failed to recreate dogstatsd client: %w", err)
	}

	d.logger.Info("DogStatsD was restarted successfully")
	d.client = c
	return nil
}
