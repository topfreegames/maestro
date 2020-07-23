package auth

import (
	e "errors"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
	"net/http"
)

func CheckAuthorization(db interfaces.DB, logger logrus.FieldLogger, r *http.Request, admins []string) (bool, error) {
	logger.Debug("checking auth")
	email := EmailFromContext(r.Context())
	if email == "" {
		return false, errors.NewAccessError("user not sent",
			e.New("user is empty (not using basicauth nor oauth) and auth is required"))
	}

	if !IsAuth(email, admins) {
		schedulerName := mux.Vars(r)["schedulerName"]
		scheduler := models.NewScheduler(schedulerName, "", "")
		err := scheduler.Load(db)
		if err != nil {
			return false, err
		}

		configYaml, _ := models.NewConfigYAML(scheduler.YAML)
		if !IsAuth(email, configYaml.AuthorizedUsers) {
			return false, nil
		}
	}

	logger.Debug("authorized user")

	return true, nil
}
