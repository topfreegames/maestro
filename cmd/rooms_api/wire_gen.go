// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/topfreegames/maestro/internal/api/handlers"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/service"
	"github.com/topfreegames/maestro/pkg/api/v1"
)

// Injectors from wire.go:

func initializeRoomsMux(ctx context.Context, conf config.Config) (*runtime.ServeMux, error) {
	clock := service.NewClockTime()
	portAllocator, err := service.NewPortAllocatorRandom(conf)
	if err != nil {
		return nil, err
	}
	roomStorage, err := service.NewRoomStorageRedis(conf)
	if err != nil {
		return nil, err
	}
	gameRoomInstanceStorage, err := service.NewGameRoomInstanceStorageRedis(conf)
	if err != nil {
		return nil, err
	}
	portsRuntime, err := service.NewRuntimeKubernetes(conf)
	if err != nil {
		return nil, err
	}
	eventsForwarder, err := service.NewEventsForwarder(conf)
	if err != nil {
		return nil, err
	}
	roomManagerConfig, err := service.NewRoomManagerConfig(conf)
	if err != nil {
		return nil, err
	}
	roomManager := room_manager.NewRoomManager(clock, portAllocator, roomStorage, gameRoomInstanceStorage, portsRuntime, eventsForwarder, roomManagerConfig)
	roomsHandler := handlers.ProvideRoomsHandler(roomManager, eventsForwarder)
	serveMux := provideRoomsMux(ctx, roomsHandler)
	return serveMux, nil
}

// wire.go:

func provideRoomsMux(ctx context.Context, roomsHandler *handlers.RoomsHandler) *runtime.ServeMux {
	mux := runtime.NewServeMux()
	_ = v1.RegisterRoomsServiceHandlerServer(ctx, mux, roomsHandler)
	return mux
}
