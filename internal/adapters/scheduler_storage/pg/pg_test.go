//+build integration

package pg

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/adapters/scheduler_storage/pg/migrations"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/require"

	"github.com/go-pg/pg"

	"github.com/orlangure/gnomock"
	ppg "github.com/orlangure/gnomock/preset/postgres"
)

var dbNumber int32 = 0
var postgresContainer *gnomock.Container
var postgresDB *pg.DB

func getPostgresDB(t *testing.T) *pg.DB {
	number := atomic.AddInt32(&dbNumber, 1)
	dbname := fmt.Sprintf("db%d", number)
	_, err := postgresDB.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE base", dbname))
	require.NoError(t, err)

	opts := &pg.Options{
		Addr:     postgresContainer.DefaultAddress(),
		User:     "postgres",
		Password: "password",
		Database: dbname,
	}

	db := pg.Connect(opts)

	t.Cleanup(func() {
		_, _ = db.Exec(fmt.Sprintf("DELETE DATABASE %s", dbname))
		_ = db.Close()
	})

	return db
}

func migrate(db *pg.DB) error {
	fileInfos, err := ioutil.ReadDir("./migrations")
	if err != nil {
		return err
	}
	migrationFiles := make([]string, 0)
	for _, info := range fileInfos {
		if !info.IsDir() {
			migrationFiles = append(migrationFiles, fmt.Sprintf("./migrations/%s", info.Name()))
		}
	}

	sort.StringSlice(migrationFiles).Sort()

	for _, migrationFile := range migrationFiles {
		query, err := ioutil.ReadFile(migrationFile)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(query)); err != nil {
			return err
		}
	}

	return nil
}

type SchedulerVersion struct {
	ID                  string      `db:"id"`
	Name                string      `db:"name"`
	Yaml                string      `db:"yaml"`
	Version             string      `db:"version"`
	CreatedAt           pg.NullTime `db:"created_at"`
	RollbackVersion     string      `db:"rollback_version"`
	RollingUpdateStatus string      `db:"rolling_update_status"`
}

func getDBSchedulerAndVersions(t *testing.T, db *pg.DB, schedulerName string) (*Scheduler, []*SchedulerVersion) {
	dbScheduler := new(Scheduler)
	res, err := db.Query(dbScheduler, "select * from schedulers where name = ?;", schedulerName)
	require.NoError(t, err)
	if res.RowsReturned() == 0 {
		return nil, nil
	}

	var versions []*SchedulerVersion
	_, err = db.Query(&versions, "select * from scheduler_versions where name = ?;", schedulerName)
	require.NoError(t, err)

	return dbScheduler, versions
}

func requireCorrectScheduler(t *testing.T, expectedScheduler *entities.Scheduler, dbScheduler *Scheduler, dbVersion *SchedulerVersion, update bool) {
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
	require.Equal(t, expectedScheduler, actualScheduler)
}

func TestMain(m *testing.M) {
	var err error

	postgresContainer, err = gnomock.Start(ppg.Preset(ppg.WithDatabase("base")))

	if err != nil {
		panic(fmt.Sprintf("error creating postgres docker instance: %s\n", err))
	}
	opts := &pg.Options{
		Addr:     postgresContainer.DefaultAddress(),
		User:     "postgres",
		Password: "password",
		Database: "base",
	}

	postgresDB = pg.Connect(opts)
	if _, err := migrations.Migrate(opts); err != nil {
		panic(fmt.Sprintf("error preparing postgres database: %s\n", err))
	}

	code := m.Run()
	_ = gnomock.Stop(postgresContainer)
	os.Exit(code)
}

func TestSchedulerStorage_GetScheduler(t *testing.T) {
	t.Run("scheduler exists and is valid", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		actualScheduler, err := storage.GetScheduler(context.Background(), "scheduler")
		require.NoError(t, err)
		require.Equal(t, expectedScheduler, actualScheduler)
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
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		_, err := db.Exec("UPDATE schedulers SET yaml = 'invalid yaml' WHERE name = 'scheduler'")
		require.NoError(t, err)

		_, err = storage.GetScheduler(context.Background(), "scheduler")
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrEncoding, err)
	})
}

