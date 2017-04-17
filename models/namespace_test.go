package models_test

import (
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
)

var _ = Describe("Namespace", func() {
	var (
		clientset *fake.Clientset
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("NewNamespace", func() {
		It("should build correct namespace struct", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			Expect(namespace.Name).To(Equal("pong-free-for-all"))
		})
	})

	Describe("Create", func() {
		It("should create a namespace", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(v1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(1))
			Expect(ns.Items[0].GetName()).To(Equal("pong-free-for-all"))
		})

		It("should return error when creating existing namespace", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = namespace.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Namespace \"pong-free-for-all\" already exists"))
		})
	})

	Describe("Exists", func() {
		It("should return false if namespace does not exist", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			exists := namespace.Exists(clientset)
			Expect(exists).To(BeFalse())
		})

		It("should return true if namespace exists", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			exists := namespace.Exists(clientset)
			Expect(exists).To(BeTrue())
		})
	})

	Describe("Delete", func() {
		It("should succeed if namespace does not exist", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			err := namespace.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if namespace exists", func() {
			namespace := models.NewNamespace("pong-free-for-all")
			err := namespace.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = namespace.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			ns, err := clientset.CoreV1().Namespaces().List(v1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Items).To(HaveLen(0))

		})
	})
})
