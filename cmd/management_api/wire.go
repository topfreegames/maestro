//+build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/topfreegames/maestro/internal/handlers"
	"github.com/topfreegames/maestro/internal/service"
)

func initializeManagementMux(ctx context.Context) (*runtime.ServeMux, error) {
	wire.Build(
		handlers.ProvidePingHandler,
		service.ProvideManagementMux,
	)

	return &runtime.ServeMux{}, nil
}
