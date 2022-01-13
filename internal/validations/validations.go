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

	"github.com/go-playground/validator/v10"
)

var (
	Validate *validator.Validate
)

func RegisterValidations() error {
	Validate = validator.New()

	err := Validate.RegisterValidation("semantic_version", SemanticValidate)
	if err != nil {
		return errors.New("could not register SemanticValidate")
	}

	err = Validate.RegisterValidation("image_pull_policy", ImagePullPolicyValidate)
	if err != nil {
		return errors.New("could not register ImagePullPolicyValidate")
	}

	err = Validate.RegisterValidation("ports_protocol", PortsProtocolValidate)
	if err != nil {
		return errors.New("could not register PortsProtocolValidate")
	}

	if Validate == nil {
		return errors.New("it was not possible to register validations")
	}
	return nil
}
