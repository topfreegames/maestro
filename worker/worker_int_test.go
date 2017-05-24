// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	mt "github.com/topfreegames/maestro/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
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
	})

	AfterEach(func() {
		for _, watcher := range w.Watchers {
			watcher.Run = false
		}

		svcs, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
		Expect(err).NotTo(HaveOccurred())
		for _, svc := range svcs.Items {
			room := models.NewRoom(svc.GetName(), svc.GetNamespace())
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
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			schedulerNames := []string{yaml.Name}
			w.EnsureRunningWatchers(schedulerNames)

			Expect(w.Watchers).To(HaveKey(schedulerNames[0]))
			Expect(w.Watchers[schedulerNames[0]].SchedulerName).To(Equal(schedulerNames[0]))
			Eventually(func() bool { return w.Watchers[schedulerNames[0]].Run }).Should(BeTrue())
			Eventually(func() int {
				list, _ := clientset.CoreV1().Pods(schedulerNames[0]).List(listOptions)
				return len(list.Items)
			}).Should(Equal(yaml.AutoScaling.Min))
		})
	})

	Describe("RemoveDeadWatchers", func() {
		It("should remove dead watchers", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

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
				svcs, err := clientset.CoreV1().Services(yaml1.Name).List(listOptions)
				Expect(err).NotTo(HaveOccurred())
				for _, svc := range svcs.Items {
					room := models.NewRoom(svc.GetName(), svc.GetNamespace())
					err = room.ClearAll(app.RedisClient)
					Expect(err).NotTo(HaveOccurred())
				}

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
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			jsonStr, err = mt.NextJsonStr()
			Expect(err).NotTo(HaveOccurred())
			recorder = httptest.NewRecorder()
			request, err = http.NewRequest("POST", url, strings.NewReader(jsonStr))
			Expect(err).NotTo(HaveOccurred())
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml1, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				svcs, _ := clientset.CoreV1().Services(yaml.Name).List(listOptions)
				return len(svcs.Items)
			}, 120*time.Second, 1*time.Second).Should(BeNumerically(">=", yaml.AutoScaling.Min))

			Eventually(func() int {
				svcs1, _ := clientset.CoreV1().Services(yaml1.Name).List(listOptions)
				return len(svcs1.Items)
			}, 120*time.Second, 1*time.Second).Should(BeNumerically(">=", yaml1.AutoScaling.Min))
		})

		It("should scale up", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			svcs, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			for _, svc := range svcs.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, svc.GetNamespace(), svc.GetName())
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

			Eventually(func() int {
				svcs, _ = clientset.CoreV1().Services(yaml.Name).List(listOptions)
				return len(svcs.Items)
			}, 120*time.Second, 1*time.Second).Should(Equal(yaml.AutoScaling.Min + yaml.AutoScaling.Up.Delta))
		})

		It("should scale down", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			svcs, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			for _, svc := range svcs.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, svc.GetNamespace(), svc.GetName())
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
					svcs, err = clientset.CoreV1().Services(yaml.Name).List(listOptions)
					Expect(err).NotTo(HaveOccurred())
					if len(svcs.Items) == yaml.AutoScaling.Min+yaml.AutoScaling.Up.Delta {
						break waitForUp
					}
				}
			}

			for _, svc := range svcs.Items {
				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, svc.GetNamespace(), svc.GetName())
				request, err := http.NewRequest("PUT", url, mt.JSONFor(mt.JSON{
					"timestamp": 1000,
					"status":    models.StatusReady,
				}))
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			}

			time.Sleep(time.Duration(yaml.AutoScaling.Up.Cooldown) * time.Second)

			newRoomNumber := yaml.AutoScaling.Min + yaml.AutoScaling.Up.Delta - yaml.AutoScaling.Down.Delta
			if newRoomNumber < yaml.AutoScaling.Min {
				newRoomNumber = yaml.AutoScaling.Min
			}

			Eventually(func() int {
				svcs, _ = clientset.CoreV1().Services(yaml.Name).List(listOptions)
				return len(svcs.Items)
			}, 120*time.Second, 1*time.Second).Should(Equal(newRoomNumber))
		})

		It("should delete scheduler", func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
			request, err := http.NewRequest("POST", url, strings.NewReader(jsonStr))
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			svcsBefore, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, yaml.Name)
			request, err = http.NewRequest("DELETE", url, nil)
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			Eventually(func() int {
				svcs, _ := clientset.CoreV1().Services(yaml.Name).List(listOptions)
				return len(svcs.Items)
			}, 120*time.Second, 1*time.Second).Should(BeZero())

			for _, svc := range svcsBefore.Items {
				room := models.NewRoom(svc.GetName(), svc.GetNamespace())
				err = room.ClearAll(app.RedisClient)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})
