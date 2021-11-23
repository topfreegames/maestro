// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.17.3
// source: api/v1/operations.proto

package v1

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// The list operation route request.
type ListOperationsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Scheduler name that the operations are part of.
	// NOTE: On http protocol, this operates as a path param.
	SchedulerName string `protobuf:"bytes,1,opt,name=scheduler_name,json=schedulerName,proto3" json:"scheduler_name,omitempty"`
	// Optional parameter for enabling operations sorting.
	// General `order_by` string syntax: <field> (<asc|desc>)
	// NOTE: On http protocol, this operates as a query parameter.
	OrderBy string `protobuf:"bytes,2,opt,name=order_by,json=orderBy,proto3" json:"order_by,omitempty"`
}

func (x *ListOperationsRequest) Reset() {
	*x = ListOperationsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_v1_operations_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListOperationsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListOperationsRequest) ProtoMessage() {}

func (x *ListOperationsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_api_v1_operations_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListOperationsRequest.ProtoReflect.Descriptor instead.
func (*ListOperationsRequest) Descriptor() ([]byte, []int) {
	return file_api_v1_operations_proto_rawDescGZIP(), []int{0}
}

func (x *ListOperationsRequest) GetSchedulerName() string {
	if x != nil {
		return x.SchedulerName
	}
	return ""
}

func (x *ListOperationsRequest) GetOrderBy() string {
	if x != nil {
		return x.OrderBy
	}
	return ""
}

// The list operation route response/reply
// There's a list for each operation major status
type ListOperationsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// List of the scheduler pending operations.
	PendingOperations []*Operation `protobuf:"bytes,1,rep,name=pending_operations,json=pendingOperations,proto3" json:"pending_operations,omitempty"`
	// List of the scheduler active operations.
	ActiveOperations []*Operation `protobuf:"bytes,2,rep,name=active_operations,json=activeOperations,proto3" json:"active_operations,omitempty"`
	// List of the scheduler finished (with error or not) operations.
	FinishedOperations []*Operation `protobuf:"bytes,3,rep,name=finished_operations,json=finishedOperations,proto3" json:"finished_operations,omitempty"`
}

func (x *ListOperationsResponse) Reset() {
	*x = ListOperationsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_v1_operations_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListOperationsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListOperationsResponse) ProtoMessage() {}

func (x *ListOperationsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_api_v1_operations_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListOperationsResponse.ProtoReflect.Descriptor instead.
func (*ListOperationsResponse) Descriptor() ([]byte, []int) {
	return file_api_v1_operations_proto_rawDescGZIP(), []int{1}
}

func (x *ListOperationsResponse) GetPendingOperations() []*Operation {
	if x != nil {
		return x.PendingOperations
	}
	return nil
}

func (x *ListOperationsResponse) GetActiveOperations() []*Operation {
	if x != nil {
		return x.ActiveOperations
	}
	return nil
}

func (x *ListOperationsResponse) GetFinishedOperations() []*Operation {
	if x != nil {
		return x.FinishedOperations
	}
	return nil
}

// The cancel operation request.
type CancelOperationRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Scheduler name where the operation relies on.
	SchedulerName string `protobuf:"bytes,1,opt,name=scheduler_name,json=schedulerName,proto3" json:"scheduler_name,omitempty"`
	// Operation identifier to be canceled.
	OperationId string `protobuf:"bytes,2,opt,name=operation_id,json=operationId,proto3" json:"operation_id,omitempty"`
}

func (x *CancelOperationRequest) Reset() {
	*x = CancelOperationRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_v1_operations_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CancelOperationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CancelOperationRequest) ProtoMessage() {}

func (x *CancelOperationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_api_v1_operations_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CancelOperationRequest.ProtoReflect.Descriptor instead.
func (*CancelOperationRequest) Descriptor() ([]byte, []int) {
	return file_api_v1_operations_proto_rawDescGZIP(), []int{2}
}

func (x *CancelOperationRequest) GetSchedulerName() string {
	if x != nil {
		return x.SchedulerName
	}
	return ""
}

func (x *CancelOperationRequest) GetOperationId() string {
	if x != nil {
		return x.OperationId
	}
	return ""
}

// Empty response of the cancel operation request.
type CancelOperationResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *CancelOperationResponse) Reset() {
	*x = CancelOperationResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_v1_operations_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CancelOperationResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CancelOperationResponse) ProtoMessage() {}

