// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"time"

	goredis "github.com/go-redis/redis"
	mtesting "github.com/topfreegames/maestro/testing"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
)

var _ = Describe("Controller", func() {
	var timeoutSec int
	var configYaml1 models.ConfigYAML
	var jsonStr string
	var roomManager models.RoomManager

	BeforeEach(func() {
		var err error
		timeoutSec = 300
		jsonStr, err = mtesting.NextJsonStr()
		Expect(err).NotTo(HaveOccurred())

		err = yaml.Unmarshal([]byte(jsonStr), &configYaml1)
		Expect(err).NotTo(HaveOccurred())
		node := &v1.Node{}
		node.SetName(configYaml1.Name)
		node.SetLabels(map[string]string{
			"game": "game-name",
		})

		_, err = clientset.CoreV1().Nodes().Create(node)
		Expect(err).NotTo(HaveOccurred())

		roomManager = &models.GameRoom{}
	})

	Describe("CreateScheduler", func() {
		It("should rollback if error updating scheduler state", func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
				},
			).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).Times(configYaml1.AutoScaling.Min)
			mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

			mtesting.MockInsertScheduler(mockDb, nil)
			mtesting.MockUpdateSchedulerStatus(mockDb, errors.New("error updating state"), nil)
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			mockRedisClient.EXPECT().
				Get(models.GlobalPortsPoolKey).Return(goredis.NewStringResult("40000-60000", nil)).Times(2)

			err := controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating status on schedulers: error updating state"))

			Eventually(func() error {
				_, err := clientset.CoreV1().Namespaces().Get(configYaml1.Name, metav1.GetOptions{})
				return err
			}).Should(HaveOccurred())
		})
	})
})
