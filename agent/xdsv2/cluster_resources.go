// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"errors"
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_aggregate_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	envoy_upstreams_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func (pr *ProxyResources) doesEnvoyClusterAlreadyExist(name string) bool {
	// TODO(proxystate): consider using a map instead of [] for this kind of lookup
	for _, envoyCluster := range pr.envoyResources[xdscommon.ClusterType] {
		if envoyCluster.(*envoy_cluster_v3.Cluster).Name == name {
			return true
		}
	}
	return false
}

func (pr *ProxyResources) makeXDSClusters() ([]proto.Message, error) {
	clusters := make([]proto.Message, 0)

	for clusterName := range pr.proxyState.Clusters {
		protoCluster, err := pr.makeClusters(clusterName)
		// TODO: aggregate errors for clusters and still return any properly formed clusters.
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, protoCluster...)
	}

	return clusters, nil
}

func (pr *ProxyResources) makeClusters(name string) ([]proto.Message, error) {
	clusters := make([]proto.Message, 0)
	proxyStateCluster, ok := pr.proxyState.Clusters[name]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", name)
	}

	if pr.doesEnvoyClusterAlreadyExist(name) {
		// don't error
		return []proto.Message{}, nil
	}

	switch proxyStateCluster.Group.(type) {
	case *pbproxystate.Cluster_FailoverGroup:
		fg := proxyStateCluster.GetFailoverGroup()
		clusters, err := pr.makeEnvoyAggregateCluster(name, proxyStateCluster.Protocol, fg)
		if err != nil {
			return nil, err
		}
		for _, c := range clusters {
			clusters = append(clusters, c)
		}

	case *pbproxystate.Cluster_EndpointGroup:
		eg := proxyStateCluster.GetEndpointGroup()
		cluster, err := pr.makeEnvoyCluster(name, proxyStateCluster.Protocol, eg)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)

	default:
		return nil, errors.New("cluster group type should be Endpoint Group or Failover Group")
	}
	return clusters, nil
}

func (pr *ProxyResources) makeEnvoyCluster(name string, protocol string, eg *pbproxystate.EndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	if eg != nil {
		switch t := eg.Group.(type) {
		case *pbproxystate.EndpointGroup_Dynamic:
			dynamic := eg.GetDynamic()
			return pr.makeEnvoyDynamicCluster(name, protocol, dynamic)
		case *pbproxystate.EndpointGroup_Static:
			static := eg.GetStatic()
			return pr.makeEnvoyStaticCluster(name, protocol, static)
		case *pbproxystate.EndpointGroup_Dns:
			dns := eg.GetDns()
			return pr.makeEnvoyDnsCluster(name, protocol, dns)
		case *pbproxystate.EndpointGroup_Passthrough:
			passthrough := eg.GetPassthrough()
			return pr.makeEnvoyPassthroughCluster(name, protocol, passthrough)
		default:
			return nil, fmt.Errorf("unsupported endpoint group type: %s", t)
		}
	}
	return nil, fmt.Errorf("no endpoint group")
}

func (pr *ProxyResources) makeEnvoyDynamicCluster(name string, protocol string, dynamic *pbproxystate.DynamicEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	cluster := &envoy_cluster_v3.Cluster{
		Name:                 name,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS},
		EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
			EdsConfig: &envoy_core_v3.ConfigSource{
				ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
				ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
					Ads: &envoy_core_v3.AggregatedConfigSource{},
				},
			},
		},
	}
	err := addHttpProtocolOptions(protocol, cluster)
	if err != nil {
		return nil, err
	}
	if dynamic.Config != nil {
		if dynamic.Config.UseAltStatName {
			cluster.AltStatName = name
		}
		cluster.ConnectTimeout = dynamic.Config.ConnectTimeout
		if !dynamic.Config.DisablePanicThreshold {
			cluster.CommonLbConfig = &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{
					Value: 0, // disable panic threshold
				},
			}
		}
		addEnvoyCircuitBreakers(dynamic.Config.CircuitBreakers, cluster)
		addEnvoyOutlierDetection(dynamic.Config.OutlierDetection, cluster)

		err := addEnvoyLBToCluster(dynamic.Config, cluster)
		if err != nil {
			return nil, err
		}
	}

	if dynamic.OutboundTls != nil {
		envoyTransportSocket, err := pr.makeEnvoyTransportSocket(dynamic.OutboundTls)
		if err != nil {
			return nil, err
		}
		cluster.TransportSocket = envoyTransportSocket
	}

	return cluster, nil

}

