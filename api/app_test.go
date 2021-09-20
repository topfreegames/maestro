// maestro
// +build unit
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/models"
)

var _ = Describe("App", func() {
	Describe("NewApp", func() {
		It("should return new app", func() {
			application, err := api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset, mockSchedulerEventStorage)
			Expect(err).NotTo(HaveOccurred())
			Expect(application).NotTo(BeNil())
			Expect(application.Address).NotTo(Equal(""))
			Expect(application.Config).To(Equal(config))
			Expect(application.DBClient.DB).To(Equal(mockDb))
			Expect(application.DBClient.CtxWrapper).To(Equal(mockCtxWrapper))
			Expect(application.KubernetesClient).To(Equal(clientset))
			Expect(application.Logger).NotTo(BeNil())
			Expect(application.NewRelic).NotTo(BeNil())
			Expect(application.Router).NotTo(BeNil())
			Expect(application.Server).NotTo(BeNil())
			Expect(application.InCluster).To(BeFalse())
			Expect(application.KubeconfigPath).To(HaveLen(0))
			Expect(application.RoomAddrGetter).To(BeAssignableToTypeOf(&models.RoomAddressesFromHostPort{}))

			// should use development environment
			config.Set(api.EnvironmentConfig, api.DevEnvironment)
			application, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset, mockSchedulerEventStorage)
			Expect(err).NotTo(HaveOccurred())
			Expect(application.RoomAddrGetter).To(BeAssignableToTypeOf(&models.RoomAddressesFromNodePort{}))
		})

		It("should fail if some error occurred", func() {
			config.Set("newrelic.key", 12345)
			application, err := api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset, mockSchedulerEventStorage)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("license length is not 40"))
			Expect(application).To(BeNil())
		})

		It("should not fail if no newrelic key is provided", func() {
			config.Set("newrelic.key", "")
			application, err := api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset, mockSchedulerEventStorage)
			Expect(err).NotTo(HaveOccurred())
			Expect(application).NotTo(BeNil())
		})
	})
})