func TestSchedulerStorage_CreateScheduler(t *testing.T) {
	t.Run("scheduler does not exist", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		dbScheduler, dbVersions := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.NotNil(t, dbScheduler)
		require.Len(t, dbVersions, 1)
		requireCorrectScheduler(t, expectedScheduler, dbScheduler, dbVersions[0], false)
	})

	t.Run("scheduler already exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))
		err := storage.CreateScheduler(context.Background(), expectedScheduler)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrAlreadyExists, err)
	})
}

func TestSchedulerStorage_UpdateScheduler(t *testing.T) {
	t.Run("scheduler exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		expectedScheduler.RollbackVersion = "v1"
		expectedScheduler.Spec.Version = "v2"
		expectedScheduler.Spec.Affinity = "whatever"

		err := storage.UpdateScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)

		dbScheduler, dbVersions := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.NotNil(t, dbScheduler)
		require.Len(t, dbVersions, 2)
		requireCorrectScheduler(t, expectedScheduler, dbScheduler, dbVersions[1], true)
	})

	t.Run("scheduler does not exist", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		err := storage.UpdateScheduler(context.Background(), expectedScheduler)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})
}

func TestSchedulerStorage_DeleteScheduler(t *testing.T) {
	t.Run("scheduler exists", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		require.NoError(t, storage.CreateScheduler(context.Background(), expectedScheduler))

		err := storage.DeleteScheduler(context.Background(), expectedScheduler)
		require.NoError(t, err)

		dbScheduler, _ := getDBSchedulerAndVersions(t, db, expectedScheduler.Name)
		require.Nil(t, dbScheduler)
	})

	t.Run("scheduler does not exist", func(t *testing.T) {
		db := getPostgresDB(t)
		storage := NewSchedulerStorage(db.Options())
		expectedScheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		err := storage.DeleteScheduler(context.Background(), expectedScheduler)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})
}

func TestSchedulerStorage_GetSchedulers(t *testing.T) {
	db := getPostgresDB(t)
	storage := NewSchedulerStorage(db.Options())
	scheduler1 := &entities.Scheduler{
		Name:            "scheduler-1",
		Game:            "game",
		State:           entities.StateCreating,
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	scheduler2 := &entities.Scheduler{
		Name:            "scheduler-2",
		Game:            "game",
		State:           entities.StateCreating,
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}

	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler1))
	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler2))

	actualSchedulers, err := storage.GetSchedulers(context.Background(), []string{"scheduler-1", "scheduler-2"})
	require.NoError(t, err)
	require.Equal(t, []*entities.Scheduler{scheduler1, scheduler2}, actualSchedulers)

	actualSchedulers, err = storage.GetSchedulers(context.Background(), []string{"scheduler-1", "scheduler-3"})
	require.NoError(t, err)
	require.Equal(t, []*entities.Scheduler{scheduler1}, actualSchedulers)

	actualSchedulers, err = storage.GetSchedulers(context.Background(), []string{"scheduler-3", "scheduler-4"})
	require.NoError(t, err)
	require.Equal(t, []*entities.Scheduler{}, actualSchedulers)
}

func TestSchedulerStorage_GetAllSchedulers(t *testing.T) {
	db := getPostgresDB(t)
	storage := NewSchedulerStorage(db.Options())
	scheduler1 := &entities.Scheduler{
		Name:            "scheduler-1",
		Game:            "game",
		State:           entities.StateCreating,
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
	scheduler2 := &entities.Scheduler{
		Name:            "scheduler-2",
		Game:            "game",
		State:           entities.StateCreating,
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}

	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler1))
	require.NoError(t, storage.CreateScheduler(context.Background(), scheduler2))

	actualSchedulers, err := storage.GetAllSchedulers(context.Background())
	require.NoError(t, err)
	require.Equal(t, []*entities.Scheduler{scheduler1, scheduler2}, actualSchedulers)
}
