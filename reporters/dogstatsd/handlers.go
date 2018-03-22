// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package dogstatsd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/topfreegames/extensions/dogstatsd"
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
func GruIncrHandler(c dogstatsd.Client, event string,
	opts map[string]string) error {
	tags := createTags(opts)
	c.Incr(event, tags, 1)
	return nil
}

// GruStatusHandler calls dogstatsd.Client.Incr with tags formatted as key:value
func GruStatusHandler(c dogstatsd.Client, event string,
	opts map[string]string) error {
	tags := createAllowedTags(opts, []string{constants.TagGame, constants.TagScheduler, constants.TagRegion})
	gauge, err := strconv.ParseFloat(opts["gauge"], 64)
	if err != nil {
		return err
	}
	c.Gauge(fmt.Sprintf("gru.%s", opts["status"]), gauge, tags, 1)
	return nil
}

// GruTimingHandler calls dogstatsd.Client.Timing with tags formatted as key:value
func GruTimingHandler(c dogstatsd.Client, event string,
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
func HistogramHandler(c dogstatsd.Client, event string, opts map[string]string) error {
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
func HTTPTimingHandler(c dogstatsd.Client, event string,
	opts map[string]string) error {
	tags := createAllowedTags(opts, []string{
		constants.TagRoute, constants.TagScheduler, constants.TagRegion,
		constants.TagHTTPStatus, constants.TagHostname})
	duration, _ := time.ParseDuration(opts[constants.TagResponseTime])
	c.Timing(constants.EventHTTPResponseTime, duration, tags, 1)
	return nil
}
