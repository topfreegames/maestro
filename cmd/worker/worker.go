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

	shutdownMetricsServerFn := service.RunMetricsServer(ctx, config)

	// TODO(gabrielcorado): support multiple workers.
	operationExecutionWorkerManager, err := initializeWorker(config, operation_execution_worker.NewOperationExecutionWorker)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize operation execution worker manager")
	}

	go func() {
		zap.L().Info("operation execution worker manager initialized, starting...")
		err := operationExecutionWorkerManager.Start(ctx)
		if err != nil {
			zap.L().With(zap.Error(err)).Info("operation execution worker manager stopped with error")
			// enforce the cancelation
			cancelFn()
		}
	}()

	<-ctx.Done()

	err = shutdownMetricsServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown metrics server")
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
