package serverlessplugin

import (
	"errors"
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_lambda_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/aws_lambda/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	pstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/api"
)

type lambdaPatcher struct {
	ARN                string `mapstructure:"ARN"`
	PayloadPassthrough bool   `mapstructure:"PayloadPassthrough"`
	Region             string `mapstructure:"Region"`
	Kind               api.ServiceKind
	InvocationMode     string `mapstructure:"InvocationMode"`
}

var _ patcher = (*lambdaPatcher)(nil)

func makeLambdaPatcher(ext api.EnvoyExtension, upstreamKind api.ServiceKind) (patcher, bool) {
	var patcher lambdaPatcher

	if ext.Name != structs.BuiltinAWSLambdaExtension {
		return nil, false
	}

	// TODO this blows up if types aren't encode properly. We need to check this earlier in the Validate RPC.
	err := mapstructure.Decode(ext.Arguments, &patcher)
	if err != nil {
		return nil, false
	}

	if patcher.ARN == "" {
		return nil, false
	}

	if patcher.Region == "" {
		return nil, false
	}

	patcher.Kind = upstreamKind

	return patcher, true
}

func toEnvoyInvocationMode(s string) envoy_lambda_v3.Config_InvocationMode {
	m := envoy_lambda_v3.Config_SYNCHRONOUS
	if s == "asynchronous" {
		m = envoy_lambda_v3.Config_ASYNCHRONOUS
	}
	return m
}

func (p lambdaPatcher) CanPatch(kind api.ServiceKind) bool {
	return kind == p.Kind
}

func (p lambdaPatcher) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	if p.Kind != api.ServiceKindTerminatingGateway {
		return route, false, nil
	}

	for _, virtualHost := range route.VirtualHosts {
		for _, route := range virtualHost.Routes {
			action, ok := route.Action.(*envoy_route_v3.Route_Route)

			if !ok {
				continue
			}

			// When auto_host_rewrite is set it conflicts with strip_any_host_port
			// on the http_connection_manager filter.
			action.Route.HostRewriteSpecifier = nil
		}
	}

	return route, true, nil
}

func (p lambdaPatcher) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	transportSocket, err := makeUpstreamTLSTransportSocket(&envoy_tls_v3.UpstreamTlsContext{
		Sni: "*.amazonaws.com",
	})

	if err != nil {
		return c, false, fmt.Errorf("failed to make transport socket: %w", err)
	}

	cluster := &envoy_cluster_v3.Cluster{
		Name:                 c.Name,
		ConnectTimeout:       c.ConnectTimeout,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_LOGICAL_DNS},
		DnsLookupFamily:      envoy_cluster_v3.Cluster_V4_ONLY,
		LbPolicy:             envoy_cluster_v3.Cluster_ROUND_ROBIN,
		Metadata: &envoy_core_v3.Metadata{
			FilterMetadata: map[string]*pstruct.Struct{
				"com.amazonaws.lambda": {
					Fields: map[string]*pstruct.Value{
						"egress_gateway": {Kind: &pstruct.Value_BoolValue{BoolValue: true}},
					},
				},
			},
		},
		LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: c.Name,
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: &envoy_core_v3.Address{
										Address: &envoy_core_v3.Address_SocketAddress{
											SocketAddress: &envoy_core_v3.SocketAddress{
												Address: fmt.Sprintf("lambda.%s.amazonaws.com", p.Region),
												PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
													PortValue: 443,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		TransportSocket: transportSocket,
	}
	return cluster, true, nil
}

func (p lambdaPatcher) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	if filter.Name != "envoy.filters.network.http_connection_manager" {
		return filter, false, nil
	}
	if typedConfig := filter.GetTypedConfig(); typedConfig == nil {
		return filter, false, errors.New("error getting typed config for http filter")
	}

	config := envoy_resource_v3.GetHTTPConnectionManager(filter)
	if config == nil {
		return filter, false, errors.New("error unmarshalling filter")
	}
	lambdaHttpFilter, err := makeEnvoyHTTPFilter(
		"envoy.filters.http.aws_lambda",
		&envoy_lambda_v3.Config{
			Arn:                p.ARN,
			PayloadPassthrough: p.PayloadPassthrough,
			InvocationMode:     toEnvoyInvocationMode(p.InvocationMode),
		},
	)
	if err != nil {
		return filter, false, err
	}

	var (
		changedFilters = make([]*envoy_http_v3.HttpFilter, 0, len(config.HttpFilters)+1)
		changed        bool
	)

	// We need to be careful about overwriting http filters completely because
	// http filters validates intentions with the RBAC filter. This inserts the
	// lambda filter before `envoy.filters.http.router` while keeping everything
	// else intact.
	for _, httpFilter := range config.HttpFilters {
		if httpFilter.Name == "envoy.filters.http.router" {
			changedFilters = append(changedFilters, lambdaHttpFilter)
			changed = true
		}
		changedFilters = append(changedFilters, httpFilter)
	}
	if changed {
		config.HttpFilters = changedFilters
	}

	config.StripPortMode = &envoy_http_v3.HttpConnectionManager_StripAnyHostPort{
		StripAnyHostPort: true,
	}
	newFilter, err := makeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}
