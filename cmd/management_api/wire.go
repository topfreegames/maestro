//+build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/service"
)

func initializeHandlers(c config.Config) (service.Handlers, error) {
	wire.Build(service.ProvideHandlers)

	return service.Handlers{}, nil
}
