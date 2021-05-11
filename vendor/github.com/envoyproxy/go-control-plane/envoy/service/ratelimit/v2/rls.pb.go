// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/service/ratelimit/v2/rls.proto

package envoy_service_ratelimit_v2

import (
	context "context"
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	ratelimit "github.com/envoyproxy/go-control-plane/envoy/api/v2/ratelimit"
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type RateLimitResponse_Code int32

const (
	RateLimitResponse_UNKNOWN    RateLimitResponse_Code = 0
	RateLimitResponse_OK         RateLimitResponse_Code = 1
	RateLimitResponse_OVER_LIMIT RateLimitResponse_Code = 2
)

var RateLimitResponse_Code_name = map[int32]string{
	0: "UNKNOWN",
	1: "OK",
	2: "OVER_LIMIT",
}

var RateLimitResponse_Code_value = map[string]int32{
	"UNKNOWN":    0,
	"OK":         1,
	"OVER_LIMIT": 2,
}

func (x RateLimitResponse_Code) String() string {
	return proto.EnumName(RateLimitResponse_Code_name, int32(x))
}

func (RateLimitResponse_Code) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_1de95711edb19ee8, []int{1, 0}
}

type RateLimitResponse_RateLimit_Unit int32

const (
	RateLimitResponse_RateLimit_UNKNOWN RateLimitResponse_RateLimit_Unit = 0
	RateLimitResponse_RateLimit_SECOND  RateLimitResponse_RateLimit_Unit = 1
	RateLimitResponse_RateLimit_MINUTE  RateLimitResponse_RateLimit_Unit = 2
	RateLimitResponse_RateLimit_HOUR    RateLimitResponse_RateLimit_Unit = 3
	RateLimitResponse_RateLimit_DAY     RateLimitResponse_RateLimit_Unit = 4
)

var RateLimitResponse_RateLimit_Unit_name = map[int32]string{
	0: "UNKNOWN",
	1: "SECOND",
	2: "MINUTE",
	3: "HOUR",
	4: "DAY",
}

var RateLimitResponse_RateLimit_Unit_value = map[string]int32{
	"UNKNOWN": 0,
	"SECOND":  1,
	"MINUTE":  2,
	"HOUR":    3,
	"DAY":     4,
}

func (x RateLimitResponse_RateLimit_Unit) String() string {
	return proto.EnumName(RateLimitResponse_RateLimit_Unit_name, int32(x))
}

func (RateLimitResponse_RateLimit_Unit) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_1de95711edb19ee8, []int{1, 0, 0}
}

type RateLimitRequest struct {
	Domain               string                           `protobuf:"bytes,1,opt,name=domain,proto3" json:"domain,omitempty"`
	Descriptors          []*ratelimit.RateLimitDescriptor `protobuf:"bytes,2,rep,name=descriptors,proto3" json:"descriptors,omitempty"`
	HitsAddend           uint32                           `protobuf:"varint,3,opt,name=hits_addend,json=hitsAddend,proto3" json:"hits_addend,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                         `json:"-"`
	XXX_unrecognized     []byte                           `json:"-"`
	XXX_sizecache        int32                            `json:"-"`
}

func (m *RateLimitRequest) Reset()         { *m = RateLimitRequest{} }
func (m *RateLimitRequest) String() string { return proto.CompactTextString(m) }
func (*RateLimitRequest) ProtoMessage()    {}
func (*RateLimitRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_1de95711edb19ee8, []int{0}
}

func (m *RateLimitRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RateLimitRequest.Unmarshal(m, b)
}
func (m *RateLimitRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RateLimitRequest.Marshal(b, m, deterministic)
}
func (m *RateLimitRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RateLimitRequest.Merge(m, src)
}
func (m *RateLimitRequest) XXX_Size() int {
	return xxx_messageInfo_RateLimitRequest.Size(m)
}
func (m *RateLimitRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RateLimitRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RateLimitRequest proto.InternalMessageInfo

func (m *RateLimitRequest) GetDomain() string {
	if m != nil {
		return m.Domain
	}
	return ""
}

func (m *RateLimitRequest) GetDescriptors() []*ratelimit.RateLimitDescriptor {
	if m != nil {
		return m.Descriptors
	}
	return nil
}

func (m *RateLimitRequest) GetHitsAddend() uint32 {
	if m != nil {
		return m.HitsAddend
	}
	return 0
}

type RateLimitResponse struct {
	OverallCode          RateLimitResponse_Code                `protobuf:"varint,1,opt,name=overall_code,json=overallCode,proto3,enum=envoy.service.ratelimit.v2.RateLimitResponse_Code" json:"overall_code,omitempty"`
	Statuses             []*RateLimitResponse_DescriptorStatus `protobuf:"bytes,2,rep,name=statuses,proto3" json:"statuses,omitempty"`
	Headers              []*core.HeaderValue                   `protobuf:"bytes,3,rep,name=headers,proto3" json:"headers,omitempty"`
	RequestHeadersToAdd  []*core.HeaderValue                   `protobuf:"bytes,4,rep,name=request_headers_to_add,json=requestHeadersToAdd,proto3" json:"request_headers_to_add,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                              `json:"-"`
	XXX_unrecognized     []byte                                `json:"-"`
	XXX_sizecache        int32                                 `json:"-"`
}

