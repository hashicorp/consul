package serverlessplugin

import (
	"errors"
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_lambda_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/aws_lambda/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	pstruct "github.com/golang/protobuf/ptypes/struct"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

const (
	lambdaPrefix                string = "serverless.consul.hashicorp.com/v1alpha1"
	lambdaEnabledTag            string = lambdaPrefix + "/lambda/enabled"
	lambdaArnTag                string = lambdaPrefix + "/lambda/arn"
	lambdaPayloadPassthroughTag string = lambdaPrefix + "/lambda/payload-passhthrough"
	lambdaRegionTag             string = lambdaPrefix + "/lambda/region"
	lambdaInvocationMode        string = lambdaPrefix + "/lambda/invocation-mode"
)

type lambdaPatcher struct {
	arn                string
	payloadPassthrough bool
	region             string
	kind               api.ServiceKind
	invocationMode     envoy_lambda_v3.Config_InvocationMode
}

var _ patcher = (*lambdaPatcher)(nil)

func makeLambdaPatcher(serviceConfig xdscommon.ServiceConfig) (patcher, bool) {
	var patcher lambdaPatcher
	if !isStringTrue(serviceConfig.Meta[lambdaEnabledTag]) {
		return patcher, true
	}

	arn := serviceConfig.Meta[lambdaArnTag]
	if arn == "" {
		return patcher, false
	}

	region := serviceConfig.Meta[lambdaRegionTag]
	if region == "" {
		return patcher, false
	}

	payloadPassthrough := isStringTrue(serviceConfig.Meta[lambdaPayloadPassthroughTag])

	invocationModeStr := serviceConfig.Meta[lambdaInvocationMode]
	invocationMode := envoy_lambda_v3.Config_SYNCHRONOUS
	if invocationModeStr == "asynchronous" {
		invocationMode = envoy_lambda_v3.Config_ASYNCHRONOUS
	}

	return lambdaPatcher{
		arn:                arn,
		payloadPassthrough: payloadPassthrough,
		region:             region,
		kind:               serviceConfig.Kind,
		invocationMode:     invocationMode,
	}, true
}

func isStringTrue(v string) bool {
	return v == "true"
}

func (p lambdaPatcher) CanPatch(kind api.ServiceKind) bool {
	return kind == p.kind
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
												Address: fmt.Sprintf("lambda.%s.amazonaws.com", p.region),
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
	httpFilter, err := makeEnvoyHTTPFilter(
		"envoy.filters.http.aws_lambda",
		&envoy_lambda_v3.Config{
			Arn:                p.arn,
			PayloadPassthrough: p.payloadPassthrough,
			InvocationMode:     p.invocationMode,
		},
	)
	if err != nil {
		return filter, false, err
	}

	config.HttpFilters = []*envoy_http_v3.HttpFilter{
		httpFilter,
		{Name: "envoy.filters.http.router"},
	}
	config.StripMatchingHostPort = true
	newFilter, err := makeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}
