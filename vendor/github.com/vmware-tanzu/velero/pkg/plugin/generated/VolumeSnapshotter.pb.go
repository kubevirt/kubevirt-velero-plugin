// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v4.25.2
// source: VolumeSnapshotter.proto

package generated

import (
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

type CreateVolumeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin     string `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	SnapshotID string `protobuf:"bytes,2,opt,name=snapshotID,proto3" json:"snapshotID,omitempty"`
	VolumeType string `protobuf:"bytes,3,opt,name=volumeType,proto3" json:"volumeType,omitempty"`
	VolumeAZ   string `protobuf:"bytes,4,opt,name=volumeAZ,proto3" json:"volumeAZ,omitempty"`
	Iops       int64  `protobuf:"varint,5,opt,name=iops,proto3" json:"iops,omitempty"`
}

func (x *CreateVolumeRequest) Reset() {
	*x = CreateVolumeRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateVolumeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateVolumeRequest) ProtoMessage() {}

func (x *CreateVolumeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateVolumeRequest.ProtoReflect.Descriptor instead.
func (*CreateVolumeRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{0}
}

func (x *CreateVolumeRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *CreateVolumeRequest) GetSnapshotID() string {
	if x != nil {
		return x.SnapshotID
	}
	return ""
}

func (x *CreateVolumeRequest) GetVolumeType() string {
	if x != nil {
		return x.VolumeType
	}
	return ""
}

func (x *CreateVolumeRequest) GetVolumeAZ() string {
	if x != nil {
		return x.VolumeAZ
	}
	return ""
}

func (x *CreateVolumeRequest) GetIops() int64 {
	if x != nil {
		return x.Iops
	}
	return 0
}

type CreateVolumeResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	VolumeID string `protobuf:"bytes,1,opt,name=volumeID,proto3" json:"volumeID,omitempty"`
}

func (x *CreateVolumeResponse) Reset() {
	*x = CreateVolumeResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateVolumeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateVolumeResponse) ProtoMessage() {}

func (x *CreateVolumeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateVolumeResponse.ProtoReflect.Descriptor instead.
func (*CreateVolumeResponse) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{1}
}

func (x *CreateVolumeResponse) GetVolumeID() string {
	if x != nil {
		return x.VolumeID
	}
	return ""
}

type GetVolumeInfoRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin   string `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	VolumeID string `protobuf:"bytes,2,opt,name=volumeID,proto3" json:"volumeID,omitempty"`
	VolumeAZ string `protobuf:"bytes,3,opt,name=volumeAZ,proto3" json:"volumeAZ,omitempty"`
}

func (x *GetVolumeInfoRequest) Reset() {
	*x = GetVolumeInfoRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetVolumeInfoRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetVolumeInfoRequest) ProtoMessage() {}

func (x *GetVolumeInfoRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetVolumeInfoRequest.ProtoReflect.Descriptor instead.
func (*GetVolumeInfoRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{2}
}

func (x *GetVolumeInfoRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *GetVolumeInfoRequest) GetVolumeID() string {
	if x != nil {
		return x.VolumeID
	}
	return ""
}

func (x *GetVolumeInfoRequest) GetVolumeAZ() string {
	if x != nil {
		return x.VolumeAZ
	}
	return ""
}

type GetVolumeInfoResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	VolumeType string `protobuf:"bytes,1,opt,name=volumeType,proto3" json:"volumeType,omitempty"`
	Iops       int64  `protobuf:"varint,2,opt,name=iops,proto3" json:"iops,omitempty"`
}

func (x *GetVolumeInfoResponse) Reset() {
	*x = GetVolumeInfoResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetVolumeInfoResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetVolumeInfoResponse) ProtoMessage() {}

func (x *GetVolumeInfoResponse) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetVolumeInfoResponse.ProtoReflect.Descriptor instead.
func (*GetVolumeInfoResponse) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{3}
}

