package models_test

import (
	"fmt"

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

var _ = Describe("Config", func() {
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

	Describe("NewConfig", func() {
		It("should build correct config struct", func() {
			config := models.NewConfig("pong-free-for-all", "pong", yaml1)
			Expect(config.Name).To(Equal("pong-free-for-all"))
			Expect(config.Game).To(Equal("pong"))
			Expect(config.YAML).To(Equal(yaml1))
		})
	})

	Describe("Save Config", func() {
		It("should save config in the database", func() {
			config := models.NewConfig("pong-free-for-all", "pong", yaml1)
			err := config.Create(db)
			Expect(err).NotTo(HaveOccurred())
			Expect(db.Execs).To(HaveLen(1))
		})
	})

	Describe("Load Config", func() {
		It("should load config from the database", func() {
			config := models.NewConfig("pong-free-for-all", "", "")
			err := config.Load(db)
			Expect(err).NotTo(HaveOccurred())
			Expect(db.Execs).To(HaveLen(1))
		})
	})
})
