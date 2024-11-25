// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: api/v1/operations.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	OperationsService_ListOperations_FullMethodName  = "/api.v1.OperationsService/ListOperations"
	OperationsService_CancelOperation_FullMethodName = "/api.v1.OperationsService/CancelOperation"
	OperationsService_GetOperation_FullMethodName    = "/api.v1.OperationsService/GetOperation"
)

// OperationsServiceClient is the client API for OperationsService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Service that deals with scheduler operations.
type OperationsServiceClient interface {
	// List operations based on a scheduler.
	ListOperations(ctx context.Context, in *ListOperationsRequest, opts ...grpc.CallOption) (*ListOperationsResponse, error)
	// Cancel operation based on scheduler name and operation ID
	CancelOperation(ctx context.Context, in *CancelOperationRequest, opts ...grpc.CallOption) (*CancelOperationResponse, error)
	// Get operation based on scheduler name and operation ID
	GetOperation(ctx context.Context, in *GetOperationRequest, opts ...grpc.CallOption) (*GetOperationResponse, error)
}

type operationsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewOperationsServiceClient(cc grpc.ClientConnInterface) OperationsServiceClient {
	return &operationsServiceClient{cc}
}

func (c *operationsServiceClient) ListOperations(ctx context.Context, in *ListOperationsRequest, opts ...grpc.CallOption) (*ListOperationsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListOperationsResponse)
	err := c.cc.Invoke(ctx, OperationsService_ListOperations_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *operationsServiceClient) CancelOperation(ctx context.Context, in *CancelOperationRequest, opts ...grpc.CallOption) (*CancelOperationResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CancelOperationResponse)
	err := c.cc.Invoke(ctx, OperationsService_CancelOperation_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *operationsServiceClient) GetOperation(ctx context.Context, in *GetOperationRequest, opts ...grpc.CallOption) (*GetOperationResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetOperationResponse)
	err := c.cc.Invoke(ctx, OperationsService_GetOperation_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OperationsServiceServer is the server API for OperationsService service.
// All implementations must embed UnimplementedOperationsServiceServer
// for forward compatibility.
//
// Service that deals with scheduler operations.
type OperationsServiceServer interface {
	// List operations based on a scheduler.
	ListOperations(context.Context, *ListOperationsRequest) (*ListOperationsResponse, error)
	// Cancel operation based on scheduler name and operation ID
	CancelOperation(context.Context, *CancelOperationRequest) (*CancelOperationResponse, error)
	// Get operation based on scheduler name and operation ID
	GetOperation(context.Context, *GetOperationRequest) (*GetOperationResponse, error)
	mustEmbedUnimplementedOperationsServiceServer()
}

// UnimplementedOperationsServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedOperationsServiceServer struct{}

func (UnimplementedOperationsServiceServer) ListOperations(context.Context, *ListOperationsRequest) (*ListOperationsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListOperations not implemented")
}
func (UnimplementedOperationsServiceServer) CancelOperation(context.Context, *CancelOperationRequest) (*CancelOperationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CancelOperation not implemented")
}
func (UnimplementedOperationsServiceServer) GetOperation(context.Context, *GetOperationRequest) (*GetOperationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetOperation not implemented")
}
func (UnimplementedOperationsServiceServer) mustEmbedUnimplementedOperationsServiceServer() {}
func (UnimplementedOperationsServiceServer) testEmbeddedByValue()                           {}

// UnsafeOperationsServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OperationsServiceServer will
// result in compilation errors.
type UnsafeOperationsServiceServer interface {
	mustEmbedUnimplementedOperationsServiceServer()
}

func RegisterOperationsServiceServer(s grpc.ServiceRegistrar, srv OperationsServiceServer) {
	// If the following call pancis, it indicates UnimplementedOperationsServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&OperationsService_ServiceDesc, srv)
}

func _OperationsService_ListOperations_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListOperationsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OperationsServiceServer).ListOperations(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OperationsService_ListOperations_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OperationsServiceServer).ListOperations(ctx, req.(*ListOperationsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _OperationsService_CancelOperation_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CancelOperationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OperationsServiceServer).CancelOperation(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OperationsService_CancelOperation_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OperationsServiceServer).CancelOperation(ctx, req.(*CancelOperationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _OperationsService_GetOperation_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetOperationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OperationsServiceServer).GetOperation(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OperationsService_GetOperation_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OperationsServiceServer).GetOperation(ctx, req.(*GetOperationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// OperationsService_ServiceDesc is the grpc.ServiceDesc for OperationsService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var OperationsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.v1.OperationsService",
	HandlerType: (*OperationsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListOperations",
			Handler:    _OperationsService_ListOperations_Handler,
		},
		{
			MethodName: "CancelOperation",
			Handler:    _OperationsService_CancelOperation_Handler,
		},
		{
			MethodName: "GetOperation",
			Handler:    _OperationsService_GetOperation_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/operations.proto",
}
