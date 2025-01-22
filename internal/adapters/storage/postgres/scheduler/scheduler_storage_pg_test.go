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

//go:build integration
// +build integration

package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-pg/pg/v10"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

type dbSchedulerVersion struct {
	ID                  string      `db:"id"`
	Name                string      `db:"name"`
	Yaml                string      `db:"yaml"`
	Version             string      `db:"version"`
	CreatedAt           pg.NullTime `db:"created_at"`
	RollbackVersion     string      `db:"rollback_version"`
	RollingUpdateStatus string      `db:"rolling_update_status"`
}

var expectedScheduler = &entities.Scheduler{
	Name:            "scheduler",
	Game:            "game",
	State:           entities.StateCreating,
	MaxSurge:        "10%",
	RollbackVersion: "",
	Spec: game_room.Spec{
		Version:                "v1",
		TerminationGracePeriod: 60,
		Containers:             []game_room.Container{},
		Toleration:             "toleration",
		Affinity:               "affinity",
	},
	PortRange: &port.PortRange{
		Start: 40000,
		End:   60000,
	},
}

func TestSchedulerStorage_GetScheduler(t *testing.T) {
	t.Run("scheduler exists and is valid", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)

		actualScheduler, err := storage.GetScheduler(context.Background(), "scheduler")
		require.NoError(t, err)
		assertSchedulers(t, []*entities.Scheduler{expectedScheduler}, []*entities.Scheduler{actualScheduler})
	})

	t.Run("scheduler exists and was created before multiple matches", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)

		actualScheduler, err := storage.GetScheduler(context.Background(), "scheduler")
		require.NoError(t, err)
		assert.NotNil(t, actualScheduler.MatchAllocation)
	})

	t.Run("scheduler does not exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		_, err := storage.GetScheduler(context.Background(), "scheduler")
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})

	t.Run("invalid scheduler", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		_, err := db.Exec("UPDATE schedulers SET yaml = 'invalid yaml' WHERE name = 'scheduler' and version = 'v1' ")
		require.NoError(t, err)
		_, err = db.Exec("UPDATE scheduler_versions SET yaml = 'invalid yaml' WHERE name = 'scheduler' and version = 'v1' ")
		require.NoError(t, err)

		_, err = storage.GetScheduler(context.Background(), "scheduler")
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrEncoding, err)
	})
}

func TestSchedulerStorage_GetSchedulers(t *testing.T) {
	db := getPostgresDB(t)
	storage := NewSchedulerStorage(db.Options())
	scheduler1 := &entities.Scheduler{
		Name:            "scheduler-1",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	scheduler2 := &entities.Scheduler{
		Name:            "scheduler-2",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}

	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler1))
	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler2))

	actualSchedulers, err := storage.GetSchedulers(context.Background(), []string{"scheduler-1", "scheduler-2"})
	require.NoError(t, err)

	assertSchedulers(t, []*entities.Scheduler{scheduler1, scheduler2}, actualSchedulers)

	actualSchedulers, err = storage.GetSchedulers(context.Background(), []string{"scheduler-1", "scheduler-3"})
	require.NoError(t, err)
	assertSchedulers(t, []*entities.Scheduler{scheduler1}, actualSchedulers)

	actualSchedulers, err = storage.GetSchedulers(context.Background(), []string{"scheduler-3", "scheduler-4"})
	require.NoError(t, err)
	assertSchedulers(t, []*entities.Scheduler{}, actualSchedulers)
}

func TestSchedulerStorage_GetAllSchedulers(t *testing.T) {
	db := getPostgresDB(t)
	storage := NewSchedulerStorage(db.Options())
	scheduler1 := &entities.Scheduler{
		Name:            "scheduler-1",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	scheduler2 := &entities.Scheduler{
		Name:            "scheduler-2",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}

	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler1))
	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler2))

	actualSchedulers, err := storage.GetAllSchedulers(context.Background())

	require.NoError(t, err)
	assertSchedulers(t, []*entities.Scheduler{scheduler1, scheduler2}, actualSchedulers)
}