func (x *CancelOperationResponse) ProtoReflect() protoreflect.Message {
	mi := &file_api_v1_operations_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CancelOperationResponse.ProtoReflect.Descriptor instead.
func (*CancelOperationResponse) Descriptor() ([]byte, []int) {
	return file_api_v1_operations_proto_rawDescGZIP(), []int{3}
}

var File_api_v1_operations_proto protoreflect.FileDescriptor

var file_api_v1_operations_proto_rawDesc = []byte{
	0x0a, 0x17, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x2f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x61, 0x70, 0x69, 0x2e, 0x76,
	0x31, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e,
	0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x15, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x59, 0x0a, 0x15, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70,
	0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x25, 0x0a, 0x0e, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x5f, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c,
	0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x5f,
	0x62, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x42,
	0x79, 0x22, 0xde, 0x01, 0x0a, 0x16, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x40, 0x0a, 0x12,
	0x70, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76,
	0x31, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x11, 0x70, 0x65, 0x6e,
	0x64, 0x69, 0x6e, 0x67, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x3e,
	0x0a, 0x11, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61, 0x70, 0x69, 0x2e,
	0x76, 0x31, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x10, 0x61, 0x63,
	0x74, 0x69, 0x76, 0x65, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x42,
	0x0a, 0x13, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61, 0x70,
	0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x12,
	0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x22, 0x62, 0x0a, 0x16, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x4f, 0x70, 0x65, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x25, 0x0a, 0x0e,
	0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x4e,
	0x61, 0x6d, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x6f, 0x70, 0x65, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x22, 0x19, 0x0a, 0x17, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c,
	0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x32, 0xb8, 0x02, 0x0a, 0x11, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x82, 0x01, 0x0a, 0x0e, 0x4c, 0x69, 0x73, 0x74,
	0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x1d, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x61, 0x70, 0x69, 0x2e,
	0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x31, 0x82, 0xd3, 0xe4, 0x93, 0x02,
	0x2b, 0x12, 0x29, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x2f, 0x7b,
	0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x3d, 0x2a,
	0x7d, 0x2f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x9d, 0x01, 0x0a,
	0x0f, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x1e, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c,
	0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x1f, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c,
	0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x49, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x43, 0x22, 0x41, 0x2f, 0x73, 0x63, 0x68, 0x65,
	0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x2f, 0x7b, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65,
	0x72, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x3d, 0x2a, 0x7d, 0x2f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x7b, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f,
	0x69, 0x64, 0x3d, 0x2a, 0x7d, 0x2f, 0x63, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x42, 0x51, 0x0a, 0x23,
	0x63, 0x6f, 0x6d, 0x2e, 0x74, 0x6f, 0x70, 0x66, 0x72, 0x65, 0x65, 0x67, 0x61, 0x6d, 0x65, 0x73,
	0x2e, 0x6d, 0x61, 0x65, 0x73, 0x74, 0x72, 0x6f, 0x2e, 0x70, 0x6b, 0x67, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x74, 0x6f, 0x70, 0x66, 0x72, 0x65, 0x65, 0x67, 0x61, 0x6d, 0x65, 0x73, 0x2f, 0x6d, 0x61, 0x65,
	0x73, 0x74, 0x72, 0x6f, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_api_v1_operations_proto_rawDescOnce sync.Once
	file_api_v1_operations_proto_rawDescData = file_api_v1_operations_proto_rawDesc
)

func file_api_v1_operations_proto_rawDescGZIP() []byte {
	file_api_v1_operations_proto_rawDescOnce.Do(func() {
		file_api_v1_operations_proto_rawDescData = protoimpl.X.CompressGZIP(file_api_v1_operations_proto_rawDescData)
	})
	return file_api_v1_operations_proto_rawDescData
}

var file_api_v1_operations_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_api_v1_operations_proto_goTypes = []interface{}{
	(*ListOperationsRequest)(nil),   // 0: api.v1.ListOperationsRequest
	(*ListOperationsResponse)(nil),  // 1: api.v1.ListOperationsResponse
	(*CancelOperationRequest)(nil),  // 2: api.v1.CancelOperationRequest
	(*CancelOperationResponse)(nil), // 3: api.v1.CancelOperationResponse
	(*Operation)(nil),               // 4: api.v1.Operation
}
var file_api_v1_operations_proto_depIdxs = []int32{
	4, // 0: api.v1.ListOperationsResponse.pending_operations:type_name -> api.v1.Operation
	4, // 1: api.v1.ListOperationsResponse.active_operations:type_name -> api.v1.Operation
	4, // 2: api.v1.ListOperationsResponse.finished_operations:type_name -> api.v1.Operation
	0, // 3: api.v1.OperationsService.ListOperations:input_type -> api.v1.ListOperationsRequest
	2, // 4: api.v1.OperationsService.CancelOperation:input_type -> api.v1.CancelOperationRequest
	1, // 5: api.v1.OperationsService.ListOperations:output_type -> api.v1.ListOperationsResponse
	3, // 6: api.v1.OperationsService.CancelOperation:output_type -> api.v1.CancelOperationResponse
	5, // [5:7] is the sub-list for method output_type
	3, // [3:5] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_api_v1_operations_proto_init() }
func file_api_v1_operations_proto_init() {
	if File_api_v1_operations_proto != nil {
		return
	}
	file_api_v1_messages_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_api_v1_operations_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListOperationsRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_v1_operations_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListOperationsResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_v1_operations_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CancelOperationRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_v1_operations_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CancelOperationResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_api_v1_operations_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_api_v1_operations_proto_goTypes,
		DependencyIndexes: file_api_v1_operations_proto_depIdxs,
		MessageInfos:      file_api_v1_operations_proto_msgTypes,
	}.Build()
	File_api_v1_operations_proto = out.File
	file_api_v1_operations_proto_rawDesc = nil
	file_api_v1_operations_proto_goTypes = nil
	file_api_v1_operations_proto_depIdxs = nil
}
