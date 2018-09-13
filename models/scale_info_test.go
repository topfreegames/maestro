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
	. "github.com/topfreegames/maestro/models"
)

var _ = Describe("ScaleInfo", func() {

	var (
		schedulerName = "scheduler"
		size          = 10
		threshold     = 70
		usage         = float32(0.8)
		key           = "maestro:scale:legacy:scheduler"
		zero64        = int64(0)
	)

	type UsageTest struct {
		AbsUsage int
		AbsTotal int
	}

	pushToList := func(
		size, currentAbsoluteUsage, currentAbsoluteTotal int,
		prevUsagesOnRedis []string,
		key string,
	) {
		size64 := int64(size)
		currentUsage := float32(currentAbsoluteUsage) / float32(currentAbsoluteTotal)

		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().LPush(key, currentUsage)
		mockPipeline.EXPECT().LTrim(key, zero64, size64)
		mockPipeline.EXPECT().Exec()
	}

	returnList := func(
		size, currentAbsoluteUsage, currentAbsoluteTotal int,
		prevUsagesOnRedis []string,
		key string,
	) {
		size64 := int64(size)
		currentUsage := float32(currentAbsoluteUsage) / float32(currentAbsoluteTotal)

		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		currentUsageStr := fmt.Sprint(currentUsage)
		mockPipeline.EXPECT().LRange(key, zero64, size64).Return(goredis.NewStringSliceResult(
			append([]string{currentUsageStr}, prevUsagesOnRedis...), nil,
		))
		mockPipeline.EXPECT().Exec()
	}

	Describe("ReturnStatus", func() {
		It("should return false", func() {
			currUsage := UsageTest{5, 10}
			scaleInfo := NewScaleInfo(size, mockRedisClient)

			returnList(size, currUsage.AbsUsage, currUsage.AbsTotal, nil, key)

			isAboveThreshold, err := scaleInfo.ReturnStatus(
				schedulerName,
				LegacyAutoScalingPolicyType,
				ScaleTypeDown,
				size,
				currUsage.AbsTotal,
				threshold,
				usage,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(isAboveThreshold).To(BeFalse())
		})

		It("should return true", func() {
			currUsage := UsageTest{4, 10}
			size := 4
			scaleInfo := NewScaleInfo(size, mockRedisClient)

			returnList(
				size, currUsage.AbsUsage, currUsage.AbsTotal,
				[]string{"0.9", "0.3", "0.5"}, key,
			)

			isAboveThreshold, err := scaleInfo.ReturnStatus(
				schedulerName,
				LegacyAutoScalingPolicyType,
				ScaleTypeDown,
				size,
				currUsage.AbsTotal,
				threshold,
				usage,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(isAboveThreshold).To(BeTrue())
		})
	})

	Describe("SendUsage", func() {
		It("should add point into redis", func() {
			currUsage := UsageTest{5, 10}

			scaleInfo := NewScaleInfo(size, mockRedisClient)
			pushToList(size, currUsage.AbsUsage, currUsage.AbsTotal, nil, key)

			err := scaleInfo.SendUsage(
				schedulerName,
				LegacyAutoScalingPolicyType,
				float32(currUsage.AbsUsage)/float32(currUsage.AbsTotal),
			)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should add points into redis", func() {
			currUsage := UsageTest{9, 10}
			size := 4
			scaleInfo := NewScaleInfo(size, mockRedisClient)

			pushToList(
				size, currUsage.AbsUsage, currUsage.AbsTotal,
				[]string{"0.5", "0.9", "0.9"}, key,
			)
			err := scaleInfo.SendUsage(
				schedulerName,
				LegacyAutoScalingPolicyType,
				float32(currUsage.AbsUsage)/float32(currUsage.AbsTotal),
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Key", func() {
		It("should return the redis key from scheduler for legacy metrics", func() {
			scaleInfo := NewScaleInfo(4, mockRedisClient)
			key := scaleInfo.Key("scheduler-name", LegacyAutoScalingPolicyType)

			Expect(key).To(Equal("maestro:scale:legacy:scheduler-name"))
		})

		It("should return the redis key from scheduler for legacy metrics", func() {
			scaleInfo := NewScaleInfo(4, mockRedisClient)
			key := scaleInfo.Key("scheduler-name", "room")

			Expect(key).To(Equal("maestro:scale:room:scheduler-name"))
		})
	})

	Describe("Size", func() {
		It("should return scale info size", func() {
			size := 7
			scaleInfo := NewScaleInfo(size, mockRedisClient)

			Expect(scaleInfo.Size()).To(Equal(size))
		})
	})
})
