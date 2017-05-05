// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import (
	"errors"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/onsi/gomega/types"
)

//ContainLogMessage validates that the specified log message exists in the given entries
func ContainLogMessage(expected string) types.GomegaMatcher {
	return &containLogMessageMatcher{
		expected: expected,
	}
}

type containLogMessageMatcher struct {
	expected string
}

func (matcher *containLogMessageMatcher) Match(actual interface{}) (success bool, err error) {
	entries, ok := actual.([]*logrus.Entry)
	if !ok {
		return false, errors.New("cannot to convert value to []*logrus.Entry")
	}
	for _, entry := range entries {
		if entry.Message == matcher.expected {
			return true, nil
		}
	}
	return false, nil
}

func (matcher *containLogMessageMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto contain log message \n\t%#v", actual, matcher.expected)
}

func (matcher *containLogMessageMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to contain log message \n\t%#v", actual, matcher.expected)
}
