// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/GuiaBolso/darwin"
	_ "github.com/lib/pq" // pg driver
	"github.com/spf13/cobra"
	"github.com/topfreegames/maestro/migrations"
)

var l *logger
var newline = []byte("\n")
var migrationInfo bool

type logger struct {
	pipe io.Writer
}

func (lg *logger) println(msg string) {
	lg.pipe.Write([]byte(msg))
	lg.pipe.Write(newline)
}

func getMigrations() []darwin.Migration {
	migNames := migrations.AssetNames()
	sort.Sort(sort.StringSlice(migNames))
	migs := make([]darwin.Migration, len(migNames))

	for i, migName := range migNames {
		contents, err := migrations.Asset(migName)
		if err != nil {
			l.panicf("Could not read migration %s!", migName)
		}
		migs[i] = darwin.Migration{
			Version:     getVersion(migName),
			Description: getDescription(migName),
			Script:      string(contents),
		}
	}

	return migs
}

func (lg *logger) panicf(msg string, args ...interface{}) {
	fMsg := fmt.Sprintf(msg, args)
	lg.pipe.Write([]byte(fMsg))
	lg.pipe.Write(newline)
	panic(fMsg)
}

func getVersion(migName string) float64 {
	parts := strings.Split(filepath.Base(migName), "-")
	migNumber, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		l.panicf("Failed to parse migration name: %s (error: %s)", migName, err.Error())
	}
	return migNumber
}

func getDescription(migName string) string {
	return filepath.Base(migName)
}

func getDB() (*sql.DB, error) {
	host := config.GetString("extensions.pg.host")
	port := config.GetInt("extensions.pg.port")
	database := config.GetString("extensions.pg.database")
	user := config.GetString("extensions.pg.user")
	pass := config.GetString("extensions.pg.pass")

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, pass, host, port, database)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func printStatus(d darwin.Darwin) error {
	infos, err := d.Info()
	if err != nil {
		return err
	}
	l.println("")
	l.println("Current database migration status")
	l.println("=================================")
	l.println("")
	l.println("Version  | Status          | Name")
	for _, info := range infos {
		status := info.Status.String()
		for i := 0; i < 15-len(info.Status.String()); i++ {
			status += " "
		}
		l.println(fmt.Sprintf(
			"%.1f      | %s | %s",
			info.Migration.Version, status, info.Migration.Description,
		))
	}
	l.println("")

	return nil
}

//RunMigrations in selected DB
func RunMigrations(info bool, writer io.Writer) error {
	if writer == nil {
		l = &logger{
			pipe: os.Stdout,
		}
	} else {
		l = &logger{
			pipe: writer,
		}
	}

	migrations := getMigrations()

	database, err := getDB()

	if err != nil {
		log.Fatal(err)
	}

	driver := darwin.NewGenericDriver(database, darwin.PostgresDialect{})

	d := darwin.New(driver, migrations, nil)

	if info {
		err = printStatus(d)
		if err != nil {
			return err
		}
	} else {
		l.println("Migrating database to latest version...")
		err = d.Migrate()

		if err != nil {
			return err
		}

		printStatus(d)
		l.println("Database migrated successfully.\n")
	}
	return nil
}

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrates the database up or down",
	Long:  `Migrate the database specified in the configuration file to the given version (or latest if none provided)`,
	Run: func(cmd *cobra.Command, args []string) {
		InitConfig()
		err := RunMigrations(migrationInfo, nil)
		if err != nil {
			log.Println(err)
			panic(err.Error())
		}
	},
}

func init() {
	RootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().BoolVarP(&migrationInfo, "info", "i", false, "Get database info")
}
