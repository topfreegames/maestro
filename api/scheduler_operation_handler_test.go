// maestro
// +build unit
// https://github.com/topfreegames/maestro
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

	goredis "github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
	"k8s.io/api/core/v1"
)

var _ = Describe("SchedulerOperationHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var url string
	var opManager *models.OperationManager
	var schedulerName = "scheduler-name"
	var yamlString = `name: scheduler-name`

	mockGetStatusFromRedis := func(m map[string]string, err error) {
		mockRedisClient.EXPECT().
			HGetAll(opManager.GetOperationKey()).
			Return(goredis.NewStringStringMapResult(m, err))
	}

	createPod := func(name, namespace, version string) {
		pod := &v1.Pod{}
		pod.SetName(name)
		pod.SetNamespace(namespace)
		pod.SetLabels(map[string]string{
			"version": version,
		})
		pod.Status = v1.PodStatus{
			Conditions: []v1.PodCondition{
				{Type: v1.PodReady, Status: v1.ConditionTrue},
			},
		}
		clientset.CoreV1().Pods(schedulerName).Create(pod)
	}

	BeforeEach(func() {
		opManager = models.NewOperationManager(schedulerName, mockRedisClient, logger)

		recorder = httptest.NewRecorder()

		url = fmt.Sprintf("http://%s/scheduler/%s/operations/%s/status",
			app.Address, schedulerName, opManager.GetOperationKey())
		request, _ = http.NewRequest("GET", url, nil)
		request.SetBasicAuth("user", "pass")
	})

	Describe("GET /scheduler/{schedulerName}/operations/{operationKey}/status", func() {
		It("should return status completed if so", func() {
			status := map[string]string{
				"status":   "200",
				"success":  "true",
				"progress": "100%",
			}
			mockGetStatusFromRedis(status, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(Equal(status))
		})

		It("should return progress when not completed", func() {
			status := map[string]string{
				"progress": "running",
			}
			mockGetStatusFromRedis(status, nil)

			// Select current scheduler
			MockSelectScheduler(yamlString, mockDb, nil)

			// Create half of the pods in version v1.0 and half in v2.0
			createPod("pod1", schedulerName, "v1.0")
			createPod("pod2", schedulerName, "v2.0")

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(HaveKeyWithValue("progress", "50.00%"))
		})
	})
})
