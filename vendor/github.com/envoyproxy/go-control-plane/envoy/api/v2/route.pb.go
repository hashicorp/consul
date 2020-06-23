// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/api/v2/route.proto

package envoy_api_v2

import (
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/golang/protobuf/proto"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
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

type RouteConfiguration struct {
	Name                            string                    `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	VirtualHosts                    []*route.VirtualHost      `protobuf:"bytes,2,rep,name=virtual_hosts,json=virtualHosts,proto3" json:"virtual_hosts,omitempty"`
	Vhds                            *Vhds                     `protobuf:"bytes,9,opt,name=vhds,proto3" json:"vhds,omitempty"`
	InternalOnlyHeaders             []string                  `protobuf:"bytes,3,rep,name=internal_only_headers,json=internalOnlyHeaders,proto3" json:"internal_only_headers,omitempty"`
	ResponseHeadersToAdd            []*core.HeaderValueOption `protobuf:"bytes,4,rep,name=response_headers_to_add,json=responseHeadersToAdd,proto3" json:"response_headers_to_add,omitempty"`
	ResponseHeadersToRemove         []string                  `protobuf:"bytes,5,rep,name=response_headers_to_remove,json=responseHeadersToRemove,proto3" json:"response_headers_to_remove,omitempty"`
	RequestHeadersToAdd             []*core.HeaderValueOption `protobuf:"bytes,6,rep,name=request_headers_to_add,json=requestHeadersToAdd,proto3" json:"request_headers_to_add,omitempty"`
	RequestHeadersToRemove          []string                  `protobuf:"bytes,8,rep,name=request_headers_to_remove,json=requestHeadersToRemove,proto3" json:"request_headers_to_remove,omitempty"`
	MostSpecificHeaderMutationsWins bool                      `protobuf:"varint,10,opt,name=most_specific_header_mutations_wins,json=mostSpecificHeaderMutationsWins,proto3" json:"most_specific_header_mutations_wins,omitempty"`
	ValidateClusters                *wrappers.BoolValue       `protobuf:"bytes,7,opt,name=validate_clusters,json=validateClusters,proto3" json:"validate_clusters,omitempty"`
	XXX_NoUnkeyedLiteral            struct{}                  `json:"-"`
	XXX_unrecognized                []byte                    `json:"-"`
	XXX_sizecache                   int32                     `json:"-"`
}

func (m *RouteConfiguration) Reset()         { *m = RouteConfiguration{} }
func (m *RouteConfiguration) String() string { return proto.CompactTextString(m) }
func (*RouteConfiguration) ProtoMessage()    {}
func (*RouteConfiguration) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f33b4742f398551, []int{0}
}

func (m *RouteConfiguration) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RouteConfiguration.Unmarshal(m, b)
}
func (m *RouteConfiguration) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RouteConfiguration.Marshal(b, m, deterministic)
}
func (m *RouteConfiguration) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RouteConfiguration.Merge(m, src)
}
func (m *RouteConfiguration) XXX_Size() int {
	return xxx_messageInfo_RouteConfiguration.Size(m)
}
func (m *RouteConfiguration) XXX_DiscardUnknown() {
	xxx_messageInfo_RouteConfiguration.DiscardUnknown(m)
}

var xxx_messageInfo_RouteConfiguration proto.InternalMessageInfo

func (m *RouteConfiguration) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *RouteConfiguration) GetVirtualHosts() []*route.VirtualHost {
	if m != nil {
		return m.VirtualHosts
	}
	return nil
}

func (m *RouteConfiguration) GetVhds() *Vhds {
	if m != nil {
		return m.Vhds
	}
	return nil
}

func (m *RouteConfiguration) GetInternalOnlyHeaders() []string {
	if m != nil {
		return m.InternalOnlyHeaders
	}
	return nil
}

func (m *RouteConfiguration) GetResponseHeadersToAdd() []*core.HeaderValueOption {
	if m != nil {
		return m.ResponseHeadersToAdd
	}
	return nil
}

func (m *RouteConfiguration) GetResponseHeadersToRemove() []string {
	if m != nil {
		return m.ResponseHeadersToRemove
	}
	return nil
}

func (m *RouteConfiguration) GetRequestHeadersToAdd() []*core.HeaderValueOption {
	if m != nil {
		return m.RequestHeadersToAdd
	}
	return nil
}

func (m *RouteConfiguration) GetRequestHeadersToRemove() []string {
	if m != nil {
		return m.RequestHeadersToRemove
	}
	return nil
}

func (m *RouteConfiguration) GetMostSpecificHeaderMutationsWins() bool {
	if m != nil {
		return m.MostSpecificHeaderMutationsWins
	}
	return false
}

func (m *RouteConfiguration) GetValidateClusters() *wrappers.BoolValue {
	if m != nil {
		return m.ValidateClusters
	}
	return nil
}

type Vhds struct {
	ConfigSource         *core.ConfigSource `protobuf:"bytes,1,opt,name=config_source,json=configSource,proto3" json:"config_source,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *Vhds) Reset()         { *m = Vhds{} }
func (m *Vhds) String() string { return proto.CompactTextString(m) }
func (*Vhds) ProtoMessage()    {}
func (*Vhds) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f33b4742f398551, []int{1}
}

func (m *Vhds) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Vhds.Unmarshal(m, b)
}
func (m *Vhds) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Vhds.Marshal(b, m, deterministic)
}
func (m *Vhds) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Vhds.Merge(m, src)
}
func (m *Vhds) XXX_Size() int {
	return xxx_messageInfo_Vhds.Size(m)
}
func (m *Vhds) XXX_DiscardUnknown() {
	xxx_messageInfo_Vhds.DiscardUnknown(m)
}

