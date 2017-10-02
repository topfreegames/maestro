package models_test

import (
	"errors"
	"time"

	"gopkg.in/pg.v5/types"

	"github.com/golang/mock/gomock"
	. "github.com/topfreegames/maestro/models"

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

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				})

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduler.YAML).To(Equal(yamlStr))
		})

		It("should load from cache for the next 10 times", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				})

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduler.YAML).To(Equal(yamlStr))

			for i := 0; i < 10; i++ {
				scheduler, err = cache.LoadScheduler(mockDb, schedulerName, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduler.YAML).To(Equal(yamlStr))
			}
		})

		It("should load from db is useCache is false", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				}).Times(2)

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduler.YAML).To(Equal(yamlStr))

			scheduler, err = cache.LoadScheduler(mockDb, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduler.YAML).To(Equal(yamlStr))
		})

		It("should return error if scheduler is not found on cache nor on db", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, "")
				})

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("scheduler \"scheduler-name\" not found"))
			Expect(scheduler).To(BeNil())
		})

		It("should return error if db fails", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Return(&types.Result{}, errors.New("db failed"))

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("db failed"))
			Expect(scheduler).To(BeNil())
		})

		It("should return error if yaml is invaild", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, "}i am invalid!{")
				})

			scheduler, err := cache.LoadScheduler(mockDb, schedulerName, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("yaml: did not find expected node content"))
			Expect(scheduler).To(BeNil())
		})
	})

	Describe("LoadConfigYaml", func() {
		It("should load from db for the first time", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				})

			newConfigYaml, err := cache.LoadConfigYaml(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(newConfigYaml).To(Equal(configYaml))
		})

		It("should load from cache for the next 10 times", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				})

			newConfigYaml, err := cache.LoadConfigYaml(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(newConfigYaml).To(Equal(configYaml))

			for i := 0; i < 10; i++ {
				newConfigYaml, err = cache.LoadConfigYaml(mockDb, schedulerName, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(newConfigYaml).To(Equal(configYaml))
			}
		})

		It("should load from db is useCache is false", func() {
			cache := NewSchedulerCache(timeout, purgeTime, logger)

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *Scheduler, query string, modifier string) {
					*scheduler = *NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
				}).Times(2)

			newConfigYaml, err := cache.LoadConfigYaml(mockDb, schedulerName, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(newConfigYaml).To(Equal(configYaml))

			newConfigYaml, err = cache.LoadConfigYaml(mockDb, schedulerName, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(newConfigYaml).To(Equal(configYaml))
		})
	})
})
