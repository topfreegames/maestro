// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package dogstatsd

import (
	"fmt"

	"github.com/topfreegames/extensions/dogstatsd"
)

func createTags(opts map[string]string) []string {
	var tags []string
	for key, value := range opts {
		tags = append(tags, fmt.Sprintf("%s:%s", key, value))
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
