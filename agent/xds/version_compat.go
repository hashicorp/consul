package xds

import (
	"errors"
	"fmt"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoy_discovery_v2 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/golang/protobuf/proto"
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

	var respv2 envoy_api_v2.DiscoveryResponse
	if err := proto.Unmarshal(b, &respv2); err != nil {
		return nil, err
	}

	if err := convertTypedConfigsToV2(&respv2); err != nil {
		return nil, err
	}

	return &respv2, nil
}

// Responses
func convertTypedConfigsToV2(pb proto.Message) error {
	switch x := pb.(type) {
	case *envoy_api_v2.DiscoveryResponse:
		if err := convertTypeUrlsToV2(&x.TypeUrl); err != nil {
			return err
		}
		for _, res := range x.Resources {
			if err := convertTypedConfigsToV2(res); err != nil {
				return err
			}
		}
		return nil
	case *any.Any:
		if err := convertTypeUrlsToV2(&x.TypeUrl); err != nil {
			return err
		}
		// TODO: also dig down one layer
		return nil
	default:
		return fmt.Errorf("could not convert unexpected type to v2: %T", pb)
	}
}

func convertTypeUrlsToV2(typeUrl *string) error {
	converted, ok := typeConvert3to2[*typeUrl]
	if !ok {
		return fmt.Errorf("could not convert type url to v2: %s", *typeUrl)
	}
	*typeUrl = converted
	return nil
}

func convertTypeUrlsToV3(typeUrl *string) error {
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
		typeConvert2to3[type2] = type3
		typeConvert3to2[type3] = type2
	}

	// primary resources
	reg("type.googleapis.com/envoy.api.v2.Cluster",
		"type.googleapis.com/envoy.config.cluster.v3.Cluster") // CDS
	reg("type.googleapis.com/envoy.api.v2.Listener",
		"type.googleapis.com/envoy.config.listener.v3.Listener") // LDS
	reg("type.googleapis.com/envoy.api.v2.RouteConfiguration",
		"type.googleapis.com/envoy.config.route.v3.RouteConfiguration") // RDS
	reg("type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
		"type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment") // EDS

	// net filters
	reg("type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager",
		"type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager") // "envoy.filters.network.http_connection_manager"
	reg("type.googleapis.com/envoy.config.filter.network.tcp_proxy.v2.TcpProxy",
		"type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy") // "envoy.filters.network.tcp_proxy"
	reg("type.googleapis.com/envoy.config.filter.network.rbac.v2.RBAC",
		"type.googleapis.com/envoy.extensions.filters.network.rbac.v3.RBAC") // "envoy.filters.network.rbac"

	// http filters
	reg("type.googleapis.com/envoy.config.filter.http.rbac.v2.RBAC",
		"type.googleapis.com/envoy.extensions.filters.http.rbac.v3.RBAC") // "envoy.filters.http.rbac

	// cluster tls
	reg("type.googleapis.com/envoy.api.v2.auth.UpstreamTlsContext",
		"type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext")
	reg("type.googleapis.com/envoy.api.v2.auth.DownstreamTlsContext",
		"type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext")

	// extension elements
	reg("type.googleapis.com/envoy.config.metrics.v2.DogStatsdSink",
		"type.googleapis.com/envoy.config.metrics.v3.DogStatsdSink")
	reg("type.googleapis.com/envoy.config.metrics.v2.StatsdSink",
		"type.googleapis.com/envoy.config.metrics.v3.StatsdSink")
	reg("type.googleapis.com/envoy.config.trace.v2.ZipkinConfig",
		"type.googleapis.com/envoy.config.trace.v3.ZipkinConfig")

}
