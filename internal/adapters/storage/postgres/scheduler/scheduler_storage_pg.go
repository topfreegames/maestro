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

package scheduler

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/go-pg/pg/extra/pgotel/v10"
	"github.com/go-pg/pg/v10"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/filters"
)

var (
	_ ports.SchedulerStorage = (*schedulerStorage)(nil)
)

type schedulerStorage struct {
	db              *pg.DB
	transactionsMap map[ports.TransactionID]*pg.Tx
}

func NewSchedulerStorage(opts *pg.Options) *schedulerStorage {
	db := pg.Connect(opts)
	return &schedulerStorage{
		db:              db,
		transactionsMap: map[ports.TransactionID]*pg.Tx{},
	}
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
	queryGetSchedulers       = `SELECT * FROM schedulers WHERE name IN (?)`
	querySimpleGetSchedulers = `SELECT * FROM schedulers`
	queryGetAllSchedulers    = `SELECT * FROM schedulers`
	queryInsertScheduler     = `
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
	queryGetSchedulerVersions = `
SELECT v.version, v.version = s.version as is_active, v.created_at
	FROM scheduler_versions v
	INNER JOIN schedulers s
	ON v.name = s.name
	WHERE v.name = ? ORDER BY created_at DESC`
)

func (s schedulerStorage) EnableTracing() {
	s.db.AddQueryHook(pgotel.NewTracingHook())
}

func (s schedulerStorage) GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbScheduler Scheduler

	queryString := queryGetActiveScheduler

	var err error
	runSchedulerStorageFunctionCollectingLatency("GetScheduler", func() {
		_, err = client.QueryOne(&dbScheduler, queryString, name)
	})
	if err == pg.ErrNoRows {
		return nil, errors.NewErrNotFound("scheduler %s not found", name)
	}
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("GetScheduler", name)
		return nil, errors.NewErrUnexpected("error getting scheduler %s", name).WithError(err)
	}
	scheduler, err := dbScheduler.ToScheduler()
	if err != nil {
		return nil, errors.NewErrEncoding("error decoding scheduler %s", name).WithError(err)
	}
	return scheduler, nil
}

func (s schedulerStorage) GetSchedulerWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) (*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbScheduler Scheduler
	queryString := queryGetScheduler

	var queryArr []interface{}
	if schedulerFilter.Name == "" {
		return nil, errors.NewErrInvalidArgument("Scheduler need at least a name")
	}
	queryArr = append(queryArr, schedulerFilter.Name)

	if schedulerFilter.Version != "" {
		queryString = queryString + " and v.version = ?"
		queryArr = append(queryArr, schedulerFilter.Version)
	}

	queryString = queryString + " order by created_at desc limit 1"

	var err error
	runSchedulerStorageFunctionCollectingLatency("GetSchedulerWithFilter", func() {
		_, err = client.QueryOne(&dbScheduler, queryString, queryArr...)
	})
	if err == pg.ErrNoRows {
		return nil, errors.NewErrNotFound("scheduler %s not found", schedulerFilter.Name)
	}
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("GetSchedulerWithFilter", schedulerFilter.Name)
		return nil, errors.NewErrUnexpected("error getting scheduler %s", schedulerFilter.Name).WithError(err)
	}
	scheduler, err := dbScheduler.ToScheduler()
	if err != nil {
		return nil, errors.NewErrEncoding("error decoding scheduler %s", schedulerFilter.Name).WithError(err)
	}
	return scheduler, nil
}

func (s schedulerStorage) GetSchedulerVersions(ctx context.Context, name string) ([]*entities.SchedulerVersion, error) {
	client := s.db.WithContext(ctx)
	var dbSchedulerVersions []SchedulerVersion
	var err error
	runSchedulerStorageFunctionCollectingLatency("GetSchedulerVersions", func() {
		_, err = client.Query(&dbSchedulerVersions, queryGetSchedulerVersions, name)
	})
	if len(dbSchedulerVersions) == 0 {
		return nil, errors.NewErrNotFound("scheduler %s not found", name)
	}
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("GetSchedulerVersion", name)
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
	var err error
	runSchedulerStorageFunctionCollectingLatency("GetSchedulers", func() {
		_, err = client.Query(&dbSchedulers, queryGetSchedulers, pg.In(names))
	})
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("GetSchedulers")
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

func (s schedulerStorage) GetSchedulersWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) ([]*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbSchedulers []Scheduler
	queryString := querySimpleGetSchedulers

	whereClauses, arrayValues := createWhereClauses(schedulerFilter)

	queryString = queryString + whereClauses

	var err error
	runSchedulerStorageFunctionCollectingLatency("GetSchedulersWithFilter", func() {
		_, err = client.Query(&dbSchedulers, queryString, arrayValues...)
	})
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("GetSchedulersWithFilter")
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

func createWhereClauses(schedulerFilter *filters.SchedulerFilter) (string, []interface{}) {
	clauses := ""
	values := make([]interface{}, 0, 3)

	if schedulerFilter != nil {
		thereIsOne := false
		if schedulerFilter.Game != "" {
			clauses = clauses + " game = ?"
			values = append(values, schedulerFilter.Game)
			thereIsOne = true
		}

		if schedulerFilter.Name != "" {
			if thereIsOne {
				clauses = clauses + " and"
			}

			clauses = clauses + " name = ?"
			values = append(values, schedulerFilter.Name)
			thereIsOne = true
		}

		if schedulerFilter.Version != "" {
			if thereIsOne {
				clauses = clauses + " and"
			}

			clauses = clauses + " version = ?"
			values = append(values, schedulerFilter.Version)
			thereIsOne = true
		}

		if thereIsOne {
			clauses = " WHERE" + clauses
		}
	}

	return clauses, values
}

func (s schedulerStorage) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	client := s.db.WithContext(ctx)
	var dbSchedulers []Scheduler
	var err error
	runSchedulerStorageFunctionCollectingLatency("GetAllSchedulers", func() {
		_, err = client.Query(&dbSchedulers, queryGetAllSchedulers)
	})
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("GetAllSchedulers")
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
	var err error
	runSchedulerStorageFunctionCollectingLatency("CreateScheduler", func() {
		_, err = client.Exec(queryInsertScheduler, dbScheduler)
	})
	if err != nil {
		if strings.Contains(err.Error(), "schedulers_name_unique") {
			return errors.NewErrAlreadyExists("error creating scheduler %s: name already exists", dbScheduler.Name)
		}
		reportSchedulerStorageFailsCounterMetric("CreateScheduler", scheduler.Name)
		return errors.NewErrUnexpected("error creating scheduler %s", dbScheduler.Name).WithError(err)
	}
	err = s.CreateSchedulerVersion(ctx, "", scheduler)
	if err != nil {
		return errors.NewErrUnexpected("error creating first scheduler version %s", dbScheduler.Name).WithError(err)
	}
	return nil
}

func (s schedulerStorage) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	client := s.db.WithContext(ctx)
	dbScheduler := NewDBScheduler(scheduler)
	var err error
	runSchedulerStorageFunctionCollectingLatency("UpdateScheduler", func() {
		_, err = client.ExecOne(queryUpdateScheduler, dbScheduler)
	})
	if err == pg.ErrNoRows {
		return errors.NewErrNotFound("scheduler %s not found", dbScheduler.Name)
	}
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("UpdateScheduler", dbScheduler.Name)
		return errors.NewErrUnexpected("error updating scheduler %s", dbScheduler.Name).WithError(err)
	}

	return nil
}

func (s schedulerStorage) DeleteScheduler(ctx context.Context, transactionID ports.TransactionID, scheduler *entities.Scheduler) error {
	var err error

	if isInTransactionalContext(transactionID) {
		txClient, ok := s.transactionsMap[transactionID]
		if !ok {
			return errors.NewErrNotFound("transaction %s not found", transactionID)
		}
		runSchedulerStorageFunctionCollectingLatency("DeleteScheduler", func() {
			_, err = txClient.Exec(queryDeleteScheduler, scheduler.Name)
		})

	} else {
		client := s.db.WithContext(ctx)
		runSchedulerStorageFunctionCollectingLatency("DeleteScheduler", func() {
			_, err = client.ExecOne(queryDeleteScheduler, scheduler.Name)
		})
	}

	if err == pg.ErrNoRows {
		return errors.NewErrNotFound("scheduler %s not found", scheduler.Name)
	}
	if err != nil {
		reportSchedulerStorageFailsCounterMetric("DeleteScheduler", scheduler.Name)
		return errors.NewErrUnexpected("error deleting scheduler %s", scheduler.Name).WithError(err)
	}
	return nil
}

func (s schedulerStorage) CreateSchedulerVersion(ctx context.Context, transactionID ports.TransactionID, scheduler *entities.Scheduler) error {
	dbScheduler := NewDBScheduler(scheduler)
	var err error
	if isInTransactionalContext(transactionID) {
		txClient, ok := s.transactionsMap[transactionID]
		if !ok {
			return errors.NewErrNotFound("transaction %s not found", transactionID)
		}
		runSchedulerStorageFunctionCollectingLatency("CreateSchedulerVersion", func() {
			_, err = txClient.Exec(queryInsertVersion, dbScheduler.Name, dbScheduler.Version, dbScheduler.Yaml, dbScheduler.RollbackVersion)
		})

	} else {
		client := s.db.WithContext(ctx)
		runSchedulerStorageFunctionCollectingLatency("CreateSchedulerVersion", func() {
			_, err = client.Exec(queryInsertVersion, dbScheduler.Name, dbScheduler.Version, dbScheduler.Yaml, dbScheduler.RollbackVersion)
		})
	}

	if err != nil {
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			return errors.NewErrUnexpected("error creating version %s for non existent scheduler \"%s\"", dbScheduler.Version, dbScheduler.Name)
		}
		reportSchedulerStorageFailsCounterMetric("CreateSchedulerVersion", scheduler.Name)
		return errors.NewErrUnexpected("error creating scheduler %s version %s", dbScheduler.Name, dbScheduler.Version).WithError(err)
	}
	return nil
}

func (s schedulerStorage) RunWithTransaction(ctx context.Context, transactionFunc func(transactionId ports.TransactionID) error) error {
	client := s.db.WithContext(ctx)

	transaction, err := client.Begin()
	if err != nil {
		return errors.NewErrUnexpected("error starting transaction").WithError(err)
	}

	transactionID := ports.TransactionID(uuid.New().String())
	s.transactionsMap[transactionID] = transaction

	defer func() {
		if err := recover(); err != nil {
			s.rollbackTransaction(transaction, transactionID)
			panic(err)
		}
	}()

	if err = transactionFunc(transactionID); err != nil {
		s.rollbackTransaction(transaction, transactionID)
		return err
	}
	return s.commitTransaction(transaction, transactionID)
}

func (s schedulerStorage) rollbackTransaction(transaction *pg.Tx, transactionID ports.TransactionID) {
	delete(s.transactionsMap, transactionID)
	_ = transaction.Rollback()
}

func (s schedulerStorage) commitTransaction(transaction *pg.Tx, transactionID ports.TransactionID) error {
	delete(s.transactionsMap, transactionID)
	return transaction.Commit()
}

func isInTransactionalContext(transactionID ports.TransactionID) bool {
	return transactionID != ""
}
