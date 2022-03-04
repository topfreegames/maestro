// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package validations

import (
	"errors"

	"github.com/topfreegames/maestro/internal/core/validations"

	"github.com/go-playground/validator/v10"
)

var (
	Validate *validator.Validate
)

func RegisterValidations() error {
	Validate = validator.New()

	err := Validate.RegisterValidation("semantic_version", semanticValidate)
	if err != nil {
		return errors.New("could not register semanticValidate")
	}

	err = Validate.RegisterValidation("image_pull_policy", imagePullPolicyValidate)
	if err != nil {
		return errors.New("could not register imagePullPolicyValidate")
	}

	err = Validate.RegisterValidation("ports_protocol", portsProtocolValidate)
	if err != nil {
		return errors.New("could not register portsProtocolValidate")
	}

	err = Validate.RegisterValidation("max_surge", maxSurgeValidate)
	if err != nil {
		return errors.New("could not register maxSurgeValidate")
	}

	err = Validate.RegisterValidation("kube_resource_name", kubeResourceNameValidate)
	if err != nil {
		return errors.New("could not register kubeResourceNameValidate")
	}

	if Validate == nil {
		return errors.New("it was not possible to register validations")
	}
	return nil
}

func imagePullPolicyValidate(fl validator.FieldLevel) bool {
	return validations.IsImagePullPolicySupported(fl.Field().String())
}

func portsProtocolValidate(fl validator.FieldLevel) bool {
	return validations.IsProtocolSupported(fl.Field().String())
}

func maxSurgeValidate(fl validator.FieldLevel) bool {
	return validations.IsMaxSurgeValid(fl.Field().String())
}

func semanticValidate(fl validator.FieldLevel) bool {
	return validations.IsVersionValid(fl.Field().String())
}

func kubeResourceNameValidate(fl validator.FieldLevel) bool {
	return validations.IsKubeResourceNameValid(fl.Field().String())
}
