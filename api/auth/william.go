package auth

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/william"
	"net/http"
	"strings"
)

type PermissionResolver interface {
	ResolvePermission(db interfaces.DB, req *http.Request) (permission, resource string, err error)
}

type actionResolver struct {
	action string
}

func (r *actionResolver) ResolvePermission(_ interfaces.DB, _ *http.Request) (permission, resource string, err error) {
	return r.action, "", nil
}

type schedulerPathResolver struct {
	action  string
	varName string
}

func (r *schedulerPathResolver) ResolvePermission(db interfaces.DB, req *http.Request) (permission, resource string, err error) {
	vars := mux.Vars(req)
	schedulerName, ok := vars[r.varName]
	if !ok {
		return "", "", fmt.Errorf(`path variable "%s" not found`, r.varName)
	}

	scheduler := models.NewScheduler(schedulerName, "", "")
	err = scheduler.Load(db)
	if err != nil {
		return "", "", err
	}

	return r.action, fmt.Sprintf("%s::%s", scheduler.Game, scheduler.Name), nil
}

type gameQueryResolver struct {
	action     string
	queryParam string
}

func (r *gameQueryResolver) ResolvePermission(_ interfaces.DB, req *http.Request) (permission, resource string, err error) {
	game := req.URL.Query().Get(r.queryParam)
	if game == "" {
		game = "*"
	}
	return r.action, game, nil
}

func ActionResolver(action string) PermissionResolver {
	return &actionResolver{action}
}

func SchedulerPathResolver(action, varName string) PermissionResolver {
	return &schedulerPathResolver{action, varName}
}

func GameQueryResolver(action, queryParam string) PermissionResolver {
	return &gameQueryResolver{action, queryParam}
}

func CheckWilliamPermission(
	db interfaces.DB,
	logger logrus.FieldLogger,
	w *william.WilliamAuth,
	r *http.Request,
	resolver PermissionResolver,
) error {
	token := r.Header.Get("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	permission, resource, err := resolver.ResolvePermission(db, r)
	if err != nil {
		logger.WithError(err).Error("error resolving permission")
		return errors.NewGenericError("error resolving permission", err)
	}

	err = w.Check(token, permission, resource)
	if err != nil {
		logger.WithError(err).Error("error checking permission")
		return err
	}

	return nil
}
