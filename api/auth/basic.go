package auth

import (
	"github.com/spf13/viper"
	"net/http"
)

func CheckBasicAuth(config *viper.Viper, r *http.Request) (authPresent, authValid bool, email string) {
	basicAuthUser := config.GetString("basicauth.username")
	basicAuthPass := config.GetString("basicauth.password")
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false, false, ""
	}
	if basicAuthUser != user || basicAuthPass != pass {
		return true, false, ""
	}

	email = r.Header.Get("x-forwarded-user-email")

	return true, true, email
}
