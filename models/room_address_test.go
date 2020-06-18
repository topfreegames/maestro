// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"encoding/json"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/golang/mock/gomock"

	"k8s.io/apimachinery/pkg/util/intstr"

	goredis "github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/maestro/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func createPodWithContainers(
	clientset *fake.Clientset,
	redisClient *redismocks.MockRedisClient,
	schedulerName, nodeName, podName string,
	containers []v1.Container,
) {
	pod := &v1.Pod{}
	pod.Spec.NodeName = nodeName
	pod.SetName(podName)
	pod.Spec.Containers = containers
	_, err := clientset.CoreV1().Pods(schedulerName).Create(pod)
	Expect(err).NotTo(HaveOccurred())

	var podModel models.Pod
	podModel.Name = podName
	podModel.Spec.NodeName = nodeName
	podModel.Spec.Containers = containers

	jsonBytes, err := podModel.MarshalToRedis()
	Expect(err).NotTo(HaveOccurred())

	redisClient.EXPECT().
		HGet(models.GetPodMapRedisKey(schedulerName), podModel.Name).
		Return(goredis.NewStringResult(string(jsonBytes), nil))
}

var _ = Describe("AddressGetter", func() {
	var (
		clientset *fake.Clientset
		command   = []string{
			"./room-binary",
			"-serverType",
			"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
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
		room                   *models.Room
		nodeName               = "node-name"
		host                   = "0.0.0.0"
		port                   = int32(1234)
		nodePort               = int32(1234)
		ipv6KubernetesLabelKey = "test.io/ipv6"
		ipv6Label              = base58.Encode([]byte("testIpv6"))
		nodeLabels             = map[string]string{ipv6KubernetesLabelKey: ipv6Label}
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		room = models.NewRoom(name, namespace)
	})

	Context("Cache usage", func() {
		Context("When in development env", func() {
			It("should not query kube api when addr is cached", func() {
				node := &v1.Node{}
				node.SetName(nodeName)
				node.SetLabels(nodeLabels)
				node.Status.Addresses = []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: host,
					},
				}
				_, err := clientset.CoreV1().Nodes().Create(node)
				Expect(err).NotTo(HaveOccurred())

				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				})

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

				step1 := len(clientset.Fake.Actions())
				addrGetter := models.NewRoomAddressesFromNodePort(logger, ipv6KubernetesLabelKey, true, 10*time.Second)
				mockRedisClient.EXPECT().Get("room-addr-pong-free-for-all-pong-free-for-all-0").
					Return(goredis.NewStringResult("", goredis.Nil))
				mockRedisClient.EXPECT().Set("room-addr-pong-free-for-all-pong-free-for-all-0", gomock.Any(), gomock.Any()).
					Return(goredis.NewStatusCmd())
				addrs1, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())

				step2 := len(clientset.Fake.Actions())
				Expect(step2).To(Equal(step1 + 2))
				b, err := json.Marshal(addrs1)
				Expect(err).NotTo(HaveOccurred())
				mockRedisClient.EXPECT().Get("room-addr-pong-free-for-all-pong-free-for-all-0").
					Return(goredis.NewStringResult(string(b), nil))
				addrs2, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())

				step3 := len(clientset.Fake.Actions())
				Expect(step3).To(Equal(step2))
				Expect(addrs2).To(Equal(addrs1))
				Expect(addrs2).NotTo(BeIdenticalTo(addrs1))
			})
		})

		Context("When in production env", func() {
			It("should not query kube api when addr is cached", func() {
				node := &v1.Node{}
				node.SetName(nodeName)
				node.Status.Addresses = []v1.NodeAddress{
					{
						Type:    v1.NodeExternalDNS,
						Address: host,
					},
				}
				_, err := clientset.CoreV1().Nodes().Create(node)
				Expect(err).NotTo(HaveOccurred())

				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				})

				step1 := len(clientset.Fake.Actions())
				addrGetter := models.NewRoomAddressesFromHostPort(logger, ipv6KubernetesLabelKey, true, 10*time.Second)
				mockRedisClient.EXPECT().Get("room-addr-pong-free-for-all-pong-free-for-all-0").
					Return(goredis.NewStringResult("", goredis.Nil))
				mockRedisClient.EXPECT().Set("room-addr-pong-free-for-all-pong-free-for-all-0", gomock.Any(), gomock.Any()).
					Return(goredis.NewStatusCmd())
				addrs1, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())

				step2 := len(clientset.Fake.Actions())
				Expect(step2).To(Equal(step1 + 1))
				b, err := json.Marshal(addrs1)
				Expect(err).NotTo(HaveOccurred())
				mockRedisClient.EXPECT().Get("room-addr-pong-free-for-all-pong-free-for-all-0").
					Return(goredis.NewStringResult(string(b), nil))
				addrs2, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())

				step3 := len(clientset.Fake.Actions())
				Expect(step3).To(Equal(step2))
				Expect(addrs2).To(Equal(addrs1))
				Expect(addrs2).NotTo(BeIdenticalTo(addrs1))
			})
		})
	})

	Context("When in development env", func() {
		var addrGetter = models.NewRoomAddressesFromNodePort(logger, ipv6KubernetesLabelKey, false, 0)

		Describe("Get", func() {
			It("should not crash if pod does not exist", func() {
				mockRedisClient.EXPECT().
					HGet(models.GetPodMapRedisKey(room.SchedulerName), room.ID).
					Return(goredis.NewStringResult("", goredis.Nil))
				_, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`pod "pong-free-for-all-0" does not exist on redis podMap`))
			})

			It("should return no address if no node assigned to the room", func() {
				createPodWithContainers(clientset, mockRedisClient, namespace, "", name, []v1.Container{})

				svc := models.NewService(name, configYaml)
				svc.Create(clientset)
				room := models.NewRoom(name, namespace)

				addresses, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(addresses.Ports)).To(Equal(0))
			})

			It("should return room address", func() {
				node := &v1.Node{}
				node.SetName(nodeName)
				node.SetLabels(nodeLabels)
				node.Status.Addresses = []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: host,
					},
				}
				_, err := clientset.CoreV1().Nodes().Create(node)
				Expect(err).NotTo(HaveOccurred())

				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				})

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

				roomAddresses, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())
				Expect(roomAddresses.Host).To(Equal(host))

				expectedIpv6Label := base58.Decode(ipv6Label)
				Expect(roomAddresses.Ipv6Label).To(Equal(string(expectedIpv6Label)))

				Expect(roomAddresses.Ports).To(HaveLen(1))
				Expect(roomAddresses.Ports[0]).To(Equal(
					&models.RoomPort{
						Name: "TCP",
						Port: port,
						Protocol: "TCP",
					}))
			})

			It("should not return error if no Ipv6 label is defined", func() {
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

				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				})

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

				roomAddresses, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).NotTo(HaveOccurred())
				Expect(roomAddresses.Host).To(Equal(host))
				Expect(roomAddresses.Ipv6Label).To(Equal(""))
				Expect(roomAddresses.Ports).To(HaveLen(1))
				Expect(roomAddresses.Ports[0]).To(Equal(
					&models.RoomPort{
						Name: "TCP",
						Port: port,
						Protocol: "TCP",
					}))
			})

			It("should return error if there is no node", func() {
				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{})

				_, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("nodes \"node-name\" not found"))
			})
		})
	})

	Context("When in production env", func() {
		var addrGetter = models.NewRoomAddressesFromHostPort(logger, ipv6KubernetesLabelKey, false, 0)

		Describe("Get", func() {
			It("should not crash if pod does not exist", func() {
				mockRedisClient.EXPECT().
					HGet(models.GetPodMapRedisKey(namespace), name).
					Return(goredis.NewStringResult("", goredis.Nil))
				_, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`pod "pong-free-for-all-0" does not exist on redis podMap`))
			})

			It("should return no address if no node assigned to the room", func() {
				createPodWithContainers(clientset, mockRedisClient, namespace, "", name, []v1.Container{})

				svc := models.NewService(name, configYaml)
				svc.Create(clientset)
				room := models.NewRoom(name, namespace)
				addresses, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
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

				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: port, Name: "TCP"},
					}},
				})

				roomAddresses, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
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
				createPodWithContainers(clientset, mockRedisClient, namespace, nodeName, name, []v1.Container{})

				_, err := addrGetter.Get(room, clientset, mockRedisClient, mmr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("nodes \"node-name\" not found"))
			})
		})
	})

})
