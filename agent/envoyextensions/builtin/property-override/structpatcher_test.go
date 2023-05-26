package propertyoverride

import (
	"fmt"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"testing"
)

func TestPatchStruct(t *testing.T) {
	makePatch := func(o Op, p string, v any) Patch {
		return Patch{
			Op:    o,
			Path:  p,
			Value: v,
		}
	}
	makeAddPatch := func(p string, v any) Patch {
		return makePatch(OpAdd, p, v)
	}
	makeRemovePatch := func(p string) Patch {
		return makePatch(OpRemove, p, nil)
	}
	expectFieldsListErr := func(resourceName string, truncatedFieldsWarning bool) func(t *testing.T, err error) {
		return func(t *testing.T, err error) {
			require.Contains(t, err.Error(), fmt.Sprintf("available %s fields:", resourceName))
			if truncatedFieldsWarning {
				require.Contains(t, err.Error(), "First 10 fields for this message included, configure with `Debug = true` to print all.")
			} else {
				require.NotContains(t, err.Error(), "First 10 fields for this message included, configure with `Debug = true` to print all.")
			}
		}
	}
	type args struct {
		k       proto.Message
		patches []Patch
		debug   bool
	}
	type testCase struct {
		args     args
		expected proto.Message
		ok       bool
		errMsg   string
		errFunc  func(*testing.T, error)
	}

	// Simplify test case construction for variants of potential input types
	uint32VariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_endpoint_v3.Endpoint{
					HealthCheckConfig: &envoy_endpoint_v3.Endpoint_HealthCheckConfig{
						PortValue: 3000,
					},
				},
				patches: []Patch{makeAddPatch(
					"/health_check_config/port_value",
					i,
				)},
			},
			expected: &envoy_endpoint_v3.Endpoint{
				HealthCheckConfig: &envoy_endpoint_v3.Endpoint_HealthCheckConfig{
					PortValue: 1234,
				},
			},
			ok: true,
		}
	}
	uint32WrapperVariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeAddPatch(
					"/outlier_detection/enforcing_consecutive_5xx",
					i,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{
					EnforcingConsecutive_5Xx: wrapperspb.UInt32(1234),
				},
			},
			ok: true,
		}
	}
	uint64WrapperVariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_cluster_v3.Cluster{
					LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
						RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
							MaximumRingSize: wrapperspb.UInt64(999999999),
						},
					},
				},
				patches: []Patch{makeAddPatch(
					"/ring_hash_lb_config/maximum_ring_size",
					i,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						MaximumRingSize: wrapperspb.UInt64(12345678),
					},
				},
			},
			ok: true,
		}
	}
	doubleVariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_cluster_v3.Cluster{
					LbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
						LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
							ActiveRequestBias: &corev3.RuntimeDouble{
								DefaultValue: 1.0,
							},
						},
					},
				},
				patches: []Patch{makeAddPatch(
					"/least_request_lb_config/active_request_bias/default_value",
					i,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
					LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
						ActiveRequestBias: &corev3.RuntimeDouble{
							DefaultValue: 1.5,
						},
					},
				},
			},
			ok: true,
		}
	}
	doubleWrapperVariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_cluster_v3.Cluster{
					PreconnectPolicy: &envoy_cluster_v3.Cluster_PreconnectPolicy{
						PerUpstreamPreconnectRatio: wrapperspb.Double(1.0),
					},
				},
				patches: []Patch{makeAddPatch(
					"/preconnect_policy/per_upstream_preconnect_ratio",
					i,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				PreconnectPolicy: &envoy_cluster_v3.Cluster_PreconnectPolicy{
					PerUpstreamPreconnectRatio: wrapperspb.Double(1.5),
				},
			},
			ok: true,
		}
	}
	enumByNumberVariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_cluster_v3.Cluster{
					LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
						RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
							HashFunction: envoy_cluster_v3.Cluster_RingHashLbConfig_XX_HASH,
						},
					},
				},
				patches: []Patch{makeAddPatch(
					"/ring_hash_lb_config/hash_function",
					i,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						HashFunction: envoy_cluster_v3.Cluster_RingHashLbConfig_MURMUR_HASH_2,
					},
				},
			},
			ok: true,
		}
	}
	repeatedIntVariantTestCase := func(i any) testCase {
		return testCase{
			args: args{
				k: &envoy_route_v3.RetryPolicy{
					RetriableStatusCodes: []uint32{429, 502},
				},
				patches: []Patch{makeAddPatch(
					"/retriable_status_codes",
					i,
				)},
			},
			expected: &envoy_route_v3.RetryPolicy{
				RetriableStatusCodes: []uint32{503, 504},
			},
			ok: true,
		}
	}

	cases := map[string]testCase{
		// Some variants of target types are covered in conversion code but missing
		// from this table due to lacking available examples in Envoy v3 protos. An
		// improvement could be a home-rolled proto with every possible target type.
		"add single field: int->uint32":             uint32VariantTestCase(int(1234)),
		"add single field: int32->uint32":           uint32VariantTestCase(int32(1234)),
		"add single field: int64->uint32":           uint32VariantTestCase(int64(1234)),
		"add single field: uint->uint32":            uint32VariantTestCase(uint(1234)),
		"add single field: uint32->uint32":          uint32VariantTestCase(uint32(1234)),
		"add single field: uint64->uint32":          uint32VariantTestCase(uint64(1234)),
		"add single field: float32->uint32":         uint32VariantTestCase(float32(1234.0)),
		"add single field: float64->uint32":         uint32VariantTestCase(float64(1234.0)),
		"add single field: int->uint32 wrapper":     uint32WrapperVariantTestCase(int(1234)),
		"add single field: int32->uint32 wrapper":   uint32WrapperVariantTestCase(int32(1234)),
		"add single field: int64->uint32 wrapper":   uint32WrapperVariantTestCase(int64(1234)),
		"add single field: uint->uint32 wrapper":    uint32WrapperVariantTestCase(uint(1234)),
		"add single field: uint32->uint32 wrapper":  uint32WrapperVariantTestCase(uint32(1234)),
		"add single field: uint64->uint32 wrapper":  uint32WrapperVariantTestCase(uint64(1234)),
		"add single field: float32->uint32 wrapper": uint32WrapperVariantTestCase(float32(1234.0)),
		"add single field: float64->uint32 wrapper": uint32WrapperVariantTestCase(float64(1234.0)),
		"add single field: int->uint64 wrapper":     uint64WrapperVariantTestCase(int(12345678)),
		"add single field: int32->uint64 wrapper":   uint64WrapperVariantTestCase(int32(12345678)),
		"add single field: int64->uint64 wrapper":   uint64WrapperVariantTestCase(int64(12345678)),
		"add single field: uint->uint64 wrapper":    uint64WrapperVariantTestCase(uint(12345678)),
		"add single field: uint32->uint64 wrapper":  uint64WrapperVariantTestCase(uint32(12345678)),
		"add single field: uint64->uint64 wrapper":  uint64WrapperVariantTestCase(uint64(12345678)),
		"add single field: float32->uint64 wrapper": uint64WrapperVariantTestCase(float32(12345678.0)),
		"add single field: float64->uint64 wrapper": uint64WrapperVariantTestCase(float64(12345678.0)),
		"add single field: float32->double":         doubleVariantTestCase(float32(1.5)),
		"add single field: float64->double":         doubleVariantTestCase(float64(1.5)),
		"add single field: float32->double wrapper": doubleWrapperVariantTestCase(float32(1.5)),
		"add single field: float64->double wrapper": doubleWrapperVariantTestCase(float64(1.5)),
		"add single field: bool": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					RespectDnsTtl: false,
				},
				patches: []Patch{makeAddPatch(
					"/respect_dns_ttl",
					true,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				RespectDnsTtl: true,
			},
			ok: true,
		},
		"add single field: bool wrapper": {
			args: args{
				k: &envoy_listener_v3.Listener{
					UseOriginalDst: wrapperspb.Bool(false),
				},
				patches: []Patch{makeAddPatch(
					"/use_original_dst",
					true,
				)},
			},
			expected: &envoy_listener_v3.Listener{
				UseOriginalDst: wrapperspb.Bool(true),
			},
			ok: true,
		},
		"add single field: string": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					AltStatName: "foo",
				},
				patches: []Patch{makeAddPatch(
					"/alt_stat_name",
					"bar",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				AltStatName: "bar",
			},
			ok: true,
		},
		"add single field: enum by name": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
						RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
							HashFunction: envoy_cluster_v3.Cluster_RingHashLbConfig_XX_HASH,
						},
					},
				},
				patches: []Patch{makeAddPatch(
					"/ring_hash_lb_config/hash_function",
					"MURMUR_HASH_2",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						HashFunction: envoy_cluster_v3.Cluster_RingHashLbConfig_MURMUR_HASH_2,
					},
				},
			},
			ok: true,
		},
		"add single field: enum by number int":    enumByNumberVariantTestCase(int(1)),
		"add single field: enum by number int32":  enumByNumberVariantTestCase(int32(1)),
		"add single field: enum by number int64":  enumByNumberVariantTestCase(int64(1)),
		"add single field: enum by number uint":   enumByNumberVariantTestCase(uint(1)),
		"add single field: enum by number uint32": enumByNumberVariantTestCase(uint32(1)),
		"add single field: enum by number uint64": enumByNumberVariantTestCase(uint64(1)),
		"add single field previously unmodified": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/alt_stat_name",
					"bar",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				AltStatName: "bar",
			},
			ok: true,
		},
		"add single field deeply nested": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					UpstreamConnectionOptions: &envoy_cluster_v3.UpstreamConnectionOptions{
						TcpKeepalive: &corev3.TcpKeepalive{
							KeepaliveProbes: wrapperspb.UInt32(2),
						},
					},
				},
				patches: []Patch{makeAddPatch(
					"/upstream_connection_options/tcp_keepalive/keepalive_probes",
					5,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				UpstreamConnectionOptions: &envoy_cluster_v3.UpstreamConnectionOptions{
					TcpKeepalive: &corev3.TcpKeepalive{
						KeepaliveProbes: wrapperspb.UInt32(5),
					},
				},
			},
			ok: true,
		},
		"add single field deeply nested with intermediate unset field": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					// Explicitly set to nil just in case defaults change.
					UpstreamConnectionOptions: nil,
				},
				patches: []Patch{makeAddPatch(
					"/upstream_connection_options/tcp_keepalive/keepalive_probes",
					1234,
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				UpstreamConnectionOptions: &envoy_cluster_v3.UpstreamConnectionOptions{
					TcpKeepalive: &corev3.TcpKeepalive{
						KeepaliveProbes: wrapperspb.UInt32(1234),
					},
				},
			},
			ok: true,
		},
		"add repeated field: int->uint32":     repeatedIntVariantTestCase([]int{503, 504}),
		"add repeated field: int32->uint32":   repeatedIntVariantTestCase([]int32{503, 504}),
		"add repeated field: int64->uint32":   repeatedIntVariantTestCase([]int64{503, 504}),
		"add repeated field: uint->uint32":    repeatedIntVariantTestCase([]uint{503, 504}),
		"add repeated field: uint32->uint32":  repeatedIntVariantTestCase([]uint32{503, 504}),
		"add repeated field: uint64->uint32":  repeatedIntVariantTestCase([]uint64{503, 504}),
		"add repeated field: float32->uint32": repeatedIntVariantTestCase([]float32{503.0, 504.0}),
		"add repeated field: float64->uint32": repeatedIntVariantTestCase([]float64{503.0, 504.0}),
		"add repeated field: string": {
			args: args{
				k: &envoy_route_v3.RouteConfiguration{},
				patches: []Patch{makeAddPatch(
					"/internal_only_headers",
					[]string{"X-Custom-Header1", "X-Custom-Header-2"},
				)},
			},
			expected: &envoy_route_v3.RouteConfiguration{
				InternalOnlyHeaders: []string{"X-Custom-Header1", "X-Custom-Header-2"},
			},
			ok: true,
		},
		"add repeated field: enum by name": {
			args: args{
				k: &corev3.HealthStatusSet{
					Statuses: []corev3.HealthStatus{corev3.HealthStatus_DRAINING},
				},
				patches: []Patch{makeAddPatch(
					"/statuses",
					[]string{"HEALTHY", "UNHEALTHY"},
				)},
			},
			expected: &corev3.HealthStatusSet{
				Statuses: []corev3.HealthStatus{corev3.HealthStatus_HEALTHY, corev3.HealthStatus_UNHEALTHY},
			},
			ok: true,
		},
		"add repeated field: enum by number": {
			args: args{
				k: &corev3.HealthStatusSet{
					Statuses: []corev3.HealthStatus{corev3.HealthStatus_DRAINING},
				},
				patches: []Patch{makeAddPatch(
					"/statuses",
					[]int{1, 2},
				)},
			},
			expected: &corev3.HealthStatusSet{
				Statuses: []corev3.HealthStatus{corev3.HealthStatus_HEALTHY, corev3.HealthStatus_UNHEALTHY},
			},
			ok: true,
		},
		"add message field: empty": {
			args: args{
				k: &envoy_listener_v3.Listener{},
				patches: []Patch{makeAddPatch(
					"/connection_balance_config/exact_balance",
					map[string]any{},
				)},
			},
			expected: &envoy_listener_v3.Listener{
				ConnectionBalanceConfig: &envoy_listener_v3.Listener_ConnectionBalanceConfig{
					BalanceType: &envoy_listener_v3.Listener_ConnectionBalanceConfig_ExactBalance_{},
				},
			},
			ok: true,
		},
		"add message field: multiple fields": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx:   wrapperspb.UInt32(9999),
						FailurePercentageThreshold: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeAddPatch(
					"/outlier_detection",
					map[string]any{
						"enforcing_consecutive_5xx":         1234,
						"failure_percentage_request_volume": 2345,
					},
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{
					EnforcingConsecutive_5Xx:       wrapperspb.UInt32(1234),
					FailurePercentageRequestVolume: wrapperspb.UInt32(2345),
				},
			},
			ok: true,
		},
		"add multiple single field patches merge with existing object": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx:   wrapperspb.UInt32(9999),
						FailurePercentageThreshold: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{
					makeAddPatch(
						"/outlier_detection/enforcing_consecutive_5xx",
						1234,
					),
					makeAddPatch(
						"/outlier_detection/failure_percentage_request_volume",
						2345,
					),
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{
					EnforcingConsecutive_5Xx:       wrapperspb.UInt32(1234),
					FailurePercentageRequestVolume: wrapperspb.UInt32(2345), // Previously unspecified field set
					FailurePercentageThreshold:     wrapperspb.UInt32(9999), // Existing unmodified field retained
				},
			},
			ok: true,
		},
		"remove single field: scalar wrapper": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeRemovePatch(
					"/outlier_detection/enforcing_consecutive_5xx",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			},
			ok: true,
		},
		"remove single field: string (reset to empty)": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					AltStatName: "foo",
				},
				patches: []Patch{makeRemovePatch(
					"/alt_stat_name",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{},
			ok:       true,
		},
		"remove single field: bool (reset to false)": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					RespectDnsTtl: true,
				},
				patches: []Patch{makeRemovePatch(
					"/respect_dns_ttl",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				RespectDnsTtl: false,
			},
			ok: true,
		},
		"remove single field: enum (reset to default)": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
						RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
							HashFunction: envoy_cluster_v3.Cluster_RingHashLbConfig_MURMUR_HASH_2,
						},
					},
				},
				patches: []Patch{makeRemovePatch(
					"/ring_hash_lb_config/hash_function",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						HashFunction: envoy_cluster_v3.Cluster_RingHashLbConfig_XX_HASH,
					},
				},
			},
			ok: true,
		},
		"remove single field: message": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx:   wrapperspb.UInt32(9999),
						FailurePercentageThreshold: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeRemovePatch(
					"/outlier_detection",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{},
			ok:       true,
		},
		"remove single field: map": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					Metadata: &corev3.Metadata{
						FilterMetadata: map[string]*_struct.Struct{
							"foo": nil,
						},
					},
				},
				patches: []Patch{makeRemovePatch(
					"/metadata/filter_metadata",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				Metadata: &corev3.Metadata{},
			},
			ok: true,
		},
		"remove single field deeply nested": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					UpstreamConnectionOptions: &envoy_cluster_v3.UpstreamConnectionOptions{
						TcpKeepalive: &corev3.TcpKeepalive{
							KeepaliveProbes: wrapperspb.UInt32(9999),
						},
					},
				},
				patches: []Patch{makeRemovePatch(
					"/upstream_connection_options/tcp_keepalive/keepalive_probes",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{
				UpstreamConnectionOptions: &envoy_cluster_v3.UpstreamConnectionOptions{
					TcpKeepalive: &corev3.TcpKeepalive{},
				},
			},
			ok: true,
		},
		"remove repeated field: message": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					Filters: []*envoy_cluster_v3.Filter{
						{
							Name: "foo",
						},
					},
				},
				patches: []Patch{makeRemovePatch(
					"/filters",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{},
			ok:       true,
		},
		"remove multiple single field patches merge with existing object": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx:   wrapperspb.UInt32(9999),
						FailurePercentageThreshold: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{
					makeRemovePatch(
						"/outlier_detection/enforcing_consecutive_5xx",
					),
					makeRemovePatch(
						"/outlier_detection/failure_percentage_request_volume", // No-op removal
					),
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{
					FailurePercentageThreshold: wrapperspb.UInt32(9999), // Existing unmodified field retained
				},
			},
			ok: true,
		},
		"remove does not instantiate intermediate fields that are unset": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					// Explicitly set to nil just in case defaults change.
					UpstreamConnectionOptions: nil,
				},
				patches: []Patch{makeRemovePatch(
					"/upstream_connection_options/tcp_keepalive/keepalive_probes",
				)},
			},
			expected: &envoy_cluster_v3.Cluster{},
			ok:       true,
		},
		"add and remove multiple single field patches merge with existing object": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						FailurePercentageRequestVolume: wrapperspb.UInt32(9999),
						FailurePercentageThreshold:     wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{
					makeAddPatch(
						"/outlier_detection/enforcing_consecutive_5xx",
						1234,
					),
					makeRemovePatch(
						"/outlier_detection/failure_percentage_request_volume",
					),
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{
					EnforcingConsecutive_5Xx:   wrapperspb.UInt32(1234), // New field added
					FailurePercentageThreshold: wrapperspb.UInt32(9999), // Existing unmodified field retained
				},
			},
			ok: true,
		},
		"add then remove respects order of operations": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{
					makeAddPatch(
						"/outlier_detection/enforcing_consecutive_5xx",
						1234,
					),
					makeRemovePatch(
						"/outlier_detection",
					),
				},
			},
			expected: &envoy_cluster_v3.Cluster{},
			ok:       true,
		},
		"remove then add respects order of operations": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						FailurePercentageRequestVolume: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{
					makeRemovePatch(
						"/outlier_detection",
					),
					makeAddPatch(
						"/outlier_detection/enforcing_consecutive_5xx",
						1234,
					),
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{
					EnforcingConsecutive_5Xx: wrapperspb.UInt32(1234), // New field added
					// Previous field removed by remove op
				},
			},
			ok: true,
		},
		"add invalid value: scalar->scalar type mismatch": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeAddPatch(
					"/outlier_detection/enforcing_consecutive_5xx",
					"NaN",
				)},
			},
			ok:     false,
			errMsg: "patch value type string could not be applied to target field type 'google.protobuf.UInt32Value'",
		},
		"add invalid value: non-scalar->scalar type mismatch": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeAddPatch(
					"/respect_dns_ttl",
					[]string{"bad", "value"},
				)},
			},
			ok:     false,
			errMsg: "patch value type []string could not be applied to target field type 'bool'",
		},
		"add invalid value: scalar->enum type mismatch": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/ring_hash_lb_config/hash_function",
					1.5,
				)},
			},
			ok:     false,
			errMsg: "patch value type float64 could not be applied to target field type 'enum'",
		},
		"add invalid value: nil scalar": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					RespectDnsTtl: false,
				},
				patches: []Patch{makeAddPatch(
					"/respect_dns_ttl",
					nil,
				)},
			},
			ok:     false,
			errMsg: "non-nil Value is required; use an empty map to reset all fields on a message or the 'remove' op to unset fields",
		},
		"add invalid value: nil wrapper": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeAddPatch(
					"/outlier_detection/enforcing_consecutive_5xx",
					nil,
				)},
			},
			ok:     false,
			errMsg: "non-nil Value is required; use an empty map to reset all fields on a message or the 'remove' op to unset fields",
		},
		"add invalid value: nil message": {

			args: args{
				k: &envoy_cluster_v3.Cluster{
					OutlierDetection: &envoy_cluster_v3.OutlierDetection{
						EnforcingConsecutive_5Xx:   wrapperspb.UInt32(9999),
						FailurePercentageThreshold: wrapperspb.UInt32(9999),
					},
				},
				patches: []Patch{makeAddPatch(
					"/outlier_detection",
					nil,
				)},
			},
			ok:     false,
			errMsg: "non-nil Value is required; use an empty map to reset all fields on a message or the 'remove' op to unset fields",
		},
		"add invalid value: mixed type scalar": {
			args: args{
				k: &envoy_route_v3.RouteConfiguration{},
				patches: []Patch{makeAddPatch(
					"/internal_only_headers",
					[]any{"X-Custom-Header1", 123},
				)},
			},
			expected: &envoy_route_v3.RouteConfiguration{
				InternalOnlyHeaders: []string{"X-Custom-Header1", "X-Custom-Header-2"},
			},
			ok:     false,
			errMsg: "patch value type []interface {} could not be applied to target field type 'repeated string'",
		},
		"add unsupported target: message with non-scalar fields": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/dns_failure_refresh_rate",
					map[string]any{
						"base_interval": map[string]any{},
					},
				)},
			},
			ok:     false,
			errMsg: "unsupported target field type 'google.protobuf.Duration'",
		},
		"add unsupported target: map field": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/metadata/filter_metadata",
					map[string]any{
						"foo": "bar",
					},
				)},
			},
			ok:     false,
			errMsg: "unsupported target field type 'map'",
		},
		"add unsupported target: repeated message": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/filters",
					[]any{}, // We don't need a value in this slice to test behavior
				)},
			},
			ok:     false,
			errMsg: "unsupported target field type 'repeated envoy.config.cluster.v3.Filter'",
		},
		"empty path": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"",
					"ignored",
				)},
			},
			ok:      false,
			errMsg:  "non-empty, non-root Path is required",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.Cluster", true),
		},
		"empty path debug mode": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"",
					"ignored",
				)},
				debug: true,
			},
			ok:      false,
			errMsg:  "non-empty, non-root Path is required",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.Cluster", false),
		},
		"root path": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/",
					"ignored",
				)},
			},
			ok:      false,
			errMsg:  "non-empty, non-root Path is required",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.Cluster", true),
		},
		"root path debug mode": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/",
					"ignored",
				)},
				debug: true,
			},
			ok:      false,
			errMsg:  "non-empty, non-root Path is required",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.Cluster", false),
		},
		"invalid path: add unknown field": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/outlier_detection/foo",
					"ignored",
				)},
			},
			ok:      false,
			errMsg:  "no match for field 'foo'!",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.OutlierDetection", true),
		},
		"invalid path: remove unknown field": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeRemovePatch(
					"/outlier_detection/foo",
				)},
			},
			ok:      false,
			errMsg:  "no match for field 'foo'!",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.OutlierDetection", true),
		},
		"invalid path: unknown field debug mode": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/outlier_detection/foo",
					"ignored",
				)},
				debug: true,
			},
			ok:      false,
			errMsg:  "no match for field 'foo'!",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.OutlierDetection", false),
		},
		"error field list includes first 10 fields when not in debug mode": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"",
					"ignored",
				)},
			},
			ok:      false,
			errMsg:  "transport_socket_matches\nname\nalt_stat_name\ntype\ncluster_type\neds_cluster_config\nconnect_timeout\nper_connection_buffer_limit_bytes\nlb_policy\nload_assignment",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.Cluster", true),
		},
		"error field list includes all fields when in debug mode": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"",
					"ignored",
				)},
				debug: true,
			},
			ok:      false,
			errMsg:  "transport_socket_matches\nname\nalt_stat_name\ntype\ncluster_type\neds_cluster_config\nconnect_timeout\nper_connection_buffer_limit_bytes\nlb_policy\nload_assignment\nhealth_checks\nmax_requests_per_connection\ncircuit_breakers\nupstream_http_protocol_options\ncommon_http_protocol_options\nhttp_protocol_options\nhttp2_protocol_options\ntyped_extension_protocol_options\ndns_refresh_rate\ndns_failure_refresh_rate\nrespect_dns_ttl\ndns_lookup_family\ndns_resolvers\nuse_tcp_for_dns_lookups\ndns_resolution_config\ntyped_dns_resolver_config\nwait_for_warm_on_init\noutlier_detection\ncleanup_interval\nupstream_bind_config\nlb_subset_config\nring_hash_lb_config\nmaglev_lb_config\noriginal_dst_lb_config\nleast_request_lb_config\nround_robin_lb_config\ncommon_lb_config\ntransport_socket\nmetadata\nprotocol_selection\nupstream_connection_options\nclose_connections_on_host_health_failure\nignore_health_on_host_removal\nfilters\nload_balancing_policy\nlrs_server\ntrack_timeout_budgets\nupstream_config\ntrack_cluster_stats\npreconnect_policy\nconnection_pool_per_downstream_connection",
			errFunc: expectFieldsListErr("envoy.config.cluster.v3.Cluster", false),
		},
		"error field list warns about first 10 fields only when > 10 available when not in debug mode": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/upstream_connection_options/tcp_keepalive/foo",
					"ignored",
				)},
				debug: false,
			},
			ok:      false,
			errMsg:  "keepalive_probes\nkeepalive_time\nkeepalive_interval",
			errFunc: expectFieldsListErr("envoy.config.core.v3.TcpKeepalive", false),
		},
		"invalid path: empty path element": {
			args: args{
				k: &envoy_cluster_v3.Cluster{},
				patches: []Patch{makeAddPatch(
					"/outlier_detection//",
					"ignored",
				)},
			},
			ok:     false,
			errMsg: "empty field name in path",
		},
		"invalid path: repeated field member": {
			args: args{
				k: &envoy_listener_v3.Listener{},
				patches: []Patch{makeRemovePatch(
					"/filter_chains/0/transport_socket_connect_timeout",
				)},
			},
			ok:     false,
			errMsg: "path contains member of repeated field 'filter_chains'; repeated field member access is not supported",
		},
		"invalid path: map field member": {
			args: args{
				k: &envoy_cluster_v3.Cluster{
					Metadata: &corev3.Metadata{
						FilterMetadata: map[string]*_struct.Struct{
							"foo": nil,
						},
					},
				},
				patches: []Patch{makeRemovePatch(
					"/metadata/filter_metadata/foo",
				)},
			},
			ok:     false,
			errMsg: "path contains member of map field 'filter_metadata'; map field member access is not supported",
		},
	}

	copyMessage := func(m proto.Message) proto.Message { return m }
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			// Copy k so that we can compare before and after
			copyOfK := copyMessage(tc.args.k)
			var err error
			for _, p := range tc.args.patches {
				// Repeatedly patch value, replacing with the new version each time
				copyOfK, err = PatchStruct(copyOfK, p, tc.args.debug)
				if err != nil {
					break // Break on the first error
				}
			}
			if tc.ok {
				require.NoError(t, err)
				if diff := cmp.Diff(tc.expected, copyOfK, protocmp.Transform()); diff != "" {
					t.Errorf("unexpected difference:\n%v", diff)
				}
			} else {
				require.Error(t, err)
				if tc.errMsg != "" {
					require.ErrorContains(t, err, tc.errMsg)
				}
				if tc.errFunc != nil {
					tc.errFunc(t, err)
				}
			}
		})
	}
}
