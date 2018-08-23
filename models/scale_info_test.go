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
		upKey         = "maestro:scale:up:scheduler"
		downKey       = "maestro:scale:down:scheduler"
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
		currentUsageStr := fmt.Sprint(currentUsage)

		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().LPush(key, currentUsage)
		mockPipeline.EXPECT().LTrim(key, zero64, size64)
		mockPipeline.EXPECT().LRange(key, zero64, size64).Return(goredis.NewStringSliceResult(
			append([]string{currentUsageStr}, prevUsagesOnRedis...), nil,
		))
		mockPipeline.EXPECT().Exec()
	}

	It("should add point into redis and return false", func() {
		currUsage := UsageTest{5, 10}

		scaleInfo := NewScaleUpInfo(size, mockRedisClient)
		pushToList(size, currUsage.AbsUsage, currUsage.AbsTotal, nil, upKey)

		isAboveThreshold, err := scaleInfo.SendUsageAndReturnStatus(
			schedulerName,
			"legacy",
			size, currUsage.AbsUsage, currUsage.AbsTotal,
			threshold,
			usage,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(isAboveThreshold).To(BeFalse())
	})

	It("should add points into redis and return true", func() {
		currUsage := UsageTest{9, 10}
		size := 4
		scaleInfo := NewScaleDownInfo(size, mockRedisClient)

		pushToList(
			size, currUsage.AbsUsage, currUsage.AbsTotal,
			[]string{"0.5", "0.9", "0.9"}, downKey,
		)
		isAboveThreshold, err := scaleInfo.SendUsageAndReturnStatus(
			schedulerName,
			"legacy",
			size, currUsage.AbsUsage, currUsage.AbsTotal,
			threshold,
			usage,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(isAboveThreshold).To(BeTrue())
	})
})
