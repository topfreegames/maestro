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

func init() {
	rootCmd.AddCommand(migrateCmd)
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrates database",
	Run: func(cmd *cobra.Command, args []string) {

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
