// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import (
	"strings"

	"github.com/spf13/viper"
)

// GetDefaultConfig returns the configuration at ./config/test.yaml
func GetDefaultConfig() (*viper.Viper, error) {
	cfg := viper.New()
	cfg.SetConfigFile("../config/test.yaml")
	cfg.SetConfigType("yaml")
	cfg.SetEnvPrefix("maestro")
	cfg.AddConfigPath(".")
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()

	// If a config file is found, read it in.
	if err := cfg.ReadInConfig(); err != nil {
		return nil, err
	}

	return cfg, nil
}
