// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package http

import (
	"github.com/topfreegames/maestro/reporters/constants"
)

var handlers = map[string]interface{}{
	constants.EventSchedulerCreate: AnyHandler,
	constants.EventSchedulerDelete: AnyHandler,
	constants.EventSchedulerUpdate: AnyHandler,
}

// Find looks for a matching handler to a given event
func Find(event string) (interface{}, bool) {
	handlerI, prs := handlers[event]
	return handlerI, prs
}

// AnyHandler sends the respective event to an HTTP endpoint
func AnyHandler(client Client, opts map[string]interface{}) error {
	return client.Send(opts)
}
