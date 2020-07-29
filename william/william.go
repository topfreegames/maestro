// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package william

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
	"net/http"
	"net/url"
	"strings"
)

type Permission struct {
	Action           string
	IncludeGame      bool
	IncludeScheduler bool
}

type IAMPermission struct {
	Prefix   string `json:"prefix"`
	Alias    string `json:"alias"`
	Complete bool   `json:"complete"`
}

type WilliamAuth struct {
	url         string
	region      string
	iamName     string
	permissions []Permission
	client      *http.Client
}

func NewWilliamAuth(config *viper.Viper, permissions []Permission) *WilliamAuth {
	client := &http.Client{
		Timeout: config.GetDuration("william.checkTimeout"),
	}
	return &WilliamAuth{
		client:      client,
		url:         config.GetString("william.url"),
		region:      config.GetString("william.region"),
		iamName:     config.GetString("william.iamName"),
		permissions: permissions,
	}
}

func (w *WilliamAuth) Permissions(db interfaces.DB, prefix string) ([]IAMPermission, error) {
	if len(w.permissions) == 0 {
		return nil, nil
	}

	schedulers, err := models.LoadAllSchedulers(db)
	if err != nil {
		return nil, err
	}

	allPermissions := buildAllPermissions(w.region, w.permissions, schedulers)
	iamPermissions := make([]IAMPermission, 0)

	for _, p := range allPermissions {
		if strings.HasPrefix(p.Prefix, prefix) && strings.Index(p.Prefix[len(prefix):], "::") < 0 {
			iamPermissions = append(iamPermissions, p)
		}
	}

	return iamPermissions, nil
}

func (w *WilliamAuth) Check(token, permission, resource string) error {
	fullPermission := fmt.Sprintf("%s::RL::%s::%s", w.iamName, permission, w.region)
	if len(resource) > 0 {
		fullPermission = fmt.Sprintf("%s::%s", fullPermission, resource)
	}
	fullUrl := fmt.Sprintf("%s/permissions/has?permission=%s", w.url, url.QueryEscape(fullPermission))
	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return errors.NewGenericError(fmt.Sprintf(`error creating request for permision "%s"`, fullPermission), err)
	}

	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	response, err := w.client.Do(request)
	if err != nil {
		return errors.NewGenericError(fmt.Sprintf(`error making request for permission "%s" on will.iam`, fullPermission), err)
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status == http.StatusOK {
		return nil
	}
	if status == http.StatusForbidden {
		return errors.NewAuthError("user not authorized", fmt.Errorf(`user does not have permission "%s"`, fullPermission))
	}

	return errors.NewAccessError("user not authorized", fmt.Errorf(`user not authorized`))
}

func buildAllPermissions(region string, permissions []Permission, schedulers []models.Scheduler) []IAMPermission {
	games := groupByGames(schedulers)
	iamPermissions := make([]IAMPermission, 0)
	iamPermissions = append(iamPermissions, IAMPermission{
		Prefix:   "*",
		Complete: false,
	})
	iamPermissions = append(iamPermissions, IAMPermission{
		Prefix:   "*::*",
		Complete: true,
	})
	for _, p := range permissions {
		iamPermissions = append(
			iamPermissions,
			IAMPermission{
				Prefix:   p.Action,
				Complete: false,
			},
			IAMPermission{
				Prefix:   fmt.Sprintf("%s::*", p.Action),
				Complete: true,
			},
			IAMPermission{
				Prefix:   fmt.Sprintf("%s::%s", p.Action, region),
				Complete: !p.IncludeGame && !p.IncludeScheduler,
			},
		)
		if p.IncludeGame {
			iamPermissions = append(iamPermissions, IAMPermission{
				Prefix:   fmt.Sprintf("%s::%s::*", p.Action, region),
				Complete: true,
			})
			for g, scheds := range games {
				iamPermissions = append(iamPermissions, IAMPermission{
					Prefix:   fmt.Sprintf("%s::%s::%s", p.Action, region, g),
					Complete: !p.IncludeScheduler,
				})
				if p.IncludeScheduler {
					iamPermissions = append(iamPermissions, IAMPermission{
						Prefix:   fmt.Sprintf("%s::%s::%s::*", p.Action, region, g),
						Complete: true,
					})
					for _, s := range scheds {
						iamPermissions = append(iamPermissions, IAMPermission{
							Prefix:   fmt.Sprintf("%s::%s::%s::%s", p.Action, region, g, s),
							Complete: true,
						})
					}
				}
			}
		}
	}
	return iamPermissions
}

func groupByGames(schedulers []models.Scheduler) map[string][]string {
	games := make(map[string][]string)
	for _, s := range schedulers {
		scheds, ok := games[s.Game]
		if !ok {
			scheds = make([]string, 0, 1)
		}
		games[s.Game] = append(scheds, s.Name)
	}
	return games
}