func TestSchedulerStorage_GetSchedulersWithFilter(t *testing.T) {
	db := getPostgresDB(t)
	storage := NewSchedulerStorage(db.Options())
	scheduler1 := &entities.Scheduler{
		Name:            "scheduler-1",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	scheduler2 := &entities.Scheduler{
		Name:            "scheduler-2",
		Game:            "game-2",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}

	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler1))
	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler2))

	t.Run("when filter is nil should return all schedulers", func(t *testing.T) {
		actualSchedulers, err := storage.GetSchedulersWithFilter(context.Background(), nil)
		require.NoError(t, err)

		assertSchedulers(t, []*entities.Scheduler{scheduler1, scheduler2}, actualSchedulers)
	})

	t.Run("when filter is empty should return all schedulers", func(t *testing.T) {
		actualSchedulers, err := storage.GetSchedulersWithFilter(context.Background(), &filters.SchedulerFilter{})
		require.NoError(t, err)

		assertSchedulers(t, []*entities.Scheduler{scheduler1, scheduler2}, actualSchedulers)
	})

	t.Run("where there is a filter should filter schedulers", func(t *testing.T) {
		actualSchedulers, err := storage.GetSchedulersWithFilter(context.Background(), &filters.SchedulerFilter{Game: "game"})
		require.NoError(t, err)

		assertSchedulers(t, []*entities.Scheduler{scheduler1}, actualSchedulers)
	})

	t.Run("where there is a filter and there is no record should return empty array", func(t *testing.T) {
		actualSchedulers, err := storage.GetSchedulersWithFilter(context.Background(), &filters.SchedulerFilter{Game: "NON_EXISTENT_GAME"})
		require.NoError(t, err)
		require.Empty(t, actualSchedulers)
	})
}

func TestSchedulerStorage_GetSchedulerWithFilter(t *testing.T) {
	t.Run("scheduler exists and is valid", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)
		err = storage.CreateSchedulerVersion(context.Background(), "", expectedScheduler)
		require.NoError(t, err)

		actualScheduler, err := storage.GetSchedulerWithFilter(context.Background(), &filters.SchedulerFilter{
			Name:    "scheduler",
			Version: "v1",
		})
		require.NoError(t, err)
		assertSchedulers(t, []*entities.Scheduler{expectedScheduler}, []*entities.Scheduler{actualScheduler})
	})

	t.Run("scheduler does not exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		_, err := storage.GetSchedulerWithFilter(context.Background(), &filters.SchedulerFilter{
			Name:    "scheduler",
			Version: "",
		})
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})

	t.Run("invalid scheduler", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", expectedScheduler))

		_, err := db.Exec("UPDATE schedulers SET yaml = 'invalid yaml' WHERE name = 'scheduler' and version = 'v1' ")
		require.NoError(t, err)
		_, err = db.Exec("UPDATE scheduler_versions SET yaml = 'invalid yaml' WHERE name = 'scheduler' and version = 'v1' ")
		require.NoError(t, err)

		_, err = storage.GetSchedulerWithFilter(context.Background(), &filters.SchedulerFilter{
			Name:    "scheduler",
			Version: "",
		})
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrEncoding, err)
	})

	t.Run("miss scheduler name", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		_, err := storage.GetSchedulerWithFilter(context.Background(), &filters.SchedulerFilter{
			Name:    "",
			Version: "",
		})
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrInvalidArgument, err)
	})
}

func TestSchedulerStorage_GetSchedulerVersions(t *testing.T) {
	t.Run("scheduler exists and is valid", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		version1 := *expectedScheduler
		version2 := *expectedScheduler
		version2.Spec.Version = "v2"
		version3 := *expectedScheduler
		version3.Spec.Version = "v3"

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", &version1))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", &version2))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", &version3))

		versions, err := storage.GetSchedulerVersions(context.Background(), expectedScheduler.Name)

		require.NoError(t, err)
		require.Len(t, versions, 3)
		require.Equal(t, "v3", versions[0].Version)
		require.Equal(t, false, versions[0].IsActive)
		require.Equal(t, "v2", versions[1].Version)
		require.Equal(t, false, versions[1].IsActive)
		require.Equal(t, "v1", versions[2].Version)
		require.Equal(t, true, versions[2].IsActive)
	})

	t.Run("scheduler does not exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		_, err := storage.GetSchedulerVersions(context.Background(), "NonExistentScheduler")
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})
}

