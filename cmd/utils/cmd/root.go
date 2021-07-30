package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"
)

var (
	logConfig  = flag.String("log-config", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	configPath = flag.String("config-path", "config/local.yaml", "path of the configuration YAML file")
	rootCmd    = &cobra.Command{
		Use:   "migrate",
		Short: "Migrates datasources of maestro",
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {

	flag.Parse()
	err := service.ConfigureLogging(*logConfig)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unabled to load logging configuration")
	}

	viper.SetConfigFile(*configPath)
}
