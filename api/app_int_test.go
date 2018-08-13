// maestro
// +build integration
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/models"
	mt "github.com/topfreegames/maestro/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("App", func() {

	var (
		recorder    *httptest.ResponseRecorder
		configYaml  *models.ConfigYAML
		err         error
		url         string
		jsonStr     string
		listOptions = metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()

		jsonStr, err = mt.NextJsonStr()
		Expect(err).NotTo(HaveOccurred())

		mockLogin.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return("user@example.com", http.StatusOK, nil).AnyTimes()
	})

	AfterEach(func() {
		pods, err := clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range pods.Items {
			room := models.NewRoom(pod.GetName(), pod.GetNamespace())
			mr := &models.MixedMetricsReporter{}
			err = room.ClearAll(app.RedisClient.Client, mr)
			Expect(err).NotTo(HaveOccurred())
		}

		exists, err := models.NewNamespace(configYaml.Name).Exists(clientset)
		Expect(err).NotTo(HaveOccurred())
		if exists {
			clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
		}
	})

	Describe("POST /scheduler", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
		})

		It("should POST a scheduler", func() {
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min))

			ns := configYaml.Name
			sch := &models.Scheduler{Name: ns}
			err = sch.Load(app.DBClient.DB)
			Expect(err).NotTo(HaveOccurred())
			Expect(sch.YAML).NotTo(HaveLen(0))

			for _, pod := range pods.Items {
				Expect(pod.Spec.Containers[0].Ports).To(HaveLen(1))
				Expect(pod.Spec.Containers[0].Ports[0].HostPort).NotTo(BeZero())

				room := models.NewRoom(pod.GetName(), ns)
				err := room.Create(app.RedisClient.Client, mockDb, mmr, configYaml)
				Expect(err).NotTo(HaveOccurred())

				pipe := app.RedisClient.Client.TxPipeline()
				roomStatuses := pipe.HMGet(room.GetRoomRedisKey(), "status", "lastPing")
				roomIsCreating := pipe.SIsMember(models.GetRoomStatusSetRedisKey(ns, room.Status), room.GetRoomRedisKey())
				roomLastPing := pipe.ZScore(models.GetRoomPingRedisKey(ns), room.ID)

				_, err = pipe.Exec()
				Expect(err).NotTo(HaveOccurred())

				stat, err := roomStatuses.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(stat[0].(string)).To(Equal(models.StatusCreating))
				Expect(stat[1].(string)).To(Equal(strconv.FormatInt(room.LastPingAt, 10)))

				isCreating, err := roomIsCreating.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(isCreating).To(BeTrue())

				lastPing, err := roomLastPing.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(lastPing).To(Equal(float64(room.LastPingAt)))
			}
		})

		It("should POST a scheduler without requests and limits", func() {
			jsonTempl := `
{
  "name": "{{.Name}}",
  "game": "game-name",
	"image": "nginx:alpine",
	"toleration": "game-name",
  "ports": [
    {
      "containerPort": 8080,
      "protocol": "TCP",
      "name": "tcp"
    }
  ],
  "shutdownTimeout": 10,
  "autoscaling": {
    "min": 2,
    "up": {
      "delta": 1,
      "trigger": {
        "usage": 70,
        "time": 1
      },
      "cooldown": 1
    },
    "down": {
      "delta": 1,
      "trigger": {
        "usage": 50,
        "time": 1
      },
      "cooldown": 1
    }
  }
}`

			var jsonStr string
			index := struct {
				Name string
			}{}

			tmpl, err := template.New("json").Parse(jsonTempl)
			Expect(err).NotTo(HaveOccurred())

			index.Name = fmt.Sprintf("maestro-test-%s", uuid.NewV4())

			buf := new(bytes.Buffer)
			err = tmpl.Execute(buf, index)
			Expect(err).NotTo(HaveOccurred())

			jsonStr = buf.String()
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min))

			ns := configYaml.Name
			sch := &models.Scheduler{Name: ns}
			err = sch.Load(app.DBClient.DB)
			Expect(err).NotTo(HaveOccurred())
			Expect(sch.YAML).NotTo(HaveLen(0))

			for _, pod := range pods.Items {
				Expect(pod.Spec.Containers[0].Ports).To(HaveLen(1))
				Expect(pod.Spec.Containers[0].Ports[0].HostPort).NotTo(BeZero())

				room := models.NewRoom(pod.GetName(), ns)
				err := room.Create(app.RedisClient.Client, mockDb, mmr, configYaml)
				Expect(err).NotTo(HaveOccurred())

				pipe := app.RedisClient.Client.TxPipeline()
				roomStatuses := pipe.HMGet(room.GetRoomRedisKey(), "status", "lastPing")
				roomIsCreating := pipe.SIsMember(models.GetRoomStatusSetRedisKey(ns, room.Status), room.GetRoomRedisKey())
				roomLastPing := pipe.ZScore(models.GetRoomPingRedisKey(ns), room.ID)

				_, err = pipe.Exec()
				Expect(err).NotTo(HaveOccurred())

				stat, err := roomStatuses.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(stat[0].(string)).To(Equal(models.StatusCreating))
				Expect(stat[1].(string)).To(Equal(strconv.FormatInt(room.LastPingAt, 10)))

				isCreating, err := roomIsCreating.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(isCreating).To(BeTrue())

				lastPing, err := roomLastPing.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(lastPing).To(Equal(float64(room.LastPingAt)))
			}
		})

		It("should return code 500 if postgres is down", func() {
			app, err := api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", nil, nil, nil, nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			err = app.DBClient.DB.Close()
			Expect(err).NotTo(HaveOccurred())

			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			resp := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveKeyWithValue("code", "MAE-001"))
			Expect(resp).To(HaveKeyWithValue("description", "pg: database is closed"))
			Expect(resp).To(HaveKeyWithValue("error", "DatabaseError"))
			Expect(resp).To(HaveKeyWithValue("success", false))
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should return 500 if timeout during creation", func() {
			body := strings.NewReader(jsonStr)
			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			timeoutSec := app.Config.GetInt("scaleUpTimeoutSeconds")
			app.Config.Set("scaleUpTimeoutSeconds", 0)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			resp := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(resp).To(HaveKeyWithValue("description", "timeout deleting namespace"))
			Expect(resp).To(HaveKeyWithValue("error", "Create scheduler failed"))
			Expect(resp).To(HaveKeyWithValue("success", false))

			app.Config.Set("scaleUpTimeoutSeconds", timeoutSec)
		})
	})

	Describe("DELETE /scheduler/{schedulerName}", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("http://%s%s", app.Address, "/scheduler")
		})

		It("should delete a created scheduler", func() {
			// Create the scheduler
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			// Delete the scheduler
			recorder = httptest.NewRecorder()
			url := fmt.Sprintf("%s/%s", url, configYaml.Name)
			request, err = http.NewRequest("DELETE", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(BeEmpty())

			ns := configYaml.Name
			nameSpace := models.NewNamespace(ns)
			exists, err := nameSpace.Exists(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			sch := &models.Scheduler{Name: ns}
			err = sch.Load(app.DBClient.DB)
			Expect(err).NotTo(HaveOccurred())
			Expect(sch.YAML).To(HaveLen(0))
		})
	})

	Describe("PUT /scheduler/{schedulerName}", func() {
		It("should update scheduler on database", func() {
			// Create the scheduler
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			url := fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			// Update the scheduler
			configYaml.AutoScaling.Min = 5
			configYaml.Image = "nginx:latest"
			configYaml.ImagePullPolicy = "Always"
			bts, err := json.Marshal(configYaml)
			Expect(err).NotTo(HaveOccurred())
			bodyRdr := bytes.NewReader(bts)

			url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, configYaml.Name)
			request, err = http.NewRequest("PUT", url, bodyRdr)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			scheduler := &models.Scheduler{Name: configYaml.Name}
			err = scheduler.Load(app.DBClient.DB)
			Expect(err).NotTo(HaveOccurred())

			newConfigYaml, err := models.NewConfigYAML(scheduler.YAML)
			newConfigYaml.Env = nil
			newConfigYaml.Cmd = nil
			newConfigYaml.Forwarders = nil
			newConfigYaml.Containers = nil
			newConfigYaml.AuthorizedUsers = nil
			configYaml.Ports[0].HostPort = newConfigYaml.Ports[0].HostPort
			Expect(err).NotTo(HaveOccurred())
			Expect(newConfigYaml).To(Equal(configYaml))
		})

		It("should return error if updating nonexisting scheduler", func() {
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, configYaml.Name)
			request, err := http.NewRequest("PUT", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			resp := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveKeyWithValue("code", "MAE-004"))
			Expect(resp["description"]).To(ContainSubstring("not found, create it first"))
			Expect(resp).To(HaveKeyWithValue("error", "ValidationFailedError"))
			Expect(resp).To(HaveKeyWithValue("success", false))
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("POST /scheduler/{schedulerName}", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
		})

		It("should manually scale up a scheduler", func() {
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min))

			bodyJSON := map[string]interface{}{"scaleup": 1}
			bts, _ := json.Marshal(bodyJSON)
			reader := strings.NewReader(string(bts))

			urlScale := fmt.Sprintf("http://%s/scheduler/%s", app.Address, configYaml.Name)
			request, err = http.NewRequest("POST", urlScale, reader)
			Expect(err).NotTo(HaveOccurred())
			request.SetBasicAuth("user", "pass")

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			pods, err = clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min + 1))
		})

		It("should manually scale down a scheduler", func() {
			body := strings.NewReader(jsonStr)

			configYaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(configYaml.Name, &metav1.DeleteOptions{})
			}()

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Bearer token")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			pods, err := clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min))

			tx := app.RedisClient.Client.TxPipeline()
			tx.SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, models.StatusReady), pods.Items[0].GetName())
			tx.Exec()

			bodyJSON := map[string]interface{}{"scaledown": 1}
			bts, _ := json.Marshal(bodyJSON)
			reader := strings.NewReader(string(bts))

			urlScale := fmt.Sprintf("http://%s/scheduler/%s", app.Address, configYaml.Name)
			request, err = http.NewRequest("POST", urlScale, reader)
			Expect(err).NotTo(HaveOccurred())
			request.SetBasicAuth("user", "pass")

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			pods, err = clientset.CoreV1().Pods(configYaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(configYaml.AutoScaling.Min - 1))
		})
	})
})
