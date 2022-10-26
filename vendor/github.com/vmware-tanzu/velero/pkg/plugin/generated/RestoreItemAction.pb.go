// Code generated by protoc-gen-go. DO NOT EDIT.
// source: RestoreItemAction.proto

package generated

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type RestoreItemActionExecuteRequest struct {
	Plugin         string `protobuf:"bytes,1,opt,name=plugin" json:"plugin,omitempty"`
	Item           []byte `protobuf:"bytes,2,opt,name=item,proto3" json:"item,omitempty"`
	Restore        []byte `protobuf:"bytes,3,opt,name=restore,proto3" json:"restore,omitempty"`
	ItemFromBackup []byte `protobuf:"bytes,4,opt,name=itemFromBackup,proto3" json:"itemFromBackup,omitempty"`
}

func (m *RestoreItemActionExecuteRequest) Reset()         { *m = RestoreItemActionExecuteRequest{} }
func (m *RestoreItemActionExecuteRequest) String() string { return proto.CompactTextString(m) }
func (*RestoreItemActionExecuteRequest) ProtoMessage()    {}
func (*RestoreItemActionExecuteRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor5, []int{0}
}

func (m *RestoreItemActionExecuteRequest) GetPlugin() string {
	if m != nil {
		return m.Plugin
	}
	return ""
}

func (m *RestoreItemActionExecuteRequest) GetItem() []byte {
	if m != nil {
		return m.Item
	}
	return nil
}

func (m *RestoreItemActionExecuteRequest) GetRestore() []byte {
	if m != nil {
		return m.Restore
	}
	return nil
}

func (m *RestoreItemActionExecuteRequest) GetItemFromBackup() []byte {
	if m != nil {
		return m.ItemFromBackup
	}
	return nil
}

type RestoreItemActionExecuteResponse struct {
	Item            []byte                `protobuf:"bytes,1,opt,name=item,proto3" json:"item,omitempty"`
	AdditionalItems []*ResourceIdentifier `protobuf:"bytes,2,rep,name=additionalItems" json:"additionalItems,omitempty"`
	SkipRestore     bool                  `protobuf:"varint,3,opt,name=skipRestore" json:"skipRestore,omitempty"`
}

func (m *RestoreItemActionExecuteResponse) Reset()         { *m = RestoreItemActionExecuteResponse{} }
func (m *RestoreItemActionExecuteResponse) String() string { return proto.CompactTextString(m) }
func (*RestoreItemActionExecuteResponse) ProtoMessage()    {}
func (*RestoreItemActionExecuteResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor5, []int{1}
}

func (m *RestoreItemActionExecuteResponse) GetItem() []byte {
	if m != nil {
		return m.Item
	}
	return nil
}

func (m *RestoreItemActionExecuteResponse) GetAdditionalItems() []*ResourceIdentifier {
	if m != nil {
		return m.AdditionalItems
	}
	return nil
}

func (m *RestoreItemActionExecuteResponse) GetSkipRestore() bool {
	if m != nil {
		return m.SkipRestore
	}
	return false
}

type RestoreItemActionAppliesToRequest struct {
	Plugin string `protobuf:"bytes,1,opt,name=plugin" json:"plugin,omitempty"`
}

func (m *RestoreItemActionAppliesToRequest) Reset()         { *m = RestoreItemActionAppliesToRequest{} }
func (m *RestoreItemActionAppliesToRequest) String() string { return proto.CompactTextString(m) }
func (*RestoreItemActionAppliesToRequest) ProtoMessage()    {}
func (*RestoreItemActionAppliesToRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor5, []int{2}
}

func (m *RestoreItemActionAppliesToRequest) GetPlugin() string {
	if m != nil {
		return m.Plugin
	}
	return ""
}

type RestoreItemActionAppliesToResponse struct {
	ResourceSelector *ResourceSelector `protobuf:"bytes,1,opt,name=ResourceSelector" json:"ResourceSelector,omitempty"`
}

func (m *RestoreItemActionAppliesToResponse) Reset()         { *m = RestoreItemActionAppliesToResponse{} }
func (m *RestoreItemActionAppliesToResponse) String() string { return proto.CompactTextString(m) }
func (*RestoreItemActionAppliesToResponse) ProtoMessage()    {}
func (*RestoreItemActionAppliesToResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor5, []int{3}
}

