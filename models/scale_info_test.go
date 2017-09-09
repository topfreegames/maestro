// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
)

var _ = Describe("ScaleInfo", func() {
	It("should add point at position 0", func() {
		var cap, threshold int = 4, 50
		var usage float32 = 0.5
		scaleInfo := models.NewScaleInfo(cap)
		scaleInfo.AddPoint(6, 10, usage)
		points := scaleInfo.GetPoints()
		Expect(points).To(HaveCap(cap))
		Expect(points[0]).To(BeNumerically("~", 0.6, 1e-6))
		Expect(scaleInfo.IsAboveThreshold(threshold)).To(BeTrue())
	})

	It("should add point at position 1", func() {
		var cap, threshold int = 4, 50
		var usage float32 = 0.5
		scaleInfo := models.NewScaleInfo(cap)
		scaleInfo.AddPoint(6, 10, usage)
		scaleInfo.AddPoint(4, 10, usage)
		points := scaleInfo.GetPoints()
		Expect(points).To(HaveCap(cap))
		Expect(points[0]).To(BeNumerically("~", 0.6, 1e-6))
		Expect(points[1]).To(BeNumerically("~", 0.4, 1e-6))
		Expect(scaleInfo.IsAboveThreshold(threshold)).To(BeFalse())
	})

	It("should add point at position 2", func() {
		var cap, threshold int = 4, 50
		var usage float32 = 0.5
		scaleInfo := models.NewScaleInfo(cap)
		scaleInfo.AddPoint(6, 10, usage)
		scaleInfo.AddPoint(4, 10, usage)
		scaleInfo.AddPoint(3, 10, usage)
		points := scaleInfo.GetPoints()
		Expect(points).To(HaveCap(cap))
		Expect(points[0]).To(BeNumerically("~", 0.6, 1e-6))
		Expect(points[1]).To(BeNumerically("~", 0.4, 1e-6))
		Expect(points[2]).To(BeNumerically("~", 0.3, 1e-6))
		Expect(scaleInfo.IsAboveThreshold(threshold)).To(BeFalse())
	})

	It("should ovewrite at position 0", func() {
		var cap, threshold int = 2, 50
		var usage float32 = 0.5
		scaleInfo := models.NewScaleInfo(cap)
		scaleInfo.AddPoint(6, 10, usage)
		scaleInfo.AddPoint(4, 10, usage)
		scaleInfo.AddPoint(3, 10, usage)
		points := scaleInfo.GetPoints()
		Expect(points).To(HaveCap(cap))
		Expect(points[0]).To(BeNumerically("~", 0.3, 1e-6))
		Expect(points[1]).To(BeNumerically("~", 0.4, 1e-6))
		Expect(scaleInfo.IsAboveThreshold(threshold)).To(BeFalse())
	})

	It("should ovewrite at position 0", func() {
		var cap, threshold int = 2, 50
		var usage float32 = 0.5
		scaleInfo := models.NewScaleInfo(cap)
		scaleInfo.AddPoint(6, 10, usage)
		scaleInfo.AddPoint(8, 10, usage)
		scaleInfo.AddPoint(7, 10, usage)
		points := scaleInfo.GetPoints()
		Expect(points).To(HaveCap(cap))
		Expect(points[0]).To(BeNumerically("~", 0.7, 1e-6))
		Expect(points[1]).To(BeNumerically("~", 0.8, 1e-6))
		Expect(scaleInfo.IsAboveThreshold(threshold)).To(BeTrue())
	})
})
