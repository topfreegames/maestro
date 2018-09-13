// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
	metricsFake "k8s.io/metrics/pkg/client/clientset_generated/clientset/fake"

	"testing"
)

func TestAutoScaler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AutoScaler Suite")
}

var (
	clientset        *fake.Clientset
	metricsClientset *metricsFake.Clientset
	schedulerName    string
)

var _ = BeforeEach(func() {
	clientset = fake.NewSimpleClientset()
	metricsClientset = metricsFake.NewSimpleClientset()
	schedulerName = "scheduler-test"
})
