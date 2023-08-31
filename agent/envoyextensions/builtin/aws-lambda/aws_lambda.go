package awslambda

import (
	"errors"
	"fmt"

	arn_sdk "github.com/aws/aws-sdk-go/aws/arn"
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

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
)

var _ extensioncommon.BasicExtension = (*awsLambda)(nil)

type awsLambda struct {
	ARN                string
	PayloadPassthrough bool
	InvocationMode     string
}

// Constructor follows a specific function signature required for the extension registration.
func Constructor(ext api.EnvoyExtension) (extensioncommon.EnvoyExtender, error) {
	var a awsLambda
	if name := ext.Name; name != api.BuiltinAWSLambdaExtension {
		return nil, fmt.Errorf("expected extension name %q but got %q", api.BuiltinAWSLambdaExtension, name)
	}
	if err := a.fromArguments(ext.Arguments); err != nil {
		return nil, err
	}
	return &extensioncommon.UpstreamEnvoyExtender{
		Extension: &a,
	}, nil
}

func (a *awsLambda) fromArguments(args map[string]interface{}) error {
	if err := mapstructure.Decode(args, a); err != nil {
		return fmt.Errorf("error decoding extension arguments: %v", err)
	}
	return a.validate()
}

func (a *awsLambda) validate() error {
	var resultErr error
	if a.ARN == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("ARN is required"))
	}
	return resultErr
}

// CanApply returns true if the kind of the provided ExtensionConfiguration matches
// the kind of the lambda configuration
func (a *awsLambda) CanApply(config *extensioncommon.RuntimeConfig) bool {
	return config.Kind == config.UpstreamOutgoingProxyKind()
}

// PatchRoute modifies the routing configuration for a service of kind TerminatingGateway. If the kind is
// not TerminatingGateway, then it can not be modified.
func (a *awsLambda) PatchRoute(r *extensioncommon.RuntimeConfig, route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	if r.Kind != api.ServiceKindTerminatingGateway {
		return route, false, nil
	}

	// Only patch outbound routes.
	if extensioncommon.IsRouteToLocalAppCluster(route) {
		return route, false, nil
	}

	for _, virtualHost := range route.VirtualHosts {
		for _, route := range virtualHost.Routes {
			action, ok := route.Action.(*envoy_route_v3.Route_Route)

			if !ok {
				continue
			}

			// When auto_host_rewrite is set it conflicts with strip_any_host_port
			// on the http_connection_manager filter, which is required to be true to support
			// lambda functions. See the patch filter method for more details.
			action.Route.HostRewriteSpecifier = nil
		}
	}

	return route, true, nil
}

// PatchCluster patches the provided envoy cluster with data required to support an AWS lambda function
func (a *awsLambda) PatchCluster(_ *extensioncommon.RuntimeConfig, c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	// Only patch outbound clusters.
	if extensioncommon.IsLocalAppCluster(c) {
		return c, false, nil
	}

	transportSocket, err := makeUpstreamTLSTransportSocket(&envoy_tls_v3.UpstreamTlsContext{
		Sni: "*.amazonaws.com",
	})

	if err != nil {
		return c, false, fmt.Errorf("failed to make transport socket: %w", err)
	}

	// Use the aws SDK to parse the ARN so that we can later extract the region
	parsedARN, err := arn_sdk.Parse(a.ARN)
	if err != nil {
		return c, false, err
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
												Address: fmt.Sprintf("lambda.%s.amazonaws.com", parsedARN.Region),
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

// PatchFilter patches the provided envoy filter with an inserted lambda filter being careful not to
// overwrite the http filters.
func (a *awsLambda) PatchFilter(_ *extensioncommon.RuntimeConfig, filter *envoy_listener_v3.Filter, isInboundListener bool) (*envoy_listener_v3.Filter, bool, error) {
	// Only patch outbound filters.
	if isInboundListener {
		return filter, false, nil
	}

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
			Arn:                a.ARN,
			PayloadPassthrough: a.PayloadPassthrough,
			InvocationMode:     toEnvoyInvocationMode(a.InvocationMode),
		},
	)
	if err != nil {
		return filter, false, err
	}

	// We need to be careful about overwriting http filters completely because
	// http filters validates intentions with the RBAC filter. This inserts the
	// lambda filter before `envoy.filters.http.router` while keeping everything
	// else intact.
	changedFilters := make([]*envoy_http_v3.HttpFilter, 0, len(config.HttpFilters)+1)
	var changed bool
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

	// StripPortMode must be set to true since all requests have to be signed using the AWS v4 signature and
	// if the port is included in the request, it will be used in the signature calculation causing AWS to reject the
	// Lambda HTTP request.
	config.StripPortMode = &envoy_http_v3.HttpConnectionManager_StripAnyHostPort{
		StripAnyHostPort: true,
	}
	newFilter, err := makeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}

func toEnvoyInvocationMode(s string) envoy_lambda_v3.Config_InvocationMode {
	m := envoy_lambda_v3.Config_SYNCHRONOUS
	if s == "asynchronous" {
		m = envoy_lambda_v3.Config_ASYNCHRONOUS
	}
	return m
}
