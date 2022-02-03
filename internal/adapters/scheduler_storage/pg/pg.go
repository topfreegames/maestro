// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package pg

import (
	"context"
	"strings"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/go-pg/pg"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/filters"
)

var (
	_ ports.SchedulerStorage = (*schedulerStorage)(nil)
)

type schedulerStorage struct {
	db *pg.DB
}

func NewSchedulerStorage(opts *pg.Options) *schedulerStorage {
	return &schedulerStorage{db: pg.Connect(opts)}
}

const (
	queryGetActiveScheduler = `
SELECT
	s.id, s.name, s.game, s.yaml, s.state, s.state_last_changed_at, last_scale_op_at, v.created_at, s.updated_at, s.version,
	v.rolling_update_status, v.rollback_version
FROM schedulers s left join scheduler_versions v ON s.name=v.name
WHERE s.name = ?
order by created_at desc limit 1
`
	queryGetScheduler = `
SELECT
	s.id, s.name, s.game, v.yaml, s.state, s.state_last_changed_at, last_scale_op_at, v.created_at, s.updated_at, v.version,
	v.rolling_update_status, v.rollback_version
FROM schedulers s join scheduler_versions v
	ON s.name=v.name
WHERE s.name = ?`
	queryGetSchedulers    = `SELECT * FROM schedulers WHERE name IN (?)`
	queryGetAllSchedulers = `SELECT * FROM schedulers`
	queryInsertScheduler  = `
INSERT INTO schedulers (name, game, yaml, state, version, state_last_changed_at)
	VALUES (?name, ?game, ?yaml, ?state, ?version, extract(epoch from now()))
	RETURNING id`
	queryUpdateScheduler = `
UPDATE schedulers
	SET (name, game, yaml, state, version, updated_at, state_last_changed_at)
      = (?name, ?game, ?yaml, ?state, ?version, now(), extract(epoch from now()))
	WHERE name=?name`
	queryDeleteScheduler = `DELETE FROM schedulers WHERE name = ?`
	queryInsertVersion   = `
INSERT INTO scheduler_versions (name, version, yaml, rollback_version)
	VALUES (?, ?, ?, ?)
	ON CONFLICT DO NOTHING`
	queryGetSchedulerVersions = ` SELECT version, created_at FROM scheduler_versions WHERE name = ? ORDER BY created_at DESC`
)

func (s schedulerStorage) GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbScheduler Scheduler

	queryString := queryGetActiveScheduler
	_, err := client.QueryOne(&dbScheduler, queryString, name)
	if err == pg.ErrNoRows {
		return nil, errors.NewErrNotFound("scheduler %s not found", name)
	}
	if err != nil {
		return nil, errors.NewErrUnexpected("error getting scheduler %s", name).WithError(err)
	}
	scheduler, err := dbScheduler.ToScheduler()
	if err != nil {
		return nil, errors.NewErrEncoding("error decoding scheduler %s", name).WithError(err)
	}
	return scheduler, nil
}

func (s schedulerStorage) GetSchedulerWithFilter(ctx context.Context, SchedulerFilter *filters.SchedulerFilter) (*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbScheduler Scheduler
	queryString := queryGetScheduler

	var queryArr []interface{}
	if SchedulerFilter.Name == "" {
		return nil, errors.NewErrInvalidArgument("Scheduler need at least a name")
	}
	queryArr = append(queryArr, SchedulerFilter.Name)

	if SchedulerFilter.Version != "" {
		queryString = queryString + " and v.version = ?"
		queryArr = append(queryArr, SchedulerFilter.Version)
	}

	queryString = queryString + " order by created_at desc limit 1"
	_, err := client.QueryOne(&dbScheduler, queryString, queryArr...)
	if err == pg.ErrNoRows {
		return nil, errors.NewErrNotFound("scheduler %s not found", SchedulerFilter.Name)
	}
	if err != nil {
		return nil, errors.NewErrUnexpected("error getting scheduler %s", SchedulerFilter.Name).WithError(err)
	}
	scheduler, err := dbScheduler.ToScheduler()
	if err != nil {
		return nil, errors.NewErrEncoding("error decoding scheduler %s", SchedulerFilter.Name).WithError(err)
	}
	return scheduler, nil
}

