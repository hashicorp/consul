// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/extensions/filters/network/ext_authz/v3/ext_authz.proto

package envoy_extensions_filters_network_ext_authz_v3

import (
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type ExtAuthz struct {
	StatPrefix             string          `protobuf:"bytes,1,opt,name=stat_prefix,json=statPrefix,proto3" json:"stat_prefix,omitempty"`
	GrpcService            *v3.GrpcService `protobuf:"bytes,2,opt,name=grpc_service,json=grpcService,proto3" json:"grpc_service,omitempty"`
	FailureModeAllow       bool            `protobuf:"varint,3,opt,name=failure_mode_allow,json=failureModeAllow,proto3" json:"failure_mode_allow,omitempty"`
	IncludePeerCertificate bool            `protobuf:"varint,4,opt,name=include_peer_certificate,json=includePeerCertificate,proto3" json:"include_peer_certificate,omitempty"`
	XXX_NoUnkeyedLiteral   struct{}        `json:"-"`
	XXX_unrecognized       []byte          `json:"-"`
	XXX_sizecache          int32           `json:"-"`
}

func (m *ExtAuthz) Reset()         { *m = ExtAuthz{} }
func (m *ExtAuthz) String() string { return proto.CompactTextString(m) }
func (*ExtAuthz) ProtoMessage()    {}
func (*ExtAuthz) Descriptor() ([]byte, []int) {
	return fileDescriptor_c579db07d85696d8, []int{0}
}

func (m *ExtAuthz) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ExtAuthz.Unmarshal(m, b)
}
func (m *ExtAuthz) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ExtAuthz.Marshal(b, m, deterministic)
}
func (m *ExtAuthz) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ExtAuthz.Merge(m, src)
}
func (m *ExtAuthz) XXX_Size() int {
	return xxx_messageInfo_ExtAuthz.Size(m)
}
func (m *ExtAuthz) XXX_DiscardUnknown() {
	xxx_messageInfo_ExtAuthz.DiscardUnknown(m)
}

var xxx_messageInfo_ExtAuthz proto.InternalMessageInfo

func (m *ExtAuthz) GetStatPrefix() string {
	if m != nil {
		return m.StatPrefix
	}
	return ""
}

func (m *ExtAuthz) GetGrpcService() *v3.GrpcService {
	if m != nil {
		return m.GrpcService
	}
	return nil
}

func (m *ExtAuthz) GetFailureModeAllow() bool {
	if m != nil {
		return m.FailureModeAllow
	}
	return false
}

func (m *ExtAuthz) GetIncludePeerCertificate() bool {
	if m != nil {
		return m.IncludePeerCertificate
	}
	return false
}

func init() {
	proto.RegisterType((*ExtAuthz)(nil), "envoy.extensions.filters.network.ext_authz.v3.ExtAuthz")
}

func init() {
	proto.RegisterFile("envoy/extensions/filters/network/ext_authz/v3/ext_authz.proto", fileDescriptor_c579db07d85696d8)
}

