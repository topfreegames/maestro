// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"
	mt "github.com/topfreegames/maestro/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/pkg/api/v1"
)

var _ = Describe("Worker", func() {
	var (
		yaml        *models.ConfigYAML
		jsonStr     string
		recorder    *httptest.ResponseRecorder
		url         string
		listOptions = metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
	)

	BeforeEach(func() {
		var err error

		recorder = httptest.NewRecorder()

		jsonStr, err = mt.NextJsonStr()
		Expect(err).NotTo(HaveOccurred())

		mockLogin.EXPECT().Authenticate(gomock.Any(), app.DB).Return("user@example.com", http.StatusOK, nil).AnyTimes()
	})

	AfterEach(func() {
		for _, watcher := range w.Watchers {
			watcher.Run = false
		}

		pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range pods.Items {
			room := models.NewRoom(pod.GetName(), pod.GetNamespace())
			err = room.ClearAll(app.RedisClient)
			Expect(err).NotTo(HaveOccurred())
		}

		exists, err := models.NewNamespace(yaml.Name).Exists(clientset)
		Expect(err).NotTo(HaveOccurred())

		if exists {
			clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
		}
	})

	Describe("EnsureRunningWatchers", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
		})

		It("should create a watcher when new scheduler is created", func() {
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			schedulerNames := []string{yaml.Name}
			w.EnsureRunningWatchers(schedulerNames)

			Expect(w.Watchers).To(HaveKey(schedulerNames[0]))
			Expect(w.Watchers[schedulerNames[0]].SchedulerName).To(Equal(schedulerNames[0]))
			Eventually(func() bool { return w.Watchers[schedulerNames[0]].Run }).Should(BeTrue())
			Eventually(func() int {
				list, _ := clientset.CoreV1().Pods(schedulerNames[0]).List(listOptions)
				return len(list.Items)
			}, 120*time.Second, 1*time.Second).Should(Equal(yaml.AutoScaling.Min))

			totalPorts := endPortRange - startPortRange + 1
			takenPorts := yaml.AutoScaling.Min * len(yaml.Ports)
			Eventually(func() (int, error) {
				cmd := app.RedisClient.Eval(`return redis.call("SCARD", KEYS[1])`, []string{models.FreePortsRedisKey()})
				amountInterface, err := cmd.Result()
				var amount int64 = 0
				if amountInterface != nil {
					amount, _ = amountInterface.(int64)
				}
				return int(amount), err
			}, 120*time.Second, 1*time.Second).
				Should(BeNumerically("<=", totalPorts-takenPorts))
		})
	})

	Describe("RemoveDeadWatchers", func() {
		It("should remove dead watchers", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			schedulerNames := []string{yaml.Name}
			w.EnsureRunningWatchers(schedulerNames)

			w.Watchers[yaml.Name].Run = false

			w.RemoveDeadWatchers()
			Expect(w.Watchers).NotTo(HaveKey(yaml.Name))
		})
	})

	Describe("Start", func() {
		var yaml1 *models.ConfigYAML

		AfterEach(func() {

			if yaml1 != nil {
				exists, err := models.NewNamespace(yaml1.Name).Exists(clientset)
				Expect(err).NotTo(HaveOccurred())
				if exists {
					clientset.CoreV1().Namespaces().Delete(yaml1.Name, &metav1.DeleteOptions{})
				}
			}
		})

		It("should have two schedulers", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			jsonStr, err = mt.NextJsonStr()
			Expect(err).NotTo(HaveOccurred())
			recorder = httptest.NewRecorder()
			request, err = http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml1, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			Eventually(func() int {
				pods, _ := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return len(pods.Items)
			}, 120*time.Second, 1*time.Second).Should(BeNumerically(">=", yaml.AutoScaling.Min))

			Eventually(func() int {
				pods1, _ := clientset.CoreV1().Pods(yaml1.Name).List(listOptions)
				return len(pods1.Items)
			}, 120*time.Second, 1*time.Second).Should(BeNumerically(">=", yaml1.AutoScaling.Min))

			totalPorts := endPortRange - startPortRange + 1
			takenPorts := yaml.AutoScaling.Min*len(yaml.Ports) + yaml1.AutoScaling.Min*len(yaml1.Ports)
			Eventually(func() (int, error) {
				cmd := app.RedisClient.Eval(`return redis.call("SCARD", KEYS[1])`, []string{models.FreePortsRedisKey()})
				amountInterface, err := cmd.Result()
				var amount int64 = 0
				if amountInterface != nil {
					amount, _ = amountInterface.(int64)
				}
				return int(amount), err
			}, 120*time.Second, 1*time.Second).
				Should(BeNumerically("<=", totalPorts-takenPorts))
		})

		It("should scale up", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			for _, pod := range pods.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, pod.GetNamespace(), pod.GetName())
				request, err := http.NewRequest("PUT", url, mt.JSONFor(mt.JSON{
					"timestamp": 1000,
					"status":    models.StatusOccupied,
				}))
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			}

			Eventually(func() int {
				pods, _ = clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return len(pods.Items)
			}, 120*time.Second, 1*time.Second).Should(BeNumerically(">=", yaml.AutoScaling.Min+yaml.AutoScaling.Up.Delta))

			totalPorts := endPortRange - startPortRange + 1
			takenPorts := (yaml.AutoScaling.Min + yaml.AutoScaling.Up.Delta) * len(yaml.Ports)
			Eventually(func() (int, error) {
				cmd := app.RedisClient.Eval(`return redis.call("SCARD", KEYS[1])`, []string{models.FreePortsRedisKey()})
				amountInterface, err := cmd.Result()
				var amount int64 = 0
				if amountInterface != nil {
					amount, _ = amountInterface.(int64)
				}
				return int(amount), err
			}, 120*time.Second, 1*time.Second).
				Should(BeNumerically("<=", totalPorts-takenPorts))
		})

		It("should delete rooms that timed out with occupied status", func() {
			jsonStr := fmt.Sprintf(`{
  "name": "%s",
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
  "limits": {
    "memory": "10Mi",
    "cpu": "10m"
  },
 "requests": {
    "memory": "10Mi",
    "cpu": "10m"
  },
	"occupiedTimeout": 1,
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
}`, fmt.Sprintf("maestro-test-%s", uuid.NewV4()))
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			for _, pod := range pods.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, pod.GetNamespace(), pod.GetName())
				request, err := http.NewRequest("PUT", url, mt.JSONFor(mt.JSON{
					"timestamp": 1000,
					"status":    models.StatusOccupied,
				}))
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			}

			Eventually(func() bool {
				for _, pod := range pods.Items {
					_, err := clientset.CoreV1().Pods(yaml.Name).Get(pod.GetName(), metav1.GetOptions{})
					if err == nil || !strings.Contains(err.Error(), "not found") {
						return false
					}
				}
				return true
			}, 120*time.Second, 1*time.Second).Should(BeTrue())
		})

		It("should scale down", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			for _, pod := range pods.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, pod.GetNamespace(), pod.GetName())
				request, err := http.NewRequest("PUT", url, mt.JSONFor(mt.JSON{
					"timestamp": 1000,
					"status":    models.StatusOccupied,
				}))
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			}

			ticker := time.NewTicker(1 * time.Second).C

		waitForUp:
			for {
				select {
				case <-ticker:
					pods, err = clientset.CoreV1().Pods(yaml.Name).List(listOptions)
					Expect(err).NotTo(HaveOccurred())
					if len(pods.Items) >= yaml.AutoScaling.Min+yaml.AutoScaling.Up.Delta {
						break waitForUp
					}
				}
			}

			for _, pod := range pods.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, pod.GetNamespace(), pod.GetName())
				request, err := http.NewRequest("PUT", url, mt.JSONFor(mt.JSON{
					"timestamp": 1000,
					"status":    models.StatusReady,
				}))
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			}

			time.Sleep(time.Duration(yaml.AutoScaling.Up.Cooldown) * time.Second)

			newRoomNumber := yaml.AutoScaling.Min + yaml.AutoScaling.Up.Delta - yaml.AutoScaling.Down.Delta
			if newRoomNumber < yaml.AutoScaling.Min {
				newRoomNumber = yaml.AutoScaling.Min
			}

			Eventually(func() int {
				pods, _ = clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return len(pods.Items)
			}, 120*time.Second, 1*time.Second).Should(Equal(newRoomNumber))

			Eventually(func() int {
				pipe := app.RedisClient.TxPipeline()
				cmd := pipe.Keys("scheduler:*:rooms:*")
				_, err := pipe.Exec()
				Expect(err).NotTo(HaveOccurred())
				keys, err := cmd.Result()
				Expect(err).NotTo(HaveOccurred())
				return len(keys)
			}, 120*time.Second, 1*time.Second).Should(Equal(newRoomNumber))

			totalPorts := endPortRange - startPortRange + 1
			takenPorts := yaml.AutoScaling.Min * len(yaml.Ports)
			Eventually(func() (int, error) {
				cmd := app.RedisClient.Eval(`return redis.call("SCARD", KEYS[1])`, []string{models.FreePortsRedisKey()})
				amountInterface, err := cmd.Result()
				var amount int64 = 0
				if amountInterface != nil {
					amount, _ = amountInterface.(int64)
				}
				return int(amount), err
			}, 120*time.Second, 1*time.Second).
				Should(BeNumerically("<=", totalPorts-takenPorts))
		})

		It("should delete scheduler", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			podsBefore, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, yaml.Name)
			request, err = http.NewRequest("DELETE", url, nil)
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			Eventually(func() int {
				pods, _ := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return len(pods.Items)
			}, 120*time.Second, 1*time.Second).Should(BeZero())

			for _, pod := range podsBefore.Items {
				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				err = room.ClearAll(app.RedisClient)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should update running scheduler", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			}()

			podsBefore, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			newEnvVar := &models.EnvVar{
				Name:  "MY_NEW_ENV_VAR",
				Value: "my_new_env_var",
			}
			yaml.AutoScaling.Min = yaml.AutoScaling.Min + 1
			yaml.Image = "nginx:latest"
			yaml.Env = append(yaml.Env, newEnvVar)
			bts, err := json.Marshal(yaml)
			Expect(err).NotTo(HaveOccurred())

			body := bytes.NewReader(bts)

			url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, yaml.Name)
			request, err = http.NewRequest("PUT", url, body)
			request.Header.Add("Authorization", "Bearer token")
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			nRoomsBefore := len(podsBefore.Items)
			newPodEnvVar := v1.EnvVar{
				Name:  newEnvVar.Name,
				Value: newEnvVar.Value,
			}

			Eventually(func() int {
				pods, _ := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return len(pods.Items)
			}, 120*time.Second, 1*time.Second).Should(Equal(nRoomsBefore + 1))

			Eventually(func() []v1.EnvVar {
				pods, _ := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return pods.Items[0].Spec.Containers[0].Env
			}, 120*time.Second, 1*time.Second).Should(ContainElement(newPodEnvVar))

			Eventually(func() string {
				pods, _ := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
				return pods.Items[0].Spec.Containers[0].Image
			}, 120*time.Second, 1*time.Second).Should(Equal(yaml.Image))

			for _, pod := range podsBefore.Items {
				room := models.NewRoom(pod.GetName(), pod.GetNamespace())
				err = room.ClearAll(app.RedisClient)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})
