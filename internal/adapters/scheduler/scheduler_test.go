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
	"fmt"
	"os"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/go-pg/pg"
	golangMigrate "github.com/golang-migrate/migrate/v4"
	"github.com/orlangure/gnomock"
	ppg "github.com/orlangure/gnomock/preset/postgres"

	"github.com/topfreegames/maestro/test"
)

var redisAddress string
var dbNumber int32 = 0
var postgresContainer *gnomock.Container
var postgresDB *pg.DB

func TestMain(m *testing.M) {
	var code int
	test.WithRedisContainer(func(redisContainerAddress string) {
		redisAddress = redisContainerAddress
		var err error

		postgresContainer, err = gnomock.Start(
			ppg.Preset(
				ppg.WithDatabase("base"),
				ppg.WithUser("maestro", "maestro"),
			))

		if err != nil {
			panic(fmt.Sprintf("error creating postgres docker instance: %s\n", err))
		}

		opts := &pg.Options{
			Addr:     postgresContainer.DefaultAddress(),
			User:     "postgres",
			Password: "password",
			Database: "base",
		}
		if err := migrate(opts); err != nil {
			panic(fmt.Sprintf("error preparing postgres database: %s\n", err))
		}

		postgresDB = pg.Connect(opts)
		code = m.Run()
	})
	_ = gnomock.Stop(postgresContainer)
	os.Exit(code)
}

func migrate(opts *pg.Options) error {
	dbUrl := getDBUrl(opts)
	m, err := golangMigrate.New("file://../../service/migrations", dbUrl)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil {
		return err
	}

	m.Close()

	return nil
}

func getDBUrl(opts *pg.Options) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", opts.User, opts.Password, opts.Addr, opts.Database)
}
