// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"errors"
	"fmt"
	"time"

	"gopkg.in/pg.v5/types"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	yaml1 = `
name: pong-free-for-all
game: pong
image: pong/pong:v123
ports:
  - containerPort: 5050
    protocol: UDP
  - containerPort: 8888
    protocol: TCP
limits:
  memory: "128Mi"
  cpu: "1"
shutdownTimeout: 180
autoscaling:
  min: 100
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
env:
  - name: EXAMPLE_ENV_VAR
    value: examplevalue
  - name: ANOTHER_ENV_VAR
    value: anothervalue
cmd:
  - "./room-binary"
  - "-serverType"
  - "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
`
)

var _ = Describe("Scheduler", func() {
	name := "pong-free-for-all"
	game := "pong"

	Describe("NewConfigYAML", func() {
		It("should build correct config yaml struct from yaml", func() {
			configYAML, err := models.NewConfigYAML(yaml1)
			Expect(err).NotTo(HaveOccurred())
			Expect(configYAML.Name).To(Equal("pong-free-for-all"))
			Expect(configYAML.Game).To(Equal("pong"))
			Expect(configYAML.Image).To(Equal("pong/pong:v123"))
			Expect(configYAML.Ports).To(HaveLen(2))
			Expect(configYAML.Ports[0].ContainerPort).To(Equal(5050))
			Expect(configYAML.Ports[0].Protocol).To(Equal("UDP"))
			Expect(configYAML.Ports[1].ContainerPort).To(Equal(8888))
			Expect(configYAML.Ports[1].Protocol).To(Equal("TCP"))
			Expect(configYAML.Limits.CPU).To(Equal("1"))
			Expect(configYAML.Limits.Memory).To(Equal("128Mi"))
			Expect(configYAML.ShutdownTimeout).To(Equal(180))
			Expect(configYAML.AutoScaling.Min).To(Equal(100))
			Expect(configYAML.AutoScaling.Up.Cooldown).To(Equal(300))
			Expect(configYAML.AutoScaling.Up.Delta).To(Equal(10))
			Expect(configYAML.AutoScaling.Up.Trigger.Time).To(Equal(600))
			Expect(configYAML.AutoScaling.Up.Trigger.Usage).To(Equal(70))
			Expect(configYAML.AutoScaling.Down.Cooldown).To(Equal(300))
			Expect(configYAML.AutoScaling.Down.Delta).To(Equal(2))
			Expect(configYAML.AutoScaling.Down.Trigger.Time).To(Equal(900))
			Expect(configYAML.AutoScaling.Down.Trigger.Usage).To(Equal(50))
			Expect(configYAML.Env).To(HaveLen(2))
			Expect(configYAML.Env[0].Name).To(Equal("EXAMPLE_ENV_VAR"))
			Expect(configYAML.Env[0].Value).To(Equal("examplevalue"))
			Expect(configYAML.Env[1].Name).To(Equal("ANOTHER_ENV_VAR"))
			Expect(configYAML.Env[1].Value).To(Equal("anothervalue"))
			Expect(configYAML.Cmd).To(HaveLen(3))
			Expect(configYAML.Cmd[0]).To(Equal("./room-binary"))
			Expect(configYAML.Cmd[1]).To(Equal("-serverType"))
			Expect(configYAML.Cmd[2]).To(Equal("6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"))
		})

		It("should fail if invalid yaml", func() {
			configYAML, err := models.NewConfigYAML("not-a-valid-yaml")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot unmarshal"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.YamlError"))
			Expect(configYAML).To(BeNil())
		})
	})

	Describe("NewScheduler", func() {
		It("should build correct scheduler struct", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			Expect(scheduler.Name).To(Equal(name))
			Expect(scheduler.Game).To(Equal(game))
			Expect(scheduler.YAML).To(Equal(yaml1))
		})
	})

	Describe("Create Scheduler", func() {
		It("should save scheduler in the database", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Query(
				scheduler,
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				scheduler,
			)
			err := scheduler.Create(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Query(
				scheduler,
				"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
				scheduler,
			).Return(&types.Result{}, errors.New("some error in pg"))
			err := scheduler.Create(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("Load Scheduler", func() {
		It("should load scheduler from the database", func() {
			scheduler := models.NewScheduler(name, "", "")
			mockDb.EXPECT().Query(scheduler, "SELECT * FROM schedulers WHERE name = ?", name)
			err := scheduler.Load(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Query(
				scheduler,
				"SELECT * FROM schedulers WHERE name = ?",
				name,
			).Return(&types.Result{}, errors.New("some error in pg"))
			err := scheduler.Load(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("Update Scheduler", func() {
		It("should update scheduler in the database", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			scheduler.State = "terminating"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			mockDb.EXPECT().Query(
				scheduler,
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				scheduler,
			)
			err := scheduler.Update(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			scheduler.State = "terminating"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			mockDb.EXPECT().Query(
				scheduler,
				"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
				scheduler,
			).Return(&types.Result{}, errors.New("some error in pg"))
			err := scheduler.Update(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})
	Describe("Delete Scheduler", func() {
		It("should delete scheduler in the database", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", name)
			err := scheduler.Delete(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if error is 'no rows in result set'", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Exec(
				"DELETE FROM schedulers WHERE name = ?",
				name,
			).Return(&types.Result{}, errors.New("pg: no rows in result set"))
			err := scheduler.Delete(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Exec(
				"DELETE FROM schedulers WHERE name = ?",
				name,
			).Return(&types.Result{}, errors.New("some error in pg"))
			err := scheduler.Delete(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("GetAutoScalingPolicy", func() {
		It("should return the scheduler auto scaling policy", func() {
			scheduler := models.NewScheduler(name, game, yaml1)
			autoScalingPolicy := scheduler.GetAutoScalingPolicy()
			Expect(autoScalingPolicy.Min).To(Equal(100))
			Expect(autoScalingPolicy.Up.Cooldown).To(Equal(300))
			Expect(autoScalingPolicy.Up.Delta).To(Equal(10))
			Expect(autoScalingPolicy.Up.Trigger.Time).To(Equal(600))
			Expect(autoScalingPolicy.Up.Trigger.Usage).To(Equal(70))
			Expect(autoScalingPolicy.Down.Cooldown).To(Equal(300))
			Expect(autoScalingPolicy.Down.Delta).To(Equal(2))
			Expect(autoScalingPolicy.Down.Trigger.Time).To(Equal(900))
			Expect(autoScalingPolicy.Down.Trigger.Usage).To(Equal(50))
		})
	})

	Describe("List Schedulers Names", func() {
		It("should get schedulers names from the database", func() {
			expectedNames := []string{"scheduler1", "scheduler2", "scheduler3"}
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Do(
				func(schedulers *[]models.Scheduler, query string) {
					expectedSchedulers := make([]models.Scheduler, len(expectedNames))
					for idx, name := range expectedNames {
						expectedSchedulers[idx] = models.Scheduler{Name: name}
					}
					*schedulers = expectedSchedulers
				},
			)
			names, err := models.ListSchedulersNames(mockDb)
			Expect(err).NotTo(HaveOccurred())
			Expect(names).To(Equal(expectedNames))
		})

		It("should succeed if error is 'no rows in result set'", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				&types.Result{}, errors.New("pg: no rows in result set"),
			)
			_, err := models.ListSchedulersNames(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				&types.Result{}, errors.New("some error in pg"),
			)
			_, err := models.ListSchedulersNames(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})
})
