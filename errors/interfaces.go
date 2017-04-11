// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package errors

//SerializableError means that an error can be transformed to JSON
type SerializableError interface {
	Serialize() []byte
}
