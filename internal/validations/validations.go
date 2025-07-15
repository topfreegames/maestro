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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/go-playground/validator/v10"
)

var (
	Validate *validator.Validate
	uni      *ut.UniversalTranslator
)

func RegisterValidations() error {
	Validate = validator.New()
	english := en.New()
	uni = ut.New(english, english)
	translator := GetDefaultTranslator()
	_ = enTranslations.RegisterDefaultTranslations(Validate, translator)

	// Register autoscaling validation translations
	addTranslation(Validate, "required_for_room_occupancy", "RoomOccupancy must not be nil for RoomOccupancy policy type")
	addTranslation(Validate, "required_for_fixed_buffer_amount", "FixedBufferAmount must not be nil for FixedBuffer policy type")

	err := Validate.RegisterValidation("semantic_version", semanticValidate)
	if err != nil {
		return errors.New("could not register semanticValidate")
	}
	addTranslation(Validate, "semantic_version", "{0} must follow the semantic version pattern")

	err = Validate.RegisterValidation("image_pull_policy", imagePullPolicyValidate)
	if err != nil {
		return errors.New("could not register imagePullPolicyValidate")
	}
	addTranslation(Validate, "image_pull_policy", "{0} must be one of the following options: Always, Never, IfNotPresent")

	err = Validate.RegisterValidation("ports_protocol", portsProtocolValidate)
	if err != nil {
		return errors.New("could not register portsProtocolValidate")
	}
	addTranslation(Validate, "ports_protocol", "{0} must be one of the following options: tcp, udp, sctp")

	err = Validate.RegisterValidation("custom_lower_than", autoscalingMinMaxValidate)
	if err != nil {
		return errors.New("could not register autoscalingMinMaxValidate")
	}
	addTranslation(Validate, "custom_lower_than", "{0} must be a number lower than {1}")

	err = Validate.RegisterValidation("max_surge", maxSurgeValidate)
	if err != nil {
		return errors.New("could not register maxSurgeValidate")
	}
	addTranslation(Validate, "max_surge", "{0} must be a number greater than zero or a number greater than zero with suffix '%'")

	err = Validate.RegisterValidation("kube_resource_name", kubeResourceNameValidate)
	if err != nil {
		return errors.New("could not register kubeResourceNameValidate")
	}
	addTranslation(Validate, "kube_resource_name", "{0} must follow the public RFC 1123 about naming conventions")

	err = Validate.RegisterValidation("forwarder_type", forwarderTypeValidate)
	if err != nil {
		return errors.New("could not register forwarderTypeValidate")
	}
	addTranslation(Validate, "forwarder_type", "{0} must be one of the following options: gRPC")

	if Validate == nil {
		return errors.New("it was not possible to register validations")
	}
	return nil
}

// RegisterAutoscalingValidations registers autoscaling-specific validations
func RegisterAutoscalingValidations() error {
	// Register struct-level validation for Policy using a generic approach
	// This will be called by the autoscaling package
	Validate.RegisterStructValidation(policyStructLevelValidation, struct {
		Type       string
		Parameters struct {
			RoomOccupancy     interface{}
			FixedBufferAmount *int
		}
	}{})
	return nil
}

// policyStructLevelValidation performs struct-level validation for Policy
func policyStructLevelValidation(sl validator.StructLevel) {
	policy := sl.Current().Interface()
	policyValue := reflect.ValueOf(policy)
	if policyValue.Kind() == reflect.Ptr {
		policyValue = policyValue.Elem()
	}

	typeField := policyValue.FieldByName("Type")
	if !typeField.IsValid() {
		sl.ReportError(nil, "Type", "Type", "type_required", "")
		return
	}

	policyType := typeField.String()
	if policyType != "roomOccupancy" && policyType != "fixedBuffer" {
		sl.ReportError(typeField.Interface(), "Type", "Type", "oneof", "roomOccupancy fixedBuffer")
		return
	}

	parametersField := policyValue.FieldByName("Parameters")
	if !parametersField.IsValid() {
		sl.ReportError(nil, "Parameters", "Parameters", "invalid_parameters", "")
		return
	}

	switch policyType {
	case "roomOccupancy":
		roomOccupancyField := parametersField.FieldByName("RoomOccupancy")
		if roomOccupancyField.IsNil() {
			sl.ReportError(roomOccupancyField.Interface(), "RoomOccupancy", "RoomOccupancy", "required_for_room_occupancy", "")
		}
	case "fixedBuffer":
		fixedBufferAmountField := parametersField.FieldByName("FixedBufferAmount")
		if fixedBufferAmountField.IsNil() {
			sl.ReportError(fixedBufferAmountField.Interface(), "FixedBufferAmount", "FixedBufferAmount", "required_for_fixed_buffer_amount", "")
		} else {
			value := fixedBufferAmountField.Elem().Int()
			if value <= 0 {
				sl.ReportError(fixedBufferAmountField.Interface(), "FixedBufferAmount", "FixedBufferAmount", "gt", "0")
			}
		}
	}
}

