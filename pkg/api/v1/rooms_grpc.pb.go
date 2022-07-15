// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion7

// RoomsServiceClient is the client API for RoomsService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RoomsServiceClient interface {
	// Updates a game room with ping data.
	UpdateRoomWithPing(ctx context.Context, in *UpdateRoomWithPingRequest, opts ...grpc.CallOption) (*UpdateRoomWithPingResponse, error)
	// Forward the incoming room event.
	ForwardRoomEvent(ctx context.Context, in *ForwardRoomEventRequest, opts ...grpc.CallOption) (*ForwardRoomEventResponse, error)
	// Forward the incoming player event.
	ForwardPlayerEvent(ctx context.Context, in *ForwardPlayerEventRequest, opts ...grpc.CallOption) (*ForwardPlayerEventResponse, error)
	// Deprecated: Do not use.
	// Endpoint created for maintaining compatibility with previous maestro version (v9). It is currently deprecated.
	UpdateRoomStatus(ctx context.Context, in *UpdateRoomStatusRequest, opts ...grpc.CallOption) (*UpdateRoomStatusResponse, error)
	// Deprecated: Do not use.
	// Gets room public addresses.
	GetRoomAddress(ctx context.Context, in *GetRoomAddressRequest, opts ...grpc.CallOption) (*GetRoomAddressResponse, error)
}

type roomsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewRoomsServiceClient(cc grpc.ClientConnInterface) RoomsServiceClient {
	return &roomsServiceClient{cc}
}

