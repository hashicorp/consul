package xds

import (
	"errors"
	"fmt"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoy_tls_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoy_core_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_listener_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoy_route_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_http_rbac_v2 "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rbac/v2"
	envoy_http_v2 "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	envoy_network_rbac_v2 "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rbac/v2"
	envoy_tcp_proxy_v2 "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_trace_v2 "github.com/envoyproxy/go-control-plane/envoy/config/trace/v2"
	envoy_http_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_network_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_discovery_v2 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc"
)

type adsServerV2Shim struct {
	srv *Server
}

// StreamAggregatedResources implements
// envoy_discovery_v2.AggregatedDiscoveryServiceServer. This is the ADS endpoint which is
// the only xDS API we directly support for now.
func (s *adsServerV2Shim) StreamAggregatedResources(stream ADSStream_v2) error {
	shim := &adsStreamV3Shim{
		stream:       stream,
		ServerStream: stream,
	}
	return s.srv.StreamAggregatedResources(shim)
}

// DeltaAggregatedResources implements envoy_discovery_v2.AggregatedDiscoveryServiceServer
func (s *adsServerV2Shim) DeltaAggregatedResources(_ envoy_discovery_v2.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return errors.New("not implemented")
}

type adsStreamV3Shim struct {
	stream ADSStream_v2
	grpc.ServerStream
}

var _ ADSStream = (*adsStreamV3Shim)(nil)

func (s *adsStreamV3Shim) Send(resp *envoy_discovery_v3.DiscoveryResponse) error {
	respv2, err := convertDiscoveryResponseToV2(resp)
	if err != nil {
		return fmt.Errorf("Error converting a v3 DiscoveryResponse to v2: %w", err)
	}

	return s.stream.Send(respv2)
}

func (s *adsStreamV3Shim) Recv() (*envoy_discovery_v3.DiscoveryRequest, error) {
	req, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}

	reqv3, err := convertDiscoveryRequestToV3(req)
	if err != nil {
		return nil, fmt.Errorf("Error converting a v2 DiscoveryRequest to v3: %w", err)
	}

	return reqv3, nil
}

func convertDiscoveryRequestToV3(req *envoy_api_v2.DiscoveryRequest) (*envoy_discovery_v3.DiscoveryRequest, error) {
	// TODO: improve this
	b, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	var reqV3 envoy_discovery_v3.DiscoveryRequest
	if err := proto.Unmarshal(b, &reqV3); err != nil {
		return nil, err
	}

	// only one field to munge
	if err := convertTypeUrlsToV3(&reqV3.TypeUrl); err != nil {
		return nil, err
	}

	return &reqV3, nil
}

func convertDiscoveryResponseToV2(resp *envoy_discovery_v3.DiscoveryResponse) (*envoy_api_v2.DiscoveryResponse, error) {
	// TODO: improve this
	b, err := proto.Marshal(resp)
	if err != nil {
		return nil, err
	}

	var respV2 envoy_api_v2.DiscoveryResponse
	if err := proto.Unmarshal(b, &respV2); err != nil {
		return nil, err
	}

	if err := convertTypedConfigsToV2(&respV2); err != nil {
		return nil, err
	}

	return &respV2, nil
}

func convertNetFilterToV2(filter *envoy_listener_v3.Filter) (*envoy_listener_v2.Filter, error) {
	// TODO: improve this
	b, err := proto.Marshal(filter)
	if err != nil {
		return nil, err
	}

	var filterV2 envoy_listener_v2.Filter
	if err := proto.Unmarshal(b, &filterV2); err != nil {
		return nil, err
	}

	if err := convertTypedConfigsToV2(&filterV2); err != nil {
		return nil, err
	}

	return &filterV2, nil
}

func convertHttpFilterToV2(filter *envoy_http_v3.HttpFilter) (*envoy_http_v2.HttpFilter, error) {
	// TODO: improve this
	b, err := proto.Marshal(filter)
	if err != nil {
		return nil, err
	}

	var filterV2 envoy_http_v2.HttpFilter
	if err := proto.Unmarshal(b, &filterV2); err != nil {
		return nil, err
	}

	if err := convertTypedConfigsToV2(&filterV2); err != nil {
		return nil, err
	}

	return &filterV2, nil
}

