// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/extensions/filters/http/lua/v3/lua.proto

package envoy_extensions_filters_http_lua_v3

import (
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
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

type Lua struct {
	InlineCode           string   `protobuf:"bytes,1,opt,name=inline_code,json=inlineCode,proto3" json:"inline_code,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Lua) Reset()         { *m = Lua{} }
func (m *Lua) String() string { return proto.CompactTextString(m) }
func (*Lua) ProtoMessage()    {}
func (*Lua) Descriptor() ([]byte, []int) {
	return fileDescriptor_9c60a8216ed71fb8, []int{0}
}

func (m *Lua) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Lua.Unmarshal(m, b)
}
func (m *Lua) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Lua.Marshal(b, m, deterministic)
}
func (m *Lua) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Lua.Merge(m, src)
}
func (m *Lua) XXX_Size() int {
	return xxx_messageInfo_Lua.Size(m)
}
func (m *Lua) XXX_DiscardUnknown() {
	xxx_messageInfo_Lua.DiscardUnknown(m)
}

var xxx_messageInfo_Lua proto.InternalMessageInfo

func (m *Lua) GetInlineCode() string {
	if m != nil {
		return m.InlineCode
	}
	return ""
}

func init() {
	proto.RegisterType((*Lua)(nil), "envoy.extensions.filters.http.lua.v3.Lua")
}

func init() {
	proto.RegisterFile("envoy/extensions/filters/http/lua/v3/lua.proto", fileDescriptor_9c60a8216ed71fb8)
}

var fileDescriptor_9c60a8216ed71fb8 = []byte{
	// 253 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xd2, 0x4b, 0xcd, 0x2b, 0xcb,
	0xaf, 0xd4, 0x4f, 0xad, 0x28, 0x49, 0xcd, 0x2b, 0xce, 0xcc, 0xcf, 0x2b, 0xd6, 0x4f, 0xcb, 0xcc,
	0x29, 0x49, 0x2d, 0x2a, 0xd6, 0xcf, 0x28, 0x29, 0x29, 0xd0, 0xcf, 0x29, 0x4d, 0xd4, 0x2f, 0x33,
	0x06, 0x51, 0x7a, 0x05, 0x45, 0xf9, 0x25, 0xf9, 0x42, 0x2a, 0x60, 0xf5, 0x7a, 0x08, 0xf5, 0x7a,
	0x50, 0xf5, 0x7a, 0x20, 0xf5, 0x7a, 0x20, 0x85, 0x65, 0xc6, 0x52, 0xb2, 0xa5, 0x29, 0x05, 0x89,
	0xfa, 0x89, 0x79, 0x79, 0xf9, 0x25, 0x89, 0x25, 0x60, 0x53, 0x8b, 0x4b, 0x12, 0x4b, 0x4a, 0x8b,
	0x21, 0x86, 0x48, 0x29, 0x62, 0x48, 0x97, 0xa5, 0x16, 0x81, 0x4c, 0xcb, 0xcc, 0x4b, 0x87, 0x2a,
	0x11, 0x2f, 0x4b, 0xcc, 0xc9, 0x4c, 0x49, 0x2c, 0x49, 0xd5, 0x87, 0x31, 0x20, 0x12, 0x4a, 0xd1,
	0x5c, 0xcc, 0x3e, 0xa5, 0x89, 0x42, 0x1a, 0x5c, 0xdc, 0x99, 0x79, 0x39, 0x99, 0x79, 0xa9, 0xf1,
	0xc9, 0xf9, 0x29, 0xa9, 0x12, 0x8c, 0x0a, 0x8c, 0x1a, 0x9c, 0x4e, 0xec, 0xbf, 0x9c, 0x58, 0x8a,
	0x98, 0x14, 0x18, 0x83, 0xb8, 0x20, 0x72, 0xce, 0xf9, 0x29, 0xa9, 0x56, 0x5a, 0xb3, 0x8e, 0x76,
	0xc8, 0xa9, 0x72, 0x29, 0x43, 0x1c, 0x9e, 0x9c, 0x9f, 0x97, 0x96, 0x99, 0x0e, 0x75, 0x34, 0x92,
	0x9b, 0x8d, 0xf4, 0x7c, 0x4a, 0x13, 0x9d, 0x3c, 0x76, 0x35, 0x9c, 0xb8, 0xc8, 0xc6, 0x24, 0xc0,
	0xc4, 0x65, 0x94, 0x99, 0x0f, 0x09, 0x9a, 0x82, 0xa2, 0xfc, 0x8a, 0x4a, 0x3d, 0x62, 0x7c, 0xed,
	0xc4, 0xe1, 0x53, 0x9a, 0x18, 0x00, 0x72, 0x64, 0x00, 0x63, 0x12, 0x1b, 0xd8, 0xb5, 0xc6, 0x80,
	0x00, 0x00, 0x00, 0xff, 0xff, 0x7b, 0x53, 0x01, 0x76, 0x60, 0x01, 0x00, 0x00,
}
