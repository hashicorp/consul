// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/config/filter/http/adaptive_concurrency/v2alpha/adaptive_concurrency.proto

package envoy_config_filter_http_adaptive_concurrency_v2alpha

import (
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	_type "github.com/envoyproxy/go-control-plane/envoy/type"
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/golang/protobuf/proto"
	duration "github.com/golang/protobuf/ptypes/duration"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	_ "google.golang.org/genproto/googleapis/api/annotations"
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

type GradientControllerConfig struct {
	SampleAggregatePercentile *_type.Percent                                              `protobuf:"bytes,1,opt,name=sample_aggregate_percentile,json=sampleAggregatePercentile,proto3" json:"sample_aggregate_percentile,omitempty"`
	ConcurrencyLimitParams    *GradientControllerConfig_ConcurrencyLimitCalculationParams `protobuf:"bytes,2,opt,name=concurrency_limit_params,json=concurrencyLimitParams,proto3" json:"concurrency_limit_params,omitempty"`
	MinRttCalcParams          *GradientControllerConfig_MinimumRTTCalculationParams       `protobuf:"bytes,3,opt,name=min_rtt_calc_params,json=minRttCalcParams,proto3" json:"min_rtt_calc_params,omitempty"`
	XXX_NoUnkeyedLiteral      struct{}                                                    `json:"-"`
	XXX_unrecognized          []byte                                                      `json:"-"`
	XXX_sizecache             int32                                                       `json:"-"`
}

func (m *GradientControllerConfig) Reset()         { *m = GradientControllerConfig{} }
func (m *GradientControllerConfig) String() string { return proto.CompactTextString(m) }
func (*GradientControllerConfig) ProtoMessage()    {}
func (*GradientControllerConfig) Descriptor() ([]byte, []int) {
	return fileDescriptor_c58a0beecb0ec580, []int{0}
}

func (m *GradientControllerConfig) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GradientControllerConfig.Unmarshal(m, b)
}
func (m *GradientControllerConfig) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GradientControllerConfig.Marshal(b, m, deterministic)
}
func (m *GradientControllerConfig) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GradientControllerConfig.Merge(m, src)
}
func (m *GradientControllerConfig) XXX_Size() int {
	return xxx_messageInfo_GradientControllerConfig.Size(m)
}
func (m *GradientControllerConfig) XXX_DiscardUnknown() {
	xxx_messageInfo_GradientControllerConfig.DiscardUnknown(m)
}

var xxx_messageInfo_GradientControllerConfig proto.InternalMessageInfo

func (m *GradientControllerConfig) GetSampleAggregatePercentile() *_type.Percent {
	if m != nil {
		return m.SampleAggregatePercentile
	}
	return nil
}

func (m *GradientControllerConfig) GetConcurrencyLimitParams() *GradientControllerConfig_ConcurrencyLimitCalculationParams {
	if m != nil {
		return m.ConcurrencyLimitParams
	}
	return nil
}

func (m *GradientControllerConfig) GetMinRttCalcParams() *GradientControllerConfig_MinimumRTTCalculationParams {
	if m != nil {
		return m.MinRttCalcParams
	}
	return nil
}

type GradientControllerConfig_ConcurrencyLimitCalculationParams struct {
	MaxConcurrencyLimit       *wrappers.UInt32Value `protobuf:"bytes,2,opt,name=max_concurrency_limit,json=maxConcurrencyLimit,proto3" json:"max_concurrency_limit,omitempty"`
	ConcurrencyUpdateInterval *duration.Duration    `protobuf:"bytes,3,opt,name=concurrency_update_interval,json=concurrencyUpdateInterval,proto3" json:"concurrency_update_interval,omitempty"`
	XXX_NoUnkeyedLiteral      struct{}              `json:"-"`
	XXX_unrecognized          []byte                `json:"-"`
	XXX_sizecache             int32                 `json:"-"`
}

func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) Reset() {
	*m = GradientControllerConfig_ConcurrencyLimitCalculationParams{}
}
func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) String() string {
	return proto.CompactTextString(m)
}
func (*GradientControllerConfig_ConcurrencyLimitCalculationParams) ProtoMessage() {}
func (*GradientControllerConfig_ConcurrencyLimitCalculationParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_c58a0beecb0ec580, []int{0, 0}
}

