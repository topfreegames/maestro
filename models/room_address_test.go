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

	"k8s.io/apimachinery/pkg/util/intstr"

	goredis "github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/maestro/models"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("AddressGetter", func() {
	var (
		clientset *fake.Clientset
		portStart = 5000
		portEnd   = 6000
		portRange = fmt.Sprintf("%d-%d", portStart, portEnd)
		command   = []string{
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
		game      = "pong"
		image     = "pong/pong:v123"
		name      = "pong-free-for-all-0"
		namespace = "pong-free-for-all"
		ports     = []*models.Port{
			{
				ContainerPort: 5050,
				HostPort:      5000,
			},
			{
				ContainerPort: 8888,
				HostPort:      5001,
			},
		}

		requests = &models.Resources{
			CPU:    "2",
			Memory: "128974848",
		}
		limits = &models.Resources{
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
		room     *models.Room
		nodeName = "node-name"
		host     = "0.0.0.0"
		port     = int32(1234)
		nodePort = int32(1234)
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		room = models.NewRoom(name, namespace)
	})

	Context("When in development env", func() {
		var addrGetter = &models.RoomAddressesFromNodePort{}

		Describe("Get", func() {
			It("should not crash if pod does not exist", func() {
				_, err := addrGetter.Get(room, clientset)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`pods "pong-free-for-all-0" not found`))
			})

			It("should return no address if no node assigned to the room", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(portRange, nil))
				mockPortChooser.EXPECT().Choose(portStart, portEnd, 2).Return([]int{5000, 5001})

				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      "pong",
					reportersConstants.TagScheduler: "pong-free-for-all",
				})

				pod, err := models.NewPod(name, env, configYaml, mockClientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
				svc := models.NewService(pod.Name, configYaml)
				svc.Create(clientset)
				room := models.NewRoom(name, namespace)
				addresses, err := addrGetter.Get(room, clientset)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(addresses.Ports)).To(Equal(0))
			})

			It("should return room address", func() {
				node := &v1.Node{}
				node.SetName(nodeName)
				node.Status.Addresses = []v1.NodeAddress{
					v1.NodeAddress{
						Type:    v1.NodeInternalIP,
						Address: host,
					},
				}
				_, err := clientset.CoreV1().Nodes().Create(node)
				Expect(err).NotTo(HaveOccurred())

				pod := &v1.Pod{}
				pod.Spec.NodeName = nodeName
				pod.SetName(name)
				pod.Spec.Containers = []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				}
				_, err = clientset.CoreV1().Pods(namespace).Create(pod)
				Expect(err).NotTo(HaveOccurred())

				service := &v1.Service{}
				service.SetName(name)
				service.Spec.Type = v1.ServiceTypeNodePort
				service.Spec.Ports = []v1.ServicePort{
					{
						Port: port,
						TargetPort: intstr.IntOrString{
							IntVal: port,
						},
						Name:     "TCP",
						Protocol: v1.ProtocolTCP,
						NodePort: nodePort,
					},
				}
				service.Spec.Selector = map[string]string{
					"app": name,
				}
				_, err = clientset.CoreV1().Services(namespace).Create(service)
				Expect(err).NotTo(HaveOccurred())

				roomAddresses, err := addrGetter.Get(room, clientset)
				Expect(err).NotTo(HaveOccurred())
				Expect(roomAddresses.Host).To(Equal(host))
				Expect(roomAddresses.Ports).To(HaveLen(1))
				Expect(roomAddresses.Ports[0]).To(Equal(
					&models.RoomPort{
						Name: "TCP",
						Port: port,
					}))
			})

			It("should return error if there is no node", func() {
				pod := &v1.Pod{}
				pod.SetName(name)
				pod.Spec.NodeName = nodeName
				_, err := clientset.CoreV1().Pods(namespace).Create(pod)
				Expect(err).NotTo(HaveOccurred())

				_, err = addrGetter.Get(room, clientset)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("nodes \"node-name\" not found"))
			})
		})
	})

	Context("When in production env", func() {
		var addrGetter = &models.RoomAddressesFromHostPort{}

		Describe("Get", func() {
			It("should not crash if pod does not exist", func() {
				_, err := addrGetter.Get(room, clientset)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`pods "pong-free-for-all-0" not found`))
			})

			It("should return no address if no node assigned to the room", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(portRange, nil))
				mockPortChooser.EXPECT().Choose(portStart, portEnd, 2).Return([]int{5000, 5001})

				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      "pong",
					reportersConstants.TagScheduler: "pong-free-for-all",
				})

				pod, err := models.NewPod(name, env, configYaml, mockClientset, mockRedisClient)
				Expect(err).NotTo(HaveOccurred())
				_, err = pod.Create(clientset)
				Expect(err).NotTo(HaveOccurred())
				svc := models.NewService(pod.Name, configYaml)
				svc.Create(clientset)
				room := models.NewRoom(name, namespace)
				addresses, err := addrGetter.Get(room, clientset)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(addresses.Ports)).To(Equal(0))
			})

			It("should return room address", func() {
				node := &v1.Node{}
				node.SetName(nodeName)
				node.Status.Addresses = []v1.NodeAddress{
					v1.NodeAddress{
						Type:    v1.NodeExternalDNS,
						Address: host,
					},
				}
				_, err := clientset.CoreV1().Nodes().Create(node)
				Expect(err).NotTo(HaveOccurred())

				pod := &v1.Pod{}
				pod.Spec.NodeName = nodeName
				pod.SetName(name)
				pod.Spec.Containers = []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				}
				_, err = clientset.CoreV1().Pods(namespace).Create(pod)
				Expect(err).NotTo(HaveOccurred())

				roomAddresses, err := addrGetter.Get(room, clientset)
				Expect(err).NotTo(HaveOccurred())
				Expect(roomAddresses.Host).To(Equal(host))
				Expect(roomAddresses.Ports).To(HaveLen(1))
				Expect(roomAddresses.Ports[0]).To(Equal(
					&models.RoomPort{
						Name: "TCP",
						Port: port,
					}))
			})

			It("should return error if there is no node", func() {
				pod := &v1.Pod{}
				pod.SetName(name)
				pod.Spec.NodeName = nodeName
				_, err := clientset.CoreV1().Pods(namespace).Create(pod)
				Expect(err).NotTo(HaveOccurred())

				_, err = addrGetter.Get(room, clientset)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("nodes \"node-name\" not found"))
			})
		})
	})

})
