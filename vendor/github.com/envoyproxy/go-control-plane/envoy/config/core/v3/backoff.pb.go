// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/config/core/v3/backoff.proto

package envoy_config_core_v3

import (
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/golang/protobuf/proto"
	duration "github.com/golang/protobuf/ptypes/duration"
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

type BackoffStrategy struct {
	BaseInterval         *duration.Duration `protobuf:"bytes,1,opt,name=base_interval,json=baseInterval,proto3" json:"base_interval,omitempty"`
	MaxInterval          *duration.Duration `protobuf:"bytes,2,opt,name=max_interval,json=maxInterval,proto3" json:"max_interval,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *BackoffStrategy) Reset()         { *m = BackoffStrategy{} }
func (m *BackoffStrategy) String() string { return proto.CompactTextString(m) }
func (*BackoffStrategy) ProtoMessage()    {}
func (*BackoffStrategy) Descriptor() ([]byte, []int) {
	return fileDescriptor_5030f1467e197113, []int{0}
}

func (m *BackoffStrategy) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BackoffStrategy.Unmarshal(m, b)
}
func (m *BackoffStrategy) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BackoffStrategy.Marshal(b, m, deterministic)
}
func (m *BackoffStrategy) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BackoffStrategy.Merge(m, src)
}
func (m *BackoffStrategy) XXX_Size() int {
	return xxx_messageInfo_BackoffStrategy.Size(m)
}
func (m *BackoffStrategy) XXX_DiscardUnknown() {
	xxx_messageInfo_BackoffStrategy.DiscardUnknown(m)
}

var xxx_messageInfo_BackoffStrategy proto.InternalMessageInfo

func (m *BackoffStrategy) GetBaseInterval() *duration.Duration {
	if m != nil {
		return m.BaseInterval
	}
	return nil
}

func (m *BackoffStrategy) GetMaxInterval() *duration.Duration {
	if m != nil {
		return m.MaxInterval
	}
	return nil
}

func init() {
	proto.RegisterType((*BackoffStrategy)(nil), "envoy.config.core.v3.BackoffStrategy")
}

func init() { proto.RegisterFile("envoy/config/core/v3/backoff.proto", fileDescriptor_5030f1467e197113) }

var fileDescriptor_5030f1467e197113 = []byte{
	// 317 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x90, 0xb1, 0x4a, 0x03, 0x31,
	0x18, 0xc7, 0xcd, 0xa1, 0xa5, 0xa4, 0x55, 0x4b, 0x11, 0xd4, 0x82, 0xc5, 0xd6, 0xa5, 0x38, 0x24,
	0xd0, 0x6e, 0xa2, 0x4b, 0x10, 0xc1, 0x45, 0x4a, 0x7d, 0x00, 0xf9, 0xae, 0xcd, 0x1d, 0xc1, 0x36,
	0xdf, 0x91, 0xcb, 0x85, 0x76, 0x73, 0x70, 0xf0, 0x19, 0x7c, 0x84, 0x3e, 0x82, 0x93, 0x8b, 0xe0,
	0x2a, 0xbe, 0x4d, 0x27, 0xe9, 0xe5, 0x0e, 0x41, 0x05, 0xb7, 0xe3, 0xbe, 0xdf, 0xff, 0xcf, 0x2f,
	0x7f, 0xda, 0x95, 0xda, 0xe1, 0x82, 0x8f, 0x51, 0x47, 0x2a, 0xe6, 0x63, 0x34, 0x92, 0xbb, 0x01,
	0x0f, 0x61, 0x7c, 0x8f, 0x51, 0xc4, 0x12, 0x83, 0x16, 0x9b, 0x7b, 0x39, 0xc3, 0x3c, 0xc3, 0xd6,
	0x0c, 0x73, 0x83, 0x56, 0x3b, 0x46, 0x8c, 0xa7, 0x92, 0xe7, 0x4c, 0x98, 0x45, 0x7c, 0x92, 0x19,
	0xb0, 0x0a, 0xb5, 0x4f, 0xb5, 0x8e, 0xb2, 0x49, 0x02, 0x1c, 0xb4, 0x46, 0x9b, 0xff, 0x4e, 0x79,
	0x6a, 0xc1, 0x66, 0x69, 0x71, 0xee, 0xfc, 0x3a, 0x3b, 0x69, 0x52, 0x85, 0x5a, 0xe9, 0xb8, 0x40,
	0xf6, 0x1d, 0x4c, 0xd5, 0x04, 0xac, 0xe4, 0xe5, 0x87, 0x3f, 0x74, 0x3f, 0x09, 0xdd, 0x15, 0x5e,
	0xf1, 0xd6, 0x1a, 0xb0, 0x32, 0x5e, 0x34, 0x6f, 0xe8, 0x76, 0x08, 0xa9, 0xbc, 0x53, 0xda, 0x4a,
	0xe3, 0x60, 0x7a, 0x40, 0x8e, 0x49, 0xaf, 0xd6, 0x3f, 0x64, 0x5e, 0x93, 0x95, 0x9a, 0xec, 0xb2,
	0xd0, 0x14, 0x3b, 0x2b, 0x51, 0x5b, 0x92, 0x6a, 0x95, 0xf4, 0x37, 0x1b, 0xaf, 0x8f, 0x17, 0xa3,
	0xfa, 0x3a, 0x7f, 0x5d, 0xc4, 0x9b, 0x57, 0xb4, 0x3e, 0x83, 0xf9, 0x77, 0x5d, 0xf0, 0x5f, 0x5d,
	0x75, 0x25, 0xb6, 0x96, 0x24, 0x38, 0xdd, 0x18, 0xd5, 0x66, 0x30, 0x2f, 0x7b, 0xce, 0x7a, 0xcf,
	0x6f, 0x4f, 0xed, 0x13, 0xda, 0xf1, 0x1b, 0x42, 0xa2, 0x98, 0xeb, 0xfb, 0x0d, 0x7f, 0xbc, 0x40,
	0x9c, 0xbf, 0x3c, 0xbc, 0x7f, 0x54, 0x82, 0x46, 0x40, 0xbb, 0x0a, 0x59, 0xce, 0x27, 0x06, 0xe7,
	0x0b, 0xf6, 0xd7, 0xfc, 0xa2, 0x5e, 0xc4, 0x87, 0x6b, 0x91, 0x21, 0x09, 0x2b, 0xb9, 0xd1, 0xe0,
	0x2b, 0x00, 0x00, 0xff, 0xff, 0x8f, 0x04, 0x4e, 0x10, 0xd1, 0x01, 0x00, 0x00,
}