// ValidateAutoscalingPolicy is a standalone function that can be used to validate autoscaling policy
// without creating import cycles
func ValidateAutoscalingPolicy(policyType string, roomOccupancy interface{}, fixedBufferAmount *int) error {
	switch policyType {
	case "roomOccupancy":
		if roomOccupancy == nil {
			return errors.New("RoomOccupancy is required when policy type is roomOccupancy")
		}
	case "fixedBuffer":
		if fixedBufferAmount == nil {
			return errors.New("FixedBufferAmount is required when policy type is fixedBuffer")
		}
		if *fixedBufferAmount <= 0 {
			return errors.New("FixedBufferAmount must be greater than 0")
		}
	default:
		return errors.New("invalid policy type")
	}
	return nil
}

func GetDefaultTranslator() ut.Translator {
	translator, _ := uni.GetTranslator("en")
	return translator
}

func imagePullPolicyValidate(fl validator.FieldLevel) bool {
	return IsImagePullPolicySupported(fl.Field().String())
}

func portsProtocolValidate(fl validator.FieldLevel) bool {
	return IsProtocolSupported(fl.Field().String())
}

func autoscalingMinMaxValidate(fl validator.FieldLevel) bool {
	field := fl.Field()
	kind := field.Kind()

	topField, topKind, _, ok := fl.GetStructFieldOK2()
	if !ok || topKind != kind {
		return false
	}

	return IsAutoscalingMinMaxValid(int(field.Int()), int(topField.Int()))
}

func maxSurgeValidate(fl validator.FieldLevel) bool {
	return IsMaxSurgeValid(fl.Field().String())
}

func semanticValidate(fl validator.FieldLevel) bool {
	return IsVersionValid(fl.Field().String())
}

func kubeResourceNameValidate(fl validator.FieldLevel) bool {
	return IsKubeResourceNameValid(fl.Field().String())
}

func forwarderTypeValidate(fl validator.FieldLevel) bool {
	return IsForwarderTypeSupported(fl.Field().String())
}

func addTranslation(validate *validator.Validate, tag string, errMessage string) {
	registerFn := func(ut ut.Translator) error {
		return ut.Add(tag, errMessage, false)
	}

	transFn := func(ut ut.Translator, fieldError validator.FieldError) string {
		param := fieldError.Param()
		tag := fieldError.Tag()

		t, err := ut.T(tag, fieldError.Field(), param)
		if err != nil {
			return fieldError.(error).Error()
		}
		return t
	}

	_ = validate.RegisterTranslation(tag, GetDefaultTranslator(), registerFn, transFn)
}

// Validation helper functions (moved from core/validations to avoid import cycles)
func IsAutoscalingMinMaxValid(min int, max int) bool {
	if max >= 0 && min > max {
		return false
	}
	if min <= 0 {
		return false
	}
	if max < -1 {
		return false
	}
	return true
}

// IsMaxSurgeValid check if MaxSurge is valid. A MaxSurge valid is a number greater than zero or a number greater than zero with suffix '%'
func IsMaxSurgeValid(maxSurge string) bool {
	if maxSurge == "" {
		return false
	}
	minMaxSurge := 1
	relativeSymbol := "%"

	isRelative := strings.HasSuffix(maxSurge, relativeSymbol)
	maxSurgeInt, err := strconv.Atoi(strings.TrimSuffix(maxSurge, relativeSymbol))
	if err != nil {
		return false
	}

	if !isRelative {
		if minMaxSurge > maxSurgeInt {
			return false
		}
	}
	return true
}

// IsImagePullPolicySupported check if received policy is supported by maestro
func IsImagePullPolicySupported(policy string) bool {
	policies := []string{"Always", "Never", "IfNotPresent"}
	for _, item := range policies {
		if item == policy {
			return true
		}
	}
	return false
}

// IsProtocolSupported check if received protocol is supported by kubernetes
func IsProtocolSupported(protocol string) bool {
	protocols := []string{"tcp", "udp", "sctp"}
	for _, item := range protocols {
		if item == protocol {
			return true
		}
	}
	return false
}

// IsVersionValid check if version value is according to semantic definitions
func IsVersionValid(version string) bool {
	_, err := semver.NewVersion(version)
	return err == nil
}

// IsKubeResourceNameValid check if name is valid for kubernetes resources (RFC 1123)
// Since regexp does not support lookbehind, we test directly the last character before
// matching the string name.
func IsKubeResourceNameValid(name string) bool {
	const (
		minNameLength           = 1
		maxNameLength           = 63
		regexCanOnlyStartWith   = "^[a-z0-9]"
		regexAcceptedCharacters = "[a-z0-9-]*$"
		cannotEndWith           = "-"
		regexValidMatch         = regexCanOnlyStartWith + regexAcceptedCharacters
	)

	if len(name) < minNameLength || len(name) > maxNameLength {
		return false
	}

	if name[len(name)-1:] == cannotEndWith {
		return false
	}

	matched, err := regexp.MatchString(regexValidMatch, name)
	if err != nil || !matched {
		return false
	}
	return true
}

// IsForwarderTypeSupported check if received forwarder type is supported by Maestro
func IsForwarderTypeSupported(forwarderType string) bool {
	types := []string{string(forwarder.TypeGrpc)}
	for _, item := range types {
		if item == forwarderType {
			return true
		}
	}
	return false
}
