package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"
)

var (
	logConfig  = flag.String("log-config", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	configPath = flag.String("config-path", "config/local.yaml", "path of the configuration YAML file")
)

func main() {
	flag.Parse()
	configureLogging(*logConfig)

	ctx, cancelFn := context.WithCancel(context.Background())

	config, err := viper.NewViperConfig(*configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unabled to load config")
	}

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		zap.L().Info("received termination")

		cancelFn()
	}()

	shutdownInternalServerFn := service.RunInternalServer(ctx, config)

	handlers, err := initializeHandlers(config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize management handlers")
	}
	shutdownManagementServerFn := service.RunManagementServer(ctx, config, handlers)

	<-ctx.Done()

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown metrics server")
	}

	err = shutdownManagementServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown management server")
	}
}

// NOTE: we can consider moving this configuration to a shared package.
func configureLogging(configPreset string) error {
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