func TestSchedulerStorage_CreateScheduler(t *testing.T) {
	t.Run("scheduler does not exist", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", expectedScheduler))

		dbScheduler, dbVersions := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.NotNil(t, dbScheduler)
		require.Len(t, dbVersions, 1)
		requireCorrectScheduler(t, expectedScheduler, dbScheduler, dbVersions[0], false)
	})

	t.Run("scheduler already exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", expectedScheduler))
		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrAlreadyExists, err)
	})
}

func TestSchedulerStorage_UpdateScheduler(t *testing.T) {
	t.Run("scheduler exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))
		require.NoError(t, storage.CreateSchedulerVersion(context.Background(), "", expectedScheduler))

		expectedScheduler.RollbackVersion = "v1"
		expectedScheduler.Spec.Version = "v2"
		expectedScheduler.Spec.Affinity = "whatever"

		err := storage.UpdateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)

		updatedScheduler, err := storage.GetScheduler(context.Background(), expectedScheduler.Name)
		require.NoError(t, err)
		require.NotNil(t, updatedScheduler)
		require.Equal(t, updatedScheduler.RollbackVersion, updatedScheduler.RollbackVersion)
		require.Equal(t, updatedScheduler.Spec.Version, updatedScheduler.Spec.Version)
		require.Equal(t, updatedScheduler.Spec.Affinity, updatedScheduler.Spec.Affinity)
	})

	t.Run("scheduler does not exist", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.UpdateScheduler(context.Background(), expectedScheduler)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})
}

func TestSchedulerStorage_DeleteScheduler(t *testing.T) {
	t.Run("delete scheduler and its versions if it exists without transaction", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)
		err = storage.DeleteScheduler(context.Background(), "", expectedScheduler)
		require.NoError(t, err)

		dbSchedulerResult, _ := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.Nil(t, dbSchedulerResult)
	})

	t.Run("delete scheduler and its versions if scheduler exists with successful transaction", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)
		err = storage.RunWithTransaction(context.Background(), func(transactionID ports.TransactionID) error {
			err = storage.DeleteScheduler(context.Background(), transactionID, expectedScheduler)
			return err
		})
		require.NoError(t, err)

		dbSchedulerResult, _ := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.Nil(t, dbSchedulerResult)
	})

	t.Run("does not delete scheduler if scheduler exists with failed transaction", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)

		err = storage.RunWithTransaction(context.Background(), func(transactionID ports.TransactionID) error {
			_ = storage.DeleteScheduler(context.Background(), transactionID, expectedScheduler)
			return errors.NewErrUnexpected("some error")
		})
		require.Error(t, err)

		dbSchedulerResult, _ := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.NotNil(t, dbSchedulerResult)
	})

	t.Run("does nothing and return error if scheduler does not exist without transaction", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.DeleteScheduler(context.Background(), "", expectedScheduler)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})
}

func TestSchedulerStorage_CreateSchedulerVersion(t *testing.T) {
	var firstVersionScheduler = entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	newVersionScheduler := firstVersionScheduler
	newVersionScheduler.Spec.Version = "v2"

	t.Run("should succeed - scheduler version created for scheduler without transactional context", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))
		err := storage.CreateSchedulerVersion(context.Background(), "", &newVersionScheduler)
		require.NoError(t, err)
		versions, err := storage.GetSchedulerVersions(context.Background(), firstVersionScheduler.Name)
		require.NoError(t, err)
		require.Len(t, versions, 2)
	})

	t.Run("should succeed - scheduler version created for scheduler with transactional context", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))

		err := storage.RunWithTransaction(context.Background(), func(transactionId ports.TransactionID) error {
			err := storage.CreateSchedulerVersion(context.Background(), transactionId, &newVersionScheduler)
			return err
		})
		require.NoError(t, err)

		versions, err := storage.GetSchedulerVersions(context.Background(), firstVersionScheduler.Name)
		require.NoError(t, err)
		require.Len(t, versions, 2)
	})

	t.Run("should fail - scheduler version is not created for scheduler with transactional context if some error occurs", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))

		err := storage.RunWithTransaction(context.Background(), func(transactionId ports.TransactionID) error {
			_ = storage.CreateSchedulerVersion(context.Background(), transactionId, &newVersionScheduler)
			return errors.NewErrUnexpected("some_error")
		})
		require.EqualError(t, err, "some_error")

		versions, err := storage.GetSchedulerVersions(context.Background(), firstVersionScheduler.Name)
		require.NoError(t, err)
		require.Len(t, versions, 1)
	})

	t.Run("should fail - transaction not found when using transactional context", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))
		err := storage.CreateSchedulerVersion(context.Background(), "id-123", &newVersionScheduler)
		require.EqualError(t, err, "transaction id-123 not found")
		versions, err := storage.GetSchedulerVersions(context.Background(), firstVersionScheduler.Name)
		require.NoError(t, err)
		require.Len(t, versions, 1)
	})

	t.Run("should fail - scheduler does not exist", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		err := storage.CreateSchedulerVersion(context.Background(), "", &firstVersionScheduler)
		require.Error(t, err)
		require.Equal(t, err.Error(), fmt.Sprintf("error creating version %s for non existent scheduler \"%s\"", firstVersionScheduler.Spec.Version, expectedScheduler.Name))
	})

	t.Run("should fail - scheduler version invalid", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())

		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))

		firstVersionScheduler.Name = ""

		err := storage.CreateSchedulerVersion(context.Background(), "", &firstVersionScheduler)
		require.Error(t, err)
	})
}

