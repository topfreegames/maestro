// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/topfreegames/maestro/internal/api/handlers"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	"github.com/topfreegames/maestro/internal/service"
	"github.com/topfreegames/maestro/pkg/api/v1"
)

// Injectors from wire.go:

func initializeManagementMux(ctx context.Context, conf config.Config) (*runtime.ServeMux, error) {
	pingHandler := handlers.ProvidePingHandler()
	schedulerStorage, err := service.NewSchedulerStoragePg(conf)
	if err != nil {
		return nil, err
	}
	operationFlow, err := service.NewOperationFlowRedis(conf)
	if err != nil {
		return nil, err
	}
	clock := service.NewClockTime()
	operationStorage, err := service.NewOperationStorageRedis(clock, conf)
	if err != nil {
		return nil, err
	}
	v := providers.ProvideDefinitionConstructors()
	operationManager := operation_manager.New(operationFlow, operationStorage, v)
	schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)
	schedulersHandler := handlers.ProvideSchedulersHandler(schedulerManager)
	operationsHandler := handlers.ProvideOperationsHandler(operationManager)
	serveMux := provideManagementMux(ctx, pingHandler, schedulersHandler, operationsHandler)
	return serveMux, nil
}

// wire.go:

func provideManagementMux(ctx context.Context, pingHandler *handlers.PingHandler, schedulersHandler *handlers.SchedulersHandler, operationsHandler *handlers.OperationsHandler) *runtime.ServeMux {
	mux := runtime.NewServeMux()
	_ = v1.RegisterPingServiceHandlerServer(ctx, mux, pingHandler)
	_ = v1.RegisterSchedulersServiceHandlerServer(ctx, mux, schedulersHandler)
	_ = v1.RegisterOperationsServiceHandlerServer(ctx, mux, operationsHandler)

	return mux
}
