package commom

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/service"
	"github.com/topfreegames/maestro/internal/validations"
	"go.uber.org/zap"
)

func ServiceSetup(ctx context.Context, cancelFn context.CancelFunc, logConfig, configPath string) (error, config.Config, func() error) {
	err := service.ConfigureLogging(logConfig)
	if err != nil {
		return fmt.Errorf("unable to configure logging: %w", err), nil, nil
	}

	err = validations.RegisterValidations()
	if err != nil {
		return fmt.Errorf("unable to register validations: %w", err), nil, nil
	}

	viperConfig, err := viper.NewViperConfig(configPath)
	if err != nil {
		return fmt.Errorf("unable to load config: %w", err), nil, nil
	}

	launchTerminatingListenerGoroutine(cancelFn)

	shutdownInternalServerFn := service.RunInternalServer(ctx, viperConfig)

	return nil, viperConfig, shutdownInternalServerFn
}

func launchTerminatingListenerGoroutine(cancelFunc context.CancelFunc) {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		zap.L().Info("received termination")

		cancelFunc()
	}()
}
