// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package errors

import "encoding/json"

//YamlError happens when an error occur when running a command in the database
type YamlError struct {
	sourceError error
	message     string
}

//NewYamlError ctor
func NewYamlError(message string, err error) *YamlError {
	if yamlError, ok := err.(*YamlError); ok {
		return yamlError
	}

	return &YamlError{
		sourceError: err,
		message:     message,
	}
}

func (e *YamlError) Error() string {
	return e.sourceError.Error()
}

//Serialize returns the error serialized
func (e *YamlError) Serialize() []byte {
	g, _ := json.Marshal(map[string]interface{}{
		"code":        "MAE-002",
		"error":       e.message,
		"description": e.sourceError.Error(),
	})

	return g
}
