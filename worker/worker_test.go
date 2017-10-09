// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

import (
	"errors"
	"time"

	"k8s.io/client-go/pkg/api/v1"

	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	mtesting "github.com/topfreegames/maestro/testing"
	"github.com/topfreegames/maestro/watcher"
	"github.com/topfreegames/maestro/worker"
	"gopkg.in/pg.v5/types"
)

var _ = Describe("Worker", func() {
	var occupiedTimeout int64 = 300
	var startPortRange, endPortRange = 40000, 50000
	yaml1 := `
name: controller-name
game: controller
image: controller/controller:v123
occupiedTimeout: 300
ports:
  - containerPort: 1234
    protocol: UDP
    name: port1
  - containerPort: 7654
    protocol: TCP
    name: port2
limits:
  memory: "66Mi"
  cpu: "2"
shutdownTimeout: 20
autoscaling:
  min: 3
  up:
    delta: 2
    trigger:
      usage: 60
      time: 100
    cooldown: 200
  down:
    delta: 1
    trigger:
      usage: 30
      time: 500
    cooldown: 500
env:
  - name: MY_ENV_VAR
    value: myvalue
cmd:
  - "./room"
`
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
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), gomock.Any(), startPortRange, endPortRange).
				Return(goredis.NewCmdResult(nil, nil))

			w.Start(startPortRange, endPortRange, false)
		})

		It("should log error and not panic if failed to list scheduler names", func() {
			// ListSchedulersNames
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Do(
				func(schedulers *[]models.Scheduler, query string) {
					w.Run = false
				},
			).Return(&types.Result{}, errors.New("some error in pg"))

			mockRedisClient.EXPECT().
				Eval(gomock.Any(), gomock.Any(), startPortRange, endPortRange).
				Return(goredis.NewCmdResult(nil, nil))

			err := w.Start(startPortRange, endPortRange, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
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
			mockDb.EXPECT().Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", schedulerNames[0]).Do(
				func(scheduler *models.Scheduler, query, name string) {
					scheduler.YAML = yaml1
				},
			)
			for _, name := range schedulerNames {
				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yaml1
					})
			}
			w.EnsureRunningWatchers(schedulerNames)
			Expect(w.Watchers).To(HaveKey(schedulerNames[0]))
			Expect(w.Watchers[schedulerNames[0]].SchedulerName).To(Equal(schedulerNames[0]))
			Eventually(func() bool { return w.Watchers[schedulerNames[0]].Run }).Should(BeTrue())
		})

		It("should set watcher.Run to true", func() {
			schedulerNames := []string{"scheduler-1"}
			for _, name := range schedulerNames {
				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yaml1
					})
			}
			watcher1 := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, schedulerNames[0], "", occupiedTimeout, []*eventforwarder.Info{})
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
			for _, name := range schedulerNames {
				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yaml1
					})
			}
			watcher1 := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, schedulerNames[0], "", occupiedTimeout, []*eventforwarder.Info{})
			w.Watchers[watcher1.SchedulerName] = watcher1
			Expect(w.Watchers[schedulerNames[0]].Run).To(BeFalse())
			watcher2 := watcher.NewWatcher(config, logger, mr, mockDb, redisClient, clientset, schedulerNames[1], "", occupiedTimeout, []*eventforwarder.Info{})
			watcher2.Run = true
			w.Watchers[watcher2.SchedulerName] = watcher2
			Expect(w.Watchers[schedulerNames[1]].Run).To(BeTrue())

			w.RemoveDeadWatchers()
			Expect(w.Watchers).NotTo(HaveKey(schedulerNames[0]))
			Expect(w.Watchers).To(HaveKey(schedulerNames[1]))
		})
	})

	Describe("RetrieveFreePorts", func() {
		var w *worker.Worker

		BeforeEach(func() {
			var err error
			mockRedisClient.EXPECT().Ping()
			w, err = worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove two ports from set if there are two rooms with one hostPort each", func() {
			var start, end int = 5000, 5010
			redisKey := "maestro:updated:free:ports"
			schedulerNames := []string{"room-0"}
			pods := []struct {
				Name string
				Port int32
			}{
				{"room-0", int32(start)},
				{"room-1", int32(start + 1)},
			}

			mockRedisClient.EXPECT().
				SetNX(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(goredis.NewBoolResult(true, nil))

			for _, pod := range pods {
				kubePod := &v1.Pod{}
				kubePod.SetName(pod.Name)
				kubePod.Spec.Containers = []v1.Container{
					{
						Ports: []v1.ContainerPort{
							{HostPort: pod.Port, Name: "TCP"},
						},
					},
				}
				_, err := clientset.CoreV1().Pods(schedulerNames[0]).Create(kubePod)
				Expect(err).NotTo(HaveOccurred())
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			for i := start; i <= end; i++ {
				mockPipeline.EXPECT().SAdd(redisKey, i)
			}

			for _, pod := range pods {
				mockPipeline.EXPECT().SRem(redisKey, pod.Port)
			}

			mockPipeline.EXPECT().Rename(redisKey, models.FreePortsRedisKey())
			mockPipeline.EXPECT().Exec()
			mockRedisClient.EXPECT().
				Eval(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(goredis.NewCmdResult(nil, nil))

			err := w.RetrieveFreePorts(start, end, schedulerNames)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should sleep and retry if can't get lock", func() {
			var w *worker.Worker
			timeout := 1
			config, err := mtesting.GetDefaultConfig()
			Expect(err).NotTo(HaveOccurred())
			config.Set("worker.getLocksTimeout", timeout)
			config.Set("worker.lockTimeoutMS", 0)
			mockRedisClient.EXPECT().Ping()
			w, err = worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SetNX(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(goredis.NewBoolResult(false, nil))

			var start, end int = 5000, 5010
			schedulerNames := []string{"room-0"}

			startTime := time.Now()
			err = w.RetrieveFreePorts(start, end, schedulerNames)
			elapsedTime := time.Now().Sub(startTime)
			Expect(elapsedTime).To(BeNumerically(">=", time.Duration(timeout)*time.Second))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error getting locks, trying again next period..."))
			Expect(hook.Entries).To(mtesting.ContainLogMessage(
				"unable to get watcher room-0 lock, maybe some other process has it...",
			))
		})

		It("should sleep and retry if error getting lock", func() {
			var w *worker.Worker
			timeout := 1
			config, err := mtesting.GetDefaultConfig()
			Expect(err).NotTo(HaveOccurred())
			config.Set("worker.getLocksTimeout", timeout)
			config.Set("worker.lockTimeoutMS", 0)
			mockRedisClient.EXPECT().Ping()
			w, err = worker.NewWorker(config, logger, mr, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().
				SetNX(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(goredis.NewBoolResult(false, errors.New("error on redis")))

			var start, end int = 5000, 5010
			schedulerNames := []string{"room-0"}

			startTime := time.Now()
			err = w.RetrieveFreePorts(start, end, schedulerNames)
			elapsedTime := time.Now().Sub(startTime)
			Expect(elapsedTime).To(BeNumerically(">=", time.Duration(timeout)*time.Second))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error getting locks, trying again next period..."))
			Expect(hook.Entries).To(mtesting.ContainLogMessage(
				"error getting watcher room-0 lock",
			))
		})
	})
})