func (m *RateLimitResponse) Reset()         { *m = RateLimitResponse{} }
func (m *RateLimitResponse) String() string { return proto.CompactTextString(m) }
func (*RateLimitResponse) ProtoMessage()    {}
func (*RateLimitResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_1de95711edb19ee8, []int{1}
}

func (m *RateLimitResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RateLimitResponse.Unmarshal(m, b)
}
func (m *RateLimitResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RateLimitResponse.Marshal(b, m, deterministic)
}
func (m *RateLimitResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RateLimitResponse.Merge(m, src)
}
func (m *RateLimitResponse) XXX_Size() int {
	return xxx_messageInfo_RateLimitResponse.Size(m)
}
func (m *RateLimitResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_RateLimitResponse.DiscardUnknown(m)
}

var xxx_messageInfo_RateLimitResponse proto.InternalMessageInfo

func (m *RateLimitResponse) GetOverallCode() RateLimitResponse_Code {
	if m != nil {
		return m.OverallCode
	}
	return RateLimitResponse_UNKNOWN
}

func (m *RateLimitResponse) GetStatuses() []*RateLimitResponse_DescriptorStatus {
	if m != nil {
		return m.Statuses
	}
	return nil
}

func (m *RateLimitResponse) GetHeaders() []*core.HeaderValue {
	if m != nil {
		return m.Headers
	}
	return nil
}

func (m *RateLimitResponse) GetRequestHeadersToAdd() []*core.HeaderValue {
	if m != nil {
		return m.RequestHeadersToAdd
	}
	return nil
}

