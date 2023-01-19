package lambda

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
	"github.com/mitchellh/mapstructure"
	pstruct "google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
)

type lambda struct {
	ARN                string
	PayloadPassthrough bool
	Region             string
	Kind               api.ServiceKind
	InvocationMode     string
}

var _ builtinextensiontemplate.Plugin = (*lambda)(nil)

// MakeLambdaExtension is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeLambdaExtension(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin lambda

	if name := ext.EnvoyExtension.Name; name != api.BuiltinAWSLambdaExtension {
		return nil, fmt.Errorf("expected extension name 'lambda' but got %q", name)
	}

	if err := mapstructure.Decode(ext.EnvoyExtension.Arguments, &plugin); err != nil {
		return nil, fmt.Errorf("error decoding extension arguments: %v", err)
	}

	if plugin.ARN == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("ARN is required"))
	}

	if plugin.Region == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("Region is required"))
	}

	plugin.Kind = ext.OutgoingProxyKind()

	return plugin, resultErr
}

func toEnvoyInvocationMode(s string) envoy_lambda_v3.Config_InvocationMode {
	m := envoy_lambda_v3.Config_SYNCHRONOUS
	if s == "asynchronous" {
		m = envoy_lambda_v3.Config_ASYNCHRONOUS
	}
	return m
}

func (p lambda) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return config.Kind == p.Kind
}

func (p lambda) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
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

func (p lambda) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
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

func (p lambda) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
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
