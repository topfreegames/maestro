package eventforwarder_test

import (
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Forward", func() {
	Describe("ForwardRoomEvent", func() {
		It("should forward room event", func() {
			mockEventForwarder.EXPECT().Forward(
				models.StatusReady,
				map[string]interface{}{
					"host":     nodeAddress,
					"port":     hostPort,
					"roomId":   roomName,
					"game":     gameName,
					"metadata": metadata,
				},
				metadata,
			).Return(int32(200), "success", nil)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]string{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagStatus:    "success",
			})

			response, err := ForwardRoomEvent(
				mockForwarders,
				mockDB,
				clientset,
				room,
				models.StatusReady,
				nil,
				cache,
				logger,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.Code).To(Equal(200))
			Expect(response.Message).To(Equal("success"))
		})

		It("should report fail if event forward fails", func() {
			errMsg := "event forward failed"
			mockEventForwarder.EXPECT().Forward(
				models.StatusReady,
				map[string]interface{}{
					"host":     nodeAddress,
					"port":     hostPort,
					"roomId":   roomName,
					"game":     gameName,
					"metadata": metadata,
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]string{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagStatus:    "failed",
				reportersConstants.TagReason:    errMsg,
			})

			response, err := ForwardRoomEvent(
				mockForwarders,
				mockDB,
				clientset,
				room,
				models.StatusReady,
				nil,
				cache,
				logger,
			)

			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("should not send reporter if reporter is not set", func() {
			errMsg := "event forward failed"
			mockEventForwarder.EXPECT().Forward(
				models.StatusReady,
				map[string]interface{}{
					"host":     nodeAddress,
					"port":     hostPort,
					"roomId":   roomName,
					"game":     gameName,
					"metadata": metadata,
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			r := reporters.GetInstance()
			r.UnsetReporter("mockReporter")

			_, err := ForwardRoomEvent(
				mockForwarders,
				mockDB,
				clientset,
				room,
				models.StatusReady,
				nil,
				cache,
				logger,
			)

			Expect(err.Error()).To(Equal(errMsg))
		})

		It("should not report if scheduler has no forwarders", func() {
			yaml := `name: scheduler
game: game
`
			mockDB.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
				Do(func(scheduler *models.Scheduler, _ string, _ string) {
					*scheduler = *models.NewScheduler(schedulerName, gameName, yaml)
				})
			_, err := cache.LoadScheduler(mockDB, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())

			response, err := ForwardRoomEvent(
				mockForwarders,
				mockDB,
				clientset,
				room,
				models.StatusReady,
				nil,
				cache,
				logger,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(BeNil())
		})
	})

	Describe("ForwardPlayerEvent", func() {
		It("should forward player event", func() {
			playerEvent := "player-event"
			mockEventForwarder.EXPECT().Forward(
				playerEvent,
				map[string]interface{}{
					"roomId": roomName,
					"game":   gameName,
				},
				metadata,
			)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]string{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagStatus:    "success",
			})

			response, err := ForwardPlayerEvent(
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
			Expect(response).NotTo(BeNil())
		})

		It("should report fail if event forward fails", func() {
			errMsg := "event forward failed"
			playerEvent := "player-event"
			mockEventForwarder.EXPECT().Forward(
				playerEvent,
				map[string]interface{}{
					"roomId": roomName,
					"game":   gameName,
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]string{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagStatus:    "failed",
				reportersConstants.TagReason:    errMsg,
			})

			_, err := ForwardPlayerEvent(
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

	Describe("ForwardRoomInfo", func() {
		It("should forward rooms infos", func() {
			mockEventForwarder.EXPECT().Forward(
				"schedulerEvent",
				map[string]interface{}{
					"game": gameName,
				},
				metadata,
			)

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]string{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagStatus:    "success",
			})

			response, err := ForwardRoomInfo(
				mockForwarders,
				mockDB,
				clientset,
				schedulerName,
				cache,
				logger,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
		})

		It("should report fail if event forward fails", func() {
			errMsg := "event forward failed"
			mockEventForwarder.EXPECT().Forward(
				"schedulerEvent",
				map[string]interface{}{
					"game": gameName,
				},
				metadata,
			).Return(int32(0), "", errors.New(errMsg))

			mockReporter.EXPECT().Report(reportersConstants.EventRPCStatus, map[string]string{
				reportersConstants.TagGame:      gameName,
				reportersConstants.TagScheduler: schedulerName,
				reportersConstants.TagStatus:    "failed",
				reportersConstants.TagReason:    errMsg,
			})

			_, err := ForwardRoomInfo(
				mockForwarders,
				mockDB,
				clientset,
				schedulerName,
				cache,
				logger,
			)

			Expect(err.Error()).To(Equal(errMsg))
		})
	})
})