var xxx_messageInfo_Vhds proto.InternalMessageInfo

func (m *Vhds) GetConfigSource() *core.ConfigSource {
	if m != nil {
		return m.ConfigSource
	}
	return nil
}

func init() {
	proto.RegisterType((*RouteConfiguration)(nil), "envoy.api.v2.RouteConfiguration")
	proto.RegisterType((*Vhds)(nil), "envoy.api.v2.Vhds")
}

func init() { proto.RegisterFile("envoy/api/v2/route.proto", fileDescriptor_1f33b4742f398551) }

var fileDescriptor_1f33b4742f398551 = []byte{
	// 610 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x53, 0xbf, 0x6f, 0xd3, 0x40,
	0x14, 0xe6, 0x9a, 0x1f, 0x4d, 0xae, 0xad, 0x14, 0xae, 0xb4, 0x31, 0x11, 0xd0, 0xa8, 0xfc, 0x50,
	0x58, 0x6c, 0x29, 0xfd, 0x0b, 0x70, 0x2b, 0x51, 0x09, 0x4a, 0x2b, 0x17, 0x85, 0xd1, 0xba, 0xda,
	0x97, 0xe4, 0x24, 0xe7, 0x9e, 0xb9, 0x3b, 0xbb, 0x64, 0x63, 0x66, 0x41, 0xea, 0xc4, 0xdf, 0xc2,
	0xc4, 0xd8, 0x95, 0x3f, 0x80, 0x9d, 0x99, 0x31, 0x03, 0x42, 0x3e, 0xdb, 0x50, 0x37, 0xed, 0xc2,
	0x12, 0xf9, 0xf2, 0xbe, 0xef, 0x7b, 0xdf, 0xbd, 0xfb, 0x1e, 0xb6, 0x98, 0x48, 0x61, 0xee, 0xd0,
	0x98, 0x3b, 0xe9, 0xd0, 0x91, 0x90, 0x68, 0x66, 0xc7, 0x12, 0x34, 0x90, 0x75, 0x53, 0xb1, 0x69,
	0xcc, 0xed, 0x74, 0xd8, 0x7b, 0x50, 0xc1, 0x05, 0x20, 0x99, 0x73, 0x46, 0x55, 0x81, 0xed, 0x3d,
	0x5d, 0xae, 0x06, 0x20, 0xc6, 0x7c, 0xe2, 0x2b, 0x48, 0x64, 0x50, 0xc2, 0x9e, 0x2f, 0x37, 0xcb,
	0x7f, 0xfd, 0x00, 0x66, 0x31, 0x08, 0x26, 0xb4, 0x2a, 0xa0, 0x8f, 0x26, 0x00, 0x93, 0x88, 0x39,
	0xe6, 0x74, 0x96, 0x8c, 0x9d, 0x73, 0x49, 0xe3, 0x98, 0xc9, 0xbf, 0xf5, 0x24, 0x8c, 0xa9, 0x43,
	0x85, 0x00, 0x4d, 0x35, 0x07, 0xa1, 0x9c, 0x19, 0x9f, 0x48, 0x5a, 0xba, 0xef, 0x3d, 0x5c, 0xaa,
	0x2b, 0x4d, 0x75, 0x52, 0xd2, 0xbb, 0x29, 0x8d, 0x78, 0x48, 0x35, 0x73, 0xca, 0x8f, 0xbc, 0xb0,
	0xfb, 0xa3, 0x81, 0x89, 0x97, 0x59, 0xda, 0x37, 0xfe, 0x13, 0x69, 0xd8, 0x84, 0xe0, 0xba, 0xa0,
	0x33, 0x66, 0xa1, 0x3e, 0x1a, 0xb4, 0x3d, 0xf3, 0x4d, 0x0e, 0xf0, 0x46, 0xca, 0xa5, 0x4e, 0x68,
	0xe4, 0x4f, 0x41, 0x69, 0x65, 0xad, 0xf4, 0x6b, 0x83, 0xb5, 0xe1, 0x8e, 0x7d, 0x75, 0x70, 0x76,
	0x3e, 0xd2, 0x51, 0x0e, 0x3c, 0x04, 0xa5, 0xbd, 0xf5, 0xf4, 0xdf, 0x41, 0x91, 0x67, 0xb8, 0x9e,
	0x4e, 0x43, 0x65, 0xb5, 0xfb, 0x68, 0xb0, 0x36, 0x24, 0x55, 0xf2, 0x68, 0x1a, 0x2a, 0xcf, 0xd4,
	0xc9, 0x01, 0xde, 0xe2, 0x42, 0x33, 0x29, 0x68, 0xe4, 0x83, 0x88, 0xe6, 0xfe, 0x94, 0xd1, 0x90,
	0x49, 0x65, 0xd5, 0xfa, 0xb5, 0x41, 0xdb, 0xed, 0x2c, 0xdc, 0x8d, 0x0b, 0x84, 0x77, 0x5b, 0xb2,
	0xf9, 0x0d, 0xa1, 0x4b, 0x74, 0xc7, 0xdb, 0x2c, 0xe1, 0xc7, 0x22, 0x9a, 0x1f, 0xe6, 0x60, 0x32,
	0xc6, 0x5d, 0xc9, 0x54, 0x0c, 0x42, 0xb1, 0x52, 0xc0, 0xd7, 0xe0, 0xd3, 0x30, 0xb4, 0xea, 0xc6,
	0xfd, 0x93, 0xaa, 0x81, 0xec, 0x29, 0xed, 0x9c, 0x3c, 0xa2, 0x51, 0xc2, 0x8e, 0xe3, 0x6c, 0x1c,
	0x6e, 0x7b, 0xe1, 0x36, 0x2f, 0x50, 0xad, 0xf3, 0x73, 0xd5, 0xbb, 0x57, 0xea, 0x15, 0x2d, 0xde,
	0xc2, 0x8b, 0x30, 0x24, 0x47, 0xb8, 0x77, 0x53, 0x1f, 0xc9, 0x66, 0x90, 0x32, 0xab, 0x71, 0x8b,
	0xe5, 0xee, 0x92, 0x96, 0x67, 0x08, 0x24, 0xc4, 0xdb, 0x92, 0xbd, 0x4f, 0x98, 0xd2, 0xd7, 0x5d,
	0x37, 0xff, 0xcf, 0xf5, 0x66, 0x21, 0x57, 0x31, 0xfd, 0x0a, 0xdf, 0xbf, 0xa1, 0x4b, 0xe1, 0xb9,
	0x75, 0x8b, 0xe7, 0xed, 0xeb, 0x4a, 0x85, 0xe5, 0xd7, 0xf8, 0xf1, 0x0c, 0x94, 0xf6, 0x55, 0xcc,
	0x02, 0x3e, 0xe6, 0x41, 0x21, 0xe9, 0xcf, 0x92, 0x22, 0x90, 0xfe, 0x39, 0x17, 0xca, 0xc2, 0x7d,
	0x34, 0x68, 0x79, 0x3b, 0x19, 0xf4, 0xb4, 0x40, 0xe6, 0x4a, 0x47, 0x25, 0xee, 0x1d, 0x17, 0x8a,
	0xbc, 0xc4, 0x77, 0xcb, 0xa0, 0xfa, 0x41, 0x94, 0x28, 0x9d, 0xbd, 0xfc, 0xaa, 0x89, 0x4c, 0xcf,
	0xce, 0x57, 0xc5, 0x2e, 0x57, 0xc5, 0x76, 0x01, 0x22, 0x73, 0x6f, 0xaf, 0x53, 0x92, 0xf6, 0x0b,
	0xce, 0xee, 0x08, 0xd7, 0xb3, 0x50, 0x91, 0x37, 0x78, 0xa3, 0xb2, 0xa1, 0x26, 0xd9, 0x4b, 0xe1,
	0x35, 0x83, 0xcc, 0x37, 0xe1, 0xd4, 0xc0, 0xdc, 0xd6, 0xc2, 0x6d, 0x7c, 0x42, 0x2b, 0x1d, 0xe4,
	0xad, 0x07, 0x57, 0xff, 0x3f, 0xfe, 0xf5, 0xe5, 0xf7, 0xe7, 0x46, 0x97, 0x6c, 0xe5, 0xfc, 0xbc,
	0x56, 0x84, 0x3f, 0xdd, 0xfb, 0xfa, 0xf1, 0xf2, 0x7b, 0x73, 0xa5, 0x83, 0x70, 0x8f, 0x43, 0xde,
	0x21, 0x96, 0xf0, 0x61, 0x5e, 0x69, 0xe6, 0x62, 0xb3, 0x77, 0x27, 0xd9, 0x2d, 0x4e, 0xd0, 0x59,
	0xd3, 0x5c, 0x67, 0xef, 0x4f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x12, 0x5f, 0x3d, 0xf9, 0xa1, 0x04,
	0x00, 0x00,
}
