package reporters

import (
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/statsd"
)

type StatsD struct {
	client *statsd.StatsD
}

func (d *StatsD) Report(str string) error {
	println(str)
	d.client.Increment(str)
	d.client.Flush()
	return nil
}

func NewStatsD(config *viper.Viper, logger *logrus.Logger) (*StatsD, error) {
	client, err := statsd.NewStatsD(config, logger)
	if err != nil {
		return nil, err
	}
	statsdR := &StatsD{client: client}
	return statsdR, nil
}
