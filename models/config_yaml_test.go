package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/maestro/models"
)

var _ = Describe("ConfigYaml", func() {
	Describe("ToYaml", func() {
		It("should return yaml for version v1", func() {
			configYaml, _ := NewConfigYAML(`name: scheduler-name
game: game
image: nginx:alpine
ports:
- containerPort: 8080
  protocol: TCP
  name: tcp
limits:
  cpu: 100m
  memory: 100Mi
requests:
  cpu: 50m
  memory: 50Mi
cmd: ["/bin/bash", "-c", "./start.sh"]
env:
- name: ENV_1
  value: VALUE_1
containers: []
`)

			result := `{"yaml":"name: scheduler-name\ngame: game\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nauthorizedUsers: []\nportRange: null\nimage: nginx:alpine\nimagePullPolicy: \"\"\nports:\n- containerPort: 8080\n  protocol: TCP\n  name: tcp\nlimits:\n  cpu: 100m\n  memory: 100Mi\nrequests:\n  cpu: 50m\n  memory: 50Mi\nenv:\n- name: ENV_1\n  value: VALUE_1\n  valueFrom:\n    secretKeyRef:\n      name: \"\"\n      key: \"\"\ncmd:\n- /bin/bash\n- -c\n- ./start.sh\n"}`

			Expect(string(configYaml.ToYAML())).To(Equal(result))
		})

		It("should return yaml for version v2", func() {
			configYaml, _ := NewConfigYAML(`name: scheduler-name
game: game
containers:
- name: container1
  image: nginx:alpine
  ports:
  - containerPort: 8080
    protocol: TCP
    name: tcp
  limits:
    cpu: 100m
    memory: 100Mi
  requests:
    cpu: 50m
    memory: 50Mi
  cmd: ["/bin/bash", "-c", "./start.sh"]
  env:
  - name: ENV_1
    value: VALUE_1
`)

			result := `{"yaml":"name: scheduler-name\ngame: game\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nauthorizedUsers: []\ncontainers:\n- name: container1\n  image: nginx:alpine\n  imagePullPolicy: \"\"\n  ports:\n  - containerPort: 8080\n    protocol: TCP\n    name: tcp\n  limits:\n    cpu: 100m\n    memory: 100Mi\n  requests:\n    cpu: 50m\n    memory: 50Mi\n  env:\n  - name: ENV_1\n    value: VALUE_1\n    valueFrom:\n      secretKeyRef:\n        name: \"\"\n        key: \"\"\n  cmd:\n  - /bin/bash\n  - -c\n  - ./start.sh\nportRange: null\n"}`

			Expect(string(configYaml.ToYAML())).To(Equal(result))
		})
	})

	Describe("Version", func() {
		It("should return version v1", func() {
			configYaml, _ := NewConfigYAML(`name: scheduler-name
game: game
image: nginx:alpine
ports:
- containerPort: 8080
  protocol: TCP
  name: tcp
limits:
  cpu: 100m
  memory: 100Mi
requests:
  cpu: 50m
  memory: 50Mi
cmd: ["/bin/bash", "-c", "./start.sh"]
env:
- name: ENV_1
  value: VALUE_1
`)
			Expect(configYaml.Version()).To(Equal("v1"))
		})

		It("should return version v2", func() {
			configYaml, _ := NewConfigYAML(`name: scheduler-name
game: game
containers:
- name: container1
  image: nginx:alpine
  ports:
  - containerPort: 8080
    protocol: TCP
    name: tcp
  limits:
    cpu: 100m
    memory: 100Mi
  requests:
    cpu: 50m
    memory: 50Mi
  cmd: ["/bin/bash", "-c", "./start.sh"]
  env:
  - name: ENV_1
    value: VALUE_1
`)
			Expect(configYaml.Version()).To(Equal("v2"))
		})
	})

	Describe("Get*", func() {
		configYaml, _ := NewConfigYAML(`name: scheduler-name
game: game
image: nginx:alpine
ports:
- containerPort: 8080
  protocol: TCP
  name: tcp
limits:
  cpu: 100m
  memory: 100Mi
requests:
  cpu: 50m
  memory: 50Mi
cmd: ["/bin/bash", "-c", "./start.sh"]
env:
- name: ENV_1
  value: VALUE_1
`)

		It("should return properties", func() {
			Expect(configYaml.GetImage()).To(Equal("nginx:alpine"))
			Expect(configYaml.GetName()).To(Equal("scheduler-name"))
			Expect(configYaml.GetPorts()).To(Equal([]*Port{
				{
					ContainerPort: 8080,
					Protocol:      "TCP",
					Name:          "tcp",
				},
			}))
			Expect(configYaml.GetLimits()).To(Equal(&Resources{
				CPU:    "100m",
				Memory: "100Mi",
			}))
			Expect(configYaml.GetRequests()).To(Equal(&Resources{
				CPU:    "50m",
				Memory: "50Mi",
			}))
			Expect(configYaml.GetCmd()).To(Equal([]string{
				"/bin/bash", "-c", "./start.sh",
			}))
			Expect(configYaml.GetEnv()).To(Equal([]*EnvVar{
				{
					Name:  "ENV_1",
					Value: "VALUE_1",
				},
			}))
		})
	})

	Describe("UpdateImage", func() {
		var configYamlV1, configYamlV2 *ConfigYAML

		BeforeEach(func() {
			configYamlV1, _ = NewConfigYAML(`name: scheduler-name
game: game
image: nginx:alpine
ports:
- containerPort: 8080
  protocol: TCP
  name: tcp
limits:
  cpu: 100m
  memory: 100Mi
requests:
  cpu: 50m
  memory: 50Mi
cmd: ["/bin/bash", "-c", "./start.sh"]
env:
- name: ENV_1
  value: VALUE_1
containers: []
`)
			configYamlV2, _ = NewConfigYAML(`name: scheduler-name
game: game
containers:
  - name: container1
    image: nginx:alpine
    ports:
    - containerPort: 8080
      protocol: TCP
      name: tcp
    limits:
      cpu: 100m
      memory: 100Mi
    requests:
      cpu: 50m
      memory: 50Mi
    cmd: ["/bin/bash", "-c", "./start.sh"]
    env:
    - name: ENV_1
      value: VALUE_1
`)
		})

		It("should update image on version v1", func() {
			var imageParams = &SchedulerImageParams{
				Image: "new-image",
			}
			updated, err := configYamlV1.UpdateImage(imageParams)
			Expect(updated).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(configYamlV1.Image).To(Equal(imageParams.Image))
		})

		It("should return not updated if image is the same", func() {
			var imageParams = &SchedulerImageParams{
				Image: configYamlV1.Image,
			}

			updated, err := configYamlV1.UpdateImage(imageParams)
			Expect(updated).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
			Expect(configYamlV1.Image).To(Equal(imageParams.Image))
		})

		It("should update image on specified container", func() {
			var imageParams = &SchedulerImageParams{
				Image:     "new-image",
				Container: "container1",
			}

			updated, err := configYamlV2.UpdateImage(imageParams)
			Expect(updated).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(configYamlV2.Containers[0].Image).To(Equal(imageParams.Image))
		})

		It("should return error if container not found", func() {
			var imageParams = &SchedulerImageParams{
				Image:     "new-image",
				Container: "container2",
			}

			updated, err := configYamlV2.UpdateImage(imageParams)
			Expect(updated).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no container with name container2"))
		})

		It("should return not updated if container image is the same", func() {
			var imageParams = &SchedulerImageParams{
				Image:     configYamlV2.Containers[0].Image,
				Container: "container1",
			}

			updated, err := configYamlV2.UpdateImage(imageParams)
			Expect(updated).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
