// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package dogstatsd

import (
	"github.com/DataDog/datadog-go/statsd"
)

func createTags(opts map[string]string) []string {
	var tags []string
	for _, value := range opts {
		tags = append(tags, value)
	}
	return tags
}

func GruIncrementHandler(c *statsd.Client, event string,
	opts map[string]string) error {
	tags := createTags(opts)
	c.Count(event, 1, tags, 1)
	return nil
}
