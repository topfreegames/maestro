// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.17.3
// source: v1/schedulers.proto

package v1

import (
	reflect "reflect"
	sync "sync"

	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EmptyRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *EmptyRequest) Reset() {
	*x = EmptyRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EmptyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EmptyRequest) ProtoMessage() {}

func (x *EmptyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EmptyRequest.ProtoReflect.Descriptor instead.
func (*EmptyRequest) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{0}
}

// The list schedulers reply message.
type ListSchedulersReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Schedulers []*Scheduler `protobuf:"bytes,1,rep,name=schedulers,proto3" json:"schedulers,omitempty"`
}

func (x *ListSchedulersReply) Reset() {
	*x = ListSchedulersReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListSchedulersReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListSchedulersReply) ProtoMessage() {}

func (x *ListSchedulersReply) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListSchedulersReply.ProtoReflect.Descriptor instead.
func (*ListSchedulersReply) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{1}
}

func (x *ListSchedulersReply) GetSchedulers() []*Scheduler {
	if x != nil {
		return x.Schedulers
	}
	return nil
}

// The scheduler DTO.
type Scheduler struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name      string     `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Game      string     `protobuf:"bytes,2,opt,name=game,proto3" json:"game,omitempty"`
	State     string     `protobuf:"bytes,3,opt,name=state,proto3" json:"state,omitempty"`
	Version   string     `protobuf:"bytes,4,opt,name=version,proto3" json:"version,omitempty"`
	PortRange *PortRange `protobuf:"bytes,5,opt,name=port_range,json=portRange,proto3" json:"port_range,omitempty"`
}

func (x *Scheduler) Reset() {
	*x = Scheduler{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Scheduler) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Scheduler) ProtoMessage() {}

func (x *Scheduler) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Scheduler.ProtoReflect.Descriptor instead.
func (*Scheduler) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{2}
}

func (x *Scheduler) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Scheduler) GetGame() string {
	if x != nil {
		return x.Game
	}
	return ""
}

func (x *Scheduler) GetState() string {
	if x != nil {
		return x.State
	}
	return ""
}

func (x *Scheduler) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *Scheduler) GetPortRange() *PortRange {
	if x != nil {
		return x.PortRange
	}
	return nil
}

// Scheduler is the struct that defines a maestro scheduler
type CreateSchedulerRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Game    string `protobuf:"bytes,2,opt,name=game,proto3" json:"game,omitempty"`
	Version string `protobuf:"bytes,3,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *CreateSchedulerRequest) Reset() {
	*x = CreateSchedulerRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateSchedulerRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateSchedulerRequest) ProtoMessage() {}

func (x *CreateSchedulerRequest) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateSchedulerRequest.ProtoReflect.Descriptor instead.
func (*CreateSchedulerRequest) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{3}
}

func (x *CreateSchedulerRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CreateSchedulerRequest) GetGame() string {
	if x != nil {
		return x.Game
	}
	return ""
}

func (x *CreateSchedulerRequest) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

// The scheduler port range DTO.
type PortRange struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Start int32 `protobuf:"varint,1,opt,name=start,proto3" json:"start,omitempty"`
	End   int32 `protobuf:"varint,2,opt,name=end,proto3" json:"end,omitempty"`
}

func (x *PortRange) Reset() {
	*x = PortRange{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PortRange) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PortRange) ProtoMessage() {}

func (x *PortRange) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PortRange.ProtoReflect.Descriptor instead.
func (*PortRange) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{4}
}

func (x *PortRange) GetStart() int32 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *PortRange) GetEnd() int32 {
	if x != nil {
		return x.End
	}
	return 0
}

type ListOperationsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Scheduler string `protobuf:"bytes,1,opt,name=scheduler,proto3" json:"scheduler,omitempty"`
}

func (x *ListOperationsRequest) Reset() {
	*x = ListOperationsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListOperationsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListOperationsRequest) ProtoMessage() {}

func (x *ListOperationsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[5]
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
	return file_v1_schedulers_proto_rawDescGZIP(), []int{5}
}

func (x *ListOperationsRequest) GetScheduler() string {
	if x != nil {
		return x.Scheduler
	}
	return ""
}

type ListOperationsReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PendingOperations  []*Operation `protobuf:"bytes,1,rep,name=pending_operations,json=pendingOperations,proto3" json:"pending_operations,omitempty"`
	ActiveOperations   []*Operation `protobuf:"bytes,2,rep,name=active_operations,json=activeOperations,proto3" json:"active_operations,omitempty"`
	FinishedOperations []*Operation `protobuf:"bytes,3,rep,name=finished_operations,json=finishedOperations,proto3" json:"finished_operations,omitempty"`
}

func (x *ListOperationsReply) Reset() {
	*x = ListOperationsReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListOperationsReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListOperationsReply) ProtoMessage() {}

func (x *ListOperationsReply) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListOperationsReply.ProtoReflect.Descriptor instead.
func (*ListOperationsReply) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{6}
}