func TestSchedulerStorage_RunWithTransaction(t *testing.T) {

	var firstVersionScheduler = entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	secondVersionScheduler := firstVersionScheduler
	secondVersionScheduler.Spec.Version = "v2"
	thirdVersionScheduler := firstVersionScheduler
	thirdVersionScheduler.Spec.Version = "v3"

	t.Run("should succeed - return nil and commit operations when no error occurs", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))

		err := storage.RunWithTransaction(context.Background(), func(transactionId ports.TransactionID) error {
			err := storage.CreateSchedulerVersion(context.Background(), transactionId, &secondVersionScheduler)
			if err != nil {
				return err
			}
			err = storage.CreateSchedulerVersion(context.Background(), transactionId, &thirdVersionScheduler)

			return err
		})
		require.NoError(t, err)

		versions, err := storage.GetSchedulerVersions(context.Background(), firstVersionScheduler.Name)
		require.NoError(t, err)
		require.Len(t, versions, 3)
	})

	t.Run("should fail - return error and don't commit operations when some error occurs", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		require.NoError(t, storage.CreateScheduler(context.Background(), &firstVersionScheduler))

		err := storage.RunWithTransaction(context.Background(), func(transactionId ports.TransactionID) error {
			_ = storage.CreateSchedulerVersion(context.Background(), transactionId, &secondVersionScheduler)

			_ = storage.CreateSchedulerVersion(context.Background(), transactionId, &thirdVersionScheduler)

			return errors.NewErrUnexpected("some_error")
		})
		require.EqualError(t, err, "some_error")

		versions, err := storage.GetSchedulerVersions(context.Background(), firstVersionScheduler.Name)
		require.NoError(t, err)
		require.Len(t, versions, 1)
	})

}

func TestSchedulerStorage_createWhereClauses(t *testing.T) {
	t.Run("when schedulerFilter is nil should return empty string and array", func(t *testing.T) {
		whereClause, arrayValues := createWhereClauses(nil)
		require.Equal(t, "", whereClause)
		require.Empty(t, arrayValues)
	})

	t.Run("should return the where clause and your values", func(t *testing.T) {
		testCases := []struct {
			title               string
			schedulerFilter     *filters.SchedulerFilter
			expectedWhereClause string
			expectedValues      []interface{}
		}{
			{
				title: "when none present",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "",
					Name:    "",
					Version: "",
				},
				expectedWhereClause: "",
				expectedValues:      []interface{}{},
			}, {
				title: "when present: name, version",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "",
					Name:    "Test Name",
					Version: "Test Version",
				},
				expectedWhereClause: " WHERE name = ? and version = ?",
				expectedValues:      []interface{}{"Test Name", "Test Version"},
			}, {
				title: "when present: game, version",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "Test Game",
					Name:    "",
					Version: "Test Version",
				},
				expectedWhereClause: " WHERE game = ? and version = ?",
				expectedValues:      []interface{}{"Test Game", "Test Version"},
			}, {
				title: "when present: game, name",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "Test Game",
					Name:    "Test Name",
					Version: "",
				},
				expectedWhereClause: " WHERE game = ? and name = ?",
				expectedValues:      []interface{}{"Test Game", "Test Name"},
			}, {
				title: "when present: version",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "",
					Name:    "",
					Version: "Test Version",
				},
				expectedWhereClause: " WHERE version = ?",
				expectedValues:      []interface{}{"Test Version"},
			}, {
				title: "when present: game",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "Test Game",
					Name:    "",
					Version: "",
				},
				expectedWhereClause: " WHERE game = ?",
				expectedValues:      []interface{}{"Test Game"},
			}, {
				title: "when present: name",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "",
					Name:    "Test Name",
					Version: "",
				},
				expectedWhereClause: " WHERE name = ?",
				expectedValues:      []interface{}{"Test Name"},
			}, {
				title: "when present: game, name, version",
				schedulerFilter: &filters.SchedulerFilter{
					Game:    "Test Game",
					Name:    "Test Name",
					Version: "Test Version",
				},
				expectedWhereClause: " WHERE game = ? and name = ? and version = ?",
				expectedValues:      []interface{}{"Test Game", "Test Name", "Test Version"},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.title, func(t *testing.T) {
				whereClause, arrayValues := createWhereClauses(testCase.schedulerFilter)
				require.Equal(t, testCase.expectedWhereClause, whereClause)
				require.Equal(t, testCase.expectedValues, arrayValues)
			})
		}
	})
}

