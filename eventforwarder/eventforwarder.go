// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package eventforwarder

import (
	pb "github.com/topfreegames/maestro/eventforwarder/generated"
)

type EventForwarder interface {
	Forward() (*pb.Response, error)
}