func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GradientControllerConfig_ConcurrencyLimitCalculationParams.Unmarshal(m, b)
}
func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GradientControllerConfig_ConcurrencyLimitCalculationParams.Marshal(b, m, deterministic)
}
func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GradientControllerConfig_ConcurrencyLimitCalculationParams.Merge(m, src)
}
func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) XXX_Size() int {
	return xxx_messageInfo_GradientControllerConfig_ConcurrencyLimitCalculationParams.Size(m)
}
func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) XXX_DiscardUnknown() {
	xxx_messageInfo_GradientControllerConfig_ConcurrencyLimitCalculationParams.DiscardUnknown(m)
}

var xxx_messageInfo_GradientControllerConfig_ConcurrencyLimitCalculationParams proto.InternalMessageInfo

func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) GetMaxConcurrencyLimit() *wrappers.UInt32Value {
	if m != nil {
		return m.MaxConcurrencyLimit
	}
	return nil
}

func (m *GradientControllerConfig_ConcurrencyLimitCalculationParams) GetConcurrencyUpdateInterval() *duration.Duration {
	if m != nil {
		return m.ConcurrencyUpdateInterval
	}
	return nil
}

type GradientControllerConfig_MinimumRTTCalculationParams struct {
	Interval             *duration.Duration    `protobuf:"bytes,1,opt,name=interval,proto3" json:"interval,omitempty"`
	RequestCount         *wrappers.UInt32Value `protobuf:"bytes,2,opt,name=request_count,json=requestCount,proto3" json:"request_count,omitempty"`
	Jitter               *_type.Percent        `protobuf:"bytes,3,opt,name=jitter,proto3" json:"jitter,omitempty"`
	MinConcurrency       *wrappers.UInt32Value `protobuf:"bytes,4,opt,name=min_concurrency,json=minConcurrency,proto3" json:"min_concurrency,omitempty"`
	Buffer               *_type.Percent        `protobuf:"bytes,5,opt,name=buffer,proto3" json:"buffer,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *GradientControllerConfig_MinimumRTTCalculationParams) Reset() {
	*m = GradientControllerConfig_MinimumRTTCalculationParams{}
}
func (m *GradientControllerConfig_MinimumRTTCalculationParams) String() string {
	return proto.CompactTextString(m)
}
func (*GradientControllerConfig_MinimumRTTCalculationParams) ProtoMessage() {}
func (*GradientControllerConfig_MinimumRTTCalculationParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_c58a0beecb0ec580, []int{0, 1}
}

func (m *GradientControllerConfig_MinimumRTTCalculationParams) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GradientControllerConfig_MinimumRTTCalculationParams.Unmarshal(m, b)
}
func (m *GradientControllerConfig_MinimumRTTCalculationParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GradientControllerConfig_MinimumRTTCalculationParams.Marshal(b, m, deterministic)
}
func (m *GradientControllerConfig_MinimumRTTCalculationParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GradientControllerConfig_MinimumRTTCalculationParams.Merge(m, src)
}
func (m *GradientControllerConfig_MinimumRTTCalculationParams) XXX_Size() int {
	return xxx_messageInfo_GradientControllerConfig_MinimumRTTCalculationParams.Size(m)
}
func (m *GradientControllerConfig_MinimumRTTCalculationParams) XXX_DiscardUnknown() {
	xxx_messageInfo_GradientControllerConfig_MinimumRTTCalculationParams.DiscardUnknown(m)
}

var xxx_messageInfo_GradientControllerConfig_MinimumRTTCalculationParams proto.InternalMessageInfo

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetInterval() *duration.Duration {
	if m != nil {
		return m.Interval
	}
	return nil
}

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetRequestCount() *wrappers.UInt32Value {
	if m != nil {
		return m.RequestCount
	}
	return nil
}

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetJitter() *_type.Percent {
	if m != nil {
		return m.Jitter
	}
	return nil
}

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetMinConcurrency() *wrappers.UInt32Value {
	if m != nil {
		return m.MinConcurrency
	}
	return nil
}

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetBuffer() *_type.Percent {
	if m != nil {
		return m.Buffer
	}
	return nil
}

type AdaptiveConcurrency struct {
	// Types that are valid to be assigned to ConcurrencyControllerConfig:
	//	*AdaptiveConcurrency_GradientControllerConfig
	ConcurrencyControllerConfig isAdaptiveConcurrency_ConcurrencyControllerConfig `protobuf_oneof:"concurrency_controller_config"`
	Enabled                     *core.RuntimeFeatureFlag                          `protobuf:"bytes,2,opt,name=enabled,proto3" json:"enabled,omitempty"`
	XXX_NoUnkeyedLiteral        struct{}                                          `json:"-"`
	XXX_unrecognized            []byte                                            `json:"-"`
	XXX_sizecache               int32                                             `json:"-"`
}

func (m *AdaptiveConcurrency) Reset()         { *m = AdaptiveConcurrency{} }
func (m *AdaptiveConcurrency) String() string { return proto.CompactTextString(m) }
func (*AdaptiveConcurrency) ProtoMessage()    {}
func (*AdaptiveConcurrency) Descriptor() ([]byte, []int) {
	return fileDescriptor_c58a0beecb0ec580, []int{1}
}

func (m *AdaptiveConcurrency) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AdaptiveConcurrency.Unmarshal(m, b)
}
func (m *AdaptiveConcurrency) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AdaptiveConcurrency.Marshal(b, m, deterministic)
}
func (m *AdaptiveConcurrency) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AdaptiveConcurrency.Merge(m, src)
}
func (m *AdaptiveConcurrency) XXX_Size() int {
	return xxx_messageInfo_AdaptiveConcurrency.Size(m)
}
func (m *AdaptiveConcurrency) XXX_DiscardUnknown() {
	xxx_messageInfo_AdaptiveConcurrency.DiscardUnknown(m)
}

var xxx_messageInfo_AdaptiveConcurrency proto.InternalMessageInfo

type isAdaptiveConcurrency_ConcurrencyControllerConfig interface {
	isAdaptiveConcurrency_ConcurrencyControllerConfig()
}

type AdaptiveConcurrency_GradientControllerConfig struct {
	GradientControllerConfig *GradientControllerConfig `protobuf:"bytes,1,opt,name=gradient_controller_config,json=gradientControllerConfig,proto3,oneof"`
}

func (*AdaptiveConcurrency_GradientControllerConfig) isAdaptiveConcurrency_ConcurrencyControllerConfig() {
}

func (m *AdaptiveConcurrency) GetConcurrencyControllerConfig() isAdaptiveConcurrency_ConcurrencyControllerConfig {
	if m != nil {
		return m.ConcurrencyControllerConfig
	}
	return nil
}

func (m *AdaptiveConcurrency) GetGradientControllerConfig() *GradientControllerConfig {
	if x, ok := m.GetConcurrencyControllerConfig().(*AdaptiveConcurrency_GradientControllerConfig); ok {
		return x.GradientControllerConfig
	}
	return nil
}

func (m *AdaptiveConcurrency) GetEnabled() *core.RuntimeFeatureFlag {
	if m != nil {
		return m.Enabled
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*AdaptiveConcurrency) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*AdaptiveConcurrency_GradientControllerConfig)(nil),
	}
}

func init() {
	proto.RegisterType((*GradientControllerConfig)(nil), "envoy.config.filter.http.adaptive_concurrency.v2alpha.GradientControllerConfig")
	proto.RegisterType((*GradientControllerConfig_ConcurrencyLimitCalculationParams)(nil), "envoy.config.filter.http.adaptive_concurrency.v2alpha.GradientControllerConfig.ConcurrencyLimitCalculationParams")
	proto.RegisterType((*GradientControllerConfig_MinimumRTTCalculationParams)(nil), "envoy.config.filter.http.adaptive_concurrency.v2alpha.GradientControllerConfig.MinimumRTTCalculationParams")
	proto.RegisterType((*AdaptiveConcurrency)(nil), "envoy.config.filter.http.adaptive_concurrency.v2alpha.AdaptiveConcurrency")
}

func init() {
	proto.RegisterFile("envoy/config/filter/http/adaptive_concurrency/v2alpha/adaptive_concurrency.proto", fileDescriptor_c58a0beecb0ec580)
}

var fileDescriptor_c58a0beecb0ec580 = []byte{
	// 737 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x55, 0xcd, 0x6e, 0xd3, 0x4c,
	0x14, 0xad, 0xd3, 0xbf, 0x68, 0xbe, 0x1f, 0x2a, 0x47, 0x80, 0x9b, 0xb6, 0xa8, 0x54, 0x20, 0xa1,
	0x22, 0x8d, 0xa5, 0x54, 0x88, 0x25, 0xaa, 0x83, 0x0a, 0x45, 0xfc, 0x44, 0xa6, 0x45, 0x62, 0x65,
	0x4d, 0x9c, 0x1b, 0x77, 0x60, 0x3c, 0x76, 0xc7, 0xe3, 0x90, 0xec, 0x58, 0xb3, 0xe9, 0xb6, 0xec,
	0x11, 0x0b, 0xf6, 0x6c, 0x78, 0x02, 0xb6, 0x6c, 0x78, 0x03, 0x5e, 0x80, 0x15, 0xea, 0x02, 0x21,
	0x7b, 0xc6, 0xa9, 0xd5, 0xa4, 0x41, 0xad, 0xba, 0x6b, 0x75, 0xee, 0x39, 0xf7, 0xde, 0x73, 0x8f,
	0x27, 0xa8, 0x05, 0xbc, 0x17, 0x0d, 0x6c, 0x3f, 0xe2, 0x5d, 0x1a, 0xd8, 0x5d, 0xca, 0x24, 0x08,
	0x7b, 0x4f, 0xca, 0xd8, 0x26, 0x1d, 0x12, 0x4b, 0xda, 0x03, 0xcf, 0x8f, 0xb8, 0x9f, 0x0a, 0x01,
	0xdc, 0x1f, 0xd8, 0xbd, 0x06, 0x61, 0xf1, 0x1e, 0x19, 0x0b, 0xe2, 0x58, 0x44, 0x32, 0x32, 0xef,
	0xe4, 0x8a, 0x58, 0x29, 0x62, 0xa5, 0x88, 0x33, 0x45, 0x3c, 0x96, 0xa4, 0x15, 0xeb, 0xcb, 0x6a,
	0x10, 0x12, 0x53, 0xbb, 0xd7, 0xb0, 0xfd, 0x48, 0x80, 0xdd, 0x26, 0x09, 0x28, 0xd1, 0xba, 0xa5,
	0x50, 0x39, 0x88, 0xc1, 0x8e, 0x41, 0xf8, 0xc0, 0xa5, 0x46, 0x96, 0x83, 0x28, 0x0a, 0x18, 0xe4,
	0x44, 0xc2, 0x79, 0x24, 0x89, 0xa4, 0x11, 0x4f, 0x34, 0x7a, 0x4d, 0xa3, 0xf9, 0x7f, 0xed, 0xb4,
	0x6b, 0x77, 0x52, 0x91, 0x17, 0x9c, 0x86, 0xbf, 0x11, 0x24, 0x8e, 0x41, 0x0c, 0xf9, 0x69, 0x27,
	0x26, 0x65, 0x5d, 0x3b, 0xa4, 0x81, 0x20, 0xb2, 0x98, 0x6b, 0x65, 0x04, 0x4f, 0x24, 0x91, 0x69,
	0x41, 0xbf, 0xda, 0x23, 0x8c, 0x76, 0x88, 0x04, 0xbb, 0xf8, 0x43, 0x01, 0x6b, 0x07, 0x55, 0x64,
	0x3d, 0x10, 0xa4, 0x43, 0x81, 0xcb, 0x66, 0xc4, 0xa5, 0x88, 0x18, 0x03, 0xd1, 0xcc, 0x3d, 0x33,
	0x9f, 0xa3, 0xa5, 0x84, 0x84, 0x31, 0x03, 0x8f, 0x04, 0x81, 0x80, 0x80, 0x48, 0xf0, 0xf4, 0xd2,
	0x94, 0x81, 0x65, 0xac, 0x1a, 0xb7, 0xfe, 0x69, 0xd4, 0xb0, 0xf2, 0x39, 0xb3, 0x04, 0xb7, 0x14,
	0xea, 0x2e, 0x2a, 0xde, 0x66, 0x41, 0x6b, 0x0d, 0x59, 0xe6, 0x67, 0x03, 0x59, 0x25, 0xdf, 0x3d,
	0x46, 0x43, 0x2a, 0xbd, 0x98, 0x08, 0x12, 0x26, 0x56, 0x25, 0x97, 0xdc, 0xc7, 0xe7, 0x3a, 0x1d,
	0x3e, 0x6d, 0x11, 0xdc, 0x3c, 0x2e, 0x7e, 0x9c, 0xb5, 0x6b, 0x12, 0xe6, 0xa7, 0x2c, 0x37, 0xaa,
	0x95, 0x37, 0x76, 0xaa, 0x47, 0xce, 0xec, 0x3b, 0xa3, 0xb2, 0x60, 0xb8, 0x57, 0xfc, 0x13, 0xc5,
	0xaa, 0xc2, 0xfc, 0x60, 0xa0, 0x5a, 0x48, 0xb9, 0x27, 0xa4, 0xf4, 0x7c, 0xc2, 0xfc, 0x62, 0xe4,
	0xe9, 0x7c, 0xe4, 0xd7, 0x17, 0x3d, 0xf2, 0x13, 0xca, 0x69, 0x98, 0x86, 0xee, 0xce, 0xce, 0xa4,
	0x61, 0x17, 0x42, 0xca, 0x5d, 0x99, 0xef, 0xa3, 0xb0, 0xfa, 0x0f, 0x03, 0x5d, 0xff, 0xeb, 0xba,
	0xe6, 0x4b, 0x74, 0x39, 0x24, 0x7d, 0x6f, 0xe4, 0x0e, 0xfa, 0x00, 0xcb, 0x58, 0xc5, 0x11, 0x17,
	0x71, 0xc4, 0xbb, 0xdb, 0x5c, 0x6e, 0x34, 0x5e, 0x10, 0x96, 0x82, 0x33, 0x7f, 0xe4, 0xcc, 0xac,
	0x57, 0x56, 0xa7, 0xdc, 0x5a, 0x48, 0xfa, 0x27, 0x7b, 0x99, 0x80, 0x96, 0xca, 0xb2, 0x69, 0x9c,
	0xa5, 0xcd, 0xa3, 0x5c, 0x82, 0xe8, 0x11, 0xa6, 0xed, 0x5a, 0x1c, 0x69, 0x70, 0x5f, 0x7f, 0x0f,
	0x0e, 0x3a, 0x72, 0xe6, 0x3f, 0x19, 0x33, 0x55, 0x63, 0x7d, 0xca, 0x5d, 0x2c, 0x29, 0xed, 0xe6,
	0x42, 0xdb, 0x5a, 0xa7, 0xfe, 0xbd, 0x82, 0x96, 0x26, 0x78, 0x64, 0x6e, 0xa2, 0xea, 0xb0, 0xa7,
	0x71, 0x96, 0x9e, 0x43, 0x9a, 0xf9, 0x08, 0xfd, 0x27, 0x60, 0x3f, 0x85, 0x44, 0x7a, 0x7e, 0x94,
	0xf2, 0x33, 0x9a, 0xf3, 0xaf, 0xe6, 0x36, 0x33, 0xaa, 0x79, 0x1b, 0xcd, 0xbd, 0xa2, 0x52, 0x82,
	0xd0, 0x06, 0x8c, 0xfd, 0x6a, 0x74, 0x89, 0xf9, 0x14, 0x5d, 0xca, 0x92, 0x56, 0x5a, 0xde, 0x9a,
	0x39, 0x4b, 0xeb, 0xff, 0x43, 0xca, 0x4b, 0x77, 0xc9, 0x9a, 0xb7, 0xd3, 0x6e, 0x17, 0x84, 0x35,
	0x3b, 0xa1, 0xb9, 0x2a, 0x59, 0x3b, 0xac, 0xa0, 0xda, 0xa6, 0x8e, 0x6c, 0x59, 0xe4, 0xbd, 0x81,
	0xea, 0x81, 0x4e, 0x6b, 0x36, 0x9a, 0x8e, 0xab, 0xa7, 0x12, 0xaf, 0x3d, 0x7e, 0x76, 0xc1, 0x9f,
	0xc1, 0x71, 0xd4, 0x1f, 0x4e, 0xb9, 0x56, 0x70, 0xda, 0x43, 0x75, 0x0f, 0xcd, 0x03, 0x27, 0x6d,
	0x06, 0x1d, 0x7d, 0xa3, 0x9b, 0x7a, 0x0e, 0x12, 0x53, 0xdc, 0x6b, 0xe0, 0xec, 0x15, 0xc7, 0x6e,
	0xca, 0x25, 0x0d, 0x61, 0x0b, 0x88, 0x4c, 0x05, 0x6c, 0x31, 0x12, 0xb8, 0x05, 0xcb, 0xb9, 0x81,
	0x56, 0xca, 0xa1, 0x1d, 0x59, 0xcf, 0x9c, 0xfe, 0xe5, 0x18, 0xce, 0x47, 0xe3, 0xe7, 0xe1, 0xef,
	0x83, 0xd9, 0xbb, 0xc5, 0x4f, 0x0b, 0xf4, 0x25, 0xf0, 0x24, 0x7f, 0xe5, 0xd5, 0xa6, 0xc9, 0xa4,
	0x55, 0x37, 0xbe, 0xbc, 0xfd, 0xfa, 0x6d, 0xae, 0xb2, 0x60, 0xa0, 0x26, 0x8d, 0xd4, 0x7c, 0xb1,
	0x88, 0xfa, 0x83, 0xf3, 0x59, 0xe6, 0x58, 0x63, 0x8e, 0xd4, 0xca, 0x62, 0xd1, 0x32, 0xda, 0x73,
	0x79, 0x3e, 0x36, 0xfe, 0x04, 0x00, 0x00, 0xff, 0xff, 0x2d, 0x38, 0xeb, 0x1d, 0x55, 0x07, 0x00,
	0x00,
}
