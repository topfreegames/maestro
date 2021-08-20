// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/topfreegames/maestro/internal/config"
)

// Injectors from wire.go:

func initializeRoomsMux(ctx context.Context, conf config.Config) (*runtime.ServeMux, error) {
	serveMux := provideRoomsMux(ctx)
	return serveMux, nil
}

// wire.go:

func provideRoomsMux(ctx context.Context) *runtime.ServeMux {
	mux := runtime.NewServeMux()

	return mux
}