func (s schedulerStorage) GetSchedulerVersions(ctx context.Context, name string) ([]*entities.SchedulerVersion, error) {
	client := s.db.WithContext(ctx)
	var dbSchedulerVersions []SchedulerVersion
	_, err := client.Query(&dbSchedulerVersions, queryGetSchedulerVersions, name)
	if len(dbSchedulerVersions) == 0 {
		return nil, errors.NewErrNotFound("scheduler %s not found", name)
	}
	if err != nil {
		return nil, errors.NewErrUnexpected("error getting scheduler versions").WithError(err)
	}
	versions := make([]*entities.SchedulerVersion, len(dbSchedulerVersions))
	for i := range dbSchedulerVersions {
		scheduler := dbSchedulerVersions[i].ToSchedulerVersion()
		versions[i] = scheduler
	}
	return versions, nil
}

func (s schedulerStorage) GetSchedulers(ctx context.Context, names []string) ([]*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbSchedulers []Scheduler
	_, err := client.Query(&dbSchedulers, queryGetSchedulers, pg.In(names))
	if err != nil {
		return nil, errors.NewErrUnexpected("error getting schedulers").WithError(err)
	}
	schedulers := make([]*entities.Scheduler, len(dbSchedulers))
	for i := range dbSchedulers {
		scheduler, err := dbSchedulers[i].ToScheduler()
		if err != nil {
			return nil, errors.NewErrEncoding("error decoding scheduler %s", dbSchedulers[i].Name).WithError(err)
		}
		schedulers[i] = scheduler
	}
	return schedulers, nil
}

func (s schedulerStorage) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbSchedulers []Scheduler
	_, err := client.Query(&dbSchedulers, queryGetAllSchedulers)
	if err != nil {
		return nil, errors.NewErrUnexpected("error getting schedulers").WithError(err)
	}
	schedulers := make([]*entities.Scheduler, len(dbSchedulers))
	for i := range dbSchedulers {
		scheduler, err := dbSchedulers[i].ToScheduler()
		if err != nil {
			return nil, errors.NewErrEncoding("error decoding scheduler %s", dbSchedulers[i].Name).WithError(err)
		}
		schedulers[i] = scheduler
	}
	return schedulers, nil
}

func (s schedulerStorage) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	client := s.db.WithContext(ctx)
	dbScheduler := NewDBScheduler(scheduler)
	_, err := client.Exec(queryInsertScheduler, dbScheduler)
	if err != nil {
		if strings.Contains(err.Error(), "schedulers_name_unique") {
			return errors.NewErrAlreadyExists("error creating scheduler %s: name already exists", dbScheduler.Name)
		}
		return errors.NewErrUnexpected("error creating scheduler %s", dbScheduler.Name).WithError(err)
	}
	err = s.CreateSchedulerVersion(ctx, scheduler)
	if err != nil {
		return errors.NewErrUnexpected("error creating first scheduler version %s", dbScheduler.Name).WithError(err)
	}
	return nil
}

func (s schedulerStorage) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	client := s.db.WithContext(ctx)
	dbScheduler := NewDBScheduler(scheduler)
	_, err := client.ExecOne(queryUpdateScheduler, dbScheduler)
	if err == pg.ErrNoRows {
		return errors.NewErrNotFound("scheduler %s not found", dbScheduler.Name)
	}
	if err != nil {
		return errors.NewErrUnexpected("error updating scheduler %s", dbScheduler.Name).WithError(err)
	}

	return nil
}

func (s schedulerStorage) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	client := s.db.WithContext(ctx)
	_, err := client.ExecOne(queryDeleteScheduler, scheduler.Name)
	if err == pg.ErrNoRows {
		return errors.NewErrNotFound("scheduler %s not found", scheduler.Name)
	}
	if err != nil {
		return errors.NewErrUnexpected("error updating scheduler %s", scheduler.Name).WithError(err)
	}
	return nil
}

func (s schedulerStorage) CreateSchedulerVersion(ctx context.Context, scheduler *entities.Scheduler) error {
	client := s.db.WithContext(ctx)
	dbScheduler := NewDBScheduler(scheduler)
	_, err := client.Exec(queryInsertVersion, dbScheduler.Name, dbScheduler.Version, dbScheduler.Yaml, dbScheduler.RollbackVersion)
	if err != nil {
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			return errors.NewErrUnexpected("error creating version %s for non existent scheduler \"%s\"", dbScheduler.Version, dbScheduler.Name)
		}
		return errors.NewErrUnexpected("error creating scheduler %s version %s", dbScheduler.Name, dbScheduler.Version).WithError(err)
	}
	return nil
}
