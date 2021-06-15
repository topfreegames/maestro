package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/GuiaBolso/darwin"
	_ "github.com/lib/pq" // pg driver

	"github.com/go-pg/pg"
)

//go:embed *.sql
var migrationsFS embed.FS

func Migrate(opts *pg.Options) ([]darwin.MigrationInfo, error) {
	migrations, err := getMigrations()
	if err != nil {
		return nil, err
	}

	db, err := getDB(opts)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	driver := darwin.NewGenericDriver(db, darwin.PostgresDialect{})
	d := darwin.New(driver, migrations, nil)
	err = d.Migrate()
	if err != nil {
		return nil, err
	}
	return d.Info()
}

func getDB(opts *pg.Options) (*sql.DB, error) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", opts.User, opts.Password, opts.Addr, opts.Database)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getMigrations() ([]darwin.Migration, error) {
	migrationNames, err := getMigrationNames()
	if err != nil {
		return nil, err
	}

	migrations := make([]darwin.Migration, 0)
	for _, migrationName := range migrationNames {
		migration, err := buildMigration(migrationName)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migration)
	}

	return migrations, err
}

func getMigrationNames() ([]string, error) {
	entries, err := migrationsFS.ReadDir(".")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	sort.StringSlice(names).Sort()

	return names, nil
}

func buildMigration(migrationName string) (darwin.Migration, error) {
	version, err := getVersion(migrationName)
	if err != nil {
		return darwin.Migration{}, err
	}
	description := getDescription(migrationName)
	contents, err := migrationsFS.ReadFile(migrationName)
	if err != nil {
		return darwin.Migration{}, err
	}
	return darwin.Migration{
		Version:     version,
		Description: description,
		Script:      string(contents),
	}, nil
}

func getVersion(migName string) (float64, error) {
	parts := strings.SplitN(filepath.Base(migName), "-", 2)
	migNumber, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	return migNumber, nil
}

func getDescription(migName string) string {
	return filepath.Base(migName)
}