type RateLimitResponse_RateLimit struct {
	RequestsPerUnit      uint32                           `protobuf:"varint,1,opt,name=requests_per_unit,json=requestsPerUnit,proto3" json:"requests_per_unit,omitempty"`
	Unit                 RateLimitResponse_RateLimit_Unit `protobuf:"varint,2,opt,name=unit,proto3,enum=envoy.service.ratelimit.v2.RateLimitResponse_RateLimit_Unit" json:"unit,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                         `json:"-"`
	XXX_unrecognized     []byte                           `json:"-"`
	XXX_sizecache        int32                            `json:"-"`
}

func (m *RateLimitResponse_RateLimit) Reset()         { *m = RateLimitResponse_RateLimit{} }
func (m *RateLimitResponse_RateLimit) String() string { return proto.CompactTextString(m) }
func (*RateLimitResponse_RateLimit) ProtoMessage()    {}
func (*RateLimitResponse_RateLimit) Descriptor() ([]byte, []int) {
	return fileDescriptor_1de95711edb19ee8, []int{1, 0}
}

func (m *RateLimitResponse_RateLimit) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RateLimitResponse_RateLimit.Unmarshal(m, b)
}
func (m *RateLimitResponse_RateLimit) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RateLimitResponse_RateLimit.Marshal(b, m, deterministic)
}
func (m *RateLimitResponse_RateLimit) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RateLimitResponse_RateLimit.Merge(m, src)
}
func (m *RateLimitResponse_RateLimit) XXX_Size() int {
	return xxx_messageInfo_RateLimitResponse_RateLimit.Size(m)
}
func (m *RateLimitResponse_RateLimit) XXX_DiscardUnknown() {
	xxx_messageInfo_RateLimitResponse_RateLimit.DiscardUnknown(m)
}

var xxx_messageInfo_RateLimitResponse_RateLimit proto.InternalMessageInfo

func (m *RateLimitResponse_RateLimit) GetRequestsPerUnit() uint32 {
	if m != nil {
		return m.RequestsPerUnit
	}
	return 0
}

func (m *RateLimitResponse_RateLimit) GetUnit() RateLimitResponse_RateLimit_Unit {
	if m != nil {
		return m.Unit
	}
	return RateLimitResponse_RateLimit_UNKNOWN
}

type RateLimitResponse_DescriptorStatus struct {
	Code                 RateLimitResponse_Code       `protobuf:"varint,1,opt,name=code,proto3,enum=envoy.service.ratelimit.v2.RateLimitResponse_Code" json:"code,omitempty"`
	CurrentLimit         *RateLimitResponse_RateLimit `protobuf:"bytes,2,opt,name=current_limit,json=currentLimit,proto3" json:"current_limit,omitempty"`
	LimitRemaining       uint32                       `protobuf:"varint,3,opt,name=limit_remaining,json=limitRemaining,proto3" json:"limit_remaining,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                     `json:"-"`
	XXX_unrecognized     []byte                       `json:"-"`
	XXX_sizecache        int32                        `json:"-"`
}

func (m *RateLimitResponse_DescriptorStatus) Reset()         { *m = RateLimitResponse_DescriptorStatus{} }
func (m *RateLimitResponse_DescriptorStatus) String() string { return proto.CompactTextString(m) }
func (*RateLimitResponse_DescriptorStatus) ProtoMessage()    {}
func (*RateLimitResponse_DescriptorStatus) Descriptor() ([]byte, []int) {
	return fileDescriptor_1de95711edb19ee8, []int{1, 1}
}

func (m *RateLimitResponse_DescriptorStatus) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RateLimitResponse_DescriptorStatus.Unmarshal(m, b)
}
func (m *RateLimitResponse_DescriptorStatus) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RateLimitResponse_DescriptorStatus.Marshal(b, m, deterministic)
}
func (m *RateLimitResponse_DescriptorStatus) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RateLimitResponse_DescriptorStatus.Merge(m, src)
}
func (m *RateLimitResponse_DescriptorStatus) XXX_Size() int {
	return xxx_messageInfo_RateLimitResponse_DescriptorStatus.Size(m)
}
func (m *RateLimitResponse_DescriptorStatus) XXX_DiscardUnknown() {
	xxx_messageInfo_RateLimitResponse_DescriptorStatus.DiscardUnknown(m)
}

var xxx_messageInfo_RateLimitResponse_DescriptorStatus proto.InternalMessageInfo

func (m *RateLimitResponse_DescriptorStatus) GetCode() RateLimitResponse_Code {
	if m != nil {
		return m.Code
	}
	return RateLimitResponse_UNKNOWN
}

func (m *RateLimitResponse_DescriptorStatus) GetCurrentLimit() *RateLimitResponse_RateLimit {
	if m != nil {
		return m.CurrentLimit
	}
	return nil
}

func (m *RateLimitResponse_DescriptorStatus) GetLimitRemaining() uint32 {
	if m != nil {
		return m.LimitRemaining
	}
	return 0
}

