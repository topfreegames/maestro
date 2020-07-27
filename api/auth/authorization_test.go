// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package auth_test

import (
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/maestro/api/auth"
	"github.com/topfreegames/maestro/testing"
	"net/http"
)

var _ = Describe("oauth", func() {
	admins := []string{"user@email.com"}
	yamlStr := `name: scheduler-name
authorizedUsers:
- scheduler_user@example.com`
	var request *http.Request

	Describe("CheckAuthorization", func() {
		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", "/scheduler/{schedulerName}", nil)
			Expect(err).ToNot(HaveOccurred())

			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})
		})

		It("should return true when email is admin", func() {
			request = request.WithContext(NewContextWithEmail(request.Context(), "user@email.com"))

			authorized, err := CheckAuthorization(mockDb, logger, request, admins)
			Expect(err).ToNot(HaveOccurred())
			Expect(authorized).To(BeTrue())
		})

		It("should return true when email is not admin but is authorized in scheduler", func() {
			request = request.WithContext(NewContextWithEmail(request.Context(), "scheduler_user@example.com"))

			testing.MockSelectScheduler(yamlStr, mockDb, nil)

			authorized, err := CheckAuthorization(mockDb, logger, request, admins)
			Expect(err).ToNot(HaveOccurred())
			Expect(authorized).To(BeTrue())
		})

		It("should return false when email is not admin and is not authorized in scheduler", func() {
			request = request.WithContext(NewContextWithEmail(request.Context(), "user@example.com"))

			testing.MockSelectScheduler(yamlStr, mockDb, nil)

			authorized, err := CheckAuthorization(mockDb, logger, request, admins)
			Expect(err).ToNot(HaveOccurred())
			Expect(authorized).To(BeFalse())
		})

		It("should return error when email is not found on context", func() {
			_, err := CheckAuthorization(mockDb, logger, request, admins)
			Expect(err).To(HaveOccurred())
		})

		It("should return error when postgres returns error", func() {
			request = request.WithContext(NewContextWithEmail(request.Context(), "user@example.com"))

			testing.MockSelectScheduler(yamlStr, mockDb, fmt.Errorf("error"))

			_, err := CheckAuthorization(mockDb, logger, request, admins)
			Expect(err).To(HaveOccurred())
		})
	})
})
