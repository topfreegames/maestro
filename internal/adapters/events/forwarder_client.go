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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Address represent a host:port to a grpc server that understand Event messages.
type Address string

var (
	_ ports.ForwarderClient = (*ForwarderClient)(nil)
)

// Default values for keep alive parameters.
const (
	DefaultKeepAliveTime                = 15 * time.Second
	DefaultKeepAliveTimeout             = 5 * time.Second
	DefaultKeepAlivePermitWithoutStream = true
)

type ForwarderClientConfig struct {
	KeepAlive        keepalive.ClientParameters
	ExtraDialOptions []grpc.DialOption // Store extra dial options for createGRPCConnection
}

// ForwarderClient is a struct that holds grpc clients to be used by forwarders.
type ForwarderClient struct {
	c      *cache.Cache
	config ForwarderClientConfig
}

// NewForwarderClient creates a new forwarder client.
// It takes keepAlive parameters and optional additional grpc.DialOptions.
func NewForwarderClient(keepAliveCfg keepalive.ClientParameters, extraDialOptions ...grpc.DialOption) *ForwarderClient {
	clientCache := cache.New(24*time.Hour, 0)
	clientCache.OnEvicted(func(_key string, clientFromCache interface{}) {
		if grpcClientConn, ok := clientFromCache.(*grpc.ClientConn); ok {
			grpcClientConn.Close()
		}
	})

	// Apply our defaults if not provided by the user
	if keepAliveCfg.Time == 0 {
		keepAliveCfg.Time = DefaultKeepAliveTime
	}
	if keepAliveCfg.Timeout == 0 {
		keepAliveCfg.Timeout = DefaultKeepAliveTimeout
	}
	// Note: PermitWithoutStream defaults to false in grpc/keepalive.ClientParameters.
	// If DefaultKeepAlivePermitWithoutStream is true, the caller must explicitly set it in keepAliveCfg.

	return &ForwarderClient{
		c: clientCache,
		config: ForwarderClientConfig{
			KeepAlive:        keepAliveCfg,
			ExtraDialOptions: extraDialOptions,
		},
	}
}

// SendRoomEvent fetch or create a grpc client and send a room event to forwarder.
func (f *ForwarderClient) SendRoomEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomEvent, opts ...grpc.CallOption) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()

	return runReportingLatencyMetrics("SendRoomEvent", func() (*pb.Response, error) {
		return client.SendRoomEvent(ctx, in, opts...)
	})
}

// SendRoomReSync fetch or create a grpc client and send a room resync to forwarder.
func (f *ForwarderClient) SendRoomReSync(ctx context.Context, forwarder forwarder.Forwarder, resyncEvent *pb.RoomStatus, opts ...grpc.CallOption) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()
	return runReportingLatencyMetrics("SendRoomResync", func() (*pb.Response, error) {
		return client.SendRoomResync(ctx, resyncEvent, opts...)
	})
}

// SendRoomStatus fetch or create a grpc client and send a room status event to forwarder.
func (f *ForwarderClient) SendRoomStatus(ctx context.Context, forwarder forwarder.Forwarder, statusEvent *pb.RoomStatus, opts ...grpc.CallOption) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()
	return runReportingLatencyMetrics("SendRoomStatus", func() (*pb.Response, error) {
		return client.SendRoomStatus(ctx, statusEvent, opts...)
	})
}

// SendPlayerEvent fetch or create a grpc client and send a player event to forwarder.
func (f *ForwarderClient) SendPlayerEvent(ctx context.Context, forwarder forwarder.Forwarder, playerEvent *pb.PlayerEvent, opts ...grpc.CallOption) (*pb.Response, error) {
	client, err := f.getGrpcClient(Address(forwarder.Address))
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout*time.Millisecond)
	defer cancel()
	return runReportingLatencyMetrics("SendPlayerEvent", func() (*pb.Response, error) {
		return client.SendPlayerEvent(ctx, playerEvent, opts...)
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
	// Attempt to get an existing client (which is pb.GRPCForwarderClient) from cache
	clientFromCacheInterface, found := f.c.Get(string(address))
	if found {
		return clientFromCacheInterface.(pb.GRPCForwarderClient), nil
	}

	// Client not in cache or expired, create a new connection and then the gRPC client
	// Pass the stored ExtraDialOptions to createGRPCConnection
	conn, err := f.createGRPCConnection(string(address), f.config.ExtraDialOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection for %s: %w", address, err)
	}

	// grpcClient := pb.NewGRPCForwarderClient(conn) // This line was unused if we cache conn and create client on demand.
	// Store the *grpc.ClientConn in the cache, as suggested by OnEvicted logic.
	f.c.Set(string(address), conn, cache.DefaultExpiration)
	return pb.NewGRPCForwarderClient(conn), nil // Return a new client created from this connection
}

func (f *ForwarderClient) createGRPCConnection(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if address == "" {
		return nil, errors.NewErrInvalidArgument("no rpc server address informed")
	}

	zap.L().Debug(fmt.Sprintf("Creating new gRPC connection to: %s", address)) // Changed to Debug

	tracer := opentracing.GlobalTracer()

	// Base dial options that are always applied
	baseOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(f.config.KeepAlive),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	}

	// Append any additionally provided options (e.g., grpc.WithBlock() for tests)
	allDialOptions := append(baseOptions, opts...)

	conn, err := grpc.NewClient(address, allDialOptions...)
	if err != nil {
		zap.L().Error(fmt.Sprintf("grpc.DialContext failed for %s: %v", address, err))
		return nil, err
	}
	zap.L().Debug(fmt.Sprintf("Successfully connected to gRPC server at: %s", address))
	return conn, nil
}
