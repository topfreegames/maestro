// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package login_test

import (
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/topfreegames/maestro/login"
	"golang.org/x/oauth2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Token", func() {
	Describe("SaveToken", func() {
		var (
			token          *oauth2.Token
			keyAccessToken string
			email          string
			refreshToken   string
			accessToken    string
		)

		BeforeEach(func() {
			refreshToken = "refresh-token"
			accessToken = "access-token"
			token = &oauth2.Token{
				RefreshToken: refreshToken,
				AccessToken:  accessToken,
			}
			keyAccessToken = accessToken
			email = "user@example.com"
		})

		It("should insert token if refresh token is not empty", func() {
			mockDb.EXPECT().Query(gomock.Any(), `INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email)
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				key_access_token = users.key_access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry`, gomock.Any())

			err := SaveToken(token, email, keyAccessToken, mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if db fails", func() {
			mockDb.EXPECT().Query(gomock.Any(), `INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email)
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				key_access_token = users.key_access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry`, gomock.Any()).Return(nil, errors.New("db error"))

			err := SaveToken(token, email, keyAccessToken, mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("db error"))
		})

		It("should update user if refresh token is empty", func() {
			mockDb.EXPECT().Query(gomock.Any(), `UPDATE users
		SET access_token = ?access_token,
				expiry = ?expiry
		WHERE email = ?email`, gomock.Any())

			token := &oauth2.Token{
				RefreshToken: "",
				AccessToken:  accessToken,
			}
			err := SaveToken(token, email, keyAccessToken, mockDb)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetToken", func() {
		It("should return token", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any())

			_, err := GetToken("access-token", mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if db fails", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).Return(nil, errors.New("db error"))

			_, err := GetToken("access-token", mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("db error"))
		})
	})
})
