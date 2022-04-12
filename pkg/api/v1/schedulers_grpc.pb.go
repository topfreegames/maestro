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
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SchedulersServiceClient is the client API for SchedulersService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SchedulersServiceClient interface {
	// Lists all schedulers.
	ListSchedulers(ctx context.Context, in *ListSchedulersRequest, opts ...grpc.CallOption) (*ListSchedulersResponse, error)
	// Get Specific Scheduler by name
	GetScheduler(ctx context.Context, in *GetSchedulerRequest, opts ...grpc.CallOption) (*GetSchedulerResponse, error)
	// Create a scheduler.
	CreateScheduler(ctx context.Context, in *CreateSchedulerRequest, opts ...grpc.CallOption) (*CreateSchedulerResponse, error)
	// Given a amount, add rooms to a scheduler.
	AddRooms(ctx context.Context, in *AddRoomsRequest, opts ...grpc.CallOption) (*AddRoomsResponse, error)
	// Given a amount, remove rooms of a scheduler.
	RemoveRooms(ctx context.Context, in *RemoveRoomsRequest, opts ...grpc.CallOption) (*RemoveRoomsResponse, error)
	// Creates new scheduler version and switch it to active version.
	NewSchedulerVersion(ctx context.Context, in *NewSchedulerVersionRequest, opts ...grpc.CallOption) (*NewSchedulerVersionResponse, error)
	// Patch a scheduler and switch it to active version.
	PatchScheduler(ctx context.Context, in *PatchSchedulerRequest, opts ...grpc.CallOption) (*PatchSchedulerResponse, error)
	// Given a Scheduler, returns it's versions
	GetSchedulerVersions(ctx context.Context, in *GetSchedulerVersionsRequest, opts ...grpc.CallOption) (*GetSchedulerVersionsResponse, error)
	// Switch Active Version to Scheduler
	SwitchActiveVersion(ctx context.Context, in *SwitchActiveVersionRequest, opts ...grpc.CallOption) (*SwitchActiveVersionResponse, error)
	// List Scheduler and Game Rooms info by Game
	GetSchedulersInfo(ctx context.Context, in *GetSchedulersInfoRequest, opts ...grpc.CallOption) (*GetSchedulersInfoResponse, error)
}

type schedulersServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSchedulersServiceClient(cc grpc.ClientConnInterface) SchedulersServiceClient {
	return &schedulersServiceClient{cc}
}

