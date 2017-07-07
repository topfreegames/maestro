// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package eventforwarder

// EventForwarder interface
type EventForwarder interface {
	Forward(event string, infos map[string]interface{}) (int32, error)
}
