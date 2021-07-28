//+build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/topfreegames/maestro/internal/api/handlers"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	"github.com/topfreegames/maestro/internal/service"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func initializeManagementMux(ctx context.Context, conf config.Config) (*runtime.ServeMux, error) {
	wire.Build(
		service.NewSchedulerStoragePg,
		scheduler_manager.NewSchedulerManager,
		handlers.ProvideSchedulerHandler,
		handlers.ProvidePingHandler,
		provideManagementMux,
	)

	return &runtime.ServeMux{}, nil
}

func provideManagementMux(ctx context.Context, pingHandler *handlers.PingHandler, schedulerHandler *handlers.SchedulerHandler) *runtime.ServeMux {
	mux := runtime.NewServeMux()
	api.RegisterPingHandlerServer(ctx, mux, pingHandler)
	api.RegisterSchedulersHandlerServer(ctx, mux, schedulerHandler)

	return mux
}
