// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build wireinject
// +build wireinject

package roomsapi

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/services/events_forwarder"

	"github.com/google/wire"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/topfreegames/maestro/internal/api/handlers"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/service"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func initializeRoomsMux(ctx context.Context, conf config.Config) (*runtime.ServeMux, error) {
	wire.Build(
		// ports + adapters
		service.NewClockTime,
		service.NewPortAllocatorRandom,
		service.NewRoomStorageRedis,
		service.NewGameRoomInstanceStorageRedis,
		service.NewSchedulerCacheRedis,
		service.NewRuntimeKubernetes,
		service.NewRoomManagerConfig,
		service.NewRoomManager,
		service.NewEventsForwarder,
		service.NewSchedulerStoragePg,
		service.NewEventsForwarderServiceConfig,

		// services
		events_forwarder.NewEventsForwarderService,

		// api handlers
		handlers.ProvideRoomsHandler,
		provideRoomsMux,
	)

	return &runtime.ServeMux{}, nil
}

func provideRoomsMux(ctx context.Context, roomsHandler *handlers.RoomsHandler) *runtime.ServeMux {
	mux := runtime.NewServeMux()
	_ = api.RegisterRoomsServiceHandlerServer(ctx, mux, roomsHandler)
	return mux
}
