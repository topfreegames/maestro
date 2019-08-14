// maestro
// +build unit
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

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/pg"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"
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
      threshold: 80
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
      threshold: 80
    cooldown: 300
env:
  - name: EXAMPLE_ENV_VAR
    value: examplevalue
  - name: ANOTHER_ENV_VAR
    value: anothervalue
  - name: SECRET_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: secretname
        key: secretkey
cmd:
  - "./room-binary"
  - "-serverType"
  - "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
forwarders:
  grpc:
    matchmaking:
      enabled: true
      metadata:
        roomType: green-ffa
        numberOfTeams: 1
        playersPerTeam: 6
        metadata:
          nested: object
        tags:
          score: score
`
	yamlReq = `
name: pong-free-for-all
game: pong
image: pong/pong:v123
ports:
  - containerPort: 5050
    protocol: UDP
  - containerPort: 8888
    protocol: TCP
requests:
  memory: "128Mi"
  cpu: "1"
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
cmd:
  - "./room-binary"
`
	yamlContReq = `
name: pong-free-for-all
game: pong
image: pong/pong:v123
ports:
  - containerPort: 5050
    protocol: UDP
  - containerPort: 8888
    protocol: TCP
containers:
- name: container1
  image: image1
  requests:
    memory: "128Mi"
    cpu: "1"
  cmd:
    - "./room-binary"
