package models_test

import (
	"strings"

	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pod", func() {
	var (
		clientset               *fake.Clientset
		command                 []string
		env                     []*models.EnvVar
		game                    string
		image                   string
		name                    string
		namespace               string
		ports                   []*models.Port
		resourcesLimitsCPU      string
		resourcesLimitsMemory   string
		resourcesRequestsCPU    string
		resourcesRequestsMemory string
		shutdownTimeout         int
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		command = []string{
			"./room-binary",
			"-serverType",
			"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
		}
		env = []*models.EnvVar{
			&models.EnvVar{
				Name:  "EXAMPLE_ENV_VAR",
				Value: "examplevalue",
			},
			&models.EnvVar{
				Name:  "ANOTHER_ENV_VAR",
				Value: "anothervalue",
			},
		}
		game = "pong"
		image = "pong/pong:v123"
		name = "pong-free-for-all-0"
		namespace = "pong-free-for-all"
		ports = []*models.Port{
			&models.Port{
				ContainerPort: 5050,
			},
			&models.Port{
				ContainerPort: 8888,
			},
		}
		resourcesLimitsCPU = "2"
		resourcesLimitsMemory = "128974848"
		resourcesRequestsCPU = "1"
		resourcesRequestsMemory = "64487424"
		shutdownTimeout = 180
	})

	Describe("NewPod", func() {
		It("should build correct pod struct", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			Expect(pod.Game).To(Equal(game))
			Expect(pod.Image).To(Equal(image))
			Expect(pod.Name).To(Equal(name))
			Expect(pod.Namespace).To(Equal(namespace))
			Expect(pod.ResourcesLimitsCPU).To(Equal(resourcesLimitsCPU))
			Expect(pod.ResourcesLimitsMemory).To(Equal(resourcesLimitsMemory))
			Expect(pod.ResourcesRequestsCPU).To(Equal(resourcesRequestsCPU))
			Expect(pod.ResourcesRequestsMemory).To(Equal(resourcesRequestsMemory))
			Expect(pod.Ports).To(Equal(ports))
			Expect(pod.ShutdownTimeout).To(Equal(shutdownTimeout))
			Expect(pod.Command).To(Equal(command))
			Expect(pod.Env).To(Equal(env))
		})
	})

	Describe("Create", func() {
		It("should create a pod", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			podv1, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(1))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
			// TODO: update go client so we can use tolerations
			Expect(podv1.Spec.Containers).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Name).To(Equal(name))
			Expect(podv1.Spec.Containers[0].Image).To(Equal(image))
			Expect(podv1.Spec.Containers[0].Ports).To(HaveLen(len(ports)))
			for idx, port := range podv1.Spec.Containers[0].Ports {
				Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
			}
			quantity := podv1.Spec.Containers[0].Resources.Limits["memory"]
			Expect((&quantity).String()).To(Equal(resourcesLimitsMemory))
			quantity = podv1.Spec.Containers[0].Resources.Limits["cpu"]
			Expect((&quantity).String()).To(Equal(resourcesLimitsCPU))
			quantity = podv1.Spec.Containers[0].Resources.Requests["memory"]
			Expect((&quantity).String()).To(Equal(resourcesRequestsMemory))
			quantity = podv1.Spec.Containers[0].Resources.Requests["cpu"]
			Expect((&quantity).String()).To(Equal(resourcesRequestsCPU))
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(len(env)))
			for idx, envVar := range podv1.Spec.Containers[0].Env {
				Expect(envVar.Name).To(Equal(env[idx].Name))
				Expect(envVar.Value).To(Equal(env[idx].Value))
			}
			Expect(podv1.Spec.Containers[0].Command).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Command[0]).To(Equal(strings.Join(command, " ")))
		})

		It("should return error when creating existing pod", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			_, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = pod.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Pod \"pong-free-for-all-0\" already exists"))
		})
	})
})
