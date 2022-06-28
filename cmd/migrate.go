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

package cmd

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	viperConfig "github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"
)

var (
	logConfig  string
	configPath string
)

func init() {
	migrateCmd.Flags().StringVarP(&logConfig, "log-config", "l", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	migrateCmd.Flags().StringVarP(&configPath, "config-path", "c", "config/config.yaml", "path of the configuration YAML file")
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrates database",
	Run: func(cmd *cobra.Command, args []string) {
		err := service.ConfigureLogging(logConfig)
		if err != nil {
			zap.L().With(zap.Error(err)).Fatal("unable to load logging configuration")
		}

		viper.SetConfigFile(configPath)

		config, err := viperConfig.NewViperConfig(viper.ConfigFileUsed())
		if err != nil {
			zap.L().With(zap.Error(err)).Fatal("failed to load configuration")
		}

		m, err := migrate.New(
			config.GetString("migration.path"),
			service.GetSchedulerStoragePostgresUrl(config))
		if err != nil {
			zap.L().With(zap.Error(err)).Fatal("failed to create migration")
		}

		// Perform the migration up
		err = m.Up()
		if err == migrate.ErrNoChange {
			zap.L().Info("No performed changes, database schema already up to date")
		} else if err != nil {
			zap.L().With(zap.Error(err)).Fatal("Database migration failed")
		} else {
			zap.L().Info("Database migration completed")
		}
	},
}
