package service

import (
	"fmt"

	"go.uber.org/zap"
)

func ConfigureLogging(configPreset string) error {
	var cfg zap.Config
	switch configPreset {
	case "development":
		cfg = zap.NewDevelopmentConfig()
	case "production":
		cfg = zap.NewProductionConfig()
	default:
		return fmt.Errorf("unexpected log_config: %v", configPreset)
	}

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}
