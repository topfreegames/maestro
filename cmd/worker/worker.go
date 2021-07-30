package main

import (
	"context"
	"flag"
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
	err := service.ConfigureLogging(*logConfig)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unabled to load logging configuration")
	}

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

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown internal server")
	}
}