func (x *ListOperationsReply) GetPendingOperations() []*Operation {
	if x != nil {
		return x.PendingOperations
	}
	return nil
}

func (x *ListOperationsReply) GetActiveOperations() []*Operation {
	if x != nil {
		return x.ActiveOperations
	}
	return nil
}

func (x *ListOperationsReply) GetFinishedOperations() []*Operation {
	if x != nil {
		return x.FinishedOperations
	}
	return nil
}

type Operation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID             string `protobuf:"bytes,1,opt,name=iD,proto3" json:"iD,omitempty"`
	Status         string `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
	DefinitionName string `protobuf:"bytes,3,opt,name=definition_name,json=definitionName,proto3" json:"definition_name,omitempty"`
	SchedulerName  string `protobuf:"bytes,4,opt,name=scheduler_name,json=schedulerName,proto3" json:"scheduler_name,omitempty"`
}

func (x *Operation) Reset() {
	*x = Operation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_schedulers_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Operation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Operation) ProtoMessage() {}

func (x *Operation) ProtoReflect() protoreflect.Message {
	mi := &file_v1_schedulers_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Operation.ProtoReflect.Descriptor instead.
func (*Operation) Descriptor() ([]byte, []int) {
	return file_v1_schedulers_proto_rawDescGZIP(), []int{7}
}

func (x *Operation) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *Operation) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *Operation) GetDefinitionName() string {
	if x != nil {
		return x.DefinitionName
	}
	return ""
}

func (x *Operation) GetSchedulerName() string {
	if x != nil {
		return x.SchedulerName
	}
	return ""
}

var File_v1_schedulers_proto protoreflect.FileDescriptor

var file_v1_schedulers_proto_rawDesc = []byte{
	0x0a, 0x13, 0x76, 0x31, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x1a, 0x1c, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x0e, 0x0a, 0x0c, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x48, 0x0a, 0x13, 0x4c,
	0x69, 0x73, 0x74, 0x53, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x12, 0x31, 0x0a, 0x0a, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e,
	0x53, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x52, 0x0a, 0x73, 0x63, 0x68, 0x65, 0x64,
	0x75, 0x6c, 0x65, 0x72, 0x73, 0x22, 0x95, 0x01, 0x0a, 0x09, 0x53, 0x63, 0x68, 0x65, 0x64, 0x75,
	0x6c, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x67, 0x61, 0x6d, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x67, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x73,
	0x74, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74,
	0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x30, 0x0a, 0x0a, 0x70,
	0x6f, 0x72, 0x74, 0x5f, 0x72, 0x61, 0x6e, 0x67, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x11, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x50, 0x6f, 0x72, 0x74, 0x52, 0x61, 0x6e,
	0x67, 0x65, 0x52, 0x09, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x61, 0x6e, 0x67, 0x65, 0x22, 0x5a, 0x0a,
	0x16, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x67,
	0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x67, 0x61, 0x6d, 0x65, 0x12,
	0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x33, 0x0a, 0x09, 0x50, 0x6f, 0x72,
	0x74, 0x52, 0x61, 0x6e, 0x67, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x12, 0x10, 0x0a, 0x03,
	0x65, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x65, 0x6e, 0x64, 0x22, 0x35,
	0x0a, 0x15, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x63, 0x68, 0x65, 0x64,
	0x75, 0x6c, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x63, 0x68, 0x65,
	0x64, 0x75, 0x6c, 0x65, 0x72, 0x22, 0xdb, 0x01, 0x0a, 0x13, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70,
	0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x40, 0x0a,
	0x12, 0x70, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61, 0x70, 0x69, 0x2e,
	0x76, 0x31, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x11, 0x70, 0x65,
	0x6e, 0x64, 0x69, 0x6e, 0x67, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12,
	0x3e, 0x0a, 0x11, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x10, 0x61,
	0x63, 0x74, 0x69, 0x76, 0x65, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12,
	0x42, 0x0a, 0x13, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x5f, 0x6f, 0x70, 0x65, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x61,
	0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x12, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x22, 0x83, 0x01, 0x0a, 0x09, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69,
	0x44, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x27, 0x0a, 0x0f, 0x64, 0x65, 0x66,
	0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x25, 0x0a, 0x0e, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x5f,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x73, 0x63, 0x68, 0x65,
	0x64, 0x75, 0x6c, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x32, 0xc0, 0x02, 0x0a, 0x0a, 0x53, 0x63,
	0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x58, 0x0a, 0x0e, 0x4c, 0x69, 0x73, 0x74,
	0x53, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x14, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x1b, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x53, 0x63,
	0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x13, 0x82,
	0xd3, 0xe4, 0x93, 0x02, 0x0d, 0x12, 0x0b, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65,
	0x72, 0x73, 0x12, 0x5c, 0x0a, 0x0f, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x63, 0x68, 0x65,
	0x64, 0x75, 0x6c, 0x65, 0x72, 0x12, 0x1e, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x43,
	0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x11, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x53,
	0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x22, 0x16, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x10,
	0x22, 0x0b, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x73, 0x3a, 0x01, 0x2a,
	0x12, 0x7a, 0x0a, 0x0e, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x12, 0x1d, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74,
	0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1b, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x4f,
	0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x2c,
	0x82, 0xd3, 0xe4, 0x93, 0x02, 0x26, 0x12, 0x24, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c,
	0x65, 0x72, 0x73, 0x2f, 0x7b, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x72, 0x3d, 0x2a,
	0x7d, 0x2f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x42, 0x51, 0x0a, 0x23,
	0x63, 0x6f, 0x6d, 0x2e, 0x74, 0x6f, 0x70, 0x66, 0x72, 0x65, 0x65, 0x67, 0x61, 0x6d, 0x65, 0x73,
	0x2e, 0x6d, 0x61, 0x65, 0x73, 0x74, 0x72, 0x6f, 0x2e, 0x70, 0x6b, 0x67, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x74, 0x6f, 0x70, 0x66, 0x72, 0x65, 0x65, 0x67, 0x61, 0x6d, 0x65, 0x73, 0x2f, 0x6d, 0x61, 0x65,
	0x73, 0x74, 0x72, 0x6f, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_v1_schedulers_proto_rawDescOnce sync.Once
	file_v1_schedulers_proto_rawDescData = file_v1_schedulers_proto_rawDesc
)

func file_v1_schedulers_proto_rawDescGZIP() []byte {
	file_v1_schedulers_proto_rawDescOnce.Do(func() {
		file_v1_schedulers_proto_rawDescData = protoimpl.X.CompressGZIP(file_v1_schedulers_proto_rawDescData)
	})
	return file_v1_schedulers_proto_rawDescData
}

var file_v1_schedulers_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_v1_schedulers_proto_goTypes = []interface{}{
	(*EmptyRequest)(nil),           // 0: api.v1.EmptyRequest
	(*ListSchedulersReply)(nil),    // 1: api.v1.ListSchedulersReply
	(*Scheduler)(nil),              // 2: api.v1.Scheduler
	(*CreateSchedulerRequest)(nil), // 3: api.v1.CreateSchedulerRequest
	(*PortRange)(nil),              // 4: api.v1.PortRange
	(*ListOperationsRequest)(nil),  // 5: api.v1.ListOperationsRequest
	(*ListOperationsReply)(nil),    // 6: api.v1.ListOperationsReply
	(*Operation)(nil),              // 7: api.v1.Operation
}
var file_v1_schedulers_proto_depIdxs = []int32{
	2, // 0: api.v1.ListSchedulersReply.schedulers:type_name -> api.v1.Scheduler
	4, // 1: api.v1.Scheduler.port_range:type_name -> api.v1.PortRange
	7, // 2: api.v1.ListOperationsReply.pending_operations:type_name -> api.v1.Operation
	7, // 3: api.v1.ListOperationsReply.active_operations:type_name -> api.v1.Operation
	7, // 4: api.v1.ListOperationsReply.finished_operations:type_name -> api.v1.Operation
	0, // 5: api.v1.Schedulers.ListSchedulers:input_type -> api.v1.EmptyRequest
	3, // 6: api.v1.Schedulers.CreateScheduler:input_type -> api.v1.CreateSchedulerRequest
	5, // 7: api.v1.Schedulers.ListOperations:input_type -> api.v1.ListOperationsRequest
	1, // 8: api.v1.Schedulers.ListSchedulers:output_type -> api.v1.ListSchedulersReply
	2, // 9: api.v1.Schedulers.CreateScheduler:output_type -> api.v1.Scheduler
	6, // 10: api.v1.Schedulers.ListOperations:output_type -> api.v1.ListOperationsReply
	8, // [8:11] is the sub-list for method output_type
	5, // [5:8] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_v1_schedulers_proto_init() }
func file_v1_schedulers_proto_init() {
	if File_v1_schedulers_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_v1_schedulers_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EmptyRequest); i {
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
		file_v1_schedulers_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListSchedulersReply); i {
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
		file_v1_schedulers_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Scheduler); i {
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
		file_v1_schedulers_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateSchedulerRequest); i {
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
		file_v1_schedulers_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PortRange); i {
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
		file_v1_schedulers_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
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
		file_v1_schedulers_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListOperationsReply); i {
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
		file_v1_schedulers_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Operation); i {
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
			RawDescriptor: file_v1_schedulers_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_v1_schedulers_proto_goTypes,
		DependencyIndexes: file_v1_schedulers_proto_depIdxs,
		MessageInfos:      file_v1_schedulers_proto_msgTypes,
	}.Build()
	File_v1_schedulers_proto = out.File
	file_v1_schedulers_proto_rawDesc = nil
	file_v1_schedulers_proto_goTypes = nil
	file_v1_schedulers_proto_depIdxs = nil
}
