// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	mtesting "github.com/topfreegames/maestro/testing"
	"gopkg.in/pg.v5/types"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Controller", func() {
	var timeoutSec int
	var jsonStr string

	BeforeEach(func() {
		var err error
		timeoutSec = 300
		jsonStr, err = mtesting.NextJsonStr()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreateScheduler", func() {
		It("should rollback if error updating scheduler state", func() {
			var configYaml1 models.ConfigYAML
			err := yaml.Unmarshal([]byte(jsonStr), &configYaml1)
			Expect(err).NotTo(HaveOccurred())

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
			mockDb.EXPECT().Query(gomock.Any(), "INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id", gomock.Any())
			mockDb.EXPECT().Query(
				gomock.Any(),
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				gomock.Any(),
			).Return(&types.Result{}, errors.New("error updating state"))
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", configYaml1.Name)

			err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml1, timeoutSec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating state"))

			Eventually(func() error {
				_, err := clientset.CoreV1().Namespaces().Get(configYaml1.Name, v1.GetOptions{})
				return err
			}).Should(HaveOccurred())
		})
	})
})
