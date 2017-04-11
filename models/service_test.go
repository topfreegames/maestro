package models_test

import (
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service", func() {
	var (
		clientset *fake.Clientset
		name      string
		namespace string
		ports     []*models.Port
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		name = "pong-free-for-all-0"
		namespace = "pong-free-for-all"
		ports = []*models.Port{
			&models.Port{
				ContainerPort: 5050,
				Protocol:      "UDP",
			},
			&models.Port{
				ContainerPort: 8888,
				Protocol:      "TCP",
			},
		}
	})

	Describe("NewService", func() {
		It("should build correct service struct", func() {
			service := models.NewService(name, namespace, ports)
			Expect(service.Name).To(Equal(name))
			Expect(service.Namespace).To(Equal(namespace))
			Expect(service.Ports).To(Equal(ports))
		})
	})

	Describe("Create", func() {
		It("should create a service", func() {
			service := models.NewService(name, namespace, ports)
			servicev1, err := service.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(servicev1.GetNamespace()).To(Equal(namespace))
			svcs, err := clientset.CoreV1().Services(namespace).List(v1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(HaveLen(1))
			Expect(svcs.Items[0].GetName()).To(Equal(name))
			Expect(servicev1.ObjectMeta.Name).To(Equal(name))
			Expect(servicev1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(servicev1.ObjectMeta.Labels).To(HaveLen(1))
			Expect(servicev1.ObjectMeta.Labels["name"]).To(Equal(name))
			Expect(servicev1.Spec.Selector).To(HaveLen(1))
			Expect(servicev1.Spec.Selector["app"]).To(Equal(name))
			Expect(servicev1.Spec.Ports).To(HaveLen(len(ports)))
			for idx, port := range servicev1.Spec.Ports {
				Expect(port.Protocol).To(BeEquivalentTo(ports[idx].Protocol))
				Expect(port.Port).To(BeEquivalentTo(ports[idx].ContainerPort))
				Expect(port.TargetPort.IntValue()).To(BeEquivalentTo(ports[idx].ContainerPort))
			}
			Expect(servicev1.Spec.Type).To(Equal(v1.ServiceTypeNodePort))
		})

		It("should return error when creating existing service", func() {
			service := models.NewService(name, namespace, ports)
			_, err := service.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = service.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Service \"pong-free-for-all-0\" already exists"))
		})
	})
})