func (pr *ProxyResources) makeEnvoyStaticCluster(name string, protocol string, static *pbproxystate.StaticEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	endpointList, ok := pr.proxyState.Endpoints[name]
	if !ok || endpointList == nil {
		return nil, fmt.Errorf("static cluster %q is missing endpoints", name)
	}
	cluster := &envoy_cluster_v3.Cluster{
		Name:                 name,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC},
		LoadAssignment:       makeEnvoyClusterLoadAssignment(name, endpointList.Endpoints),
	}
	err := addHttpProtocolOptions(protocol, cluster)
	if err != nil {
		return nil, err
	}

	if static.Config != nil {
		cluster.ConnectTimeout = static.Config.ConnectTimeout
		addEnvoyCircuitBreakers(static.GetConfig().CircuitBreakers, cluster)
	}
	return cluster, nil
}
func (pr *ProxyResources) makeEnvoyDnsCluster(name string, protocol string, dns *pbproxystate.DNSEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	return nil, nil
}
func (pr *ProxyResources) makeEnvoyPassthroughCluster(name string, protocol string, passthrough *pbproxystate.PassthroughEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	cluster := &envoy_cluster_v3.Cluster{
		Name:                 name,
		ConnectTimeout:       passthrough.Config.ConnectTimeout,
		LbPolicy:             envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_ORIGINAL_DST},
	}
	if passthrough.OutboundTls != nil {
		envoyTransportSocket, err := pr.makeEnvoyTransportSocket(passthrough.OutboundTls)
		if err != nil {
			return nil, err
		}
		cluster.TransportSocket = envoyTransportSocket
	}
	err := addHttpProtocolOptions(protocol, cluster)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

func (pr *ProxyResources) makeEnvoyAggregateCluster(name string, protocol string, fg *pbproxystate.FailoverGroup) ([]*envoy_cluster_v3.Cluster, error) {
	var clusters []*envoy_cluster_v3.Cluster
	if fg != nil {

		var egNames []string
		for _, eg := range fg.EndpointGroups {
			cluster, err := pr.makeEnvoyCluster(name, protocol, eg)
			if err != nil {
				return nil, err
			}
			egNames = append(egNames, cluster.Name)
			clusters = append(clusters, cluster)
		}
		aggregateClusterConfig, err := anypb.New(&envoy_aggregate_cluster_v3.ClusterConfig{
			Clusters: egNames,
		})

		if err != nil {
			return nil, err
		}

		c := &envoy_cluster_v3.Cluster{
			Name:           name,
			AltStatName:    name,
			ConnectTimeout: fg.Config.ConnectTimeout,
			LbPolicy:       envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_ClusterType{
				ClusterType: &envoy_cluster_v3.Cluster_CustomClusterType{
					Name:        "envoy.clusters.aggregate",
					TypedConfig: aggregateClusterConfig,
				},
			},
		}
		err = addHttpProtocolOptions(protocol, c)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}

func addHttpProtocolOptions(protocol string, c *envoy_cluster_v3.Cluster) error {
	if !(protocol == "http2" || protocol == "grpc") {
		// do not error.  returning nil means it won't get set.
		return nil
	}
	cfg := &envoy_upstreams_v3.HttpProtocolOptions{
		UpstreamProtocolOptions: &envoy_upstreams_v3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &envoy_upstreams_v3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &envoy_upstreams_v3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
					Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
				},
			},
		},
	}
	any, err := anypb.New(cfg)
	if err != nil {
		return err
	}
	c.TypedExtensionProtocolOptions = map[string]*anypb.Any{
		"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": any,
	}
	return nil
}

