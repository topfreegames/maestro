package auth

import (
	"github.com/spf13/viper"
	"net/http"
)

func BasicAuthEnabled(config *viper.Viper) bool {
	basicAuthUser := config.GetString("basicauth.username")
	basicAuthPass := config.GetString("basicauth.password")
	return basicAuthUser != "" && basicAuthPass != ""
}

func CheckBasicAuth(config *viper.Viper, r *http.Request) (AuthenticationResult, string) {
	basicAuthUser := config.GetString("basicauth.username")
	basicAuthPass := config.GetString("basicauth.password")
	user, pass, ok := r.BasicAuth()
	if !ok {
		return AuthenticationMissing, ""
	}
	if basicAuthUser != user || basicAuthPass != pass {
		return AuthenticationInvalid, ""
	}

	email := r.Header.Get("x-forwarded-user-email")

	return AuthenticationOk, email
}
