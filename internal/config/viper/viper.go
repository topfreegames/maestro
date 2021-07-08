package viper

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/internal/config"
)

func NewViperConfig(configPath string) (config.Config, error) {
	config := viper.New()
	config.SetEnvPrefix("maestro")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.AutomaticEnv()

	config.SetConfigType("yaml")
	config.SetConfigFile(configPath)
	err := config.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return config, nil
}
