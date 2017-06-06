// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("Namespace", func() {
	var (
		name        = "pong-free-for-all"
		listOptions = metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
		namespace *models.Namespace
	)

	BeforeEach(func() {
		name = fmt.Sprintf("maestro-test-%s", uuid.NewV4())
	})

	AfterEach(func() {
		namespace.Delete(clientset)
	})

	Describe("DeletePods", func() {
		It("should succeed if namespace exists and has pods", func() {
			namespace = models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			pod := models.NewPod(
				"game", "image", "name", namespace.Name, "1", "1", "1", "1", 0,
				[]*models.Port{{ContainerPort: 5050}},
				[]string{"command"},
				[]*models.EnvVar{},
			)
			_, err = pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				pods, err := clientset.CoreV1().Pods(namespace.Name).List(listOptions)
				Expect(err).NotTo(HaveOccurred())
				return len(pods.Items)
			}).Should(Equal(1))

			err = namespace.DeletePods(clientset)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				pods, err := clientset.CoreV1().Pods(namespace.Name).List(listOptions)
				Expect(err).NotTo(HaveOccurred())
				return len(pods.Items)
			}).Should(BeZero())
		})
	})
})
