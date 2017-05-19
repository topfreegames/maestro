// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mTest "github.com/topfreegames/maestro/testing"
	"k8s.io/client-go/kubernetes"
)

var clientset kubernetes.Interface

func TestIntModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Integration Suite")
}

var _ = BeforeSuite(func() {
	config, err := mTest.MinikubeConfig()
	Expect(err).NotTo(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())
})