func (x *GetVolumeInfoResponse) GetVolumeType() string {
	if x != nil {
		return x.VolumeType
	}
	return ""
}

func (x *GetVolumeInfoResponse) GetIops() int64 {
	if x != nil {
		return x.Iops
	}
	return 0
}

type CreateSnapshotRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin   string            `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	VolumeID string            `protobuf:"bytes,2,opt,name=volumeID,proto3" json:"volumeID,omitempty"`
	VolumeAZ string            `protobuf:"bytes,3,opt,name=volumeAZ,proto3" json:"volumeAZ,omitempty"`
	Tags     map[string]string `protobuf:"bytes,4,rep,name=tags,proto3" json:"tags,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *CreateSnapshotRequest) Reset() {
	*x = CreateSnapshotRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateSnapshotRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateSnapshotRequest) ProtoMessage() {}

func (x *CreateSnapshotRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateSnapshotRequest.ProtoReflect.Descriptor instead.
func (*CreateSnapshotRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{4}
}

func (x *CreateSnapshotRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *CreateSnapshotRequest) GetVolumeID() string {
	if x != nil {
		return x.VolumeID
	}
	return ""
}

func (x *CreateSnapshotRequest) GetVolumeAZ() string {
	if x != nil {
		return x.VolumeAZ
	}
	return ""
}

func (x *CreateSnapshotRequest) GetTags() map[string]string {
	if x != nil {
		return x.Tags
	}
	return nil
}

type CreateSnapshotResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SnapshotID string `protobuf:"bytes,1,opt,name=snapshotID,proto3" json:"snapshotID,omitempty"`
}

func (x *CreateSnapshotResponse) Reset() {
	*x = CreateSnapshotResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateSnapshotResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateSnapshotResponse) ProtoMessage() {}

func (x *CreateSnapshotResponse) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateSnapshotResponse.ProtoReflect.Descriptor instead.
func (*CreateSnapshotResponse) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{5}
}

func (x *CreateSnapshotResponse) GetSnapshotID() string {
	if x != nil {
		return x.SnapshotID
	}
	return ""
}

type DeleteSnapshotRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin     string `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	SnapshotID string `protobuf:"bytes,2,opt,name=snapshotID,proto3" json:"snapshotID,omitempty"`
}

func (x *DeleteSnapshotRequest) Reset() {
	*x = DeleteSnapshotRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteSnapshotRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteSnapshotRequest) ProtoMessage() {}

func (x *DeleteSnapshotRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteSnapshotRequest.ProtoReflect.Descriptor instead.
func (*DeleteSnapshotRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{6}
}

func (x *DeleteSnapshotRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *DeleteSnapshotRequest) GetSnapshotID() string {
	if x != nil {
		return x.SnapshotID
	}
	return ""
}

type GetVolumeIDRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin           string `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	PersistentVolume []byte `protobuf:"bytes,2,opt,name=persistentVolume,proto3" json:"persistentVolume,omitempty"`
}

func (x *GetVolumeIDRequest) Reset() {
	*x = GetVolumeIDRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetVolumeIDRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetVolumeIDRequest) ProtoMessage() {}

func (x *GetVolumeIDRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetVolumeIDRequest.ProtoReflect.Descriptor instead.
func (*GetVolumeIDRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{7}
}

func (x *GetVolumeIDRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *GetVolumeIDRequest) GetPersistentVolume() []byte {
	if x != nil {
		return x.PersistentVolume
	}
	return nil
}

type GetVolumeIDResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	VolumeID string `protobuf:"bytes,1,opt,name=volumeID,proto3" json:"volumeID,omitempty"`
}

func (x *GetVolumeIDResponse) Reset() {
	*x = GetVolumeIDResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetVolumeIDResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetVolumeIDResponse) ProtoMessage() {}

