// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"

	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Namespace", func() {
	var clientset *fake.Clientset
	name := "pong-free-for-all"

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("NewNamespace", func() {
		It("should build correct namespace struct", func() {
			namespace := models.NewNamespace(name)
			Expect(namespace.Name).To(Equal(name))
		})
	})

	Describe("Create", func() {
		It("should create a namespace in kubernetes", func() {
			namespace := models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal(name))
		})

		It("should return error when creating existing namespace", func() {
			namespace := models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = namespace.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("Namespace \"%s\" already exists", name)))
		})
	})

	Describe("Exists", func() {
		It("should return false if namespace does not exist", func() {
			namespace := models.NewNamespace(name)
			exists, err := namespace.Exists(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should return true if namespace exists", func() {
			namespace := models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			exists, err := namespace.Exists(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	Describe("Delete", func() {
		It("should succeed if namespace does not exist", func() {
			namespace := models.NewNamespace(name)
			err := namespace.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if namespace exists", func() {
			namespace := models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = namespace.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

		})
	})

	Describe("DeletePods", func() {
		It("should fail if namespace does not exist", func() {
			namespace := models.NewNamespace(name)
			err := namespace.DeletePods(clientset, mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if namespace exists and has no pods", func() {
			namespace := models.NewNamespace(name)
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = namespace.DeletePods(clientset, mockRedisClient)
			Expect(err).NotTo(HaveOccurred())

			pods, err := clientset.CoreV1().Pods(namespace.Name).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(0))
		})
	})
})
