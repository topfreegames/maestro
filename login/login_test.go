// maestro api
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package login_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/topfreegames/maestro/login"
	"golang.org/x/oauth2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", func() {
	var login *Login

	BeforeEach(func() {
		os.Setenv("MAESTRO_GOOGLE_CLIENT_ID", "value")
		os.Setenv("MAESTRO_GOOGLE_CLIENT_SECRET", "value")

		login = NewLogin()
		login.Setup()
	})

	AfterEach(func() {
		os.Unsetenv("MAESTRO_GOOGLE_CLIENT_ID")
		os.Unsetenv("MAESTRO_GOOGLE_CLIENT_SECRET")
	})

	Describe("Setup", func() {
		It("should setup", func() {
			Expect(login.Setup).NotTo(Panic())
		})
	})

	Describe("GenerateLoginURL", func() {
		It("should return error if ClientSecret is not set", func() {
			os.Unsetenv("MAESTRO_GOOGLE_CLIENT_SECRET")
			_, err := login.GenerateLoginURL("state")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Define your app's OAuth2 Client Secret on MAESTRO_GOOGLE_CLIENT_SECRET environment variable and run again"))
		})

		It("should return error if ClientID is not set", func() {
			os.Unsetenv("MAESTRO_GOOGLE_CLIENT_ID")
			_, err := login.GenerateLoginURL("state")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Define your app's OAuth2 Client ID on MAESTRO_GOOGLE_CLIENT_ID environment variable and run again"))
		})

		It("should return correct url", func() {
			login.GoogleOauthConfig = mockOauth
			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			mockOauth.EXPECT().
				AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce).
				Return("oauth.com")

			url, err := login.GenerateLoginURL("state")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("oauth.com"))
		})
	})

	Describe("GetAccessToken", func() {
		It("should return token", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}
			mockOauth.EXPECT().Exchange(oauth2.NoContext, "code", "").Return(token, nil)

			newToken, err := login.GetAccessToken("code", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(newToken).To(Equal(token))
		})

		It("should return error if exchange fails", func() {
			login.GoogleOauthConfig = mockOauth
			mockOauth.EXPECT().Exchange(oauth2.NoContext, "code", "").Return(nil, errors.New("exchange error"))

			_, err := login.GetAccessToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("exchange error"))
		})

		It("should return error if token is invalid", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{}
			mockOauth.EXPECT().Exchange(oauth2.NoContext, "code", "").Return(token, nil)

			_, err := login.GetAccessToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid token received from Authorization Server"))
		})
	})

	Describe("Authenticate", func() {
		It("should authenticate still valid token", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			mockOauth.EXPECT().Client(oauth2.NoContext, token).Return(mockClient)
			body := &http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"email": "user@example.com"}`)),
				StatusCode: http.StatusOK,
			}
			mockClient.EXPECT().Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=access-token").Return(body, nil)

			email, status, err := login.Authenticate(token, mockDb)
			Expect(email).To(Equal("user@example.com"))
			Expect(status).To(Equal(http.StatusOK))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if credentials are not defined", func() {
			os.Unsetenv("MAESTRO_GOOGLE_CLIENT_ID")
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			_, _, err := login.Authenticate(token, mockDb)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if status is not 200", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			mockOauth.EXPECT().Client(oauth2.NoContext, token).Return(mockClient)
			body := &http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString("error")),
				StatusCode: http.StatusBadRequest,
			}
			mockClient.EXPECT().Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=access-token").Return(body, nil)

			msg, status, err := login.Authenticate(token, mockDb)
			Expect(msg).To(Equal("error"))
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if get request fails", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			mockOauth.EXPECT().Client(oauth2.NoContext, token).Return(mockClient)
			mockClient.EXPECT().Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=access-token").Return(nil, errors.New("error"))

			_, _, err := login.Authenticate(token, mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error"))
		})

		It("should return new token if old one is expired", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(-1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}
			newToken := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			mockOauth.EXPECT().Client(oauth2.NoContext, newToken).Return(mockClient)
			body := &http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"email": "user@example.com"}`)),
				StatusCode: http.StatusOK,
			}
			mockClient.EXPECT().Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=access-token").Return(body, nil)

			tokenSource := &MyTokenSource{
				MyToken: newToken,
				Err:     nil,
			}
			mockOauth.EXPECT().TokenSource(oauth2.NoContext, token).Return(tokenSource)

			mockDb.EXPECT().Query(gomock.Any(), `UPDATE users
		SET access_token = ?access_token,
				expiry = ?expiry
		WHERE email = ?email`, gomock.Any())

			_, _, err := login.Authenticate(token, mockDb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if fails to save new token on db", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(-1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}
			newToken := &oauth2.Token{
				Expiry:       time.Now().Add(1 * time.Hour),
				RefreshToken: "",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			mockOauth.EXPECT().Client(oauth2.NoContext, newToken).Return(mockClient)
			body := &http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"email": "user@example.com"}`)),
				StatusCode: http.StatusOK,
			}
			mockClient.EXPECT().Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=access-token").Return(body, nil)

			tokenSource := &MyTokenSource{
				MyToken: newToken,
				Err:     nil,
			}
			mockOauth.EXPECT().TokenSource(oauth2.NoContext, token).Return(tokenSource)

			mockDb.EXPECT().Query(gomock.Any(), `UPDATE users
		SET access_token = ?access_token,
				expiry = ?expiry
		WHERE email = ?email`, gomock.Any()).Return(nil, errors.New("error"))

			_, _, err := login.Authenticate(token, mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error"))
		})

		It("should return if tokenSource fails", func() {
			login.GoogleOauthConfig = mockOauth
			token := &oauth2.Token{
				Expiry:       time.Now().Add(-1 * time.Hour),
				RefreshToken: "refresh-token",
				AccessToken:  "access-token",
				TokenType:    "Bearer",
			}

			mockOauth.EXPECT().SetClientID(gomock.Any())
			mockOauth.EXPECT().SetClientSecret(gomock.Any())
			tokenSource := &MyTokenSource{
				MyToken: nil,
				Err:     errors.New("error"),
			}
			mockOauth.EXPECT().TokenSource(oauth2.NoContext, token).Return(tokenSource)

			_, _, err := login.Authenticate(token, mockDb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error"))
		})
	})
})
