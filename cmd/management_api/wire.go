//+build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/topfreegames/maestro/internal/api/handlers"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func initializeManagementMux(ctx context.Context) (*runtime.ServeMux, error) {
	wire.Build(
		handlers.ProvidePingHandler,
		provideManagementMux,
	)

	return &runtime.ServeMux{}, nil
}

func provideManagementMux(ctx context.Context, pingHandler *handlers.PingHandler) *runtime.ServeMux {
	mux := runtime.NewServeMux()
	api.RegisterPingHandlerServer(ctx, mux, pingHandler)

	return mux
}