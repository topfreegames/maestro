// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAutoScaler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AutoScaler Suite")
}