// addEnvoyOutlierDetection will add outlier detection config to the cluster, and if nil, add empty OutlierDetection to
// enable it with default values
func addEnvoyOutlierDetection(outlierDetection *pbproxystate.OutlierDetection, c *envoy_cluster_v3.Cluster) {
	if outlierDetection == nil {
		return
	}
	od := &envoy_cluster_v3.OutlierDetection{
		BaseEjectionTime:         outlierDetection.GetBaseEjectionTime(),
		Consecutive_5Xx:          outlierDetection.GetConsecutive_5Xx(),
		EnforcingConsecutive_5Xx: outlierDetection.GetEnforcingConsecutive_5Xx(),
		Interval:                 outlierDetection.GetInterval(),
		MaxEjectionPercent:       outlierDetection.GetMaxEjectionPercent(),
	}
	c.OutlierDetection = od

}

func addEnvoyCircuitBreakers(circuitBreakers *pbproxystate.CircuitBreakers, c *envoy_cluster_v3.Cluster) {
	if circuitBreakers != nil {
		if circuitBreakers.UpstreamLimits == nil {
			c.CircuitBreakers = &envoy_cluster_v3.CircuitBreakers{}
			return
		}
		threshold := &envoy_cluster_v3.CircuitBreakers_Thresholds{}
		threshold.MaxConnections = circuitBreakers.UpstreamLimits.MaxConnections
		threshold.MaxPendingRequests = circuitBreakers.UpstreamLimits.MaxPendingRequests
		threshold.MaxRequests = circuitBreakers.UpstreamLimits.MaxConcurrentRequests

		c.CircuitBreakers = &envoy_cluster_v3.CircuitBreakers{
			Thresholds: []*envoy_cluster_v3.CircuitBreakers_Thresholds{threshold},
		}
	}
}

func addEnvoyLBToCluster(dynamicConfig *pbproxystate.DynamicEndpointGroupConfig, c *envoy_cluster_v3.Cluster) error {
	if dynamicConfig == nil || dynamicConfig.LbPolicy == nil {
		return nil
	}

	switch d := dynamicConfig.LbPolicy.(type) {
	case *pbproxystate.DynamicEndpointGroupConfig_LeastRequest:
		c.LbPolicy = envoy_cluster_v3.Cluster_LEAST_REQUEST

		lb := dynamicConfig.LbPolicy.(*pbproxystate.DynamicEndpointGroupConfig_LeastRequest)
		if lb.LeastRequest != nil {
			c.LbConfig = &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
				LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
					ChoiceCount: lb.LeastRequest.ChoiceCount,
				},
			}
		}
	case *pbproxystate.DynamicEndpointGroupConfig_RoundRobin:
		c.LbPolicy = envoy_cluster_v3.Cluster_ROUND_ROBIN

	case *pbproxystate.DynamicEndpointGroupConfig_Random:
		c.LbPolicy = envoy_cluster_v3.Cluster_RANDOM

	case *pbproxystate.DynamicEndpointGroupConfig_RingHash:
		c.LbPolicy = envoy_cluster_v3.Cluster_RING_HASH

		lb := dynamicConfig.LbPolicy.(*pbproxystate.DynamicEndpointGroupConfig_RingHash)
		if lb.RingHash != nil {
			c.LbConfig = &envoy_cluster_v3.Cluster_RingHashLbConfig_{
				RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
					MinimumRingSize: lb.RingHash.MinimumRingSize,
					MaximumRingSize: lb.RingHash.MaximumRingSize,
				},
			}
		}
	case *pbproxystate.DynamicEndpointGroupConfig_Maglev:
		c.LbPolicy = envoy_cluster_v3.Cluster_MAGLEV

	default:
		return fmt.Errorf("unsupported load balancer policy %q for cluster %q", d, c.Name)
	}
	return nil
}

func makeAddress(ip string, port uint32) *envoy_core_v3.Address {
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_core_v3.SocketAddress{
				Address: ip,
				PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
					PortValue: port,
				},
			},
		},
	}
}

// TODO(proxystate): In a future PR this will create clusters and add it to ProxyResources.proxyState
func (pr *ProxyResources) makeEnvoyClusterFromL4Destination(name string) error {
	return nil
}