func (c *schedulersServiceClient) ListSchedulers(ctx context.Context, in *ListSchedulersRequest, opts ...grpc.CallOption) (*ListSchedulersResponse, error) {
	out := new(ListSchedulersResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/ListSchedulers", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) GetScheduler(ctx context.Context, in *GetSchedulerRequest, opts ...grpc.CallOption) (*GetSchedulerResponse, error) {
	out := new(GetSchedulerResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/GetScheduler", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) CreateScheduler(ctx context.Context, in *CreateSchedulerRequest, opts ...grpc.CallOption) (*CreateSchedulerResponse, error) {
	out := new(CreateSchedulerResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/CreateScheduler", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) AddRooms(ctx context.Context, in *AddRoomsRequest, opts ...grpc.CallOption) (*AddRoomsResponse, error) {
	out := new(AddRoomsResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/AddRooms", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) RemoveRooms(ctx context.Context, in *RemoveRoomsRequest, opts ...grpc.CallOption) (*RemoveRoomsResponse, error) {
	out := new(RemoveRoomsResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/RemoveRooms", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) NewSchedulerVersion(ctx context.Context, in *NewSchedulerVersionRequest, opts ...grpc.CallOption) (*NewSchedulerVersionResponse, error) {
	out := new(NewSchedulerVersionResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/NewSchedulerVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) PatchScheduler(ctx context.Context, in *PatchSchedulerRequest, opts ...grpc.CallOption) (*PatchSchedulerResponse, error) {
	out := new(PatchSchedulerResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/PatchScheduler", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) GetSchedulerVersions(ctx context.Context, in *GetSchedulerVersionsRequest, opts ...grpc.CallOption) (*GetSchedulerVersionsResponse, error) {
	out := new(GetSchedulerVersionsResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/GetSchedulerVersions", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) SwitchActiveVersion(ctx context.Context, in *SwitchActiveVersionRequest, opts ...grpc.CallOption) (*SwitchActiveVersionResponse, error) {
	out := new(SwitchActiveVersionResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/SwitchActiveVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulersServiceClient) GetSchedulersInfo(ctx context.Context, in *GetSchedulersInfoRequest, opts ...grpc.CallOption) (*GetSchedulersInfoResponse, error) {
	out := new(GetSchedulersInfoResponse)
	err := c.cc.Invoke(ctx, "/api.v1.SchedulersService/GetSchedulersInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SchedulersServiceServer is the server API for SchedulersService service.
// All implementations must embed UnimplementedSchedulersServiceServer
// for forward compatibility
type SchedulersServiceServer interface {
	// Lists all schedulers.
	ListSchedulers(context.Context, *ListSchedulersRequest) (*ListSchedulersResponse, error)
	// Get Specific Scheduler by name
	GetScheduler(context.Context, *GetSchedulerRequest) (*GetSchedulerResponse, error)
	// Create a scheduler.
	CreateScheduler(context.Context, *CreateSchedulerRequest) (*CreateSchedulerResponse, error)
	// Given a amount, add rooms to a scheduler.
	AddRooms(context.Context, *AddRoomsRequest) (*AddRoomsResponse, error)
	// Given a amount, remove rooms of a scheduler.
	RemoveRooms(context.Context, *RemoveRoomsRequest) (*RemoveRoomsResponse, error)
	// Creates new scheduler version and switch it to active version.
	NewSchedulerVersion(context.Context, *NewSchedulerVersionRequest) (*NewSchedulerVersionResponse, error)
	// Patch a scheduler and switch it to active version.
	PatchScheduler(context.Context, *PatchSchedulerRequest) (*PatchSchedulerResponse, error)
	// Given a Scheduler, returns it's versions
	GetSchedulerVersions(context.Context, *GetSchedulerVersionsRequest) (*GetSchedulerVersionsResponse, error)
	// Switch Active Version to Scheduler
	SwitchActiveVersion(context.Context, *SwitchActiveVersionRequest) (*SwitchActiveVersionResponse, error)
	// List Scheduler and Game Rooms info by Game
	GetSchedulersInfo(context.Context, *GetSchedulersInfoRequest) (*GetSchedulersInfoResponse, error)
	mustEmbedUnimplementedSchedulersServiceServer()
}

// UnimplementedSchedulersServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSchedulersServiceServer struct {
}

func (UnimplementedSchedulersServiceServer) ListSchedulers(context.Context, *ListSchedulersRequest) (*ListSchedulersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSchedulers not implemented")
}
func (UnimplementedSchedulersServiceServer) GetScheduler(context.Context, *GetSchedulerRequest) (*GetSchedulerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetScheduler not implemented")
}
func (UnimplementedSchedulersServiceServer) CreateScheduler(context.Context, *CreateSchedulerRequest) (*CreateSchedulerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateScheduler not implemented")
}
func (UnimplementedSchedulersServiceServer) AddRooms(context.Context, *AddRoomsRequest) (*AddRoomsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddRooms not implemented")
}
func (UnimplementedSchedulersServiceServer) RemoveRooms(context.Context, *RemoveRoomsRequest) (*RemoveRoomsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveRooms not implemented")
}
func (UnimplementedSchedulersServiceServer) NewSchedulerVersion(context.Context, *NewSchedulerVersionRequest) (*NewSchedulerVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NewSchedulerVersion not implemented")
}
func (UnimplementedSchedulersServiceServer) PatchScheduler(context.Context, *PatchSchedulerRequest) (*PatchSchedulerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PatchScheduler not implemented")
}
func (UnimplementedSchedulersServiceServer) GetSchedulerVersions(context.Context, *GetSchedulerVersionsRequest) (*GetSchedulerVersionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSchedulerVersions not implemented")
}
func (UnimplementedSchedulersServiceServer) SwitchActiveVersion(context.Context, *SwitchActiveVersionRequest) (*SwitchActiveVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SwitchActiveVersion not implemented")
}
func (UnimplementedSchedulersServiceServer) GetSchedulersInfo(context.Context, *GetSchedulersInfoRequest) (*GetSchedulersInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSchedulersInfo not implemented")
}
func (UnimplementedSchedulersServiceServer) mustEmbedUnimplementedSchedulersServiceServer() {}

// UnsafeSchedulersServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SchedulersServiceServer will
// result in compilation errors.
type UnsafeSchedulersServiceServer interface {
	mustEmbedUnimplementedSchedulersServiceServer()
}

func RegisterSchedulersServiceServer(s grpc.ServiceRegistrar, srv SchedulersServiceServer) {
	s.RegisterService(&SchedulersService_ServiceDesc, srv)
}

func _SchedulersService_ListSchedulers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSchedulersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).ListSchedulers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/ListSchedulers",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).ListSchedulers(ctx, req.(*ListSchedulersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_GetScheduler_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSchedulerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).GetScheduler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/GetScheduler",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).GetScheduler(ctx, req.(*GetSchedulerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_CreateScheduler_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSchedulerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).CreateScheduler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/CreateScheduler",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).CreateScheduler(ctx, req.(*CreateSchedulerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_AddRooms_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddRoomsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).AddRooms(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/AddRooms",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).AddRooms(ctx, req.(*AddRoomsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_RemoveRooms_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveRoomsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).RemoveRooms(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/RemoveRooms",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).RemoveRooms(ctx, req.(*RemoveRoomsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_NewSchedulerVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NewSchedulerVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).NewSchedulerVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/NewSchedulerVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).NewSchedulerVersion(ctx, req.(*NewSchedulerVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_PatchScheduler_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PatchSchedulerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).PatchScheduler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/PatchScheduler",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).PatchScheduler(ctx, req.(*PatchSchedulerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_GetSchedulerVersions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSchedulerVersionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).GetSchedulerVersions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/GetSchedulerVersions",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).GetSchedulerVersions(ctx, req.(*GetSchedulerVersionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_SwitchActiveVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SwitchActiveVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).SwitchActiveVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/SwitchActiveVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).SwitchActiveVersion(ctx, req.(*SwitchActiveVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchedulersService_GetSchedulersInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSchedulersInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulersServiceServer).GetSchedulersInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v1.SchedulersService/GetSchedulersInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulersServiceServer).GetSchedulersInfo(ctx, req.(*GetSchedulersInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SchedulersService_ServiceDesc is the grpc.ServiceDesc for SchedulersService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SchedulersService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.v1.SchedulersService",
	HandlerType: (*SchedulersServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListSchedulers",
			Handler:    _SchedulersService_ListSchedulers_Handler,
		},
		{
			MethodName: "GetScheduler",
			Handler:    _SchedulersService_GetScheduler_Handler,
		},
		{
			MethodName: "CreateScheduler",
			Handler:    _SchedulersService_CreateScheduler_Handler,
		},
		{
			MethodName: "AddRooms",
			Handler:    _SchedulersService_AddRooms_Handler,
		},
		{
			MethodName: "RemoveRooms",
			Handler:    _SchedulersService_RemoveRooms_Handler,
		},
		{
			MethodName: "NewSchedulerVersion",
			Handler:    _SchedulersService_NewSchedulerVersion_Handler,
		},
		{
			MethodName: "PatchScheduler",
			Handler:    _SchedulersService_PatchScheduler_Handler,
		},
		{
			MethodName: "GetSchedulerVersions",
			Handler:    _SchedulersService_GetSchedulerVersions_Handler,
		},
		{
			MethodName: "SwitchActiveVersion",
			Handler:    _SchedulersService_SwitchActiveVersion_Handler,
		},
		{
			MethodName: "GetSchedulersInfo",
			Handler:    _SchedulersService_GetSchedulersInfo_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/schedulers.proto",
}