var fileDescriptor_c579db07d85696d8 = []byte{
	// 388 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x91, 0x31, 0x6b, 0xdb, 0x40,
	0x1c, 0xc5, 0x91, 0x6a, 0x5c, 0xf7, 0xdc, 0x82, 0xd1, 0xd0, 0x0a, 0x43, 0x8b, 0xdc, 0xa5, 0x1a,
	0xda, 0x13, 0xb5, 0x16, 0xd3, 0xd2, 0xc1, 0x6a, 0x4b, 0xa7, 0x82, 0x70, 0xa1, 0xab, 0xb8, 0x9c,
	0xfe, 0x52, 0x8e, 0x28, 0x77, 0xe2, 0x74, 0x92, 0xe5, 0x4c, 0x19, 0xf3, 0x19, 0x42, 0x3e, 0x49,
	0xf6, 0x40, 0xd6, 0x7c, 0x9d, 0x4c, 0xe1, 0x74, 0x32, 0x4a, 0x48, 0x96, 0x6c, 0x3a, 0xfd, 0xee,
	0xbd, 0x7b, 0xff, 0xf7, 0x47, 0x3f, 0x80, 0x37, 0x62, 0x17, 0x40, 0xab, 0x80, 0x57, 0x4c, 0xf0,
	0x2a, 0xc8, 0x58, 0xa1, 0x40, 0x56, 0x01, 0x07, 0xb5, 0x15, 0xf2, 0x48, 0xa3, 0x84, 0xd4, 0xea,
	0xf0, 0x24, 0x68, 0xc2, 0xe1, 0x80, 0x4b, 0x29, 0x94, 0x70, 0xbe, 0x74, 0x72, 0x3c, 0xc8, 0x71,
	0x2f, 0xc7, 0xbd, 0x1c, 0x0f, 0x8a, 0x26, 0x9c, 0x7f, 0x32, 0xaf, 0x51, 0xc1, 0x33, 0x96, 0x07,
	0x54, 0x48, 0xd0, 0xa6, 0xb9, 0x2c, 0x69, 0x52, 0x81, 0x6c, 0x18, 0x05, 0xe3, 0x3b, 0x7f, 0x5f,
	0xa7, 0x25, 0x09, 0x08, 0xe7, 0x42, 0x11, 0xd5, 0xc5, 0xaa, 0x14, 0x51, 0x75, 0xd5, 0xe3, 0xc5,
	0x23, 0xdc, 0x80, 0xd4, 0xef, 0x33, 0x9e, 0xf7, 0x57, 0xde, 0x35, 0xa4, 0x60, 0x29, 0x51, 0x10,
	0xec, 0x3f, 0x0c, 0xf8, 0x78, 0x61, 0xa3, 0xc9, 0xef, 0x56, 0xad, 0x75, 0x26, 0xc7, 0x47, 0x53,
	0x6d, 0x9c, 0x94, 0x12, 0x32, 0xd6, 0xba, 0x96, 0x67, 0xf9, 0xaf, 0xa2, 0x97, 0xb7, 0xd1, 0x48,
	0xda, 0x9e, 0xb5, 0x41, 0x9a, 0xc5, 0x1d, 0x72, 0x7e, 0xa1, 0xd7, 0xf7, 0x73, 0xba, 0xb6, 0x67,
	0xf9, 0xd3, 0xe5, 0x02, 0x9b, 0x02, 0xcc, 0x44, 0x58, 0x4f, 0x84, 0x9b, 0x10, 0xff, 0x91, 0x25,
	0xfd, 0x67, 0x2e, 0x6e, 0xa6, 0xf9, 0x70, 0x70, 0x3e, 0x23, 0x27, 0x23, 0xac, 0xa8, 0x25, 0x24,
	0xc7, 0x22, 0x85, 0x84, 0x14, 0x85, 0xd8, 0xba, 0x2f, 0x3c, 0xcb, 0x9f, 0x6c, 0x66, 0x3d, 0xf9,
	0x2b, 0x52, 0x58, 0xeb, 0xff, 0xce, 0x0a, 0xb9, 0x8c, 0xd3, 0xa2, 0x4e, 0x21, 0x29, 0x01, 0x64,
	0x42, 0x41, 0x2a, 0x96, 0x31, 0x4a, 0x14, 0xb8, 0xa3, 0x4e, 0xf3, 0xb6, 0xe7, 0x31, 0x80, 0xfc,
	0x39, 0xd0, 0x6f, 0xab, 0xf3, 0xab, 0xb3, 0x0f, 0x21, 0xfa, 0xfa, 0x20, 0x9d, 0x59, 0xcd, 0x53,
	0x9b, 0x59, 0xe2, 0x7d, 0x23, 0xd1, 0xff, 0xcb, 0xd3, 0xeb, 0x9b, 0xb1, 0x3d, 0xb3, 0xd1, 0x77,
	0x26, 0xcc, 0x74, 0xa5, 0x14, 0xed, 0x0e, 0x3f, 0x6b, 0xd3, 0xd1, 0x9b, 0xbd, 0x61, 0xac, 0x4b,
	0x8f, 0xad, 0x83, 0x71, 0xd7, 0x7e, 0x78, 0x17, 0x00, 0x00, 0xff, 0xff, 0x74, 0xfb, 0x28, 0x3d,
	0x71, 0x02, 0x00, 0x00,
}
