// maestro
//go:build unit
// +build unit

// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"golang.org/x/oauth2"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/login"
)

var _ = Describe("Login", func() {
	var recorder *httptest.ResponseRecorder
	var request *http.Request
	var err error

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
	})

	Context("LoginUrlHandler", func() {
		var state string

		BeforeEach(func() {
			state = "some-str"
			url := fmt.Sprintf("http://%s/login?state=%s", app.Address, state)
			request, err = http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return correct url", func() {
			mockLogin.EXPECT().GenerateLoginURL(state).
				Return("url.example.com", nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Body.String()).To(Equal(`{"url":"url.example.com"}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should return error if state is not informed", func() {
			url := fmt.Sprintf("http://%s/login", app.Address)
			request, err = http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))

			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(body).To(HaveKeyWithValue("description", "state must not be empty"))
			Expect(body).To(HaveKeyWithValue("error", "state must not be empty"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})

		It("should return status 500 if failed to generate url", func() {
			mockLogin.EXPECT().GenerateLoginURL(state).
				Return("", errors.New("error on generation"))

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(body).To(HaveKeyWithValue("description", "error on generation"))
			Expect(body).To(HaveKeyWithValue("error", "undefined env vars"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})
	})

	Context("LoginAccessHandler", func() {
		var code string

		BeforeEach(func() {
			code = "some-code"
			url := fmt.Sprintf("http://%s/access?code=%s", app.Address, code)
			request, err = http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return access token", func() {
			token := &oauth2.Token{
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
			}
			mockLogin.EXPECT().GetAccessToken(code, "").Return(token, nil)
			mockLogin.EXPECT().Authenticate(token, app.DBClient.DB).Return("user@example.com", 0, nil)
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			mockDb.EXPECT().Query(
				gomock.Any(),
				`INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email)
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				key_access_token = users.key_access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry`,
				gomock.Any(),
			)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT key_access_token FROM users WHERE email = ?",
				gomock.Any(),
			).Do(func(user *login.User, query string, modifier string) {
				user.KeyAccessToken = token.AccessToken
			})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			body := fmt.Sprintf(`{"token": "%s"}`, token.AccessToken)
			Expect(recorder.Body.String()).To(Equal(body))
		})

		It("should return status code 400 if code is not sent", func() {
			url := fmt.Sprintf("http://%s/access", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))

			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(body).To(HaveKeyWithValue("description", "code must not be empty"))
			Expect(body).To(HaveKeyWithValue("error", "code must not be empty"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})

		It("should return status code 400 if access token failed with error", func() {
			mockLogin.EXPECT().GetAccessToken(code, "").Return(nil, errors.New("token error"))

			url := fmt.Sprintf("http://%s/access?code=%s", app.Address, code)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))

			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(body).To(HaveKeyWithValue("description", "failed to get access token"))
			Expect(body).To(HaveKeyWithValue("error", "failed to get access token"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})

		It("should return status code 401 if email is not valid", func() {
			token := &oauth2.Token{
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
			}
			mockLogin.EXPECT().GetAccessToken(code, "").Return(token, nil)
			mockLogin.EXPECT().Authenticate(token, app.DBClient.DB).Return("user@invalidemail.com", 0, nil)
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))

			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-005"))
			Expect(body).To(HaveKeyWithValue("description", "the email on OAuth authorization is not from domain [example.com other.com]"))
			Expect(body).To(HaveKeyWithValue("error", "authorization access error"))
		})

		It("should return status code 400 if get access token on db", func() {
			token := &oauth2.Token{
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
			}
			mockLogin.EXPECT().GetAccessToken(code, "").Return(token, nil)
			mockLogin.EXPECT().Authenticate(token, app.DBClient.DB).Return("user@example.com", 0, nil)
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			mockDb.EXPECT().Query(
				gomock.Any(),
				`INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email)
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				key_access_token = users.key_access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry`,
				gomock.Any(),
			).Return(nil, errors.New("get access token error"))

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))

			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-001"))
			Expect(body).To(HaveKeyWithValue("description", "get access token error"))
			Expect(body).To(HaveKeyWithValue("error", "DatabaseError"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})
	})

	Context("LoginAccessHandler with redirect_uri", func() {
		It("asdf", func() {
			code := "some-code"
			redirectURI := "redirect.uri.com"
			url := fmt.Sprintf("http://%s/access?code=%s&redirect_uri=%s", app.Address, code, redirectURI)
			request, err = http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			token := &oauth2.Token{
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
			}
			mockLogin.EXPECT().GetAccessToken(code, redirectURI).Return(token, nil)
			mockLogin.EXPECT().Authenticate(token, app.DBClient.DB).Return("user@example.com", 0, nil)
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			mockDb.EXPECT().Query(
				gomock.Any(),
				`INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email)
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				key_access_token = users.key_access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry`,
				gomock.Any(),
			)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT key_access_token FROM users WHERE email = ?",
				gomock.Any(),
			).Do(func(user *login.User, query string, modifier string) {
				user.KeyAccessToken = token.AccessToken
			})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			body := fmt.Sprintf(`{"token": "%s"}`, token.AccessToken)
			Expect(recorder.Body.String()).To(Equal(body))
		})
	})
})
