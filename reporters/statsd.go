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
	d.client.Increment(str)
	d.client.Flush()
	return nil
}

func MakeStatsD(config *viper.Viper, logger *logrus.Logger) {
	r := GetInstance()
	statsdR, err := NewStatsD(config, logger)
	name := config.GetString("reporters.statsd.name")

	if err == nil {
		r.SetReporter(name, statsdR)
	}
}

func NewStatsD(config *viper.Viper, logger *logrus.Logger) (*StatsD, error) {
	client, err := statsd.NewStatsD(config, logger)
	if err != nil {
		return nil, err
	}
	statsdR := &StatsD{client: client}
	return statsdR, nil
}
