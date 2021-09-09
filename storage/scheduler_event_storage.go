// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package storage

import (
	"github.com/topfreegames/maestro/models"
)

type SchedulerEventStorage interface {
	PersistSchedulerEvent(event *models.SchedulerEvent) error
	LoadSchedulerEvents(schedulerName string, page int) ([]*models.SchedulerEvent, error)
}