func (m *RestoreItemActionAppliesToResponse) GetResourceSelector() *ResourceSelector {
	if m != nil {
		return m.ResourceSelector
	}
	return nil
}

func init() {
	proto.RegisterType((*RestoreItemActionExecuteRequest)(nil), "generated.RestoreItemActionExecuteRequest")
	proto.RegisterType((*RestoreItemActionExecuteResponse)(nil), "generated.RestoreItemActionExecuteResponse")
	proto.RegisterType((*RestoreItemActionAppliesToRequest)(nil), "generated.RestoreItemActionAppliesToRequest")
	proto.RegisterType((*RestoreItemActionAppliesToResponse)(nil), "generated.RestoreItemActionAppliesToResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for RestoreItemAction service

type RestoreItemActionClient interface {
	AppliesTo(ctx context.Context, in *RestoreItemActionAppliesToRequest, opts ...grpc.CallOption) (*RestoreItemActionAppliesToResponse, error)
	Execute(ctx context.Context, in *RestoreItemActionExecuteRequest, opts ...grpc.CallOption) (*RestoreItemActionExecuteResponse, error)
}

type restoreItemActionClient struct {
	cc *grpc.ClientConn
}

func NewRestoreItemActionClient(cc *grpc.ClientConn) RestoreItemActionClient {
	return &restoreItemActionClient{cc}
}

func (c *restoreItemActionClient) AppliesTo(ctx context.Context, in *RestoreItemActionAppliesToRequest, opts ...grpc.CallOption) (*RestoreItemActionAppliesToResponse, error) {
	out := new(RestoreItemActionAppliesToResponse)
	err := grpc.Invoke(ctx, "/generated.RestoreItemAction/AppliesTo", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restoreItemActionClient) Execute(ctx context.Context, in *RestoreItemActionExecuteRequest, opts ...grpc.CallOption) (*RestoreItemActionExecuteResponse, error) {
	out := new(RestoreItemActionExecuteResponse)
	err := grpc.Invoke(ctx, "/generated.RestoreItemAction/Execute", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for RestoreItemAction service

type RestoreItemActionServer interface {
	AppliesTo(context.Context, *RestoreItemActionAppliesToRequest) (*RestoreItemActionAppliesToResponse, error)
	Execute(context.Context, *RestoreItemActionExecuteRequest) (*RestoreItemActionExecuteResponse, error)
}

func RegisterRestoreItemActionServer(s *grpc.Server, srv RestoreItemActionServer) {
	s.RegisterService(&_RestoreItemAction_serviceDesc, srv)
}

func _RestoreItemAction_AppliesTo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RestoreItemActionAppliesToRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RestoreItemActionServer).AppliesTo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/generated.RestoreItemAction/AppliesTo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RestoreItemActionServer).AppliesTo(ctx, req.(*RestoreItemActionAppliesToRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RestoreItemAction_Execute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RestoreItemActionExecuteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RestoreItemActionServer).Execute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/generated.RestoreItemAction/Execute",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RestoreItemActionServer).Execute(ctx, req.(*RestoreItemActionExecuteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RestoreItemAction_serviceDesc = grpc.ServiceDesc{
	ServiceName: "generated.RestoreItemAction",
	HandlerType: (*RestoreItemActionServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AppliesTo",
			Handler:    _RestoreItemAction_AppliesTo_Handler,
		},
		{
			MethodName: "Execute",
			Handler:    _RestoreItemAction_Execute_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "RestoreItemAction.proto",
}

func init() { proto.RegisterFile("RestoreItemAction.proto", fileDescriptor5) }

var fileDescriptor5 = []byte{
	// 332 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x52, 0xdd, 0x4e, 0xc2, 0x30,
	0x14, 0x4e, 0x81, 0x80, 0x1c, 0x88, 0x3f, 0xbd, 0xd0, 0x06, 0x63, 0x9c, 0xbb, 0x30, 0xc4, 0x1f,
	0x2e, 0xf0, 0xd2, 0x2b, 0x4c, 0x94, 0x70, 0x5b, 0x7c, 0x81, 0xb1, 0x1d, 0xa1, 0x61, 0x5b, 0x6b,
	0xdb, 0x25, 0xbe, 0x85, 0xcf, 0xe0, 0xa3, 0xf9, 0x26, 0x86, 0x31, 0x96, 0xc1, 0x74, 0x72, 0xd7,
	0x73, 0xfa, 0x7d, 0xe7, 0xfb, 0xbe, 0xf6, 0xc0, 0x19, 0x47, 0x63, 0xa5, 0xc6, 0x89, 0xc5, 0x68,
	0xe4, 0x5b, 0x21, 0xe3, 0x81, 0xd2, 0xd2, 0x4a, 0xda, 0x9e, 0x63, 0x8c, 0xda, 0xb3, 0x18, 0xf4,
	0xba, 0xd3, 0x85, 0xa7, 0x31, 0x58, 0x5f, 0xb8, 0x9f, 0x04, 0x2e, 0x4b, 0xa4, 0xe7, 0x0f, 0xf4,
	0x13, 0x8b, 0x1c, 0xdf, 0x13, 0x34, 0x96, 0x9e, 0x42, 0x53, 0x85, 0xc9, 0x5c, 0xc4, 0x8c, 0x38,
	0xa4, 0xdf, 0xe6, 0x59, 0x45, 0x29, 0x34, 0x84, 0xc5, 0x88, 0xd5, 0x1c, 0xd2, 0xef, 0xf2, 0xf4,
	0x4c, 0x19, 0xb4, 0xf4, 0x7a, 0x1c, 0xab, 0xa7, 0xed, 0x4d, 0x49, 0xaf, 0xe1, 0x70, 0x85, 0x78,
	0xd1, 0x32, 0x7a, 0xf2, 0xfc, 0x65, 0xa2, 0x58, 0x23, 0x05, 0xec, 0x74, 0xdd, 0x2f, 0x02, 0xce,
	0xdf, 0x8e, 0x8c, 0x92, 0xb1, 0xc1, 0x5c, 0x9a, 0x14, 0xa4, 0xc7, 0x70, 0xe4, 0x05, 0x81, 0x58,
	0xc1, 0xbd, 0x70, 0x45, 0x35, 0xac, 0xe6, 0xd4, 0xfb, 0x9d, 0xe1, 0xc5, 0x20, 0x4f, 0x3f, 0xe0,
	0x68, 0x64, 0xa2, 0x7d, 0x9c, 0x04, 0x18, 0x5b, 0xf1, 0x26, 0x50, 0xf3, 0x5d, 0x16, 0x75, 0xa0,
	0x63, 0x96, 0x42, 0xf1, 0x42, 0x8e, 0x03, 0x5e, 0x6c, 0xb9, 0x8f, 0x70, 0x55, 0xb2, 0x38, 0x52,
	0x2a, 0x14, 0x68, 0x5e, 0xe5, 0x3f, 0xcf, 0xe6, 0x46, 0xe0, 0x56, 0x91, 0xb3, 0x84, 0x63, 0x38,
	0xde, 0x78, 0x9d, 0x62, 0x88, 0xbe, 0x95, 0x3a, 0x9d, 0xd3, 0x19, 0x9e, 0xff, 0x12, 0x67, 0x03,
	0xe1, 0x25, 0xd2, 0xf0, 0x9b, 0xc0, 0x49, 0x49, 0x8f, 0x2e, 0xa0, 0x9d, 0x6b, 0xd2, 0xbb, 0xed,
	0x89, 0xd5, 0xb9, 0x7a, 0xf7, 0x7b, 0xa2, 0xb3, 0x20, 0x33, 0x68, 0x65, 0xbf, 0x47, 0x6f, 0xaa,
	0x98, 0xdb, 0x4b, 0xd7, 0xbb, 0xdd, 0x0b, 0xbb, 0xd6, 0x98, 0x35, 0xd3, 0x65, 0x7e, 0xf8, 0x09,
	0x00, 0x00, 0xff, 0xff, 0x1b, 0x4c, 0xdc, 0xb7, 0x00, 0x03, 0x00, 0x00,
}
