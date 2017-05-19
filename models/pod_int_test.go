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
	"strings"

	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pod", func() {
	var (
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
		command = []string{
			"./room-binary",
			"-serverType",
			"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
		}
		env = []*models.EnvVar{
			{
				Name:  "EXAMPLE_ENV_VAR",
				Value: "examplevalue",
			},
			{
				Name:  "ANOTHER_ENV_VAR",
				Value: "anothervalue",
			},
		}
		game = "pong"
		image = "pong/pong:v123"
		name = "pong-free-for-all-0"
		namespace = fmt.Sprintf("maestro-test-%s", uuid.NewV4())
		ports = []*models.Port{
			{
				ContainerPort: 5050,
			},
			{
				ContainerPort: 8888,
			},
		}
		resourcesLimitsCPU = "2"
		resourcesLimitsMemory = "128974848"
		resourcesRequestsCPU = "1"
		resourcesRequestsMemory = "64487424"
		shutdownTimeout = 180
	})

	AfterEach(func() {
		err := clientset.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Create", func() {
		It("should create a pod in kubernetes", func() {
			ns := models.NewNamespace(namespace)
			err := ns.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

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
			Expect(podv1.Spec.Tolerations).To(HaveLen(1))
			Expect(podv1.Spec.Tolerations[0].Key).To(Equal("game"))
			Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
			Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
			Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))

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
	})
})
