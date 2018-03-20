// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package models_test

import (
	"errors"
	"net/http"
	"time"

	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/topfreegames/maestro/models"
)

var _ = Describe("OperationManager", func() {
	var opManager *OperationManager
	var schedulerName = "scheduler-name"
	var opName = "SomeOperation"
	var timeout = 10 * time.Minute
	var errDB = errors.New("db failed")
	var initialStatus = map[string]interface{}{
		"operation":   opName,
		"description": "waiting for lock",
	}

	toMapStringString := func(m map[string]interface{}) map[string]string {
		n := map[string]string{}
		for key, value := range m {
			n[key] = value.(string)
		}
		return n
	}

	mockStartOnRedis := func(m map[string]interface{}, err error) {
		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().HMSet(opManager.GetOperationKey(), gomock.Any()).
			Do(func(_ string, n map[string]interface{}) {
				Expect(n).To(Equal(m))
			})
		mockPipeline.EXPECT().Expire(opManager.GetOperationKey(), timeout)
		mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeout)
		mockPipeline.EXPECT().Exec().Return(nil, err)
	}

	mockFinishOnRedis := func(m map[string]interface{}, err error) {
		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().HMSet(opManager.GetOperationKey(), gomock.Any()).
			Do(func(_ string, n map[string]interface{}) {
				Expect(n).To(Equal(m))
			})
		mockPipeline.EXPECT().Expire(opManager.GetOperationKey(), timeout)
		mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
		mockPipeline.EXPECT().Exec().Return(nil, err)
	}

	mockGetStatusFromRedis := func(m map[string]string, err error) {
		mockRedisClient.EXPECT().
			HGetAll(opManager.GetOperationKey()).
			Return(goredis.NewStringStringMapResult(m, err))
	}

	mockDeleteStatusFromRedis := func(err error) {
		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().Del(opManager.GetOperationKey())
		mockPipeline.EXPECT().Exec().Return(nil, err)
	}

	BeforeEach(func() {
		opManager = NewOperationManager(schedulerName, mockRedisClient, logger)
		opManager.SetLoopTime(1 * time.Millisecond)
	})

	Describe("GetOperationKey", func() {
		It("should return operation key", func() {
			Expect(opManager.GetOperationKey()).To(ContainSubstring("opmanager:scheduler-name:"))
		})
	})

	Describe("Start", func() {
		It("should save on redis and start goroutine", func() {
			mockStartOnRedis(initialStatus, nil)
			mockGetStatusFromRedis(toMapStringString(initialStatus), nil)
			mockGetStatusFromRedis(nil, nil)

			err := opManager.Start(timeout, opName)
			Expect(err).ToNot(HaveOccurred())

			Eventually(opManager.IsStopped).Should(BeTrue())
		})

		It("should not return error if goroutine gets error from redis", func() {
			mockStartOnRedis(initialStatus, nil)
			mockGetStatusFromRedis(toMapStringString(initialStatus), errDB)
			mockGetStatusFromRedis(nil, nil)

			err := opManager.Start(timeout, opName)
			Expect(err).ToNot(HaveOccurred())

			Eventually(opManager.IsStopped).Should(BeTrue())
		})

		It("should return error if failed to save initial status on redis", func() {
			mockStartOnRedis(initialStatus, errDB)

			err := opManager.Start(timeout, opName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errDB.Error()))
		})
	})

	Describe("WasCanceled", func() {
		It("should return false if opManager was canceled", func() {
			Expect(opManager.WasCanceled()).To(BeFalse())
		})

		It("should return false if opManager is nil", func() {
			var opManager *OperationManager
			Expect(opManager.WasCanceled()).To(BeFalse())
		})
	})

	Describe("Cancel", func() {
		It("should delete from hash redis", func() {
			mockDeleteStatusFromRedis(nil)

			err := opManager.Cancel(opManager.GetOperationKey())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error if operation key is invalid", func() {
			err := opManager.Cancel("invalid-key")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("operationKey is not valid: invalid-key"))
		})

		It("should return error if redis failed", func() {
			mockDeleteStatusFromRedis(errDB)

			err := opManager.Cancel(opManager.GetOperationKey())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errDB.Error()))
		})
	})

	Describe("Finish", func() {
		var status = http.StatusOK
		var description = "finished"

		It("should save result on redis when not error", func() {
			mockFinishOnRedis(map[string]interface{}{
				"success":     true,
				"status":      status,
				"operation":   "",
				"progress":    "100%",
				"description": description,
			}, nil)

			err := opManager.Finish(status, description, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should save result on redis when error", func() {
			mockFinishOnRedis(map[string]interface{}{
				"success":     false,
				"status":      status,
				"operation":   "",
				"description": description,
				"error":       errDB.Error(),
			}, nil)

			err := opManager.Finish(status, description, errDB)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return no error if error from redis is redis.Nil", func() {
			mockFinishOnRedis(map[string]interface{}{
				"success":     true,
				"status":      status,
				"operation":   "",
				"progress":    "100%",
				"description": description,
			}, goredis.Nil)

			err := opManager.Finish(status, description, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error if error from redis is not redis.Nil", func() {
			mockFinishOnRedis(map[string]interface{}{
				"success":     true,
				"status":      status,
				"operation":   "",
				"progress":    "100%",
				"description": description,
			}, errDB)

			err := opManager.Finish(status, description, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errDB.Error()))
		})
	})

	Describe("Get", func() {
		It("should return status from redis", func() {
			mockGetStatusFromRedis(toMapStringString(initialStatus), nil)
			m, err := opManager.Get(opManager.GetOperationKey())
			Expect(err).ToNot(HaveOccurred())

			for key, value := range m {
				Expect(initialStatus[key]).To(BeEquivalentTo(value))
			}
		})

		It("should return error if operationKey is invalid", func() {
			_, err := opManager.Get("invalid-key")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("operationKey is not valid: invalid-key"))
		})

		It("should return nil when redis returns goredis.Nil", func() {
			mockGetStatusFromRedis(toMapStringString(initialStatus), goredis.Nil)
			m, err := opManager.Get(opManager.GetOperationKey())
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(BeNil())
		})
	})
})
