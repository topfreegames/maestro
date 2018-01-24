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

			result := `{"yaml":"name: scheduler-name\ngame: game\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nimage: nginx:alpine\nports:\n- containerPort: 8080\n  protocol: TCP\n  name: tcp\nlimits:\n  cpu: 100m\n  memory: 100Mi\nrequests:\n  cpu: 50m\n  memory: 50Mi\nenv:\n- name: ENV_1\n  value: VALUE_1\n  valueFrom:\n    secretKeyRef:\n      name: \"\"\n      key: \"\"\ncmd:\n- /bin/bash\n- -c\n- ./start.sh\n"}`

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

			result := `{"yaml":"name: scheduler-name\ngame: game\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\ncontainers:\n- name: container1\n  image: nginx:alpine\n  ports:\n  - containerPort: 8080\n    protocol: TCP\n    name: tcp\n  limits:\n    cpu: 100m\n    memory: 100Mi\n  requests:\n    cpu: 50m\n    memory: 50Mi\n  env:\n  - name: ENV_1\n    value: VALUE_1\n    valueFrom:\n      secretKeyRef:\n        name: \"\"\n        key: \"\"\n  cmd:\n  - /bin/bash\n  - -c\n  - ./start.sh\n"}`

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
})
