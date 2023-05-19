// maestro
//go:build unit
// +build unit

// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package auth_test

import (
	"fmt"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/extensions/pg"
	. "github.com/topfreegames/maestro/api/auth"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/login"
	"net/http"
)

var _ = Describe("oauth", func() {
	var domains = []string{"email.com"}
	Describe("CheckOauthToken", func() {
		It("should return email when token is valid and email has valid domain", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.Header.Add("Authorization", "Bearer token")

			mockDb.EXPECT().
				Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, "token").
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				})

			mockLogin.EXPECT().Authenticate(gomock.Any(), mockDb).Return("user@email.com", http.StatusOK, nil)

			email, err := CheckOauthToken(mockLogin, mockDb, logger, request, domains)
			Expect(err).ToNot(HaveOccurred())
			Expect(email).To(Equal("user@email.com"))
		})

		It("should return AccessError when token is valid and email doest not have a valid domain", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.Header.Add("Authorization", "Bearer token")

			mockDb.EXPECT().
				Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, "token").
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				})

			mockLogin.EXPECT().Authenticate(gomock.Any(), mockDb).Return("user@email2.com", http.StatusOK, nil)

			_, err = CheckOauthToken(mockLogin, mockDb, logger, request, domains)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&errors.AccessError{}))
		})

		It("should return AccessError when token response status is different from 200", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.Header.Add("Authorization", "Bearer token")

			mockDb.EXPECT().
				Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, "token").
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				})

			mockLogin.EXPECT().Authenticate(gomock.Any(), mockDb).Return("user@email.com", http.StatusBadRequest, nil)

			_, err = CheckOauthToken(mockLogin, mockDb, logger, request, domains)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&errors.AccessError{}))
		})

		It("should return an error when Authenticate returns error", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.Header.Add("Authorization", "Bearer token")

			mockDb.EXPECT().
				Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, "token").
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				})

			mockLogin.EXPECT().Authenticate(gomock.Any(), mockDb).Return("user@email.com", http.StatusBadRequest, fmt.Errorf("error"))

			_, err = CheckOauthToken(mockLogin, mockDb, logger, request, domains)
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeAssignableToTypeOf(&errors.AccessError{}))
		})

		It("should return an error when postgres returns error", func() {
			request, err := http.NewRequest("GET", "/scheduler", nil)
			Expect(err).ToNot(HaveOccurred())

			request.Header.Add("Authorization", "Bearer token")

			mockDb.EXPECT().
				Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, "token").
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				}).Return(pg.NewTestResult(nil, 1), fmt.Errorf("error"))

			_, err = CheckOauthToken(mockLogin, mockDb, logger, request, domains)
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeAssignableToTypeOf(&errors.AccessError{}))
		})
	})
})