func (x *GetVolumeIDResponse) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetVolumeIDResponse.ProtoReflect.Descriptor instead.
func (*GetVolumeIDResponse) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{8}
}

func (x *GetVolumeIDResponse) GetVolumeID() string {
	if x != nil {
		return x.VolumeID
	}
	return ""
}

type SetVolumeIDRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin           string `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	PersistentVolume []byte `protobuf:"bytes,2,opt,name=persistentVolume,proto3" json:"persistentVolume,omitempty"`
	VolumeID         string `protobuf:"bytes,3,opt,name=volumeID,proto3" json:"volumeID,omitempty"`
}

func (x *SetVolumeIDRequest) Reset() {
	*x = SetVolumeIDRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetVolumeIDRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetVolumeIDRequest) ProtoMessage() {}

func (x *SetVolumeIDRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetVolumeIDRequest.ProtoReflect.Descriptor instead.
func (*SetVolumeIDRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{9}
}

func (x *SetVolumeIDRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *SetVolumeIDRequest) GetPersistentVolume() []byte {
	if x != nil {
		return x.PersistentVolume
	}
	return nil
}

func (x *SetVolumeIDRequest) GetVolumeID() string {
	if x != nil {
		return x.VolumeID
	}
	return ""
}

type SetVolumeIDResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PersistentVolume []byte `protobuf:"bytes,1,opt,name=persistentVolume,proto3" json:"persistentVolume,omitempty"`
}

func (x *SetVolumeIDResponse) Reset() {
	*x = SetVolumeIDResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetVolumeIDResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetVolumeIDResponse) ProtoMessage() {}

func (x *SetVolumeIDResponse) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetVolumeIDResponse.ProtoReflect.Descriptor instead.
func (*SetVolumeIDResponse) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{10}
}

func (x *SetVolumeIDResponse) GetPersistentVolume() []byte {
	if x != nil {
		return x.PersistentVolume
	}
	return nil
}

type VolumeSnapshotterInitRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Plugin string            `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	Config map[string]string `protobuf:"bytes,2,rep,name=config,proto3" json:"config,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *VolumeSnapshotterInitRequest) Reset() {
	*x = VolumeSnapshotterInitRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_VolumeSnapshotter_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VolumeSnapshotterInitRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VolumeSnapshotterInitRequest) ProtoMessage() {}

func (x *VolumeSnapshotterInitRequest) ProtoReflect() protoreflect.Message {
	mi := &file_VolumeSnapshotter_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VolumeSnapshotterInitRequest.ProtoReflect.Descriptor instead.
func (*VolumeSnapshotterInitRequest) Descriptor() ([]byte, []int) {
	return file_VolumeSnapshotter_proto_rawDescGZIP(), []int{11}
}

func (x *VolumeSnapshotterInitRequest) GetPlugin() string {
	if x != nil {
		return x.Plugin
	}
	return ""
}

func (x *VolumeSnapshotterInitRequest) GetConfig() map[string]string {
	if x != nil {
		return x.Config
	}
	return nil
}

var File_VolumeSnapshotter_proto protoreflect.FileDescriptor

var file_VolumeSnapshotter_proto_rawDesc = []byte{
	0x0a, 0x17, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74,
	0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09, 0x67, 0x65, 0x6e, 0x65, 0x72,
	0x61, 0x74, 0x65, 0x64, 0x1a, 0x0c, 0x53, 0x68, 0x61, 0x72, 0x65, 0x64, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0x9d, 0x01, 0x0a, 0x13, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x56, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x6c,
	0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x70, 0x6c, 0x75, 0x67,
	0x69, 0x6e, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x49, 0x44,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74,
	0x49, 0x44, 0x12, 0x1e, 0x0a, 0x0a, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x54, 0x79, 0x70, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x41, 0x5a, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x41, 0x5a, 0x12, 0x12,
	0x0a, 0x04, 0x69, 0x6f, 0x70, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x69, 0x6f,
	0x70, 0x73, 0x22, 0x32, 0x0a, 0x14, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x56, 0x6f, 0x6c, 0x75,
	0x6d, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f,
	0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x76, 0x6f,
	0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x22, 0x66, 0x0a, 0x14, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16,
	0x0a, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65,
	0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65,
	0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x41, 0x5a, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x41, 0x5a, 0x22, 0x4b,
	0x0a, 0x15, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x76, 0x6f, 0x6c, 0x75, 0x6d,
	0x65, 0x54, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x76, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x69, 0x6f, 0x70, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x69, 0x6f, 0x70, 0x73, 0x22, 0xe0, 0x01, 0x0a, 0x15,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x1a, 0x0a,
	0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x41, 0x5a, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x76, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x41, 0x5a, 0x12, 0x3e, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x04, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x61, 0x67, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52,
	0x04, 0x74, 0x61, 0x67, 0x73, 0x1a, 0x37, 0x0a, 0x09, 0x54, 0x61, 0x67, 0x73, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x38,
	0x0a, 0x16, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x6e, 0x61, 0x70,
	0x73, 0x68, 0x6f, 0x74, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73, 0x6e,
	0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x49, 0x44, 0x22, 0x4f, 0x0a, 0x15, 0x44, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x6e, 0x61,
	0x70, 0x73, 0x68, 0x6f, 0x74, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73,
	0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x49, 0x44, 0x22, 0x58, 0x0a, 0x12, 0x47, 0x65, 0x74,
	0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x16, 0x0a, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x2a, 0x0a, 0x10, 0x70, 0x65, 0x72, 0x73, 0x69,
	0x73, 0x74, 0x65, 0x6e, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x10, 0x70, 0x65, 0x72, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x74, 0x56, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x22, 0x31, 0x0a, 0x13, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65,
	0x49, 0x44, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f,
	0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x76, 0x6f,
	0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x22, 0x74, 0x0a, 0x12, 0x53, 0x65, 0x74, 0x56, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06,
	0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x70, 0x6c,
	0x75, 0x67, 0x69, 0x6e, 0x12, 0x2a, 0x0a, 0x10, 0x70, 0x65, 0x72, 0x73, 0x69, 0x73, 0x74, 0x65,
	0x6e, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x10,
	0x70, 0x65, 0x72, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65,
	0x12, 0x1a, 0x0a, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x76, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x22, 0x41, 0x0a, 0x13,
	0x53, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x2a, 0x0a, 0x10, 0x70, 0x65, 0x72, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e,
	0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x10, 0x70,
	0x65, 0x72, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x22,
	0xbe, 0x01, 0x0a, 0x1c, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68,
	0x6f, 0x74, 0x74, 0x65, 0x72, 0x49, 0x6e, 0x69, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x16, 0x0a, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x4b, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72,
	0x61, 0x74, 0x65, 0x64, 0x2e, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73,
	0x68, 0x6f, 0x74, 0x74, 0x65, 0x72, 0x49, 0x6e, 0x69, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x06, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x1a, 0x39, 0x0a, 0x0b, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01,
	0x32, 0xc0, 0x04, 0x0a, 0x11, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73,
	0x68, 0x6f, 0x74, 0x74, 0x65, 0x72, 0x12, 0x41, 0x0a, 0x04, 0x49, 0x6e, 0x69, 0x74, 0x12, 0x27,
	0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x56, 0x6f, 0x6c, 0x75, 0x6d,
	0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x74, 0x65, 0x72, 0x49, 0x6e, 0x69, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61,
	0x74, 0x65, 0x64, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x5b, 0x0a, 0x18, 0x43, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x46, 0x72, 0x6f, 0x6d, 0x53, 0x6e, 0x61,
	0x70, 0x73, 0x68, 0x6f, 0x74, 0x12, 0x1e, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65,
	0x64, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1f, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65,
	0x64, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x52, 0x0a, 0x0d, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x1f, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61,
	0x74, 0x65, 0x64, 0x2e, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x6e, 0x66,
	0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x20, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72,
	0x61, 0x74, 0x65, 0x64, 0x2e, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x6e,
	0x66, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x55, 0x0a, 0x0e, 0x43, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x12, 0x20, 0x2e, 0x67,
	0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53,
	0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x21,
	0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x44, 0x0a, 0x0e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73,
	0x68, 0x6f, 0x74, 0x12, 0x20, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e,
	0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65,
	0x64, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x4c, 0x0a, 0x0b, 0x47, 0x65, 0x74, 0x56, 0x6f,
	0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x12, 0x1d, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74,
	0x65, 0x64, 0x2e, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65,
	0x64, 0x2e, 0x47, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4c, 0x0a, 0x0b, 0x53, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75,
	0x6d, 0x65, 0x49, 0x44, 0x12, 0x1d, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64,
	0x2e, 0x53, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e,
	0x53, 0x65, 0x74, 0x56, 0x6f, 0x6c, 0x75, 0x6d, 0x65, 0x49, 0x44, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x42, 0x35, 0x5a, 0x33, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x76, 0x6d, 0x77, 0x61, 0x72, 0x65, 0x2d, 0x74, 0x61, 0x6e, 0x7a, 0x75, 0x2f, 0x76,
	0x65, 0x6c, 0x65, 0x72, 0x6f, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e,
	0x2f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_VolumeSnapshotter_proto_rawDescOnce sync.Once
	file_VolumeSnapshotter_proto_rawDescData = file_VolumeSnapshotter_proto_rawDesc
)

func file_VolumeSnapshotter_proto_rawDescGZIP() []byte {
	file_VolumeSnapshotter_proto_rawDescOnce.Do(func() {
		file_VolumeSnapshotter_proto_rawDescData = protoimpl.X.CompressGZIP(file_VolumeSnapshotter_proto_rawDescData)
	})
	return file_VolumeSnapshotter_proto_rawDescData
}

var file_VolumeSnapshotter_proto_msgTypes = make([]protoimpl.MessageInfo, 14)
var file_VolumeSnapshotter_proto_goTypes = []interface{}{
	(*CreateVolumeRequest)(nil),          // 0: generated.CreateVolumeRequest
	(*CreateVolumeResponse)(nil),         // 1: generated.CreateVolumeResponse
	(*GetVolumeInfoRequest)(nil),         // 2: generated.GetVolumeInfoRequest
	(*GetVolumeInfoResponse)(nil),        // 3: generated.GetVolumeInfoResponse
	(*CreateSnapshotRequest)(nil),        // 4: generated.CreateSnapshotRequest
	(*CreateSnapshotResponse)(nil),       // 5: generated.CreateSnapshotResponse
	(*DeleteSnapshotRequest)(nil),        // 6: generated.DeleteSnapshotRequest
	(*GetVolumeIDRequest)(nil),           // 7: generated.GetVolumeIDRequest
	(*GetVolumeIDResponse)(nil),          // 8: generated.GetVolumeIDResponse
	(*SetVolumeIDRequest)(nil),           // 9: generated.SetVolumeIDRequest
	(*SetVolumeIDResponse)(nil),          // 10: generated.SetVolumeIDResponse
	(*VolumeSnapshotterInitRequest)(nil), // 11: generated.VolumeSnapshotterInitRequest
	nil,                                  // 12: generated.CreateSnapshotRequest.TagsEntry
	nil,                                  // 13: generated.VolumeSnapshotterInitRequest.ConfigEntry
	(*Empty)(nil),                        // 14: generated.Empty
}
var file_VolumeSnapshotter_proto_depIdxs = []int32{
	12, // 0: generated.CreateSnapshotRequest.tags:type_name -> generated.CreateSnapshotRequest.TagsEntry
	13, // 1: generated.VolumeSnapshotterInitRequest.config:type_name -> generated.VolumeSnapshotterInitRequest.ConfigEntry
	11, // 2: generated.VolumeSnapshotter.Init:input_type -> generated.VolumeSnapshotterInitRequest
	0,  // 3: generated.VolumeSnapshotter.CreateVolumeFromSnapshot:input_type -> generated.CreateVolumeRequest
	2,  // 4: generated.VolumeSnapshotter.GetVolumeInfo:input_type -> generated.GetVolumeInfoRequest
	4,  // 5: generated.VolumeSnapshotter.CreateSnapshot:input_type -> generated.CreateSnapshotRequest
	6,  // 6: generated.VolumeSnapshotter.DeleteSnapshot:input_type -> generated.DeleteSnapshotRequest
	7,  // 7: generated.VolumeSnapshotter.GetVolumeID:input_type -> generated.GetVolumeIDRequest
	9,  // 8: generated.VolumeSnapshotter.SetVolumeID:input_type -> generated.SetVolumeIDRequest
	14, // 9: generated.VolumeSnapshotter.Init:output_type -> generated.Empty
	1,  // 10: generated.VolumeSnapshotter.CreateVolumeFromSnapshot:output_type -> generated.CreateVolumeResponse
	3,  // 11: generated.VolumeSnapshotter.GetVolumeInfo:output_type -> generated.GetVolumeInfoResponse
	5,  // 12: generated.VolumeSnapshotter.CreateSnapshot:output_type -> generated.CreateSnapshotResponse
	14, // 13: generated.VolumeSnapshotter.DeleteSnapshot:output_type -> generated.Empty
	8,  // 14: generated.VolumeSnapshotter.GetVolumeID:output_type -> generated.GetVolumeIDResponse
	10, // 15: generated.VolumeSnapshotter.SetVolumeID:output_type -> generated.SetVolumeIDResponse
	9,  // [9:16] is the sub-list for method output_type
	2,  // [2:9] is the sub-list for method input_type
	2,  // [2:2] is the sub-list for extension type_name
	2,  // [2:2] is the sub-list for extension extendee
	0,  // [0:2] is the sub-list for field type_name
}

func init() { file_VolumeSnapshotter_proto_init() }
func file_VolumeSnapshotter_proto_init() {
	if File_VolumeSnapshotter_proto != nil {
		return
	}
	file_Shared_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_VolumeSnapshotter_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateVolumeRequest); i {
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
		file_VolumeSnapshotter_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateVolumeResponse); i {
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
		file_VolumeSnapshotter_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetVolumeInfoRequest); i {
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
		file_VolumeSnapshotter_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetVolumeInfoResponse); i {
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
		file_VolumeSnapshotter_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateSnapshotRequest); i {
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
		file_VolumeSnapshotter_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateSnapshotResponse); i {
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
		file_VolumeSnapshotter_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteSnapshotRequest); i {
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
		file_VolumeSnapshotter_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetVolumeIDRequest); i {
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
		file_VolumeSnapshotter_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetVolumeIDResponse); i {
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
		file_VolumeSnapshotter_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetVolumeIDRequest); i {
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
		file_VolumeSnapshotter_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetVolumeIDResponse); i {
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
		file_VolumeSnapshotter_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VolumeSnapshotterInitRequest); i {
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
			RawDescriptor: file_VolumeSnapshotter_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   14,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_VolumeSnapshotter_proto_goTypes,
		DependencyIndexes: file_VolumeSnapshotter_proto_depIdxs,
		MessageInfos:      file_VolumeSnapshotter_proto_msgTypes,
	}.Build()
	File_VolumeSnapshotter_proto = out.File
	file_VolumeSnapshotter_proto_rawDesc = nil
	file_VolumeSnapshotter_proto_goTypes = nil
	file_VolumeSnapshotter_proto_depIdxs = nil
}