func assertSchedulers(t *testing.T, expectedSchedulers []*entities.Scheduler, actualSchedulers []*entities.Scheduler) {
	for i, expectedScheduler := range expectedSchedulers {
		require.Equal(t, expectedScheduler.Name, actualSchedulers[i].Name)
		require.Equal(t, expectedScheduler.Game, actualSchedulers[i].Game)
		require.Equal(t, expectedScheduler.State, actualSchedulers[i].State)
		require.Equal(t, expectedScheduler.RollbackVersion, actualSchedulers[i].RollbackVersion)
		require.Equal(t, expectedScheduler.Spec, actualSchedulers[i].Spec)
		require.Equal(t, expectedScheduler.PortRange, actualSchedulers[i].PortRange)
	}
}

func getPostgresDB(t *testing.T) *pg.DB {
	number := atomic.AddInt32(&dbNumber, 1)
	dbname := fmt.Sprintf("db%d", number)
	_, err := postgresDB.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE base", dbname))
	require.NoError(t, err)

	opts := &pg.Options{
		Addr:     postgresContainer.DefaultAddress(),
		User:     "maestro",
		Password: "maestro",
		Database: dbname,
	}

	db := pg.Connect(opts)

	t.Cleanup(func() {
		_, _ = db.Exec(fmt.Sprintf("DELETE DATABASE %s", dbname))
		_ = db.Close()
	})

	return db
}

func getDBSchedulerAndVersions(t *testing.T, db *pg.DB, schedulerName string) (*Scheduler, []*dbSchedulerVersion) {
	dbScheduler := new(Scheduler)
	res, err := db.Query(dbScheduler, "select * from schedulers where name = ?;", schedulerName)
	require.NoError(t, err)
	if res.RowsReturned() == 0 {
		return nil, nil
	}

	var versions []*dbSchedulerVersion
	_, err = db.Query(&versions, "select * from scheduler_versions where name = ?;", schedulerName)
	require.NoError(t, err)

	return dbScheduler, versions
}

func requireCorrectScheduler(t *testing.T, expectedScheduler *entities.Scheduler, dbScheduler *Scheduler, dbVersion *dbSchedulerVersion, update bool) {
	// postgres scheduler version is valid
	require.Equal(t, dbScheduler.Name, dbVersion.Name)
	require.Equal(t, dbScheduler.Yaml, dbVersion.Yaml)
	require.Equal(t, dbScheduler.Version, dbVersion.Version)
	require.Equal(t, expectedScheduler.RollbackVersion, dbVersion.RollbackVersion)

	// postgres scheduler is valid
	require.NotEqual(t, time.Time{}, dbScheduler.CreatedAt.Time)
	require.Greater(t, dbScheduler.StateLastChangedAt, int64(0))
	if update {
		require.NotEqual(t, time.Time{}, dbScheduler.UpdatedAt.Time)
	}

	actualScheduler, err := dbScheduler.ToScheduler()
	require.NoError(t, err)

	actualScheduler.RollbackVersion = dbVersion.RollbackVersion

	assertSchedulers(t, []*entities.Scheduler{expectedScheduler}, []*entities.Scheduler{actualScheduler})
}
