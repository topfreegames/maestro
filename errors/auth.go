// maestro api
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package errors

import "encoding/json"

//AuthError happens when an user is not forbidden to execute an specfic action
type AuthError struct {
	Message     string
	SourceError error
}

//NewAuthError ctor
func NewAuthError(message string, err error) *AuthError {
	return &AuthError{
		Message:     message,
		SourceError: err,
	}
}

func (e *AuthError) Error() string {
	return e.SourceError.Error()
}

//Serialize returns the error serialized
func (e *AuthError) Serialize() []byte {
	g, _ := json.Marshal(map[string]interface{}{
		"code":        "MAE-006",
		"error":       e.Message,
		"description": e.SourceError.Error(),
	})

	return g
}