- name: container2
  image: image2
  requests:
    memory: "128Mi"
    cpu: "1"
  cmd:
    - "./room-binary"
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
`
)

var _ = Describe("Scheduler", func() {
	name := "pong-free-for-all"
	game := "pong"
	errDB := errors.New("db failed")

	Describe("NewConfigYAML", func() {
		It("should build correct config yaml struct from yaml", func() {
			configYAML, err := NewConfigYAML(yaml1)
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
			Expect(configYAML.Env).To(HaveLen(3))
			Expect(configYAML.Env[0].Name).To(Equal("EXAMPLE_ENV_VAR"))
			Expect(configYAML.Env[0].Value).To(Equal("examplevalue"))
			Expect(configYAML.Env[1].Name).To(Equal("ANOTHER_ENV_VAR"))
			Expect(configYAML.Env[1].Value).To(Equal("anothervalue"))
			Expect(configYAML.Env[2].Name).To(Equal("SECRET_ENV_VAR"))
			Expect(configYAML.Env[2].ValueFrom.SecretKeyRef.Name).To(Equal("secretname"))
			Expect(configYAML.Env[2].ValueFrom.SecretKeyRef.Key).To(Equal("secretkey"))
			Expect(configYAML.Cmd).To(HaveLen(3))
			Expect(configYAML.Cmd[0]).To(Equal("./room-binary"))
			Expect(configYAML.Cmd[1]).To(Equal("-serverType"))
			Expect(configYAML.Cmd[2]).To(Equal("6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"))
			Expect(configYAML.Forwarders).To(HaveKey("grpc"))
			Expect(configYAML.Forwarders["grpc"]).To(HaveKey("matchmaking"))
			Expect(configYAML.Forwarders["grpc"]["matchmaking"].Enabled).To(BeTrue())
			Expect(configYAML.Forwarders["grpc"]["matchmaking"].Metadata["roomType"]).To(Equal("green-ffa"))
			Expect(configYAML.Forwarders["grpc"]["matchmaking"].Metadata["numberOfTeams"]).To(Equal(1))
			Expect(configYAML.Forwarders["grpc"]["matchmaking"].Metadata["playersPerTeam"]).To(Equal(6))
			Expect(configYAML.Forwarders["grpc"]["matchmaking"].Metadata["metadata"].(map[interface{}]interface{})["nested"]).To(Equal("object"))
		})

		It("should fail if invalid yaml", func() {
			configYAML, err := NewConfigYAML("not-a-valid-yaml")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot unmarshal"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.YamlError"))
			Expect(configYAML).To(BeNil())
		})
	})

	Describe("NewScheduler", func() {
		It("should build correct scheduler struct", func() {
			scheduler := NewScheduler(name, game, yaml1)
			Expect(scheduler.Name).To(Equal(name))
			Expect(scheduler.Game).To(Equal(game))
			Expect(scheduler.YAML).To(Equal(yaml1))
		})
	})

	Describe("Create Scheduler", func() {
		It("should save scheduler in the database", func() {
			scheduler := NewScheduler(name, game, yaml1)
			testing.MockInsertScheduler(mockDb, nil)
			err := scheduler.Create(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := NewScheduler(name, game, yaml1)
			testing.MockInsertScheduler(mockDb, errDB)
			err := scheduler.Create(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errDB.Error()))
		})
	})

	Describe("Load Scheduler", func() {
		It("should load scheduler from the database", func() {
			scheduler := NewScheduler(name, "", "")
			mockDb.EXPECT().Query(scheduler, "SELECT * FROM schedulers WHERE name = ?", name)
			err := scheduler.Load(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Query(
				scheduler,
				"SELECT * FROM schedulers WHERE name = ?",
				name,
			).Return(pg.NewTestResult(errors.New("some error in pg"), 0), errors.New("some error in pg"))
			err := scheduler.Load(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("LoadSchedulers", func() {
		It("should load schedulers from the database", func() {
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name IN (?)",
				gomock.Any(),
			)
			names := []string{"s1", "s2"}
			_, err := LoadSchedulers(mockDb, names)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name IN (?)",
				gomock.Any(),
			).Return(pg.NewTestResult(errors.New("some error in pg"), 0), errors.New("some error in pg"))
			names := []string{"s1", "s2"}
			_, err := LoadSchedulers(mockDb, names)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("Update Scheduler", func() {
		It("should update scheduler in the database", func() {
			scheduler := NewScheduler(name, game, yaml1)
			scheduler.State = "terminating"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			testing.MockUpdateScheduler(mockDb, nil, nil)
			err := scheduler.Update(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := NewScheduler(name, game, yaml1)
			scheduler.State = "terminating"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			testing.MockUpdateScheduler(mockDb, errDB, nil)
			err := scheduler.Update(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error updating status on schedulers: db failed"))
		})

		It("should return an error if db returns an error on insert", func() {
			scheduler := NewScheduler(name, game, yaml1)
			scheduler.State = "terminating"
			scheduler.StateLastChangedAt = time.Now().Unix()
			scheduler.LastScaleOpAt = time.Now().Unix()
			testing.MockUpdateScheduler(mockDb, nil, errDB)
			err := scheduler.Update(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error inserting on scheduler_versions: db failed"))
		})
	})
	Describe("Delete Scheduler", func() {
		It("should delete scheduler in the database", func() {
			scheduler := NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", name)
			err := scheduler.Delete(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if error is 'no rows in result set'", func() {
			scheduler := NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Exec(
				"DELETE FROM schedulers WHERE name = ?",
				name,
			).Return(pg.NewTestResult(errors.New("pg: no rows in result set"), 0), errors.New("pg: no rows in result set"))
			err := scheduler.Delete(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			scheduler := NewScheduler(name, game, yaml1)
			mockDb.EXPECT().Exec(
				"DELETE FROM schedulers WHERE name = ?",
				name,
			).Return(pg.NewTestResult(errors.New("some error in pg"), 0), errors.New("some error in pg"))
			err := scheduler.Delete(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("GetAutoScalingPolicy", func() {
		It("should return the scheduler auto scaling policy", func() {
			scheduler := NewScheduler(name, game, yaml1)
			autoScalingPolicy := scheduler.GetAutoScalingPolicy()
			Expect(autoScalingPolicy.Min).To(Equal(100))
			Expect(autoScalingPolicy.Up.Cooldown).To(Equal(300))
			Expect(autoScalingPolicy.Up.Delta).To(Equal(10))
			Expect(autoScalingPolicy.Up.Trigger.Time).To(Equal(600))
			Expect(autoScalingPolicy.Up.Trigger.Usage).To(Equal(70))
			Expect(autoScalingPolicy.Up.Trigger.Threshold).To(Equal(80))
			Expect(autoScalingPolicy.Down.Cooldown).To(Equal(300))
			Expect(autoScalingPolicy.Down.Delta).To(Equal(2))
			Expect(autoScalingPolicy.Down.Trigger.Time).To(Equal(900))
			Expect(autoScalingPolicy.Down.Trigger.Usage).To(Equal(50))
			Expect(autoScalingPolicy.Down.Trigger.Threshold).To(Equal(80))
		})
	})

	Describe("SavePodsMetricsUtilizationPipeAndExec", func() {
		It("should save pods metricses", func() {
			roomUsages := make([]*RoomUsage, 5)

			for i := 0; i < 5; i++ {
				roomUsages[i] = &RoomUsage{Name: name, Usage: float64(100)}
			}
			scheduler := NewScheduler(name, game, yaml1)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().ZAdd(GetRoomMetricsRedisKey(name, string(CPUAutoScalingPolicyType)), gomock.Any()).Times(5)
			mockPipeline.EXPECT().Exec()

			scheduler.SavePodsMetricsUtilizationPipeAndExec(
				mockRedisClient,
				metricsClientset,
				mmr,
				CPUAutoScalingPolicyType,
				roomUsages,
			)
		})

		It("should not error when no rooms", func() {
			scheduler := NewScheduler(name, game, yaml1)

			scheduler.SavePodsMetricsUtilizationPipeAndExec(
				mockRedisClient,
				metricsClientset,
				mmr,
				CPUAutoScalingPolicyType,
				[]*RoomUsage{},
			)
		})
	})

	Describe("List Schedulers Names", func() {
		It("should get schedulers names from the database", func() {
			expectedNames := []string{"scheduler1", "scheduler2", "scheduler3"}
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Do(
				func(schedulers *[]Scheduler, query string) {
					expectedSchedulers := make([]Scheduler, len(expectedNames))
					for idx, name := range expectedNames {
						expectedSchedulers[idx] = Scheduler{Name: name}
					}
					*schedulers = expectedSchedulers
				},
			)
			names, err := ListSchedulersNames(mockDb)
			Expect(err).NotTo(HaveOccurred())
			Expect(names).To(Equal(expectedNames))
		})

		It("should succeed if error is 'no rows in result set'", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				pg.NewTestResult(errors.New("pg: no rows in result set"), 0), errors.New("pg: no rows in result set"),
			)
			_, err := ListSchedulersNames(mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if db returns an error", func() {
			mockDb.EXPECT().Query(gomock.Any(), "SELECT name FROM schedulers").Return(
				pg.NewTestResult(errors.New("some error in pg"), 0), errors.New("some error in pg"),
			)
			_, err := ListSchedulersNames(mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in pg"))
		})
	})

	Describe("LoadConfig", func() {
		It("should get schedulers yaml from the database", func() {
			yamlStr := "scheduler: example"
			name := "scheduler-name"
			mockDb.EXPECT().Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", name).Do(
				func(scheduler *Scheduler, query string, name string) {
					scheduler.YAML = yamlStr
				},
			)
			yamlRes, err := LoadConfig(mockDb, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(yamlRes).To(Equal(yamlStr))
		})
	})

	Describe("UpdateVersion", func() {
		var maxVersions = 10
		var query string
		var errDB = errors.New("db failed")

		It("should increment version and insert into scheduler_versions table", func() {
			scheduler := NewScheduler(name, game, yaml1)

			query = "UPDATE schedulers SET (game, yaml, version) = (?game, ?yaml, ?version) WHERE id = ?id"
			mockDb.EXPECT().Query(scheduler, query, scheduler).Return(pg.NewTestResult(nil, 1), nil)

			query = `INSERT INTO scheduler_versions (name, version, yaml, rolling_update_status)
	VALUES (?, ?, ?, ?)`
			mockDb.EXPECT().
				Query(scheduler, query, name, scheduler.Version, yaml1, gomock.Any()).
				Return(pg.NewTestResult(nil, 1), nil)

			query = "SELECT COUNT(*) FROM scheduler_versions WHERE name = ?"
			mockDb.EXPECT().
				Query(gomock.Any(), query, name).
				Do(func(count *int, _ string, _ string) {
					*count = maxVersions
				}).Return(pg.NewTestResult(nil, 1), nil)

			created, err := scheduler.UpdateVersion(mockDb, maxVersions)
			Expect(created).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete older versions from scheduler_versions table is more than max", func() {
			scheduler := NewScheduler(name, game, yaml1)

			testing.MockUpdateSchedulersTable(mockDb, nil)
			testing.MockInsertIntoVersionsTable(scheduler, mockDb, nil)
			testing.MockCountNumberOfVersions(scheduler, maxVersions+1, mockDb, nil)
			testing.MockDeleteOldVersions(scheduler, 1, mockDb, nil)

			created, err := scheduler.UpdateVersion(mockDb, maxVersions)
			Expect(created).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error if update schedulers table failed", func() {
			scheduler := NewScheduler(name, game, yaml1)

			testing.MockUpdateSchedulersTable(mockDb, errDB)

			created, err := scheduler.UpdateVersion(mockDb, maxVersions)
			Expect(created).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error to update scheduler on schedulers table: db failed"))
		})

		It("should return error if insert into scheduler_versions table failed", func() {
			scheduler := NewScheduler(name, game, yaml1)

			testing.MockUpdateSchedulersTable(mockDb, nil)
			testing.MockInsertIntoVersionsTable(scheduler, mockDb, errDB)

			created, err := scheduler.UpdateVersion(mockDb, maxVersions)
			Expect(created).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error to insert into scheduler_versions table: db failed"))
		})

		It("should return error if count on scheduler_versions table failed", func() {
			scheduler := NewScheduler(name, game, yaml1)

			testing.MockUpdateSchedulersTable(mockDb, nil)
			testing.MockInsertIntoVersionsTable(scheduler, mockDb, nil)
			testing.MockCountNumberOfVersions(scheduler, maxVersions+1, mockDb, errDB)

			created, err := scheduler.UpdateVersion(mockDb, maxVersions)
			Expect(created).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error to select count on scheduler_versions table: db failed"))
		})

		It("should return error if delete on scheduler_versions table failed", func() {
			scheduler := NewScheduler(name, game, yaml1)

			testing.MockUpdateSchedulersTable(mockDb, nil)
			testing.MockInsertIntoVersionsTable(scheduler, mockDb, nil)
			testing.MockCountNumberOfVersions(scheduler, maxVersions+1, mockDb, nil)
			testing.MockDeleteOldVersions(scheduler, 1, mockDb, errDB)

			created, err := scheduler.UpdateVersion(mockDb, maxVersions)
			Expect(created).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error to delete from scheduler_versions table: db failed"))
		})
	})

	Describe("LoadConfigWithVersion", func() {
		It("should return current version if none is specified", func() {
			testing.MockSelectYaml(yaml1, mockDb, nil)

			configYaml, err := LoadConfigWithVersion(mockDb, name, "")
			Expect(configYaml).To(Equal(yaml1))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should select version from database", func() {
			testing.MockSelectYamlWithVersion(yaml1, "v2", mockDb, nil)

			configYaml, err := LoadConfigWithVersion(mockDb, name, "v2")
			Expect(configYaml).To(Equal(yaml1))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ListSchedulerReleases", func() {
		It("should return scheduler versions", func() {
			versions := []string{"v1.0", "v1.1", "v2.0"}
			testing.MockSelectSchedulerVersions(yaml1, versions, mockDb, nil)

			rVersions, err := ListSchedulerReleases(mockDb, name)
			Expect(err).ToNot(HaveOccurred())

			Expect(rVersions[0].Version).To(Equal("v1.0"))
			Expect(rVersions[1].Version).To(Equal("v1.1"))
			Expect(rVersions[2].Version).To(Equal("v2.0"))
		})

		It("should return error if db failed", func() {
			versions := []string{"1", "2", "3"}
			testing.MockSelectSchedulerVersions(yaml1, versions, mockDb, errDB)

			_, err := ListSchedulerReleases(mockDb, name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errDB.Error()))
		})
	})

	Describe("PreviousVersion", func() {
		It("should select previous scheduler on db", func() {
			version := "v1.0"

			mockDb.EXPECT().Query(gomock.Any(), `SELECT *
	FROM scheduler_versions
	WHERE created_at < (
		SELECT created_at
		FROM scheduler_versions
		WHERE name = ?name AND version = ?version
	) AND name = ?name
	ORDER BY created_at DESC
	LIMIT 1`, gomock.Any())

			scheduler, err := PreviousVersion(mockDb, name, version)
			Expect(err).ToNot(HaveOccurred())
			Expect(scheduler.Version).To(Equal(version))
			Expect(scheduler.Name).To(Equal(name))
		})
	})

	Describe("Next*Version", func() {
		It("should return next minor version", func() {
			scheduler := NewScheduler(name, game, yaml1)
			Expect(scheduler.Version).To(Equal("v1.0"))

			scheduler.NextMinorVersion()
			Expect(scheduler.Version).To(Equal("v1.1"))
		})

		It("should return next major version", func() {
			scheduler := NewScheduler(name, game, yaml1)
			Expect(scheduler.Version).To(Equal("v1.0"))

			scheduler.NextMinorVersion()

			scheduler.NextMajorVersion()
			Expect(scheduler.Version).To(Equal("v2.0"))
		})
	})
})

var _ = Describe("Container", func() {
	container := &Container{
		Name:  "name",
		Image: "image",
		Ports: []*Port{
			{
				Name:          "port1",
				ContainerPort: 80,
			},
		},
		Limits:   &Resources{CPU: "1", Memory: "100Mi"},
		Requests: &Resources{CPU: "200m", Memory: "10Mi"},
		Command:  []string{"/bin/bash", "-c", "./start.sh"},
		Env: []*EnvVar{
			{
				Name:  "ENV_1",
				Value: "value_1",
			},
			{
				Name: "ENV_2",
				ValueFrom: ValueFrom{
					SecretKeyRef: SecretKeyRef{
						Key:  "SECRET_KEY",
						Name: "SECRET_NAME",
					},
				},
			},
		},
	}

	Describe("NewWithCopiedEnvs", func() {
		It("should return same parameters with a copied array of envs", func() {
			newContainer := container.NewWithCopiedEnvs()

			Expect(newContainer.Name).To(Equal(container.Name))
			Expect(newContainer.Image).To(Equal(container.Image))
			Expect(newContainer.Ports).To(Equal(container.Ports))
			Expect(newContainer.Limits).To(Equal(container.Limits))
			Expect(newContainer.Requests).To(Equal(container.Requests))
			Expect(newContainer.Command).To(Equal(container.Command))
			Expect(newContainer.Env).To(Equal(container.Env))

			Expect(fmt.Sprintf("%p", newContainer.Env)).
				ToNot(Equal(fmt.Sprintf("%p", container.Env)))
		})

		It("should not panic if env is nil", func() {
			container := &Container{
				Name:  "name",
				Image: "image",
				Ports: []*Port{
					{
						Name:          "port1",
						ContainerPort: 80,
					},
				},
				Limits:   &Resources{CPU: "1", Memory: "100Mi"},
				Requests: &Resources{CPU: "200m", Memory: "10Mi"},
				Command:  []string{"/bin/bash", "-c", "./start.sh"},
			}

			newContainer := container.NewWithCopiedEnvs()
			Expect(newContainer.Env).To(BeEmpty())
		})
	})

	Describe("GetResourcesRequests", func() {
		It("should return global request if no containers", func() {
			scheduler := NewScheduler("pong-free-for-all", "pong", yamlReq)
			req := scheduler.GetResourcesRequests()
			Expect(req[CPUAutoScalingPolicyType]).To(BeEquivalentTo(1000))
			Expect(req[MemAutoScalingPolicyType]).To(BeEquivalentTo(134217728))
		})

		It("should return summed container requests if containers", func() {
			scheduler := NewScheduler("pong-free-for-all", "pong", yamlContReq)
			req := scheduler.GetResourcesRequests()
			Expect(req[CPUAutoScalingPolicyType]).To(BeEquivalentTo(2000))
			Expect(req[MemAutoScalingPolicyType]).To(BeEquivalentTo(268435456))
		})

		It("should return 0 if no requests", func() {
			scheduler := NewScheduler("pong-free-for-all", "pong", yaml1)
			req := scheduler.GetResourcesRequests()
			Expect(req[CPUAutoScalingPolicyType]).To(BeEquivalentTo(0))
			Expect(req[MemAutoScalingPolicyType]).To(BeEquivalentTo(0))
		})
	})
})
