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

package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/patrickmn/go-cache"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	_ ports.ForwarderGrpc = (*forwarderGrpc)(nil)
)

type forwarderGrpc struct {
	c *cache.Cache
}

func NewForwarderGrpc() *forwarderGrpc {
	return &forwarderGrpc{
		c: cache.New(5*time.Minute, 10*time.Minute),
	}
}

func (f *forwarderGrpc) SendRoomEvent(ctx context.Context, in *pb.RoomEvent, opts ...grpc.CallOption) (*pb.Response, error) {
	forwarderAddress := "address"
	client, err := f.getGrpcClient(forwarderAddress)
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarderAddress).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10)
	defer cancel()

	return client.SendRoomEvent(ctx, in)
}

func (f *forwarderGrpc) SendRoomResync(ctx context.Context, in *pb.RoomStatus, opts ...grpc.CallOption) (*pb.Response, error) {
	forwarderAddress := "address"
	client, err := f.getGrpcClient(forwarderAddress)
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarderAddress).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10)
	defer cancel()

	return client.SendRoomResync(ctx, in)
}

func (f *forwarderGrpc) SendPlayerEvent(ctx context.Context, in *pb.PlayerEvent, opts ...grpc.CallOption) (*pb.Response, error) {
	forwarderAddress := "address"
	client, err := f.getGrpcClient(forwarderAddress)
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarderAddress).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10)
	defer cancel()

	return client.SendPlayerEvent(ctx, in)
}

func (f *forwarderGrpc) getGrpcClient(address string) (pb.GRPCForwarderClient, error) {
	clientFromCache, found := f.c.Get(address)
	if !found {
		client, err := f.configureGrpcClient(address)
		if err != nil {
			return nil, err
		}
		f.c.Set(address, client, cache.DefaultExpiration)
		return client, nil
	}
	return clientFromCache.(pb.GRPCForwarderClient), nil
}

func (f *forwarderGrpc) configureGrpcClient(address string) (pb.GRPCForwarderClient, error) {
	if address == "" {
		return nil, errors.NewErrInvalidArgument("no grpc server address informed")
	}

	zap.L().Info(fmt.Sprintf("connecting to grpc server at: %s", address))

	tracer := opentracing.GlobalTracer()
	conn, err := grpc.Dial(
		address,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	)
	if err != nil {
		return nil, err
	}
	return pb.NewGRPCForwarderClient(conn), nil
}