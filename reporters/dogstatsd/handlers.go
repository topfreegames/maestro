// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package dogstatsd

import (
	"fmt"
	"github.com/DataDog/datadog-go/v5/statsd"
	"strconv"
	"time"

	"github.com/topfreegames/maestro/reporters/constants"
)

var handlers = map[string]interface{}{
	constants.EventGruNew:           GruIncrHandler,
	constants.EventGruDelete:        GruIncrHandler,
	constants.EventGruPing:          GruIncrHandler,
	constants.EventGruStatus:        GruStatusHandler,
	constants.EventRPCStatus:        GruIncrHandler,
	constants.EventRPCDuration:      GruTimingHandler,
	constants.EventHTTPResponseTime: HTTPTimingHandler,
	constants.EventPodLastStatus:    GaugeHandler,
	constants.EventPodStatus:        GaugeHandler,
	constants.EventResponseTime:     TimingHandler,
	constants.EventGruMetricUsage:   GaugeHandler,
}

// Find looks for a matching handler to a given event
func Find(event string) (interface{}, bool) {
	handlerI, prs := handlers[event]
	return handlerI, prs
}

func createTags(opts map[string]string) []string {
	var tags []string
	for key, value := range opts {
		tags = append(tags, fmt.Sprintf("%s:%s", key, value))
	}
	return tags
}

func createAllowedTags(opts map[string]string, allowed []string) []string {
	var tags []string
	for _, tag := range allowed {
		val, prs := opts[tag]
		if prs {
			tags = append(tags, fmt.Sprintf("%s:%s", tag, val))
		}
	}
	return tags
}

// GruIncrHandler calls dogstatsd.Client.Incr with tags formatted as key:value
func GruIncrHandler(c statsd.ClientInterface, event string,
	opts map[string]string) error {
	tags := createTags(opts)
	c.Incr(event, tags, 1)
	return nil
}

// GruStatusHandler calls dogstatsd.Client.Incr with tags formatted as key:value
func GruStatusHandler(c statsd.ClientInterface, event string,
	opts map[string]string) error {
	tags := createAllowedTags(opts, []string{constants.TagGame, constants.TagScheduler, constants.TagRegion})
	gauge, err := strconv.ParseFloat(opts["gauge"], 64)
	if err != nil {
		return err
	}
	if err := c.Gauge(fmt.Sprintf("gru.%s", opts["status"]), gauge, tags, 1); err != nil {
		return fmt.Errorf("dogstatsd failed to GRU Status gauge %s: %w", opts["status"], err)
	}
	return nil
}

// GruTimingHandler calls dogstatsd.Client.Timing with tags formatted as key:value
func GruTimingHandler(c statsd.ClientInterface, event string,
	opts map[string]string) error {
	tags := createAllowedTags(opts, []string{
		constants.TagGame, constants.TagScheduler, constants.TagRegion,
		constants.TagHostname, constants.TagRoute, constants.TagStatus,
	})
	duration, _ := time.ParseDuration(opts[constants.TagResponseTime])
	c.Timing(constants.EventRPCDuration, duration, tags, 1)
	return nil
}

// HistogramHandler calls dogstatsd.Client.Histogram with tags formatted as key:value
func HistogramHandler(c statsd.ClientInterface, event string, opts map[string]string) error {
	tags := createAllowedTags(opts, []string{
		constants.TagGame, constants.TagScheduler, constants.TagRegion, constants.TagHTTPStatus})
	histogram, err := strconv.ParseFloat(opts["histogram"], 64)
	if err != nil {
		return err
	}
	c.Histogram(opts["name"], histogram, tags, 1)
	return nil
}

// HTTPTimingHandler calls dogstatsd.Client.Timing with tags formatted as key:value for http calls
func HTTPTimingHandler(c statsd.ClientInterface, event string,
	opts map[string]string) error {
	tags := createAllowedTags(opts, []string{
		constants.TagRoute, constants.TagScheduler, constants.TagRegion,
		constants.TagHTTPStatus, constants.TagHostname})
	duration, _ := time.ParseDuration(opts[constants.TagResponseTime])
	c.Timing(constants.EventHTTPResponseTime, duration, tags, 1)
	return nil
}

// TimingHandler calls dogstatsd.Client.Timing with tags formatted as key:value for internal calls
func TimingHandler(c statsd.ClientInterface, event string,
	opts map[string]string) error {
	tags := createAllowedTags(opts, []string{
		constants.TagScheduler, constants.TagRegion,
		constants.TagSegment, constants.TagRoute, constants.TagType, constants.TagTable, constants.TagError})
	duration, _ := time.ParseDuration(opts[constants.TagResponseTime])
	c.Timing(constants.EventResponseTime, duration, tags, 1)
	return nil
}

// GaugeHandler calls dogstatsd.Client.Gauge with tags formatted as key:value
func GaugeHandler(
	c statsd.ClientInterface,
	event string,
	opts map[string]string,
) error {
	tags := createAllowedTags(opts, []string{
		constants.TagGame, constants.TagScheduler, constants.TagRegion,
		constants.TagReason, constants.TagMetric, constants.TagPodStatus,
	})
	gauge, err := strconv.ParseFloat(opts["gauge"], 64)
	if err != nil {
		return err
	}
	if err := c.Gauge(event, gauge, tags, 1); err != nil {
		return fmt.Errorf("dogstatsd failed to gauge %s: %w", opts["status"], err)
	}
	return nil
}
