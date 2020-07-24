package auth

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/login"
	logininterfaces "github.com/topfreegames/maestro/login/interfaces"
	"net/http"
	"strings"
)

func CheckOauthToken(
	l logininterfaces.Login,
	db interfaces.DB,
	logger logrus.FieldLogger,
	r *http.Request,
	emailDomains []string,
) (string, error) {
	logger.Debug("Checking access token")

	accessToken := r.Header.Get("Authorization")
	accessToken = strings.TrimPrefix(accessToken, "Bearer ")

	token, err := login.GetToken(accessToken, db)
	if err != nil {
		return "", err
	}
	if token.RefreshToken == "" {
		return "", errors.NewAccessError("access token was not found on db", fmt.Errorf("access token error"))
	}

	msg, status, err := l.Authenticate(token, db)
	if err != nil {
		logger.WithError(err).Error("error fetching googleapis")
		return "", errors.NewGenericError("Error fetching googleapis", err)
	}

	if status == http.StatusBadRequest {
		logger.WithError(err).Error("error validating access token")
		return "", errors.NewAccessError("Unauthorized access token", fmt.Errorf(msg))
	}

	if status != http.StatusOK {
		return "", errors.NewAccessError("invalid access token", fmt.Errorf(msg))
	}

	email := msg
	if !VerifyEmailDomain(email, emailDomains) {
		logger.WithError(err).Error("Invalid email")
		err := errors.NewAccessError(
			"authorization access error",
			fmt.Errorf("the email on OAuth authorization is not from domain %s", emailDomains),
		)
		return "", err
	}

	logger.Debug("Access token checked")

	return email, nil
}
