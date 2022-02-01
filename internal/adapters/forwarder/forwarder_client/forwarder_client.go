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

package forwarder_client

import (
	"context"
	"fmt"

	"time"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/patrickmn/go-cache"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	port "github.com/topfreegames/maestro/internal/core/ports/forwarder"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Address string

var (
	_ port.ForwarderClient = (*forwarderClient)(nil)
)

type forwarderClient struct {
	c *cache.Cache
}

func NewForwarderClient() *forwarderClient {
	return &forwarderClient{
		c: cache.New(5*time.Minute, 10*time.Minute),
	}
}

func (f *forwarderClient) SendRoomEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomEvent) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()

	return client.SendRoomEvent(ctx, in)
}

func (f *forwarderClient) SendRoomReSync(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomStatus) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()

	return client.SendRoomResync(ctx, in)
}

func (f *forwarderClient) SendPlayerEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.PlayerEvent) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()

	return client.SendPlayerEvent(ctx, in)
}

func (f *forwarderClient) CacheFlush() {
	f.c.Flush()
}

func (f *forwarderClient) CacheDelete(forwarderAddress string) error {
	if forwarderAddress == "" {
		return errors.NewErrInvalidArgument("no grpc server address informed")
	}
	_, found := f.c.Get(forwarderAddress)
	if !found {
		return errors.NewErrNotFound("could not found forwarder Address in cache %s", forwarderAddress)
	}
	f.c.Delete(forwarderAddress)
	return nil
}

func (f *forwarderClient) getGrpcClient(address Address) (pb.GRPCForwarderClient, error) {
	if address == "" {
		return nil, errors.NewErrInvalidArgument("no grpc server address informed")
	}

	clientFromCache, found := f.c.Get(string(address))
	if !found {
		client, err := f.configureGrpcClient(string(address))
		if err != nil {
			return nil, err
		}
		f.c.Set(string(address), client, cache.DefaultExpiration)
		return client, nil
	}
	return clientFromCache.(pb.GRPCForwarderClient), nil
}

func (f *forwarderClient) configureGrpcClient(address string) (pb.GRPCForwarderClient, error) {
	if address == "" {
		return nil, errors.NewErrInvalidArgument("no rpc server address informed")
	}

	zap.L().Info(fmt.Sprintf("connecting to grpc server at: %s", address))

	tracer := opentracing.GlobalTracer()
	conn, err := grpc.Dial(
		address,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	)
	if err != nil {
		zap.L().Error(fmt.Sprintf("failed to connect to grpc server at: %s", address))
		return nil, err
	}
	zap.L().Info(fmt.Sprintf("connected to grpc server at: %s with success", address))
	return pb.NewGRPCForwarderClient(conn), nil
}
