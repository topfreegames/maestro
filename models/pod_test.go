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

	goredis "github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

var _ = Describe("Pod", func() {
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

	createPod := func() (*models.Pod, error) {
		mr.EXPECT().Report("gru.new", map[string]string{
			reportersConstants.TagGame:      "pong",
			reportersConstants.TagScheduler: "pong-free-for-all",
		})

		pod, err := models.NewPod(name, env, configYaml, mockClientset, mockRedisClient)
		Expect(err).NotTo(HaveOccurred())

		return pod, err
	}

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

	Describe("NewPod", func() {
		BeforeEach(func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().Exec()
		})

		It("should build correct pod struct", func() {
			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Game).To(Equal(game))
			Expect(pod.Name).To(Equal(name))
			Expect(pod.Namespace).To(Equal(namespace))
			Expect(pod.ShutdownTimeout).To(Equal(shutdownTimeout))
			Expect(pod.Containers[0].Image).To(Equal(image))
			Expect(pod.Containers[0].Limits.CPU).To(Equal(limits.CPU))
			Expect(pod.Containers[0].Limits.Memory).To(Equal(limits.Memory))
			Expect(pod.Containers[0].Requests.CPU).To(Equal(requests.CPU))
			Expect(pod.Containers[0].Requests.Memory).To(Equal(requests.Memory))
			Expect(pod.Containers[0].Ports).To(Equal(ports))
			Expect(pod.Containers[0].Command).To(Equal(command))
			Expect(pod.Containers[0].Env).To(Equal(env))
		})

		Describe("Calling Reporters' singleton instance", func() {
			It("should report gru.new on models.NewPod()", func() {
				createPod()
			})

			It("should report gru.delete on pod.Delete()", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).Return(goredis.NewStringResult("5000-6000", nil))
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), 5000)
				mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), 5001)
				mockPipeline.EXPECT().Exec()

				pod, err := createPod()

				_, err = pod.Create(mockClientset)
				Expect(err).NotTo(HaveOccurred())

				mr.EXPECT().Report("gru.delete", map[string]string{
					reportersConstants.TagGame:      "pong",
					reportersConstants.TagScheduler: "pong-free-for-all",
					reportersConstants.TagReason:    "deletion_reason",
				})
				err = pod.Delete(mockClientset, mockRedisClient, "deletion_reason", configYaml)
				Expect(err).NotTo(HaveOccurred())
			})
		})

	})

	Describe("NewPodWithContainers", func() {
		It("should create pod with two containers", func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().Exec()

			mr.EXPECT().Report("gru.new", map[string]string{
				reportersConstants.TagGame:      "pong",
				reportersConstants.TagScheduler: "pong-free-for-all",
			})

			pod, err := models.NewPodWithContainers(
				name,
				[]*models.Container{
					{
						Name:     "container1",
						Image:    "pong/pong:v123",
						Limits:   limits,
						Requests: requests,
						Ports:    ports[0:1],
						Env:      env,
						Command:  command,
					},
					{
						Name:     "container2",
						Image:    "ping/ping:v123",
						Limits:   limits,
						Requests: requests,
						Ports:    ports[1:2],
						Env:      env,
						Command:  command,
					},
				},
				configYaml, mockClientset, mockRedisClient,
			)

			Expect(err).NotTo(HaveOccurred())
			pod.SetToleration(game)
			pod.SetVersion("v1.0")
			podv1, err := pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := mockClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(3))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(podv1.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
			Expect(podv1.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
			Expect(podv1.Spec.Tolerations).To(HaveLen(1))
			Expect(podv1.Spec.Tolerations[0].Key).To(Equal("dedicated"))
			Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
			Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
			Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))

			podv1.Spec.Containers[0].Image = "pong/pong:v123"
			podv1.Spec.Containers[0].Name = "container1"
			podv1.Spec.Containers[1].Image = "ping/ping:v123"
			podv1.Spec.Containers[1].Name = "container2"

			Expect(podv1.Spec.Containers).To(HaveLen(2))
			for cIdx, container := range podv1.Spec.Containers {
				Expect(container.Ports).To(HaveLen(1))
				for _, port := range container.Ports {
					Expect(port.ContainerPort).To(BeEquivalentTo(ports[cIdx].ContainerPort))
					Expect(port.HostPort).To(BeEquivalentTo(ports[cIdx].HostPort))
					Expect(string(port.Protocol)).To(Equal(ports[cIdx].Protocol))
				}
				quantity := container.Resources.Limits["memory"]
				Expect((&quantity).String()).To(Equal(limits.Memory))
				quantity = container.Resources.Limits["cpu"]
				Expect((&quantity).String()).To(Equal(limits.CPU))
				quantity = container.Resources.Requests["memory"]
				Expect((&quantity).String()).To(Equal(requests.Memory))
				quantity = container.Resources.Requests["cpu"]
				Expect((&quantity).String()).To(Equal(requests.CPU))
				Expect(container.Env).To(HaveLen(len(env)))
				for idx, envVar := range container.Env {
					Expect(envVar.Name).To(Equal(env[idx].Name))
					Expect(envVar.Value).To(Equal(env[idx].Value))
				}
				Expect(container.Command).To(HaveLen(3))
				Expect(container.Command).To(Equal(command))
			}

			Expect(podv1.GetAnnotations()).To(HaveKeyWithValue("cluster-autoscaler.kubernetes.io/safe-to-evict", "true"))
		})

		It("should return correct ports if pod already exists", func() {
			mr.EXPECT().Report("gru.new", map[string]string{
				reportersConstants.TagGame:      "pong",
				reportersConstants.TagScheduler: "pong-free-for-all",
			})

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().Exec()

			mr.EXPECT().Report("gru.new", map[string]string{
				reportersConstants.TagGame:      "pong",
				reportersConstants.TagScheduler: "pong-free-for-all",
			})

			firstPod, err := models.NewPodWithContainers(
				name,
				[]*models.Container{
					{
						Name:     "container1",
						Image:    "pong/pong:v123",
						Limits:   limits,
						Requests: requests,
						Ports:    ports[0:1],
						Env:      env,
						Command:  command,
					},
					{
						Name:     "container2",
						Image:    "ping/ping:v123",
						Limits:   limits,
						Requests: requests,
						Ports:    ports[1:2],
						Env:      env,
						Command:  command,
					},
				},
				configYaml, mockClientset, mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			firstPod.SetToleration(game)
			podv1, err := firstPod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			pod, err := models.NewPodWithContainers(
				name,
				[]*models.Container{
					{
						Name:     "container1",
						Image:    "pong/pong:v123",
						Limits:   limits,
						Requests: requests,
						Ports:    ports[0:1],
						Env:      env,
						Command:  command,
					},
					{
						Name:     "container2",
						Image:    "ping/ping:v123",
						Limits:   limits,
						Requests: requests,
						Ports:    ports[1:2],
						Env:      env,
						Command:  command,
					},
				},
				configYaml, mockClientset, mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			pod.SetToleration(game)

			for i, container := range pod.Containers {
				Expect(container.Ports).To(Equal(firstPod.Containers[i].Ports))
			}

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := mockClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(3))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
			Expect(podv1.Spec.Tolerations).To(HaveLen(1))
			Expect(podv1.Spec.Tolerations[0].Key).To(Equal("dedicated"))
			Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
			Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
			Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))

			podv1.Spec.Containers[0].Image = "pong/pong:v123"
			podv1.Spec.Containers[0].Name = "container1"
			podv1.Spec.Containers[1].Image = "ping/ping:v123"
			podv1.Spec.Containers[1].Name = "container2"

			Expect(podv1.Spec.Containers).To(HaveLen(2))
			for cIdx, container := range podv1.Spec.Containers {
				Expect(container.Ports).To(HaveLen(1))
				for _, port := range container.Ports {
					Expect(port.ContainerPort).To(BeEquivalentTo(ports[cIdx].ContainerPort))
					Expect(port.HostPort).To(BeEquivalentTo(ports[cIdx].HostPort))
					Expect(string(port.Protocol)).To(Equal(ports[cIdx].Protocol))
				}
				quantity := container.Resources.Limits["memory"]
				Expect((&quantity).String()).To(Equal(limits.Memory))
				quantity = container.Resources.Limits["cpu"]
				Expect((&quantity).String()).To(Equal(limits.CPU))
				quantity = container.Resources.Requests["memory"]
				Expect((&quantity).String()).To(Equal(requests.Memory))
				quantity = container.Resources.Requests["cpu"]
				Expect((&quantity).String()).To(Equal(requests.CPU))
				Expect(container.Env).To(HaveLen(len(env)))
				for idx, envVar := range container.Env {
					Expect(envVar.Name).To(Equal(env[idx].Name))
					Expect(envVar.Value).To(Equal(env[idx].Value))
				}
				Expect(container.Command).To(HaveLen(3))
				Expect(container.Command).To(Equal(command))
			}

			Expect(podv1.GetAnnotations()).To(HaveKeyWithValue("cluster-autoscaler.kubernetes.io/safe-to-evict", "true"))
		})
	})

	Describe("Create", func() {
		BeforeEach(func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().Exec()
		})

		It("should create a pod in kubernetes", func() {
			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			pod.SetToleration(game)
			podv1, err := pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := mockClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(3))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
			Expect(podv1.Spec.Tolerations).To(HaveLen(1))
			Expect(podv1.Spec.Tolerations[0].Key).To(Equal("dedicated"))
			Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
			Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
			Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))

			Expect(podv1.Spec.Containers).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Name).To(Equal(name))
			Expect(podv1.Spec.Containers[0].Image).To(Equal(image))
			Expect(podv1.Spec.Containers[0].Ports).To(HaveLen(len(ports)))
			for idx, port := range podv1.Spec.Containers[0].Ports {
				Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
				Expect(port.HostPort).To(BeEquivalentTo(ports[idx].HostPort))
				Expect(string(port.Protocol)).To(Equal(ports[idx].Protocol))
			}
			quantity := podv1.Spec.Containers[0].Resources.Limits["memory"]
			Expect((&quantity).String()).To(Equal(limits.Memory))
			quantity = podv1.Spec.Containers[0].Resources.Limits["cpu"]
			Expect((&quantity).String()).To(Equal(limits.CPU))
			quantity = podv1.Spec.Containers[0].Resources.Requests["memory"]
			Expect((&quantity).String()).To(Equal(requests.Memory))
			quantity = podv1.Spec.Containers[0].Resources.Requests["cpu"]
			Expect((&quantity).String()).To(Equal(requests.CPU))
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(len(env)))
			for idx, envVar := range podv1.Spec.Containers[0].Env {
				Expect(envVar.Name).To(Equal(env[idx].Name))
				Expect(envVar.Value).To(Equal(env[idx].Value))
			}
			Expect(podv1.Spec.Containers[0].Command).To(HaveLen(3))
			Expect(podv1.Spec.Containers[0].Command).To(Equal(command))

			Expect(podv1.GetAnnotations()).To(HaveKeyWithValue("cluster-autoscaler.kubernetes.io/safe-to-evict", "true"))
		})

		It("should create pod without requests and limits", func() {
			mr.EXPECT().Report("gru.new", map[string]string{
				reportersConstants.TagGame:      "pong",
				reportersConstants.TagScheduler: "pong-free-for-all",
			})

			configYaml = &models.ConfigYAML{
				Name:            namespace,
				Game:            game,
				Image:           image,
				Limits:          nil,
				Requests:        nil,
				ShutdownTimeout: shutdownTimeout,
				Ports:           ports,
				Cmd:             command,
			}

			pod, err := models.NewPod(name, env, configYaml, mockClientset, mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
			podv1, err := pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := mockClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(3))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))

			Expect(podv1.Spec.Containers).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Name).To(Equal(name))
			Expect(podv1.Spec.Containers[0].Image).To(Equal(image))
			Expect(podv1.Spec.Containers[0].Ports).To(HaveLen(len(ports)))
			for idx, port := range podv1.Spec.Containers[0].Ports {
				Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
			}
			quantity := podv1.Spec.Containers[0].Resources.Limits["memory"]
			Expect((&quantity).String()).To(Equal("0"))
			quantity = podv1.Spec.Containers[0].Resources.Limits["cpu"]
			Expect((&quantity).String()).To(Equal("0"))
			quantity = podv1.Spec.Containers[0].Resources.Requests["memory"]
			Expect((&quantity).String()).To(Equal("0"))
			quantity = podv1.Spec.Containers[0].Resources.Requests["cpu"]
			Expect((&quantity).String()).To(Equal("0"))
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(len(env)))
			for idx, envVar := range podv1.Spec.Containers[0].Env {
				Expect(envVar.Name).To(Equal(env[idx].Name))
				Expect(envVar.Value).To(Equal(env[idx].Value))
			}
			Expect(podv1.Spec.Containers[0].Command).To(HaveLen(3))
			Expect(podv1.Spec.Containers[0].Command).To(Equal(command))
		})

		It("should create pod with node affinity", func() {
			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			pod.SetAffinity(game)
			podv1, err := pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(game))
			Expect(podv1.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Operator).To(Equal(v1.NodeSelectorOpIn))
			Expect(podv1.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values).To(ConsistOf("true"))
		})

		It("should create a pod that uses secret", func() {
			env = []*models.EnvVar{
				{
					Name:  "EXAMPLE_ENV_VAR",
					Value: "examplevalue",
				},
				{
					Name: "SECRET_ENV_VAR",
					ValueFrom: models.ValueFrom{
						SecretKeyRef: models.SecretKeyRef{
							Name: "my-secret",
							Key:  "secret-env-var",
						},
					},
				},
			}

			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			podv1, err := pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(2))
			Expect(podv1.Spec.Containers[0].Env[0].Name).To(Equal("EXAMPLE_ENV_VAR"))
			Expect(podv1.Spec.Containers[0].Env[0].Value).To(Equal("examplevalue"))
			Expect(podv1.Spec.Containers[0].Env[1].Name).To(Equal("SECRET_ENV_VAR"))
			Expect(podv1.Spec.Containers[0].Env[1].ValueFrom.SecretKeyRef.Name).To(Equal("my-secret"))
			Expect(podv1.Spec.Containers[0].Env[1].ValueFrom.SecretKeyRef.Key).To(Equal("secret-env-var"))
		})

		It("should return error when creating existing pod", func() {
			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = pod.Create(mockClientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("Pod \"%s\" already exists", name)))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().Exec()
		})

		It("should delete a pod from kubernetes", func() {
			mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).Return(goredis.NewStringResult("5000-6000", nil))
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), 5000)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), 5001)
			mockPipeline.EXPECT().Exec()

			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(mockClientset)
			Expect(err).NotTo(HaveOccurred())

			mr.EXPECT().Report("gru.delete", map[string]string{
				reportersConstants.TagGame:      "pong",
				reportersConstants.TagScheduler: "pong-free-for-all",
				reportersConstants.TagReason:    "deletion_reason",
			})
			err = pod.Delete(mockClientset, mockRedisClient, "deletion_reason", configYaml)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when deleting non existent pod", func() {
			pod, err := createPod()
			Expect(err).NotTo(HaveOccurred())
			err = pod.Delete(mockClientset, mockRedisClient, "deletion_reason", configYaml)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Pod \"pong-free-for-all-0\" not found"))
		})
	})

	Describe("PodExists", func() {
		It("should return true if pod exists", func() {
			name := "pod-name"
			namespace := "pod-ns"
			pod := &v1.Pod{}
			pod.SetName(name)
			_, err := mockClientset.CoreV1().Pods(namespace).Create(pod)
			Expect(err).NotTo(HaveOccurred())

			exists, err := models.PodExists(name, namespace, mockClientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("should return false if pod does not exist", func() {
			name := "pod-name"
			namespace := "pod-ns"

			exists, err := models.PodExists(name, namespace, mockClientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})
})
