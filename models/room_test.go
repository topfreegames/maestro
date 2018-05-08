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
	"strconv"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/go-redis/redis"
	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Room", func() {
	var clientset *fake.Clientset
	schedulerName := uuid.NewV4().String()
	name := uuid.NewV4().String()
	configYaml := &models.ConfigYAML{Game: "game-name"}

	reportStatus := func(scheduler, status, roomKey, statusKey string) {
		mockRedisClient.EXPECT().HGetAll(roomKey).
			Return(redis.NewStringStringMapResult(map[string]string{
				"status": "diff",
			}, nil))

		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().SCard(statusKey).Return(redis.NewIntResult(int64(5), nil))
		mockPipeline.EXPECT().Exec()

		mr.EXPECT().Report("gru.status", map[string]string{
			reportersConstants.TagGame:      "game-name",
			reportersConstants.TagScheduler: scheduler,
			"status":                        status,
			"gauge":                         "5",
		})
	}

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("NewRoom", func() {
		It("should build correct room struct", func() {
			room := models.NewRoom(name, schedulerName)
			Expect(room.ID).To(Equal(name))
			Expect(room.SchedulerName).To(Equal(schedulerName))
			Expect(room.Status).To(Equal("creating"))
			Expect(room.LastPingAt).To(BeEquivalentTo(0))
		})
	})

	Describe("Get Room Redis Key", func() {
		It("should build the redis key using the scheduler name and the id", func() {
			room := models.NewRoom(name, schedulerName)
			key := room.GetRoomRedisKey()
			Expect(key).To(ContainSubstring(schedulerName))
			Expect(key).To(ContainSubstring(room.ID))
		})
	})

	Describe("Create Room", func() {
		It("should call redis successfully", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			sKey := models.GetRoomStatusSetRedisKey(schedulerName, room.Status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Exec()
			reportStatus(room.SchedulerName, room.Status, rKey, sKey)

			err := room.Create(mockRedisClient, mockDb, mmr, configYaml)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			sKey := models.GetRoomStatusSetRedisKey(schedulerName, room.Status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().HGetAll(rKey).
				Return(redis.NewStringStringMapResult(map[string]string{
					"status": "diff",
				}, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.Create(mockRedisClient, mockDb, mmr, configYaml)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("Set Status", func() {
		allStatus := []string{
			models.StatusCreating,
			models.StatusOccupied,
			models.StatusTerminating,
			models.StatusTerminated,
		}

		It("should call redis successfully", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusReady
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(status))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied), name)
			mockPipeline.EXPECT().SAdd(newSKey, rKey)

			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating),
			).Return(goredis.NewIntResult(0, nil))

			mockPipeline.EXPECT().Exec()
			reportStatus(room.SchedulerName, status, rKey, newSKey)

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, configYaml, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomsCountByStatus).NotTo(BeNil())
			Expect(roomsCountByStatus.Total()).To(Equal(5))
		})

		It("should remove from redis is status is 'terminated'", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusTerminated
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady), rKey)
			mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, models.StatusReady), name)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
				mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, st), name)
			}
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().Exec()

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, configYaml, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomsCountByStatus).To(BeNil())
		})

		It("should return an error if redis returns an error", func() {
			status := models.StatusReady
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().HGetAll(rKey).
				Return(redis.NewStringStringMapResult(map[string]string{
					"status": "diff",
				}, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(status))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied), name)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, configYaml, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
			Expect(roomsCountByStatus).To(BeNil())
		})

		It("should insert into zadd if status is occupied", func() {
			status := models.StatusOccupied
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(status))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Eval(models.ZaddIfNotExists, gomock.Any(), name)
			allStatus := []string{models.StatusCreating, models.StatusReady, models.StatusTerminating, models.StatusTerminated}
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()
			reportStatus(room.SchedulerName, status, rKey, newSKey)

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, configYaml, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(roomsCountByStatus.Total()).To(Equal(10))
		})
	})

	Describe("GetAddresses", func() {
		It("should not crash if pod does not exist", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			room := models.NewRoom(name, scheduler)
			_, err := room.GetAddresses(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`pods "pong-free-for-all-0" not found`))
		})

		It("should return no address if no node assigned to the room", func() {
			command := []string{
				"./room-binary",
				"-serverType",
				"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
			}
			env := []*models.EnvVar{
				{
					Name:  "EXAMPLE_ENV_VAR",
					Value: "examplevalue",
				},
				{
					Name:  "ANOTHER_ENV_VAR",
					Value: "anothervalue",
				},
			}
			game := "pong"
			image := "pong/pong:v123"
			name := "pong-free-for-all-0"
			namespace := "pong-free-for-all"
			ports := []*models.Port{
				{
					ContainerPort: 5050,
					HostPort:      5000,
				},
				{
					ContainerPort: 8888,
					HostPort:      5001,
				},
			}

			requests := &models.Resources{
				CPU:    "2",
				Memory: "128974848",
			}
			limits := &models.Resources{
				CPU:    "1",
				Memory: "64487424",
			}
			shutdownTimeout := 180

			configYaml := &models.ConfigYAML{
				Name:            namespace,
				Game:            game,
				Image:           image,
				Limits:          limits,
				Requests:        requests,
				ShutdownTimeout: shutdownTimeout,
				Ports:           ports,
				Cmd:             command,
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5001", nil))
			mockPipeline.EXPECT().Exec()

			mr.EXPECT().Report("gru.new", map[string]string{
				reportersConstants.TagGame:      "pong",
				reportersConstants.TagScheduler: "pong-free-for-all",
			})
			pod, err := models.NewPod(name, env, configYaml, mockClientset, mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			room := models.NewRoom(name, namespace)
			addresses, err := room.GetAddresses(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(addresses.Ports)).To(Equal(0))
		})

		It("should return room address", func() {
			var err error
			name := "pong-free-for-all-0"
			schedulerName := "pong-free-for-all"
			nodeName := "node-name"
			room := models.NewRoom(name, schedulerName)
			ip := "192.168.10.11"
			var port int32 = 1234

			node := &v1.Node{}
			node.SetName(nodeName)
			node.Status.Addresses = []v1.NodeAddress{
				v1.NodeAddress{
					Type:    v1.NodeExternalIP,
					Address: ip,
				},
			}
			_, err = clientset.CoreV1().Nodes().Create(node)
			Expect(err).NotTo(HaveOccurred())

			pod := &v1.Pod{}
			pod.Spec.NodeName = nodeName
			pod.SetName(name)
			pod.Spec.Containers = []v1.Container{
				{Ports: []v1.ContainerPort{
					{HostPort: port, Name: "TCP"},
				}},
			}
			_, err = clientset.CoreV1().Pods(schedulerName).Create(pod)
			Expect(err).NotTo(HaveOccurred())

			roomAddresses, err := room.GetAddresses(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomAddresses.Host).To(Equal(ip))
			Expect(roomAddresses.Ports).To(HaveLen(1))
			Expect(roomAddresses.Ports[0]).To(Equal(
				&models.RoomPort{
					Name: "TCP",
					Port: port,
				}))
		})

		It("should return error if there is no node", func() {
			name := "pong-free-for-all-0"
			schedulerName := "pong-free-for-all"
			nodeName := "node-name"
			room := models.NewRoom(name, schedulerName)

			pod := &v1.Pod{}
			pod.SetName(name)
			pod.Spec.NodeName = nodeName
			_, err := clientset.CoreV1().Pods(schedulerName).Create(pod)
			Expect(err).NotTo(HaveOccurred())

			_, err = room.GetAddresses(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("nodes \"node-name\" not found"))
		})
	})

	Describe("ClearAll", func() {
		allStatus := []string{
			models.StatusCreating,
			models.StatusReady,
			models.StatusOccupied,
			models.StatusTerminating,
			models.StatusTerminated,
		}

		It("should call redis successfully", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			room := models.NewRoom(name, scheduler)
			rKey := room.GetRoomRedisKey()
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, st), rKey)
				mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(room.SchedulerName, st), room.ID)
			}
			mockPipeline.EXPECT().Exec()

			err := room.ClearAll(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			room := models.NewRoom(name, scheduler)
			rKey := room.GetRoomRedisKey()
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, st), rKey)
				mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(room.SchedulerName, st), room.ID)
			}
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.ClearAll(mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("GetRoomsNoPingSince", func() {
		It("should call redis successfully", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			expectedRooms := []string{"room1", "room2", "room3"}
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult(expectedRooms, nil))
			rooms, err := models.GetRoomsNoPingSince(mockRedisClient, scheduler, since)
			Expect(err).NotTo(HaveOccurred())
			Expect(rooms).To(Equal(expectedRooms))
		})

		It("should return an error if redis returns an error", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult([]string{}, errors.New("some error")))
			_, err := models.GetRoomsNoPingSince(mockRedisClient, scheduler, since)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error"))
		})
	})
})
