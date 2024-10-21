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

package events

import (
	"context"
	"fmt"
	"time"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/patrickmn/go-cache"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// Address represent a host:port to a grpc server that understand Event messages.
type Address string

var (
	_ ports.ForwarderClient = (*ForwarderClient)(nil)
)

const (
	DefaultKeepAliveTime                = 30 * time.Second
	DefaultKeepAliveTimeout             = 10 * time.Second
	DefaultKeepAlivePermitWithoutStream = true
)

type ForwarderClientConfig struct {
	KeepAlive keepalive.ClientParameters
}

// ForwarderClient is a struct that holds grpc clients to be used by forwarders.
type ForwarderClient struct {
	c      *cache.Cache
	config ForwarderClientConfig
}

// NewForwarderClient instantiate a new grpc forwarder client.
func NewForwarderClient(keepAliveCfg keepalive.ClientParameters) *ForwarderClient {
	cache := cache.New(24*time.Hour, 0)
	cache.OnEvicted(func(_key string, clientFromCache interface{}) {
		ForwarderClient := clientFromCache.(*grpc.ClientConn)
		ForwarderClient.Close()
	})
	config := ForwarderClientConfig{
		KeepAlive: keepalive.ClientParameters{
			Time:                DefaultKeepAliveTime,
			Timeout:             DefaultKeepAliveTimeout,
			PermitWithoutStream: DefaultKeepAlivePermitWithoutStream,
		},
	}
	if keepAliveCfg.Time > 0 {
		config.KeepAlive.Time = keepAliveCfg.Time
	}
	if keepAliveCfg.Timeout > 0 {
		config.KeepAlive.Timeout = keepAliveCfg.Timeout
	}
	return &ForwarderClient{
		c:      cache,
		config: config,
	}
}

// SendRoomEvent fetch or create a grpc client and send a room event to forwarder.
func (f *ForwarderClient) SendRoomEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomEvent) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()

	return runReportingLatencyMetrics("SendRoomEvent", func() (*pb.Response, error) {
		return client.SendRoomEvent(ctx, in)
	})
}

// SendRoomReSync fetch or create a grpc client and send a room resync to forwarder.
func (f *ForwarderClient) SendRoomReSync(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomStatus) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()
	return runReportingLatencyMetrics("SendRoomResync", func() (*pb.Response, error) {
		return client.SendRoomResync(ctx, in)
	})
}

// SendRoomStatus fetch or create a grpc client and send a room status event to forwarder.
func (f *ForwarderClient) SendRoomStatus(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomStatus) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()
	return runReportingLatencyMetrics("SendRoomStatus", func() (*pb.Response, error) {
		return client.SendRoomStatus(ctx, in)
	})
}

// SendPlayerEvent fetch or create a grpc client and send a player event to forwarder.
func (f *ForwarderClient) SendPlayerEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.PlayerEvent) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()
	return runReportingLatencyMetrics("SendPlayerEvent", func() (*pb.Response, error) {
		return client.SendPlayerEvent(ctx, in)
	})
}

// CacheFlush clean the connections cache.
func (f *ForwarderClient) CacheFlush() {
	f.c.Flush()
}

// CacheDelete delete a connection from the cache.
func (f *ForwarderClient) CacheDelete(forwarderAddress string) error {
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

func (f *ForwarderClient) getGrpcClient(address Address) (pb.GRPCForwarderClient, error) {
	if address == "" {
		return nil, errors.NewErrInvalidArgument("no grpc server address informed")
	}

	clientFromCacheInterface, found := f.c.Get(string(address))
	if !found {
		client, err := f.createGRPCConnection(string(address))
		if err != nil {
			return nil, err
		}
		f.c.DeleteExpired()
		f.c.Set(string(address), client, cache.DefaultExpiration)
		return pb.NewGRPCForwarderClient(client), nil
	}
	return pb.NewGRPCForwarderClient((clientFromCacheInterface).(*grpc.ClientConn)), nil
}

func (f *ForwarderClient) createGRPCConnection(address string) (*grpc.ClientConn, error) {
	if address == "" {
		return nil, errors.NewErrInvalidArgument("no rpc server address informed")
	}

	zap.L().Info(fmt.Sprintf("connecting to grpc server at: %s", address))

	tracer := opentracing.GlobalTracer()
	dialOption := grpc.WithInsecure() //nolint:staticcheck // I want to use deprecated method.
	conn, err := grpc.Dial(
		address,
		dialOption,
		grpc.WithKeepaliveParams(f.config.KeepAlive),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	)
	if err != nil {
		zap.L().Error(fmt.Sprintf("failed to connect to grpc server at: %s", address))
		return nil, err
	}
	zap.L().Info(fmt.Sprintf("connected to grpc server at: %s with success", address))
	return conn, nil
}