func (c *roomsServiceClient) UpdateRoomWithPing(ctx context.Context, in *UpdateRoomWithPingRequest, opts ...grpc.CallOption) (*UpdateRoomWithPingResponse, error) {
	out := new(UpdateRoomWithPingResponse)
	err := c.cc.Invoke(ctx, "/api.v1.RoomsService/UpdateRoomWithPing", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *roomsServiceClient) ForwardRoomEvent(ctx context.Context, in *ForwardRoomEventRequest, opts ...grpc.CallOption) (*ForwardRoomEventResponse, error) {
	out := new(ForwardRoomEventResponse)
	err := c.cc.Invoke(ctx, "/api.v1.RoomsService/ForwardRoomEvent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *roomsServiceClient) ForwardPlayerEvent(ctx context.Context, in *ForwardPlayerEventRequest, opts ...grpc.CallOption) (*ForwardPlayerEventResponse, error) {
	out := new(ForwardPlayerEventResponse)
	err := c.cc.Invoke(ctx, "/api.v1.RoomsService/ForwardPlayerEvent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Deprecated: Do not use.
func (c *roomsServiceClient) UpdateRoomStatus(ctx context.Context, in *UpdateRoomStatusRequest, opts ...grpc.CallOption) (*UpdateRoomStatusResponse, error) {
	out := new(UpdateRoomStatusResponse)
	err := c.cc.Invoke(ctx, "/api.v1.RoomsService/UpdateRoomStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Deprecated: Do not use.
func (c *roomsServiceClient) GetRoomAddress(ctx context.Context, in *GetRoomAddressRequest, opts ...grpc.CallOption) (*GetRoomAddressResponse, error) {
	out := new(GetRoomAddressResponse)
	err := c.cc.Invoke(ctx, "/api.v1.RoomsService/GetRoomAddress", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RoomsServiceServer is the server API for RoomsService service.
// All implementations must embed UnimplementedRoomsServiceServer
// for forward compatibility
type RoomsServiceServer interface {
	// Updates a game room with ping data.
	UpdateRoomWithPing(context.Context, *UpdateRoomWithPingRequest) (*UpdateRoomWithPingResponse, error)
	// Forward the incoming room event.
	ForwardRoomEvent(context.Context, *ForwardRoomEventRequest) (*ForwardRoomEventResponse, error)
	// Forward the incoming player event.
	ForwardPlayerEvent(context.Context, *ForwardPlayerEventRequest) (*ForwardPlayerEventResponse, error)
	// Deprecated: Do not use.
	// Endpoint created for maintaining compatibility with previous maestro version (v9). It is currently deprecated.
	UpdateRoomStatus(context.Context, *UpdateRoomStatusRequest) (*UpdateRoomStatusResponse, error)
	// Deprecated: Do not use.
	// Gets room public addresses.
	GetRoomAddress(context.Context, *GetRoomAddressRequest) (*GetRoomAddressResponse, error)
	mustEmbedUnimplementedRoomsServiceServer()
}

// UnimplementedRoomsServiceServer must be embedded to have forward compatible implementations.
type UnimplementedRoomsServiceServer struct {
}

func (UnimplementedRoomsServiceServer) UpdateRoomWithPing(context.Context, *UpdateRoomWithPingRequest) (*UpdateRoomWithPingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateRoomWithPing not implemented")
}
func (UnimplementedRoomsServiceServer) ForwardRoomEvent(context.Context, *ForwardRoomEventRequest) (*ForwardRoomEventResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ForwardRoomEvent not implemented")
}
func (UnimplementedRoomsServiceServer) ForwardPlayerEvent(context.Context, *ForwardPlayerEventRequest) (*ForwardPlayerEventResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ForwardPlayerEvent not implemented")
}
func (UnimplementedRoomsServiceServer) UpdateRoomStatus(context.Context, *UpdateRoomStatusRequest) (*UpdateRoomStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateRoomStatus not implemented")
}
func (UnimplementedRoomsServiceServer) GetRoomAddress(context.Context, *GetRoomAddressRequest) (*GetRoomAddressResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRoomAddress not implemented")
}
func (UnimplementedRoomsServiceServer) mustEmbedUnimplementedRoomsServiceServer() {}

// UnsafeRoomsServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RoomsServiceServer will
// result in compilation errors.
type UnsafeRoomsServiceServer interface {
	mustEmbedUnimplementedRoomsServiceServer()
}

func RegisterRoomsServiceServer(s *grpc.Server, srv RoomsServiceServer) {
	s.RegisterService(&_RoomsService_serviceDesc, srv)
}

func _RoomsService_UpdateRoomWithPing_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateRoomWithPingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomsServiceServer).UpdateRoomWithPing(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.RoomsService/UpdateRoomWithPing",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomsServiceServer).UpdateRoomWithPing(ctx, req.(*UpdateRoomWithPingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RoomsService_ForwardRoomEvent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ForwardRoomEventRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomsServiceServer).ForwardRoomEvent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.RoomsService/ForwardRoomEvent",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomsServiceServer).ForwardRoomEvent(ctx, req.(*ForwardRoomEventRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RoomsService_ForwardPlayerEvent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ForwardPlayerEventRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomsServiceServer).ForwardPlayerEvent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.RoomsService/ForwardPlayerEvent",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomsServiceServer).ForwardPlayerEvent(ctx, req.(*ForwardPlayerEventRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RoomsService_UpdateRoomStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateRoomStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomsServiceServer).UpdateRoomStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.RoomsService/UpdateRoomStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomsServiceServer).UpdateRoomStatus(ctx, req.(*UpdateRoomStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RoomsService_GetRoomAddress_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRoomAddressRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomsServiceServer).GetRoomAddress(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.RoomsService/GetRoomAddress",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomsServiceServer).GetRoomAddress(ctx, req.(*GetRoomAddressRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RoomsService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.v1.RoomsService",
	HandlerType: (*RoomsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UpdateRoomWithPing",
			Handler:    _RoomsService_UpdateRoomWithPing_Handler,
		},
		{
			MethodName: "ForwardRoomEvent",
			Handler:    _RoomsService_ForwardRoomEvent_Handler,
		},
		{
			MethodName: "ForwardPlayerEvent",
			Handler:    _RoomsService_ForwardPlayerEvent_Handler,
		},
		{
			MethodName: "UpdateRoomStatus",
			Handler:    _RoomsService_UpdateRoomStatus_Handler,
		},
		{
			MethodName: "GetRoomAddress",
			Handler:    _RoomsService_GetRoomAddress_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/rooms.proto",
}
