// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/topfreegames/maestro/models"
	"net/http"
)

type PermissionResolver interface {
	ResolvePermission(app *App, req *http.Request) (permission, resource string, err error)
}

type actionResolver struct {
	action string
}

func (r actionResolver) ResolvePermission(app *App, req *http.Request) (permission, resource string, err error) {
	return r.action, "", nil
}

type schedulerPathResolver struct {
	action  string
	varName string
}

func (r schedulerPathResolver) ResolvePermission(app *App, req *http.Request) (permission, resource string, err error) {
	vars := mux.Vars(req)
	schedulerName, ok := vars[r.varName]
	if !ok {
		return "", "", fmt.Errorf(`path variable "%s" not found`, r.varName)
	}

	ctx := req.Context()
	db := app.DBClient.WithContext(ctx)

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

func (r gameQueryResolver) ResolvePermission(app *App, req *http.Request) (permission, resource string, err error) {
	game := req.URL.Query().Get(r.queryParam)
	if game == "" {
		game = "*"
	}
	return r.action, game, nil
}

func ActionResolver(action string) PermissionResolver {
	return actionResolver{action}
}

func SchedulerPathResolver(action, varName string) PermissionResolver {
	return schedulerPathResolver{action, varName}
}

func GameQueryResolver(action, queryParam string) PermissionResolver {
	return gameQueryResolver{action, queryParam}
}
