// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package william

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/pg/interfaces"
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
	Prefix   string
	Alias    string
	Complete bool
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

func (w *WilliamAuth) Check(token, permission, resource string) (bool, error) {
	fullPermission := fmt.Sprintf("%s::RL::%s::%s", w.iamName, permission, w.region)
	if len(resource) > 0 {
		fullPermission = fmt.Sprintf("%s::%s", fullPermission, resource)
	}
	fullUrl := fmt.Sprintf("%s/permissions/has?permission=%s", w.url, url.QueryEscape(fullPermission))
	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return false, errors.Wrapf(err, `error creating request for permision "%s"`, fullPermission)
	}

	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	response, err := w.client.Do(request)
	if err != nil {
		return false, errors.Wrapf(err, `error making request for permission "%s" on will.iam`, fullPermission)
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status == http.StatusOK {
		return true, nil
	}
	if status == http.StatusForbidden {
		return false, nil
	}

	return false, fmt.Errorf(`will.iam returned invalid status code %d for permission "%s"`, status, fullPermission)
}

func buildAllPermissions(region string, permissions []Permission, schedulers []models.Scheduler) []IAMPermission {
	games := groupByGames(schedulers)
	iamPermissions := make([]IAMPermission, 0)
	iamPermissions = append(iamPermissions, IAMPermission{
		Prefix:   "*",
		Complete: false,
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