func init() {
	proto.RegisterEnum("envoy.service.ratelimit.v2.RateLimitResponse_Code", RateLimitResponse_Code_name, RateLimitResponse_Code_value)
	proto.RegisterEnum("envoy.service.ratelimit.v2.RateLimitResponse_RateLimit_Unit", RateLimitResponse_RateLimit_Unit_name, RateLimitResponse_RateLimit_Unit_value)
	proto.RegisterType((*RateLimitRequest)(nil), "envoy.service.ratelimit.v2.RateLimitRequest")
	proto.RegisterType((*RateLimitResponse)(nil), "envoy.service.ratelimit.v2.RateLimitResponse")
	proto.RegisterType((*RateLimitResponse_RateLimit)(nil), "envoy.service.ratelimit.v2.RateLimitResponse.RateLimit")
	proto.RegisterType((*RateLimitResponse_DescriptorStatus)(nil), "envoy.service.ratelimit.v2.RateLimitResponse.DescriptorStatus")
}

func init() {
	proto.RegisterFile("envoy/service/ratelimit/v2/rls.proto", fileDescriptor_1de95711edb19ee8)
}

var fileDescriptor_1de95711edb19ee8 = []byte{
	// 669 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xa4, 0x94, 0xcd, 0x6e, 0xd3, 0x4a,
	0x14, 0xc7, 0xeb, 0xc4, 0x37, 0x6d, 0x4f, 0xfa, 0xe1, 0xce, 0x95, 0xda, 0x5c, 0xeb, 0xde, 0xb6,
	0x8a, 0xae, 0x20, 0xa2, 0x60, 0x4b, 0x66, 0xc1, 0x06, 0x55, 0x4a, 0x3f, 0x50, 0xab, 0xb6, 0x49,
	0x34, 0x69, 0x8a, 0x8a, 0x90, 0xac, 0x69, 0x3c, 0x6a, 0x47, 0x72, 0x3d, 0x66, 0x66, 0x6c, 0xd1,
	0x1d, 0x0b, 0x16, 0xec, 0xd8, 0x22, 0x1e, 0x85, 0x27, 0x80, 0x25, 0xe2, 0x09, 0x78, 0x05, 0x1e,
	0x00, 0x21, 0x8f, 0x9d, 0x8f, 0xb6, 0x02, 0xb5, 0xb0, 0xb3, 0xcf, 0x39, 0xff, 0x9f, 0xe7, 0x7f,
	0xce, 0x19, 0xc3, 0xff, 0x34, 0x4a, 0xf9, 0x85, 0x2b, 0xa9, 0x48, 0x59, 0x9f, 0xba, 0x82, 0x28,
	0x1a, 0xb2, 0x73, 0xa6, 0xdc, 0xd4, 0x73, 0x45, 0x28, 0x9d, 0x58, 0x70, 0xc5, 0x91, 0xad, 0xab,
	0x9c, 0xa2, 0xca, 0x19, 0x56, 0x39, 0xa9, 0x67, 0xff, 0x9b, 0x13, 0x48, 0xcc, 0x32, 0x4d, 0x9f,
	0x0b, 0xea, 0x9e, 0x10, 0x49, 0x73, 0xa5, 0x7d, 0xe7, 0x52, 0x76, 0x84, 0x1f, 0x21, 0xf2, 0xba,
	0xe5, 0x24, 0x88, 0x89, 0x4b, 0xa2, 0x88, 0x2b, 0xa2, 0x18, 0x8f, 0xa4, 0x7b, 0xce, 0x4e, 0xb3,
	0xa2, 0x22, 0xff, 0xdf, 0xb5, 0xbc, 0x54, 0x44, 0x25, 0xc5, 0x01, 0xed, 0xa5, 0x94, 0x84, 0x2c,
	0x20, 0x8a, 0xba, 0x83, 0x87, 0x3c, 0x51, 0x7f, 0x6f, 0x80, 0x85, 0x89, 0xa2, 0xfb, 0xd9, 0xb7,
	0x30, 0x7d, 0x91, 0x50, 0xa9, 0xd0, 0x22, 0x54, 0x02, 0x7e, 0x4e, 0x58, 0x54, 0x33, 0x56, 0x8d,
	0xc6, 0x34, 0x2e, 0xde, 0xd0, 0x01, 0x54, 0x03, 0x2a, 0xfb, 0x82, 0xc5, 0x8a, 0x0b, 0x59, 0x2b,
	0xad, 0x96, 0x1b, 0x55, 0x6f, 0xcd, 0xc9, 0xcd, 0x93, 0x98, 0x39, 0xa9, 0x37, 0xe6, 0x7d, 0x88,
	0xdd, 0x1a, 0x6a, 0xf0, 0xb8, 0x1e, 0xad, 0x40, 0xf5, 0x8c, 0x29, 0xe9, 0x93, 0x20, 0xa0, 0x51,
	0x50, 0x2b, 0xaf, 0x1a, 0x8d, 0x59, 0x0c, 0x59, 0xa8, 0xa9, 0x23, 0xf5, 0x2f, 0x15, 0x58, 0x18,
	0x3b, 0x9c, 0x8c, 0x79, 0x24, 0x29, 0xea, 0xc1, 0x0c, 0x4f, 0xa9, 0x20, 0x61, 0xe8, 0xf7, 0x79,
	0x40, 0xf5, 0x19, 0xe7, 0x3c, 0xcf, 0xf9, 0xf9, 0x0c, 0x9c, 0x6b, 0x10, 0x67, 0x93, 0x07, 0x14,
	0x57, 0x0b, 0x4e, 0xf6, 0x82, 0x9e, 0xc1, 0x54, 0xde, 0x32, 0x3a, 0x70, 0xb6, 0x7e, 0x3b, 0xe4,
	0xc8, 0x66, 0x57, 0x73, 0xf0, 0x90, 0x87, 0x8e, 0x61, 0xf2, 0x8c, 0x92, 0x80, 0x0a, 0x59, 0x2b,
	0x6b, 0xf4, 0xf2, 0xe5, 0xa6, 0x65, 0x5b, 0xe1, 0xec, 0xe8, 0x8a, 0x23, 0x12, 0x26, 0x74, 0x63,
	0xe5, 0xdb, 0xbb, 0xef, 0x6f, 0xff, 0xfa, 0x07, 0x96, 0x44, 0x41, 0xf7, 0x0b, 0xbd, 0xaf, 0x78,
	0xd6, 0x2f, 0x3c, 0xe0, 0xa1, 0x2e, 0x2c, 0x8a, 0x7c, 0x6c, 0x57, 0x4a, 0x6a, 0xe6, 0x4d, 0xbe,
	0x84, 0xff, 0x2e, 0xd4, 0x79, 0x4c, 0x1e, 0xf2, 0x66, 0x10, 0xd8, 0x9f, 0x0c, 0x98, 0x1e, 0x1a,
	0x44, 0xf7, 0x60, 0xa1, 0x28, 0x92, 0x7e, 0x4c, 0x85, 0x9f, 0x44, 0x4c, 0xe9, 0xae, 0xcf, 0xe2,
	0xf9, 0x41, 0xa2, 0x43, 0x45, 0x2f, 0x62, 0x0a, 0x75, 0xc0, 0xd4, 0xe9, 0x92, 0x1e, 0xca, 0xe3,
	0xdb, 0x75, 0x70, 0x18, 0x71, 0x32, 0x16, 0xd6, 0xa4, 0xfa, 0x3a, 0x98, 0x9a, 0x5c, 0x85, 0xc9,
	0x5e, 0x6b, 0xaf, 0xd5, 0x7e, 0xda, 0xb2, 0x26, 0x10, 0x40, 0xa5, 0xbb, 0xbd, 0xd9, 0x6e, 0x6d,
	0x59, 0x46, 0xf6, 0x7c, 0xb0, 0xdb, 0xea, 0x1d, 0x6e, 0x5b, 0x25, 0x34, 0x05, 0xe6, 0x4e, 0xbb,
	0x87, 0xad, 0x32, 0x9a, 0x84, 0xf2, 0x56, 0xf3, 0xd8, 0x32, 0xed, 0xaf, 0x06, 0x58, 0x57, 0x47,
	0x83, 0x9e, 0x80, 0xf9, 0x87, 0xbb, 0xa3, 0xf5, 0xe8, 0x39, 0xcc, 0xf6, 0x13, 0x21, 0x68, 0xa4,
	0x7c, 0x2d, 0xd0, 0xbe, 0xab, 0xde, 0xa3, 0xdf, 0xf4, 0x8d, 0x67, 0x0a, 0x5a, 0xde, 0xf8, 0xbb,
	0x30, 0xaf, 0x55, 0xbe, 0xa0, 0xd9, 0xfd, 0x63, 0xd1, 0x69, 0x71, 0x49, 0xe6, 0xc2, 0x5c, 0x5f,
	0x44, 0xeb, 0x6b, 0x60, 0xea, 0x1d, 0xbe, 0xd4, 0xa3, 0x0a, 0x94, 0xda, 0x7b, 0x96, 0x81, 0xe6,
	0x00, 0xda, 0x47, 0xdb, 0xd8, 0xdf, 0xdf, 0x3d, 0xd8, 0x3d, 0xb4, 0x4a, 0xde, 0xeb, 0xf1, 0x2b,
	0xdf, 0xcd, 0x4f, 0x88, 0x62, 0x98, 0xef, 0x9e, 0xf1, 0x24, 0x0c, 0x46, 0x63, 0xbf, 0x7f, 0x43,
	0x13, 0x7a, 0x01, 0xec, 0x07, 0xb7, 0xb2, 0x5c, 0x9f, 0xd8, 0x68, 0x7e, 0x78, 0xf5, 0xf1, 0x73,
	0xa5, 0x64, 0x19, 0xd0, 0x60, 0x3c, 0x17, 0xc7, 0x82, 0xbf, 0xbc, 0xf8, 0x05, 0x67, 0x63, 0x0a,
	0x87, 0xb2, 0x93, 0xfd, 0xb7, 0x3a, 0xc6, 0x1b, 0xc3, 0x38, 0xa9, 0xe8, 0x7f, 0xd8, 0xc3, 0x1f,
	0x01, 0x00, 0x00, 0xff, 0xff, 0xe9, 0x0d, 0xcc, 0x20, 0xa5, 0x05, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// RateLimitServiceClient is the client API for RateLimitService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RateLimitServiceClient interface {
	ShouldRateLimit(ctx context.Context, in *RateLimitRequest, opts ...grpc.CallOption) (*RateLimitResponse, error)
}

