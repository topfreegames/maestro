// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

import (
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/watcher"
	"github.com/topfreegames/maestro/worker"
	"gopkg.in/pg.v5/types"
)

var _ = Describe("Worker", func() {
	Describe("NewWorker", func() {
		It("should return configured new worker", func() {
			mockRedisClient.EXPECT().Ping()
			w, err := worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(w).NotTo(BeNil())
			Expect(w.Config).To(Equal(config))
			Expect(w.DB).To(Equal(mockDb))
			Expect(w.InCluster).To(BeFalse())
			Expect(w.KubeconfigPath).To(HaveLen(0))
			Expect(w.KubernetesClient).To(Equal(clientset))
			Expect(w.Logger).NotTo(BeNil())
			Expect(w.MetricsReporter).NotTo(BeNil())
			Expect(w.RedisClient).NotTo(BeNil())
			Expect(w.RedisClient.Client).To(Equal(mockRedisClient))
			Expect(w.Run).To(BeFalse())
			Expect(w.SyncPeriod).NotTo(Equal(0))
			Expect(w.Watchers).To(HaveLen(0))
		})
	})

	Describe("Start", func() {
		var w *worker.Worker

		BeforeEach(func() {
			config.Set("worker.syncPeriod", 1)
			var err error
			mockRedisClient.EXPECT().Ping()
			w, err = worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(w.Watchers).To(HaveLen(0))
		})

		It("should run and list scheduler names", func() {
			// ListSchedulersNames
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Do(
				func(schedulers *[]models.Scheduler, query string) {
					w.Run = false
				},
			)
			w.Start()
		})

		It("should log error and not panic if failed to list scheduler names", func() {
			// ListSchedulersNames
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Do(
				func(schedulers *[]models.Scheduler, query string) {
					w.Run = false
				},
			).Return(&types.Result{}, errors.New("some error in pg"))
			w.Start()
			Expect(hook.LastEntry().Message).To(Equal("error listing schedulers"))
		})

	})

	Describe("EnsureRunningWatchers", func() {
		var w *worker.Worker

		BeforeEach(func() {
			var err error
			mockRedisClient.EXPECT().Ping()
			w, err = worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(w.Watchers).To(HaveLen(0))
		})

		It("should add watcher to watchers map and run it in a goroutine", func() {
			schedulerNames := []string{"scheduler-1"}
			w.EnsureRunningWatchers(schedulerNames)
			Expect(w.Watchers).To(HaveKey(schedulerNames[0]))
			Expect(w.Watchers[schedulerNames[0]].SchedulerName).To(Equal(schedulerNames[0]))
			Eventually(func() bool { return w.Watchers[schedulerNames[0]].Run }).Should(BeTrue())
		})

		It("should set watcher.Run to true", func() {
			schedulerNames := []string{"scheduler-1"}
			watcher1 := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, schedulerNames[0])
			w.Watchers[watcher1.SchedulerName] = watcher1
			Expect(w.Watchers[schedulerNames[0]].Run).To(BeFalse())

			w.EnsureRunningWatchers(schedulerNames)
			Expect(w.Watchers[schedulerNames[0]].Run).To(BeTrue())
		})
	})

	Describe("RemoveDeadWatchers", func() {
		var w *worker.Worker

		BeforeEach(func() {
			var err error
			mockRedisClient.EXPECT().Ping()
			w, err = worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(w.Watchers).To(HaveLen(0))
		})

		It("should remove watcher if it should not be running", func() {
			schedulerNames := []string{"scheduler-1", "scheduler-2"}
			watcher1 := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, schedulerNames[0])
			w.Watchers[watcher1.SchedulerName] = watcher1
			Expect(w.Watchers[schedulerNames[0]].Run).To(BeFalse())
			watcher2 := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, schedulerNames[1])
			watcher2.Run = true
			w.Watchers[watcher2.SchedulerName] = watcher2
			Expect(w.Watchers[schedulerNames[1]].Run).To(BeTrue())

			w.RemoveDeadWatchers()
			Expect(w.Watchers).NotTo(HaveKey(schedulerNames[0]))
			Expect(w.Watchers).To(HaveKey(schedulerNames[1]))
		})
	})
})
