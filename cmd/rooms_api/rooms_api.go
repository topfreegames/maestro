package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"
)

var (
	logConfig  = flag.String("log-config", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	configPath = flag.String("config-path", "config/rooms-api.local.yaml", "path of the configuration YAML file")
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

	mux, err := initializeRoomsMux(ctx, config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize rooms mux")
	}
	shutdownRoomsServerFn := runRoomsServer(ctx, config, mux)

	<-ctx.Done()

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown metrics server")
	}

	err = shutdownRoomsServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown management server")
	}
}
