// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package auth_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/maestro/api/auth"
	"net/http"
)

var _ = Describe("Basic Auth", func() {
	Describe("CheckBasicAuth", func() {
		It("should return AuthenticationOk and email when user and pass are valid and x-forwarded-user-email is sent", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.Header.Add("x-forwarded-user-email", "user@email.com")
			request.SetBasicAuth(config.GetString("basicauth.username"), config.GetString("basicauth.password"))

			result, email := CheckBasicAuth(config, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(AuthenticationOk))
			Expect(email).To(Equal("user@email.com"))
		})

		It("should return AuthenticationOk and empty email when user and pass are valid and x-forwarded-user-email is not sent", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.SetBasicAuth(config.GetString("basicauth.username"), config.GetString("basicauth.password"))

			result, email := CheckBasicAuth(config, request)
			Expect(result).To(Equal(AuthenticationOk))
			Expect(email).To(BeEmpty())
		})

		It("should return AuthenticationInvalid when user and pass are not valid", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.SetBasicAuth("abcd", "1234")

			result, _ := CheckBasicAuth(config, request)
			Expect(result).To(Equal(AuthenticationInvalid))
		})

		It("should return AuthenticationMissing when basic auth is not sent", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			result, _ := CheckBasicAuth(config, request)
			Expect(result).To(Equal(AuthenticationMissing))
		})
	})
})
