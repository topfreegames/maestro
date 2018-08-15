// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
)

var _ = Describe("AccessMiddleware", func() {
	var (
		recorder   *httptest.ResponseRecorder
		yamlString string
	)

	yamlString = `{
  "name": "scheduler-name",
  "game": "game-name",
  "image": "somens/someimage:v123",
  "ports": [
    {
      "containerPort": 5050,
      "protocol": "UDP",
      "name": "port1"
    },
    {
      "containerPort": 8888,
      "protocol": "TCP",
      "name": "port2"
    }
  ],
  "limits": {
    "memory": "128Mi",
    "cpu": "1"
  },
  "shutdownTimeout": 180,
  "autoscaling": {
    "min": 100,
    "up": {
      "delta": 10,
      "trigger": {
        "usage": 70,
        "time": 600
      },
      "cooldown": 300
    },
    "down": {
      "delta": 2,
      "trigger": {
        "usage": 50,
        "time": 900
      },
      "cooldown": 300
    }
  },
  "env": [
    {
      "name": "EXAMPLE_ENV_VAR",
      "value": "examplevalue"
    },
    {
      "name": "ANOTHER_ENV_VAR",
      "value": "anothervalue"
    }
  ],
  "cmd": [
    "./room-binary",
    "-serverType",
    "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
  ],
  "forwarders": {
    "mockplugin": {
      "mockfwd": {
        "enabled": true,
        "medatada": {
          "send": "me"
        }
      }
    }
  }
}`

	BeforeEach(func() {
		// Record HTTP responses.
		recorder = httptest.NewRecorder()
		node := &v1.Node{}
		node.SetName("node-name")
		node.SetLabels(map[string]string{
			"game": "controller",
		})
		_, err := clientset.CoreV1().Nodes().Create(node)
		Expect(err).NotTo(HaveOccurred())

		mockRedisClient.EXPECT().Ping().AnyTimes()
	})

	Context("When authentication is Basic Auth + X-Forwarded-Email", func() {
		BeforeEach(func() {
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB).AnyTimes()
		})

		Describe("GET /scheduler", func() {
			It("should list schedulers", func() {
				var configYaml models.ConfigYAML
				expectedNames := []string{"scheduler1", "scheduler2", "scheduler3"}
				err := json.Unmarshal([]byte(yamlString), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				url := fmt.Sprintf("http://%s/scheduler", app.Address)
				request, err := http.NewRequest("GET", url, nil)
				user := app.Config.GetString("basicauth.username")
				pass := app.Config.GetString("basicauth.password")
				request.Header.Add("X-Forwarded-User-Email", "user@example.com")
				request.SetBasicAuth(user, pass)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT name FROM schedulers").Do(
					func(schedulers *[]models.Scheduler, query string) {
						expectedSchedulers := make([]models.Scheduler, len(expectedNames))
						for idx, name := range expectedNames {
							expectedSchedulers[idx] = models.Scheduler{Name: name}
						}
						*schedulers = expectedSchedulers
					})
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"schedulers":["scheduler1","scheduler2","scheduler3"]}`))
			})
		})
	})
})
