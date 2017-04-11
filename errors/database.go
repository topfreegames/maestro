// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package errors

import "encoding/json"

//DatabaseError happens when an error occur when running a command in the database
type DatabaseError struct {
	SourceError error
}

//NewDatabaseError ctor
func NewDatabaseError(err error) *DatabaseError {
	return &DatabaseError{
		SourceError: err,
	}
}

func (e *DatabaseError) Error() string {
	return e.SourceError.Error()
}

//Serialize returns the error serialized
func (e *DatabaseError) Serialize() []byte {
	g, _ := json.Marshal(map[string]interface{}{
		"code":        "MAE-001",
		"error":       "database error",
		"description": e.SourceError.Error(),
	})

	return g
}
