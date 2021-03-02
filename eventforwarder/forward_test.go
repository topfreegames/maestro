package eventforwarder_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	mt "github.com/topfreegames/maestro/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Forward", func() {
	Describe("ForwardRoomEvent", func() {
		It("should forward room event", func() {
			ctx := context.Background()
			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusReady,
				map[string]interface{}{
					"host":   nodeAddress,
					"port":   hostPort,
					"roomId": roomName,
					"game":   gameName,
					"metadata": map[string]interface{}{
						"ipv6Label": ipv6Label,
						"ports":     fmt.Sprintf(`[{"name":"port","port":%d,"protocol":""}]`, hostPort),
					},
				},
				metadata,
			).Return(int32(200), "success", nil)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RouteRoomEvent,
				reportersConstants.TagStatus:    "success",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventNodeIpv6Status, map[string]interface{}{
				reportersConstants.TagNodeHost: nodeAddress,
				reportersConstants.TagStatus:   "success",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			response, err := ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				"",
				nil,
				cache,
				logger,
				roomAddrGetter,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Code).To(Equal(200))
			Expect(response.Message).To(Equal("success"))
		})

		It("should forward pingTimeout and occupiedTimeout room events", func() {
			ctx := context.Background()
			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusReady,
				map[string]interface{}{
					"host":     "",
					"port":     int32(0),
					"roomId":   roomName,
					"game":     gameName,
					"metadata": map[string]interface{}{},
				},
				metadata,
			).Return(int32(200), "success", nil)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RouteRoomEvent,
				reportersConstants.TagStatus:    "success",
			}).Times(2)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any()).Times(2)

			response, err := ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				PingTimeoutEvent,
				nil,
				cache,
				logger,
				roomAddrGetter,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Code).To(Equal(200))
			Expect(response.Message).To(Equal("success"))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RouteRoomEvent,
				reportersConstants.TagStatus:    "success",
			})

			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusTerminated,
				map[string]interface{}{
					"host":     "",
					"port":     int32(0),
					"roomId":   roomName,
					"game":     gameName,
					"metadata": map[string]interface{}{},
				},
				metadata,
			).Return(int32(200), "success", nil)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			response, err = ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusTerminated,
				OccupiedTimeoutEvent,
				nil,
				cache,
				logger,
				roomAddrGetter,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Code).To(Equal(200))
			Expect(response.Message).To(Equal("success"))
		})

		It("should report fail if event forward fails", func() {
			errMsg := "event forward failed"
			ctx := context.Background()
			noIpv6roomAddrGetter := models.NewRoomAddressesFromHostPort(logger, "", false, 0)
			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusReady,
				map[string]interface{}{
					"host":   nodeAddress,
					"port":   hostPort,
					"roomId": roomName,
					"game":   gameName,
					"metadata": map[string]interface{}{
						"ipv6Label": "",
						"ports":     fmt.Sprintf(`[{"name":"port","port":%d,"protocol":""}]`, hostPort),
					},
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RouteRoomEvent,
				reportersConstants.TagStatus:    "failed",
				reportersConstants.TagReason:    errMsg,
			})

			mockReporter.EXPECT().Report(reportersConstants.EventNodeIpv6Status, map[string]interface{}{
				reportersConstants.TagNodeHost: nodeAddress,
				reportersConstants.TagStatus:   "failed",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			response, err := ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				"",
				nil,
				cache,
				logger,
				noIpv6roomAddrGetter,
			)

			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("should not send reporter if reporter is not set", func() {
			errMsg := "event forward failed"
			ctx := context.Background()
			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusReady,
				map[string]interface{}{
					"host":   nodeAddress,
					"port":   hostPort,
					"roomId": roomName,
					"game":   gameName,
					"metadata": map[string]interface{}{
						"ipv6Label": ipv6Label,
						"ports":     fmt.Sprintf(`[{"name":"port","port":%d,"protocol":""}]`, hostPort),
					},
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			r := reporters.GetInstance()
			r.UnsetReporter("mockReporter")

			_, err := ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				"",
				nil,
				cache,
				logger,
				roomAddrGetter,
			)

			Expect(err.Error()).To(Equal(errMsg))
		})

		It("should forward room event but not return its response", func() {
			yaml := `name: scheduler
game: game
forwarders:
  mockplugin:
    mockfwd:
      enabled: true
      forwardResponse: false
`
			mt.MockLoadScheduler(schedulerName, mockDB).
				Do(func(scheduler *models.Scheduler, _ string, _ string) {
					*scheduler = *models.NewScheduler(schedulerName, gameName, yaml)
				})
			_, err := cache.LoadScheduler(mockDB, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())

			ctx := context.Background()
			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusReady,
				map[string]interface{}{
					"host":   nodeAddress,
					"port":   hostPort,
					"roomId": roomName,
					"game":   gameName,
					"metadata": map[string]interface{}{
						"ipv6Label": ipv6Label,
						"ports":     fmt.Sprintf(`[{"name":"port","port":%d,"protocol":""}]`, hostPort),
					},
				},
				metadata,
			).Return(int32(200), "success", nil)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RouteRoomEvent,
				reportersConstants.TagStatus:    "success",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventNodeIpv6Status, map[string]interface{}{
				reportersConstants.TagNodeHost: nodeAddress,
				reportersConstants.TagStatus:   "success",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			response, err := ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				"",
				nil,
				cache,
				logger,
				roomAddrGetter,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("should return as success but report error if forward fails", func() {
			yaml := `name: scheduler
game: game
forwarders:
  mockplugin:
    mockfwd:
      enabled: true
      forwardResponse: false
`
			mt.MockLoadScheduler(schedulerName, mockDB).
				Do(func(scheduler *models.Scheduler, _ string, _ string) {
					*scheduler = *models.NewScheduler(schedulerName, gameName, yaml)
				})
			_, err := cache.LoadScheduler(mockDB, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())

			errMsg := "event forward failed"
			ctx := context.Background()
			noIpv6roomAddrGetter := models.NewRoomAddressesFromHostPort(logger, "", false, 0)
			mockEventForwarder.EXPECT().Forward(
				ctx,
				models.StatusReady,
				map[string]interface{}{
					"host":   nodeAddress,
					"port":   hostPort,
					"roomId": roomName,
					"game":   gameName,
					"metadata": map[string]interface{}{
						"ipv6Label": "",
						"ports":     fmt.Sprintf(`[{"name":"port","port":%d,"protocol":""}]`, hostPort),
					},
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RouteRoomEvent,
				reportersConstants.TagStatus:    "failed",
				reportersConstants.TagReason:    errMsg,
			})

			mockReporter.EXPECT().Report(reportersConstants.EventNodeIpv6Status, map[string]interface{}{
				reportersConstants.TagNodeHost: nodeAddress,
				reportersConstants.TagStatus:   "failed",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			response, err := ForwardRoomEvent(
				ctx,
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				"",
				nil,
				cache,
				logger,
				noIpv6roomAddrGetter,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("should not report if scheduler has no forwarders", func() {
			yaml := `name: scheduler
game: game
`
			mt.MockLoadScheduler(schedulerName, mockDB).
				Do(func(scheduler *models.Scheduler, _ string, _ string) {
					*scheduler = *models.NewScheduler(schedulerName, gameName, yaml)
				})
			_, err := cache.LoadScheduler(mockDB, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())

			response, err := ForwardRoomEvent(
				context.Background(),
				mockForwarders,
				mockRedisClient,
				mockDB,
				clientset,
				mmr,
				room,
				models.StatusReady,
				"",
				nil,
				cache,
				logger,
				roomAddrGetter,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(BeNil())
		})
	})

	Describe("ForwardPlayerEvent", func() {
		It("should forward player event", func() {
			playerEvent := "player-event"
			ctx := context.Background()
			mockEventForwarder.EXPECT().Forward(
				ctx,
				playerEvent,
				map[string]interface{}{
					"roomId": roomName,
					"game":   gameName,
				},
				metadata,
			)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RoutePlayerEvent,
				reportersConstants.TagStatus:    "success",
			})

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			_, err := ForwardPlayerEvent(
				ctx,
				mockForwarders,
				mockDB,
				clientset,
				room,
				playerEvent,
				make(map[string]interface{}),
				cache,
				logger,
			)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should report fail if event forward fails", func() {
			errMsg := "event forward failed"
			playerEvent := "player-event"
			ctx := context.Background()
			mockEventForwarder.EXPECT().Forward(
				ctx,
				playerEvent,
				map[string]interface{}{
					"roomId": roomName,
					"game":   gameName,
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]interface{}{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagHostname:  Hostname(),
				reportersConstants.TagRoute:     RoutePlayerEvent,
				reportersConstants.TagStatus:    "failed",
				reportersConstants.TagReason:    errMsg,
			})

			mockReporter.EXPECT().Report(reportersConstants.EventRPCDuration, gomock.Any())

			_, err := ForwardPlayerEvent(
				ctx,
				mockForwarders,
				mockDB,
				clientset,
				room,
				playerEvent,
				make(map[string]interface{}),
				cache,
				logger,
			)

			Expect(err.Error()).To(Equal(errMsg))
		})
	})
})
