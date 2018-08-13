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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"

	"github.com/topfreegames/maestro/models"
)

var _ = Describe("Service", func() {
	var (
		command         []string
		env             []*models.EnvVar
		game            string
		image           string
		name            string
		namespace       string
		ports           []*models.Port
		requests        *models.Resources
		limits          *models.Resources
		shutdownTimeout int
		configYaml      *models.ConfigYAML
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
		namespace = "pong-free-for-all"
		ports = []*models.Port{
			{
				ContainerPort: 5050,
				HostPort:      5000,
				Protocol:      "TCP",
			},
			{
				ContainerPort: 8888,
				HostPort:      5001,
				Protocol:      "UDP",
			},
		}
		limits = &models.Resources{
			CPU:    "2",
			Memory: "128974848",
		}
		requests = &models.Resources{
			CPU:    "1",
			Memory: "64487424",
		}
		shutdownTimeout = 180

		configYaml = &models.ConfigYAML{
			Name:            namespace,
			Game:            game,
			Image:           image,
			Limits:          limits,
			Requests:        requests,
			ShutdownTimeout: shutdownTimeout,
			Ports:           ports,
			Cmd:             command,
		}
	})

	Describe("NewService", func() {
		It("should build correct service struct", func() {
			service := models.NewService(name, configYaml)
			Expect(service.Name).To(Equal(name))
			Expect(service.Namespace).To(Equal(namespace))
			Expect(service.Port).To(Equal(ports[0]))
		})
	})

	Describe("Create", func() {

		It("should create a service in kubernetes", func() {
			service := models.NewService(name, configYaml)
			servicev1, err := service.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(servicev1.GetNamespace()).To(Equal(namespace))
			Expect(servicev1.ObjectMeta.Name).To(Equal(name))
			Expect(servicev1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(servicev1.ObjectMeta.Labels).To(HaveLen(1))
			Expect(servicev1.ObjectMeta.Labels["name"]).To(Equal(name))
			Expect(servicev1.Spec.Selector["app"]).To(Equal(name))
			Expect(servicev1.Spec.Type).To(Equal(v1.ServiceTypeNodePort))
		})

		It("should return error when creating existing pod", func() {
			service := models.NewService(name, configYaml)
			_, err := service.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = service.Create(mockClientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("services \"%s\" already exists", name)))
		})
	})

	Describe("Delete", func() {

		It("should delete a service from kubernetes", func() {
			service := models.NewService(name, configYaml)
			_, err := service.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			err = service.Delete(mockClientset, "deletion_reason", configYaml)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when deleting non existent pod", func() {
			service := models.NewService(name, configYaml)
			err := service.Delete(mockClientset, "deletion_reason", configYaml)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("services \"pong-free-for-all-0\" not found"))
		})
	})
})
