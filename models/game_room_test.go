// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"errors"
	"time"

	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/topfreegames/maestro/models"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
)

const (
	schedulerYaml_game_room_test = `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
limits:
  memory: "66Mi"
  cpu: "2"
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
      threshold: 80
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
      threshold: 80
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
forwarders:
  plugin:
    name:
      enabled: true
`
)

var _ = Describe("GameRoomManagement", func() {
	var (
		roomManager models.RoomManager
		command     = []string{
			"./room-binary",
			"-serverType",
			"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
		}
		game      = "pong"
		image     = "pong/pong:v123"
		namespace = "pong-free-for-all"
		portStart = 5000
		portEnd   = 6000
		portRange = models.NewPortRange(portStart, portEnd).String()
		ports     = []*models.Port{
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
		env = []*models.EnvVar{
			{
				Name:  "MAESTRO_SCHEDULER_NAME",
				Value: namespace,
			},
			{
				Name:  "MAESTRO_ROOM_ID",
				Value: namespace,
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
			NodeToleration:  game,
		}
		configYaml1 models.ConfigYAML
		scheduler   *models.Scheduler
	)

	BeforeEach(func() {
		err := yaml.Unmarshal([]byte(schedulerYaml_game_room_test), &configYaml1)
		Expect(err).NotTo(HaveOccurred())
		scheduler = models.NewScheduler(configYaml1.Name, configYaml1.Game, yaml1)
	})

	Context("GameRoom", func() {
		BeforeEach(func() {
			roomManager = &models.GameRoom{}
		})

		Describe("Create", func() {
			It("Should create a pod", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(portRange, nil))
				mockPortChooser.EXPECT().Choose(portStart, portEnd, 2).Return([]int{5000, 5001})
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().SCard(models.GetRoomStatusSetRedisKey(namespace, models.StatusCreating)).Return(goredis.NewIntResult(int64(2), nil))
				mockPipeline.EXPECT().Exec().AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()
				mr.EXPECT().Report("gru.status", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
					"status":                        models.StatusCreating,
					"gauge":                         "2",
				})
				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
				})

				podv1, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					mockClientset,
					configYaml,
					scheduler,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(podv1.ObjectMeta.Name).To(ContainSubstring(namespace))
				Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
				Expect(podv1.ObjectMeta.Labels).To(HaveLen(3))
				Expect(podv1.ObjectMeta.Labels["app"]).To(ContainSubstring(namespace))
				Expect(podv1.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
				Expect(podv1.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
				Expect(podv1.Spec.Tolerations).To(HaveLen(1))
				Expect(podv1.Spec.Tolerations[0].Key).To(Equal("dedicated"))
				Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
				Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
				Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))
				Expect(podv1.Spec.Containers).To(HaveLen(1))
				for _, container := range podv1.Spec.Containers {
					Expect(container.Ports).To(HaveLen(2))
					for idx, port := range container.Ports {
						Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
						Expect(port.HostPort).To(BeEquivalentTo(ports[idx].HostPort))
						Expect(string(port.Protocol)).To(Equal(ports[idx].Protocol))
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
						Expect(envVar.Value).To(ContainSubstring(env[idx].Value))
					}
					Expect(container.Command).To(HaveLen(3))
					Expect(container.Command).To(Equal(command))
				}
			})
			It("Should return room creation error if redis returns an error", func() {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().Exec().Return(nil, errors.New("")).AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()

				_, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					mockClientset,
					configYaml,
					scheduler,
				)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("Delete", func() {
			It("Should delete a pod", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(portRange, nil))
				mockPortChooser.EXPECT().Choose(portStart, portEnd, 2).Return([]int{5000, 5001})
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().SCard(models.GetRoomStatusSetRedisKey(namespace, models.StatusCreating)).Return(goredis.NewIntResult(int64(2), nil))
				mockPipeline.EXPECT().Exec().AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()
				mr.EXPECT().Report("gru.status", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
					"status":                        models.StatusCreating,
					"gauge":                         "2",
				})
				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
				})

				podv1, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					mockClientset,
					configYaml,
					scheduler,
				)
				Expect(err).NotTo(HaveOccurred())

				mr.EXPECT().Report("gru.delete", map[string]string{
					reportersConstants.TagGame:      "pong",
					reportersConstants.TagScheduler: "pong-free-for-all",
					reportersConstants.TagReason:    "deletion_reason",
				})
				err = roomManager.Delete(logger, mmr, mockClientset, mockRedisClient, configYaml, podv1.Name, "deletion_reason")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should error if pod does not exists", func() {
				err := roomManager.Delete(logger, mmr, mockClientset, mockRedisClient, configYaml, namespace, "deletion_reason")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("pods \"pong-free-for-all\" not found"))
			})
		})
	})

	Context("GameRoomWithService", func() {
		BeforeEach(func() {
			roomManager = &models.GameRoomWithService{}
		})

		Describe("Create", func() {
			It("Should create a pod", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(portRange, nil))
				mockPortChooser.EXPECT().Choose(portStart, portEnd, 2).Return([]int{5000, 5001})
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().SCard(models.GetRoomStatusSetRedisKey(namespace, models.StatusCreating)).Return(goredis.NewIntResult(int64(2), nil))
				mockPipeline.EXPECT().Exec().AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()
				mr.EXPECT().Report("gru.status", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
					"status":                        models.StatusCreating,
					"gauge":                         "2",
				})
				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
				})

				podv1, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					mockClientset,
					configYaml,
					scheduler,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(podv1.ObjectMeta.Name).To(ContainSubstring(namespace))
				Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
				Expect(podv1.ObjectMeta.Labels).To(HaveLen(3))
				Expect(podv1.ObjectMeta.Labels["app"]).To(ContainSubstring(namespace))
				Expect(podv1.ObjectMeta.Labels["heritage"]).To(Equal("maestro"))
				Expect(podv1.ObjectMeta.Labels["version"]).To(Equal("v1.0"))
				Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
				Expect(podv1.Spec.Tolerations).To(HaveLen(1))
				Expect(podv1.Spec.Tolerations[0].Key).To(Equal("dedicated"))
				Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
				Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
				Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))
				Expect(podv1.Spec.Containers).To(HaveLen(1))
				for _, container := range podv1.Spec.Containers {
					Expect(container.Ports).To(HaveLen(2))
					for idx, port := range container.Ports {
						Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
						Expect(port.HostPort).To(BeEquivalentTo(ports[idx].HostPort))
						Expect(string(port.Protocol)).To(Equal(ports[idx].Protocol))
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
						Expect(envVar.Value).To(ContainSubstring(env[idx].Value))
					}
					Expect(container.Command).To(HaveLen(3))
					Expect(container.Command).To(Equal(command))
				}
				svc, err := mockClientset.CoreV1().Services(namespace).Get(podv1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.ObjectMeta.Name).To(Equal(podv1.Name))
				Expect(svc.Spec.Selector["app"]).To(Equal(podv1.Name))
				Expect(svc.Spec.Type).To(Equal(v1.ServiceTypeNodePort))

			})
			It("Should return room creation error if redis returns an error", func() {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().Exec().Return(nil, errors.New("")).AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()

				_, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					mockClientset,
					configYaml,
					scheduler,
				)
				Expect(err).To(HaveOccurred())

				svcs, err := mockClientset.CoreV1().Services(namespace).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(svcs.Items)).To(Equal(0))
			})
			It("Should return error and remove created pod if service fails to create", func() {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().SCard(models.GetRoomStatusSetRedisKey(namespace, models.StatusCreating)).Return(goredis.NewIntResult(int64(2), nil))
				mockPipeline.EXPECT().Exec().AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()
				mr.EXPECT().Report("gru.status", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
					"status":                        models.StatusCreating,
					"gauge":                         "2",
				})
				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
				})
				mr.EXPECT().Report("gru.delete", map[string]string{
					reportersConstants.TagGame:      "pong",
					reportersConstants.TagScheduler: "pong-free-for-all",
					reportersConstants.TagReason:    "failed_to_create_service_for_pod",
				})

				clientset := &fake.Clientset{}
				clientset.Fake.AddReactor("create", "services", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("Failed to create service for pod")
				})
				_, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					clientset,
					configYaml,
					scheduler,
				)
				Expect(err).To(HaveOccurred())

				svcs, err := mockClientset.CoreV1().Services(namespace).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(svcs.Items)).To(Equal(0))

				pods, err := mockClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(0))
			})
		})

		Describe("Delete", func() {
			It("Should delete a pod", func() {
				mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(portRange, nil))
				mockPortChooser.EXPECT().Choose(portStart, portEnd, 2).Return([]int{5000, 5001})
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).AnyTimes()
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(namespace, "creating"), gomock.Any())
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(namespace), gomock.Any())
				mockPipeline.EXPECT().SCard(models.GetRoomStatusSetRedisKey(namespace, models.StatusCreating)).Return(goredis.NewIntResult(int64(2), nil))
				mockPipeline.EXPECT().Exec().AnyTimes()
				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()
				mr.EXPECT().Report("gru.status", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
					"status":                        models.StatusCreating,
					"gauge":                         "2",
				})
				mr.EXPECT().Report("gru.new", map[string]string{
					reportersConstants.TagGame:      game,
					reportersConstants.TagScheduler: namespace,
				})

				podv1, err := roomManager.Create(
					logger,
					mmr,
					mockRedisClient,
					mockDb,
					mockClientset,
					configYaml,
					scheduler,
				)
				Expect(err).NotTo(HaveOccurred())

				mr.EXPECT().Report("gru.delete", map[string]string{
					reportersConstants.TagGame:      "pong",
					reportersConstants.TagScheduler: "pong-free-for-all",
					reportersConstants.TagReason:    "deletion_reason",
				})
				err = roomManager.Delete(logger, mmr, mockClientset, mockRedisClient, configYaml, podv1.Name, "deletion_reason")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should error if pod does not exists", func() {
				err := roomManager.Delete(logger, mmr, mockClientset, mockRedisClient, configYaml, namespace, "deletion_reason")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("pods \"pong-free-for-all\" not found"))
			})
		})
	})
})
