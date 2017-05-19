// maestro
// +build integration
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		yaml        *models.ConfigYAML
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
	})

	AfterEach(func() {
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
			err := clientset.CoreV1().Namespaces().Delete(yaml.Name, &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("POST /scheduler", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("http://%s/scheduler", app.Address)
		})

		It("should POST a scheduler", func() {
			body := strings.NewReader(jsonStr)

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(yaml.AutoScaling.Min))

			svcs, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svcs.Items)).To(Equal(yaml.AutoScaling.Min))

			ns := yaml.Name
			sch := &models.Scheduler{Name: ns}
			err = sch.Load(app.DB)
			Expect(err).NotTo(HaveOccurred())
			Expect(sch.YAML).NotTo(HaveLen(0))

			for _, svc := range svcs.Items {
				Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))

				room := models.NewRoom(svc.GetName(), ns)
				err := room.Create(app.RedisClient)
				Expect(err).NotTo(HaveOccurred())

				pipe := app.RedisClient.TxPipeline()
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

		It("should return 422 if missing body parameter", func() {
			body := strings.NewReader("")

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			resp := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveKeyWithValue("code", "MAE-004"))
			Expect(resp).To(HaveKeyWithValue("description", "EOF"))
			Expect(resp).To(HaveKeyWithValue("error", "ValidationFailedError"))
			Expect(resp).To(HaveKeyWithValue("success", false))
			Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

			pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(BeEmpty())

			svcs, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(BeEmpty())
		})

		It("should return code 500 if postgres is down", func() {
			app, err := api.NewApp("0.0.0.0", 9998, config, logger, false, "", nil, nil, clientset)
			Expect(err).NotTo(HaveOccurred())

			err = app.DB.Close()
			Expect(err).NotTo(HaveOccurred())

			body := strings.NewReader(jsonStr)

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			resp := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(resp).To(HaveKeyWithValue("description", "pg: database is closed"))
			Expect(resp).To(HaveKeyWithValue("error", "Create scheduler failed"))
			Expect(resp).To(HaveKeyWithValue("success", false))
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("DELETE /scheduler/{schedulerName}", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("http://%s%s", app.Address, "/scheduler")
		})

		It("should delete a created scheduler", func() {
			then := time.Now()

			// Create the scheduler
			body := strings.NewReader(jsonStr)

			yaml, err = models.NewConfigYAML(jsonStr)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", url, body)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			svcsBefore, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())

			// Delete the scheduler
			recorder = httptest.NewRecorder()
			url := fmt.Sprintf("%s/%s", url, yaml.Name)
			request, err = http.NewRequest("DELETE", url, body)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))

			pods, err := clientset.CoreV1().Pods(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(BeEmpty())

			svcs, err := clientset.CoreV1().Services(yaml.Name).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(svcs.Items).To(BeEmpty())

			ns := yaml.Name
			nameSpace := models.NewNamespace(ns)
			exists, err := nameSpace.Exists(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			sch := &models.Scheduler{Name: ns}
			err = sch.Load(app.DB)
			Expect(err).NotTo(HaveOccurred())
			Expect(sch.YAML).To(HaveLen(0))

			for _, svc := range svcsBefore.Items {
				room := models.NewRoom(svc.GetName(), ns)

				url = fmt.Sprintf("http://%s/scheduler/%s/rooms/%s/status", app.Address, sch.Name, room.ID)
				request, err := http.NewRequest("PUT", url, mt.JSONFor(mt.JSON{
					"timestamp": time.Now().Sub(then),
					"status":    models.StatusTerminated,
				}))
				Expect(err).NotTo(HaveOccurred())
				recorder = httptest.NewRecorder()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))

				pipe := app.RedisClient.TxPipeline()
				roomStatus := pipe.HExists(room.GetRoomRedisKey(), "status")
				roomLastPing := pipe.HExists(room.GetRoomRedisKey(), "lastPing")
				roomIsCreating := pipe.SIsMember(models.GetRoomStatusSetRedisKey(ns, room.Status), room.GetRoomRedisKey())
				roomLastPingScore := pipe.ZScore(models.GetRoomPingRedisKey(ns), room.ID)

				_, err = pipe.Exec()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("redis: nil"))

				exists, err := roomStatus.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())

				exists, err = roomLastPing.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())

				exists, err = roomIsCreating.Result()
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())

				_, err = roomLastPingScore.Result()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("redis: nil"))
			}
		})
	})
})
