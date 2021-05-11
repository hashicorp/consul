// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/extensions/filters/http/adaptive_concurrency/v3/adaptive_concurrency.proto

package envoy_extensions_filters_http_adaptive_concurrency_v3

import (
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	v31 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
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
	SampleAggregatePercentile *v3.Percent                                                 `protobuf:"bytes,1,opt,name=sample_aggregate_percentile,json=sampleAggregatePercentile,proto3" json:"sample_aggregate_percentile,omitempty"`
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
	return fileDescriptor_0f6af5b2d621e04b, []int{0}
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

func (m *GradientControllerConfig) GetSampleAggregatePercentile() *v3.Percent {
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
	return fileDescriptor_0f6af5b2d621e04b, []int{0, 0}
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
	Jitter               *v3.Percent           `protobuf:"bytes,3,opt,name=jitter,proto3" json:"jitter,omitempty"`
	MinConcurrency       *wrappers.UInt32Value `protobuf:"bytes,4,opt,name=min_concurrency,json=minConcurrency,proto3" json:"min_concurrency,omitempty"`
	Buffer               *v3.Percent           `protobuf:"bytes,5,opt,name=buffer,proto3" json:"buffer,omitempty"`
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
	return fileDescriptor_0f6af5b2d621e04b, []int{0, 1}
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

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetJitter() *v3.Percent {
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

func (m *GradientControllerConfig_MinimumRTTCalculationParams) GetBuffer() *v3.Percent {
	if m != nil {
		return m.Buffer
	}
	return nil
}

type AdaptiveConcurrency struct {
	// Types that are valid to be assigned to ConcurrencyControllerConfig:
	//	*AdaptiveConcurrency_GradientControllerConfig
	ConcurrencyControllerConfig isAdaptiveConcurrency_ConcurrencyControllerConfig `protobuf_oneof:"concurrency_controller_config"`
	Enabled                     *v31.RuntimeFeatureFlag                           `protobuf:"bytes,2,opt,name=enabled,proto3" json:"enabled,omitempty"`
	XXX_NoUnkeyedLiteral        struct{}                                          `json:"-"`
	XXX_unrecognized            []byte                                            `json:"-"`
	XXX_sizecache               int32                                             `json:"-"`
}

func (m *AdaptiveConcurrency) Reset()         { *m = AdaptiveConcurrency{} }
func (m *AdaptiveConcurrency) String() string { return proto.CompactTextString(m) }
func (*AdaptiveConcurrency) ProtoMessage()    {}
func (*AdaptiveConcurrency) Descriptor() ([]byte, []int) {
	return fileDescriptor_0f6af5b2d621e04b, []int{1}
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

func (m *AdaptiveConcurrency) GetEnabled() *v31.RuntimeFeatureFlag {
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
	proto.RegisterType((*GradientControllerConfig)(nil), "envoy.extensions.filters.http.adaptive_concurrency.v3.GradientControllerConfig")
	proto.RegisterType((*GradientControllerConfig_ConcurrencyLimitCalculationParams)(nil), "envoy.extensions.filters.http.adaptive_concurrency.v3.GradientControllerConfig.ConcurrencyLimitCalculationParams")
	proto.RegisterType((*GradientControllerConfig_MinimumRTTCalculationParams)(nil), "envoy.extensions.filters.http.adaptive_concurrency.v3.GradientControllerConfig.MinimumRTTCalculationParams")
	proto.RegisterType((*AdaptiveConcurrency)(nil), "envoy.extensions.filters.http.adaptive_concurrency.v3.AdaptiveConcurrency")
}

func init() {
	proto.RegisterFile("envoy/extensions/filters/http/adaptive_concurrency/v3/adaptive_concurrency.proto", fileDescriptor_0f6af5b2d621e04b)
}

var fileDescriptor_0f6af5b2d621e04b = []byte{
	// 777 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x55, 0x3d, 0x6f, 0xdb, 0x48,
	0x10, 0x35, 0xe5, 0x4f, 0xec, 0x7d, 0x19, 0x34, 0xce, 0x47, 0x4b, 0xb6, 0xcf, 0x36, 0xae, 0x30,
	0x5c, 0x2c, 0x01, 0x0b, 0xd7, 0xa8, 0x33, 0x75, 0xf0, 0x9d, 0x2f, 0x89, 0x43, 0x10, 0xb6, 0x81,
	0x54, 0xc4, 0x8a, 0x5a, 0xd1, 0xeb, 0x2c, 0x97, 0xf4, 0x72, 0x49, 0x4b, 0x5d, 0xca, 0x20, 0xff,
	0x20, 0xa9, 0x53, 0xa5, 0x4f, 0x93, 0x3e, 0x40, 0xba, 0x20, 0x75, 0x80, 0xfc, 0x83, 0xf4, 0x81,
	0xab, 0x60, 0xb9, 0x2b, 0x9b, 0xb1, 0x3e, 0x02, 0x3b, 0xea, 0x24, 0xcd, 0xbc, 0xf7, 0x66, 0xde,
	0xcc, 0x8e, 0x80, 0x8b, 0x59, 0x1e, 0xf7, 0x6c, 0xdc, 0x15, 0x98, 0xa5, 0x24, 0x66, 0xa9, 0xdd,
	0x21, 0x54, 0x60, 0x9e, 0xda, 0xa7, 0x42, 0x24, 0x36, 0x6a, 0xa3, 0x44, 0x90, 0x1c, 0xfb, 0x41,
	0xcc, 0x82, 0x8c, 0x73, 0xcc, 0x82, 0x9e, 0x9d, 0xd7, 0x87, 0xfe, 0x0e, 0x13, 0x1e, 0x8b, 0xd8,
	0xfc, 0xbb, 0x60, 0x84, 0xd7, 0x8c, 0x50, 0x33, 0x42, 0xc9, 0x08, 0x87, 0x22, 0xf3, 0x7a, 0xf5,
	0x4f, 0x55, 0x48, 0x10, 0xb3, 0x0e, 0x09, 0xed, 0x20, 0xe6, 0x58, 0xea, 0xb4, 0x50, 0x8a, 0x15,
	0x6f, 0xb5, 0xa6, 0x12, 0x44, 0x2f, 0x29, 0x22, 0x09, 0xe6, 0x01, 0x66, 0x42, 0x07, 0x57, 0xc3,
	0x38, 0x0e, 0x29, 0xb6, 0x51, 0x42, 0x6c, 0xc4, 0x58, 0x2c, 0x90, 0x28, 0xa4, 0x55, 0x74, 0x5d,
	0x47, 0x8b, 0x6f, 0xad, 0xac, 0x63, 0xb7, 0x33, 0x5e, 0x24, 0x8c, 0x8a, 0x5f, 0x70, 0x94, 0x24,
	0xb2, 0x64, 0x15, 0x5f, 0xcb, 0xda, 0x09, 0x2a, 0xf3, 0xda, 0xa9, 0x40, 0x22, 0xeb, 0x87, 0x37,
	0x07, 0xc2, 0x39, 0xe6, 0xb2, 0x75, 0xc2, 0x42, 0x9d, 0xf2, 0x47, 0x8e, 0x28, 0x69, 0x23, 0x81,
	0xed, 0xfe, 0x07, 0x15, 0xd8, 0xfa, 0x08, 0x80, 0xf5, 0x2f, 0x47, 0x6d, 0x82, 0x99, 0x68, 0xc6,
	0x4c, 0xf0, 0x98, 0x52, 0xcc, 0x9b, 0x85, 0x0b, 0xe6, 0x09, 0xa8, 0xa5, 0x28, 0x4a, 0x28, 0xf6,
	0x51, 0x18, 0x72, 0x1c, 0x22, 0x81, 0x7d, 0xdd, 0x37, 0xa1, 0xd8, 0x32, 0x36, 0x8c, 0xed, 0x9f,
	0x76, 0x97, 0xa1, 0x32, 0x5c, 0x1a, 0x03, 0xf3, 0x3a, 0x74, 0x55, 0x82, 0xb7, 0xa2, 0xa0, 0x7b,
	0x7d, 0xa4, 0x7b, 0x05, 0x34, 0x5f, 0x1b, 0xc0, 0x2a, 0xd9, 0xef, 0x53, 0x12, 0x11, 0xe1, 0x27,
	0x88, 0xa3, 0x28, 0xb5, 0x2a, 0x05, 0xeb, 0x39, 0xbc, 0xd3, 0x18, 0xe1, 0xa8, 0x5e, 0x60, 0xf3,
	0x3a, 0xef, 0xbe, 0x94, 0x6b, 0x22, 0x1a, 0x64, 0xb4, 0xf0, 0xcb, 0x2d, 0x84, 0x9d, 0x85, 0x4b,
	0x67, 0xf6, 0x99, 0x51, 0x59, 0x34, 0xbc, 0xe5, 0xe0, 0x46, 0xb2, 0xca, 0x30, 0x5f, 0x1a, 0x60,
	0x29, 0x22, 0xcc, 0xe7, 0x42, 0xf8, 0x01, 0xa2, 0x41, 0xbf, 0xe4, 0xe9, 0xa2, 0xe4, 0xc7, 0x93,
	0x2e, 0xf9, 0x01, 0x61, 0x24, 0xca, 0x22, 0xef, 0xe8, 0x68, 0x5c, 0xb1, 0x8b, 0x11, 0x61, 0x9e,
	0x28, 0xfa, 0x51, 0xb1, 0xea, 0xa7, 0x0a, 0xd8, 0xfc, 0x6e, 0xbb, 0xe6, 0x23, 0xf0, 0x7b, 0x84,
	0xba, 0xfe, 0xc0, 0x1c, 0xf4, 0x00, 0x56, 0xa1, 0x5a, 0x4a, 0xd8, 0x5f, 0x4a, 0x78, 0x7c, 0xc0,
	0x44, 0x7d, 0xf7, 0x04, 0xd1, 0x0c, 0x3b, 0xf3, 0x97, 0xce, 0xcc, 0x4e, 0x65, 0x63, 0xca, 0x5b,
	0x8a, 0x50, 0xf7, 0xa6, 0x96, 0x89, 0x41, 0xad, 0x4c, 0x9b, 0x25, 0x72, 0xe1, 0x7c, 0xc2, 0x04,
	0xe6, 0x39, 0xa2, 0xda, 0xae, 0x95, 0x01, 0x81, 0x7f, 0xf4, 0xab, 0x70, 0xc0, 0xa5, 0x33, 0xff,
	0xca, 0x98, 0x59, 0x30, 0x76, 0xa6, 0xbc, 0x95, 0x12, 0xd3, 0x71, 0x41, 0x74, 0xa0, 0x79, 0x1a,
	0x17, 0x2f, 0xde, 0x3e, 0x5d, 0xe7, 0x20, 0x51, 0xb6, 0xab, 0x97, 0xab, 0x2d, 0x1f, 0xe7, 0xf8,
	0x2e, 0xa2, 0xc9, 0x29, 0xfa, 0x81, 0x4d, 0xa9, 0xbe, 0x9f, 0x06, 0xb5, 0x31, 0xc3, 0x31, 0xf7,
	0xc0, 0xc2, 0x55, 0xb3, 0xc6, 0x6d, 0x9a, 0xbd, 0x82, 0x99, 0xff, 0x83, 0x5f, 0x38, 0x3e, 0xcf,
	0x70, 0x2a, 0xfc, 0x20, 0xce, 0xd8, 0x2d, 0xa7, 0xf2, 0xb3, 0xc6, 0x36, 0x25, 0xd4, 0x84, 0x60,
	0xee, 0x8c, 0x08, 0x81, 0xb9, 0x76, 0x7e, 0xd4, 0x8b, 0xd5, 0x59, 0xe6, 0x21, 0xf8, 0x4d, 0x6e,
	0x79, 0xc9, 0x34, 0x6b, 0xe6, 0x36, 0xea, 0xbf, 0x46, 0x84, 0x95, 0x4c, 0x94, 0xfa, 0xad, 0xac,
	0xd3, 0xc1, 0xdc, 0x9a, 0x1d, 0xaf, 0xaf, 0xb2, 0x1a, 0xe7, 0x72, 0xae, 0x14, 0x9c, 0x4d, 0x78,
	0xae, 0x63, 0x26, 0xd6, 0x38, 0x96, 0x92, 0x2e, 0x38, 0x9c, 0xac, 0xe4, 0xd6, 0xe7, 0x0a, 0x58,
	0xda, 0xd3, 0xc8, 0xb2, 0x23, 0xcf, 0x0d, 0x50, 0x0d, 0x35, 0x48, 0x32, 0x6a, 0x94, 0xaf, 0x84,
	0xf5, 0xce, 0x3c, 0x9c, 0xf0, 0x3d, 0xb9, 0xbe, 0x19, 0xff, 0x4d, 0x79, 0x56, 0x38, 0xea, 0xe8,
	0x3b, 0x60, 0x1e, 0x33, 0xd4, 0xa2, 0xb8, 0xad, 0x77, 0x6e, 0x1b, 0x7e, 0xe3, 0x8a, 0xfc, 0x6b,
	0x94, 0x32, 0x5e, 0xc6, 0x04, 0x89, 0xf0, 0x3e, 0x46, 0x22, 0xe3, 0x78, 0x9f, 0xa2, 0xd0, 0xeb,
	0x03, 0x1b, 0xae, 0xb4, 0xf3, 0x1e, 0x38, 0xb8, 0x9b, 0x9d, 0x43, 0x1c, 0x73, 0xfe, 0x02, 0x6b,
	0xe5, 0x93, 0x32, 0xe0, 0x99, 0x39, 0xfd, 0xc5, 0x31, 0x1c, 0xf2, 0xe6, 0xc9, 0xbb, 0x0f, 0x73,
	0x95, 0xc5, 0x0a, 0x68, 0x92, 0x58, 0x95, 0x9d, 0xf0, 0xb8, 0xdb, 0xbb, 0x9b, 0x93, 0x8e, 0x35,
	0xa4, 0x12, 0x57, 0xae, 0xbe, 0x6b, 0xb4, 0xe6, 0x8a, 0x37, 0x50, 0xff, 0x1a, 0x00, 0x00, 0xff,
	0xff, 0x7a, 0xf7, 0xd0, 0x7a, 0xc1, 0x08, 0x00, 0x00,
}
