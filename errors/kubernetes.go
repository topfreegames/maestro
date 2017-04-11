// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package errors

import "encoding/json"

//KubernetesError happens when an error occur when running a command in the database
type KubernetesError struct {
	sourceError error
	message     string
}

//NewKubernetesError ctor
func NewKubernetesError(message string, err error) *KubernetesError {
	if yamlError, ok := err.(*KubernetesError); ok {
		return yamlError
	}

	return &KubernetesError{
		sourceError: err,
		message:     message,
	}
}

func (e *KubernetesError) Error() string {
	return e.sourceError.Error()
}

//Serialize returns the error serialized
func (e *KubernetesError) Serialize() []byte {
	g, _ := json.Marshal(map[string]interface{}{
		"code":        "MAE-003",
		"error":       e.message,
		"description": e.sourceError.Error(),
	})

	return g
}
