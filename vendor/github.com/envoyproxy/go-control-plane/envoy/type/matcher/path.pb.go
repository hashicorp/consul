// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/type/matcher/path.proto

package envoy_type_matcher

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

type PathMatcher struct {
	// Types that are valid to be assigned to Rule:
	//	*PathMatcher_Path
	Rule                 isPathMatcher_Rule `protobuf_oneof:"rule"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *PathMatcher) Reset()         { *m = PathMatcher{} }
func (m *PathMatcher) String() string { return proto.CompactTextString(m) }
func (*PathMatcher) ProtoMessage()    {}
func (*PathMatcher) Descriptor() ([]byte, []int) {
	return fileDescriptor_bec7ed88adc90b4e, []int{0}
}

func (m *PathMatcher) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PathMatcher.Unmarshal(m, b)
}
func (m *PathMatcher) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PathMatcher.Marshal(b, m, deterministic)
}
func (m *PathMatcher) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PathMatcher.Merge(m, src)
}
func (m *PathMatcher) XXX_Size() int {
	return xxx_messageInfo_PathMatcher.Size(m)
}
func (m *PathMatcher) XXX_DiscardUnknown() {
	xxx_messageInfo_PathMatcher.DiscardUnknown(m)
}

var xxx_messageInfo_PathMatcher proto.InternalMessageInfo

type isPathMatcher_Rule interface {
	isPathMatcher_Rule()
}

type PathMatcher_Path struct {
	Path *StringMatcher `protobuf:"bytes,1,opt,name=path,proto3,oneof"`
}

func (*PathMatcher_Path) isPathMatcher_Rule() {}

func (m *PathMatcher) GetRule() isPathMatcher_Rule {
	if m != nil {
		return m.Rule
	}
	return nil
}

func (m *PathMatcher) GetPath() *StringMatcher {
	if x, ok := m.GetRule().(*PathMatcher_Path); ok {
		return x.Path
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*PathMatcher) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*PathMatcher_Path)(nil),
	}
}

func init() {
	proto.RegisterType((*PathMatcher)(nil), "envoy.type.matcher.PathMatcher")
}

func init() { proto.RegisterFile("envoy/type/matcher/path.proto", fileDescriptor_bec7ed88adc90b4e) }

var fileDescriptor_bec7ed88adc90b4e = []byte{
	// 219 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x4d, 0xcd, 0x2b, 0xcb,
	0xaf, 0xd4, 0x2f, 0xa9, 0x2c, 0x48, 0xd5, 0xcf, 0x4d, 0x2c, 0x49, 0xce, 0x48, 0x2d, 0xd2, 0x2f,
	0x48, 0x2c, 0xc9, 0xd0, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x12, 0x02, 0x4b, 0xeb, 0x81, 0xa4,
	0xf5, 0xa0, 0xd2, 0x52, 0xf2, 0x58, 0xb4, 0x14, 0x97, 0x14, 0x65, 0xe6, 0xa5, 0x43, 0x34, 0x49,
	0xc9, 0x96, 0xa6, 0x14, 0x24, 0xea, 0x27, 0xe6, 0xe5, 0xe5, 0x97, 0x24, 0x96, 0x64, 0xe6, 0xe7,
	0x15, 0xeb, 0x17, 0x97, 0x24, 0x96, 0x94, 0x16, 0x43, 0xa5, 0xc5, 0xcb, 0x12, 0x73, 0x32, 0x53,
	0x12, 0x4b, 0x52, 0xf5, 0x61, 0x0c, 0x88, 0x84, 0x52, 0x2c, 0x17, 0x77, 0x40, 0x62, 0x49, 0x86,
	0x2f, 0xc4, 0x4c, 0x21, 0x47, 0x2e, 0x16, 0x90, 0x4b, 0x24, 0x18, 0x15, 0x18, 0x35, 0xb8, 0x8d,
	0x14, 0xf5, 0x30, 0x9d, 0xa2, 0x17, 0x0c, 0xb6, 0x16, 0xaa, 0xc1, 0x89, 0xe3, 0x97, 0x13, 0x6b,
	0x17, 0x23, 0x93, 0x00, 0xa3, 0x07, 0x43, 0x10, 0x58, 0xab, 0x13, 0x37, 0x17, 0x4b, 0x51, 0x69,
	0x4e, 0xaa, 0x10, 0xf3, 0x0f, 0x27, 0x46, 0x27, 0xf3, 0x5d, 0x0d, 0x27, 0x2e, 0xb2, 0x31, 0x09,
	0x30, 0x72, 0x29, 0x64, 0xe6, 0x43, 0x4c, 0x2b, 0x28, 0xca, 0xaf, 0xa8, 0xc4, 0x62, 0xb0, 0x13,
	0x27, 0xc8, 0x21, 0x01, 0x20, 0x57, 0x05, 0x30, 0x26, 0xb1, 0x81, 0x9d, 0x67, 0x0c, 0x08, 0x00,
	0x00, 0xff, 0xff, 0xd2, 0xe6, 0x6d, 0x00, 0x2c, 0x01, 0x00, 0x00,
}