// Responses
func convertTypedConfigsToV2(pb proto.Message) error {
	// if true {
	// 	return nil
	// }
	// TODO: api/resource version downgrades
	// TODO: config sources and xDS things
	switch x := pb.(type) {
	case *envoy_api_v2.DiscoveryResponse:
		if err := convertTypeUrlsToV2(&x.TypeUrl); err != nil {
			return fmt.Errorf("%T: %w", x, err)
		}
		for _, res := range x.Resources {
			if err := convertTypedConfigsToV2(res); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *any.Any:
		// first flip the any.Any to v2
		if err := convertTypeUrlsToV2(&x.TypeUrl); err != nil {
			return fmt.Errorf("%T(%s) convert type urls in envelope: %w", x, x.TypeUrl, err)
		}

		// now decode into a v2 type
		var dynAny ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(x, &dynAny); err != nil {
			return fmt.Errorf("%T(%s) dynamic unmarshal: %w", x, x.TypeUrl, err)
		}

		// handle the contents and then put them back in the any.Any
		// handle contents first
		if err := convertTypedConfigsToV2(dynAny.Message); err != nil {
			return fmt.Errorf("%T(%s) convert type urls in body: %w", x, x.TypeUrl, err)
		}
		anyFixed, err := ptypes.MarshalAny(dynAny.Message)
		if err != nil {
			return fmt.Errorf("%T(%s) dynamic re-marshal: %w", x, x.TypeUrl, err)
		}
		x.Value = anyFixed.Value
		return nil
	case *envoy_api_v2.Listener:
		for _, chain := range x.FilterChains {
			if err := convertTypedConfigsToV2(chain); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		for _, filter := range x.ListenerFilters {
			if err := convertTypedConfigsToV2(filter); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		if x.UdpListenerConfig != nil {
			if err := convertTypedConfigsToV2(x.UdpListenerConfig); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_listener_v2.ListenerFilter:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_listener_v2.ListenerFilter_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_listener_v2.UdpListenerConfig:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_listener_v2.UdpListenerConfig_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_listener_v2.FilterChain:
		for _, filter := range x.Filters {
			if err := convertTypedConfigsToV2(filter); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		if x.TransportSocket != nil {
			if err := convertTypedConfigsToV2(x.TransportSocket); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_listener_v2.Filter:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_listener_v2.Filter_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_core_v2.TransportSocket:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_core_v2.TransportSocket_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_api_v2.ClusterLoadAssignment:
		return nil
	case *envoy_api_v2.Cluster:
		if x.TransportSocket != nil {
			if err := convertTypedConfigsToV2(x.TransportSocket); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		for _, tsm := range x.TransportSocketMatches {
			if tsm.TransportSocket != nil {
				if err := convertTypedConfigsToV2(tsm.TransportSocket); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		if x.EdsClusterConfig != nil {
			if x.EdsClusterConfig.EdsConfig != nil {
				if err := convertTypedConfigsToV2(x.EdsClusterConfig.EdsConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		if x.LrsServer != nil { // TODO: NOPE
			if err := convertTypedConfigsToV2(x.LrsServer); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_api_v2.RouteConfiguration:
		for _, vhost := range x.VirtualHosts {
			if err := convertTypedConfigsToV2(vhost); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		if x.Vhds != nil && x.Vhds.ConfigSource != nil {
			if err := convertTypedConfigsToV2(x.Vhds.ConfigSource); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_route_v2.VirtualHost:
		if x.RetryPolicy != nil {
			if err := convertTypedConfigsToV2(x.RetryPolicy); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_route_v2.RetryPolicy:
		if x.RetryPriority != nil {
			if err := convertTypedConfigsToV2(x.RetryPriority); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		for _, pred := range x.RetryHostPredicate {
			if err := convertTypedConfigsToV2(pred); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_route_v2.RetryPolicy_RetryPriority:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_route_v2.RetryPolicy_RetryPriority_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				// TODO: add some of these?
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_route_v2.RetryPolicy_RetryHostPredicate:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_route_v2.RetryPolicy_RetryHostPredicate_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				// TODO: add some of these?
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_http_v2.HttpFilter:
		if x.ConfigType != nil {
			tc, ok := x.ConfigType.(*envoy_http_v2.HttpFilter_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: ConfigType type %T not handled", x, x.ConfigType)
			}
			if tc.TypedConfig != nil {
				if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
					return fmt.Errorf("%T: %w", x, err)
				}
			}
		}
		return nil
	case *envoy_core_v2.ConfigSource:
		if x.ConfigSourceSpecifier != nil {
			if _, ok := x.ConfigSourceSpecifier.(*envoy_core_v2.ConfigSource_Ads); !ok {
				return fmt.Errorf("%T: ConfigSourceSpecifier type %T not handled", x, x.ConfigSourceSpecifier)
			}
		}
		x.ResourceApiVersion = envoy_core_v2.ApiVersion_V2
		return nil
	case *envoy_http_v2.HttpConnectionManager:
		if x.RouteSpecifier != nil {
			switch spec := x.RouteSpecifier.(type) {
			case *envoy_http_v2.HttpConnectionManager_Rds:
				if spec.Rds != nil && spec.Rds.ConfigSource != nil {
					if err := convertTypedConfigsToV2(spec.Rds.ConfigSource); err != nil {
						return fmt.Errorf("%T: %w", x, err)
					}
				}
			case *envoy_http_v2.HttpConnectionManager_RouteConfig:
				if spec.RouteConfig != nil {
					if err := convertTypedConfigsToV2(spec.RouteConfig); err != nil {
						return fmt.Errorf("%T: %w", x, err)
					}
				}
			default:
				return fmt.Errorf("%T: RouteSpecifier type %T not handled", x, spec)
			}
		}
		for _, filter := range x.HttpFilters {
			if err := convertTypedConfigsToV2(filter); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		if x.Tracing != nil && x.Tracing.Provider != nil && x.Tracing.Provider.ConfigType != nil {
			tc, ok := x.Tracing.Provider.ConfigType.(*envoy_trace_v2.Tracing_Http_TypedConfig)
			if !ok {
				return fmt.Errorf("%T: Tracing.Provider.ConfigType type %T not handled", x, x.Tracing.Provider.ConfigType)
			}
			if err := convertTypedConfigsToV2(tc.TypedConfig); err != nil {
				return fmt.Errorf("%T: %w", x, err)
			}
		}
		return nil
	case *envoy_tcp_proxy_v2.TcpProxy:
		return nil
	case *envoy_network_rbac_v2.RBAC:
		return nil
	case *envoy_http_rbac_v2.RBAC:
		return nil
	case *envoy_tls_v2.UpstreamTlsContext:
		return nil
	case *envoy_tls_v2.DownstreamTlsContext:
		return nil
	default:
		return fmt.Errorf("could not convert unexpected type to v2: %T", pb)
	}
}

func convertTypeUrlsToV2(typeUrl *string) error {
	if _, ok := typeConvert2to3[*typeUrl]; ok {
		return nil // already happened
	}

	converted, ok := typeConvert3to2[*typeUrl]
	if !ok {
		return fmt.Errorf("could not convert type url to v2: %s", *typeUrl)
	}
	*typeUrl = converted
	return nil
}

func convertTypeUrlsToV3(typeUrl *string) error {
	if _, ok := typeConvert3to2[*typeUrl]; ok {
		return nil // already happened
	}

	converted, ok := typeConvert2to3[*typeUrl]
	if !ok {
		return fmt.Errorf("could not convert type url to v3: %s", *typeUrl)
	}
	*typeUrl = converted
	return nil
}

var (
	typeConvert2to3 map[string]string
	typeConvert3to2 map[string]string
)

func init() {
	typeConvert2to3 = make(map[string]string)
	typeConvert3to2 = make(map[string]string)

	reg := func(type2, type3 string) {
		if type2 == "" {
			panic("v2 type is empty")
		}
		if type3 == "" {
			panic("v3 type is empty")
		}
		typeConvert2to3[type2] = type3
		typeConvert3to2[type3] = type2
	}
	reg2 := func(pb2, pb3 proto.Message) {
		any2, err := ptypes.MarshalAny(pb2)
		if err != nil {
			panic(err)
		}
		any3, err := ptypes.MarshalAny(pb3)
		if err != nil {
			panic(err)
		}

		reg(any2.TypeUrl, any3.TypeUrl)
	}

	// primary resources
	reg2(&envoy_api_v2.Listener{}, &envoy_listener_v3.Listener{})                           // LDS
	reg2(&envoy_api_v2.Cluster{}, &envoy_cluster_v3.Cluster{})                              // CDS
	reg2(&envoy_api_v2.RouteConfiguration{}, &envoy_route_v3.RouteConfiguration{})          // RDS
	reg2(&envoy_api_v2.ClusterLoadAssignment{}, &envoy_endpoint_v3.ClusterLoadAssignment{}) // EDS

	// filters
	reg2(&envoy_http_v2.HttpConnectionManager{}, &envoy_http_v3.HttpConnectionManager{}) // "envoy.filters.network.http_connection_manager"
	reg2(&envoy_tcp_proxy_v2.TcpProxy{}, &envoy_tcp_proxy_v3.TcpProxy{})                 // "envoy.filters.network.tcp_proxy"
	reg2(&envoy_network_rbac_v2.RBAC{}, &envoy_network_rbac_v3.RBAC{})                   // "envoy.filters.network.rbac"
	reg2(&envoy_http_rbac_v2.RBAC{}, &envoy_http_rbac_v3.RBAC{})                         // "envoy.filters.http.rbac

	// cluster tls
	reg2(&envoy_tls_v2.UpstreamTlsContext{}, &envoy_tls_v3.UpstreamTlsContext{})
	reg2(&envoy_tls_v2.DownstreamTlsContext{}, &envoy_tls_v3.DownstreamTlsContext{})

	// extension elements
	reg("type.googleapis.com/envoy.config.metrics.v2.DogStatsdSink",
		"type.googleapis.com/envoy.config.metrics.v3.DogStatsdSink")
	reg("type.googleapis.com/envoy.config.metrics.v2.StatsdSink",
		"type.googleapis.com/envoy.config.metrics.v3.StatsdSink")
	reg("type.googleapis.com/envoy.config.trace.v2.ZipkinConfig",
		"type.googleapis.com/envoy.config.trace.v3.ZipkinConfig")
}
