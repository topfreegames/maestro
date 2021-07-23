package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/core/workers/operation_execution_worker"
	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"
)

var (
	logConfig  = flag.String("log-config", "development", "")
	configPath = flag.String("config-path", "config/local.yaml", "")
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

	shutdownMetricsServerFn := service.RunMetricsServer(ctx, config)

	// TODO(gabrielcorado): support multiple workers.
	workersManager, err := initializeWorker(config, operation_execution_worker.NewOperationExecutionWorker)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("fail to initialize workers manager")
	}

	zap.L().Info("initialized workers manager, starting...")
	go workersManager.Start(ctx)

	<-ctx.Done()
	// TODO(gabrielcorado): call the workers manager stop.
	shutdownMetricsServerFn()
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