type rateLimitServiceClient struct {
	cc *grpc.ClientConn
}

func NewRateLimitServiceClient(cc *grpc.ClientConn) RateLimitServiceClient {
	return &rateLimitServiceClient{cc}
}

func (c *rateLimitServiceClient) ShouldRateLimit(ctx context.Context, in *RateLimitRequest, opts ...grpc.CallOption) (*RateLimitResponse, error) {
	out := new(RateLimitResponse)
	err := c.cc.Invoke(ctx, "/envoy.service.ratelimit.v2.RateLimitService/ShouldRateLimit", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RateLimitServiceServer is the server API for RateLimitService service.
type RateLimitServiceServer interface {
	ShouldRateLimit(context.Context, *RateLimitRequest) (*RateLimitResponse, error)
}

// UnimplementedRateLimitServiceServer can be embedded to have forward compatible implementations.
type UnimplementedRateLimitServiceServer struct {
}

func (*UnimplementedRateLimitServiceServer) ShouldRateLimit(ctx context.Context, req *RateLimitRequest) (*RateLimitResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ShouldRateLimit not implemented")
}

func RegisterRateLimitServiceServer(s *grpc.Server, srv RateLimitServiceServer) {
	s.RegisterService(&_RateLimitService_serviceDesc, srv)
}

func _RateLimitService_ShouldRateLimit_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RateLimitRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RateLimitServiceServer).ShouldRateLimit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/envoy.service.ratelimit.v2.RateLimitService/ShouldRateLimit",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RateLimitServiceServer).ShouldRateLimit(ctx, req.(*RateLimitRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RateLimitService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "envoy.service.ratelimit.v2.RateLimitService",
	HandlerType: (*RateLimitServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ShouldRateLimit",
			Handler:    _RateLimitService_ShouldRateLimit_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "envoy/service/ratelimit/v2/rls.proto",
}
