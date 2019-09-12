package models_test

import (
	"errors"
	"time"

	"github.com/topfreegames/extensions/pg"
	. "github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SchedulerCache", func() {
	var (
		schedulerName string = "scheduler-name"
		yamlStr       string = `
name: scheduler-name
game: game
image: image:v1
autoscaling:
  min: 100
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
      limit: 90
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
`
		configYaml         *ConfigYAML
		timeout, purgeTime time.Duration = 5 * time.Minute, 10 * time.Minute
		err                error
	)

	BeforeEach(func() {
		configYaml, err = NewConfigYAML(yamlStr)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("LoadScheduler", func() {
		It("should load from db for the first time", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				})

			cachedScheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(cachedScheduler.Scheduler).NotTo(BeNil())
			Expect(cachedScheduler.Scheduler.YAML).To(Equal(yamlStr))
			Expect(cachedScheduler.ConfigYAML).NotTo(BeNil())
		})

		It("should load from cache for the next 10 times", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				})

			cachedScheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(cachedScheduler.Scheduler).NotTo(BeNil())
			Expect(cachedScheduler.Scheduler.YAML).To(Equal(yamlStr))
			Expect(cachedScheduler.ConfigYAML).NotTo(BeNil())

			for i := 0; i < 10; i++ {
				cachedScheduler, err = cache.LoadScheduler(mockDb, schedulerName, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(cachedScheduler.Scheduler).NotTo(BeNil())
				Expect(cachedScheduler.Scheduler.YAML).To(Equal(yamlStr))
				Expect(cachedScheduler.ConfigYAML).NotTo(BeNil())
			}
		})

		It("should load from db is useCache is false", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				}).Times(2)

			cachedScheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(cachedScheduler.Scheduler).NotTo(BeNil())
			Expect(cachedScheduler.Scheduler.YAML).To(Equal(yamlStr))
			Expect(cachedScheduler.ConfigYAML).NotTo(BeNil())

			cachedScheduler, err = cache.LoadScheduler(mockDb, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(cachedScheduler.Scheduler).NotTo(BeNil())
			Expect(cachedScheduler.Scheduler.YAML).To(Equal(yamlStr))
			Expect(cachedScheduler.ConfigYAML).NotTo(BeNil())
		})

		It("should return error if scheduler is not found on cache nor on db", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, "")
				})

			cachedScheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler \"scheduler-name\" not found"))
			Expect(cachedScheduler).To(BeNil())
		})

		It("should return error if db fails", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			testing.MockLoadScheduler(schedulerName, mockDb).
				Return(pg.NewTestResult(errors.New("db failed"), 0), errors.New("db failed"))

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("db failed"))
			Expect(scheduler).To(BeNil())
		})

		It("should return error if yaml is invaild", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			testing.MockLoadScheduler(configYaml.Name, mockDb).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, "}i am invalid!{")
				})

			cachedScheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("yaml: did not find expected node content"))
			Expect(cachedScheduler).To(BeNil())
		})
	})
})
