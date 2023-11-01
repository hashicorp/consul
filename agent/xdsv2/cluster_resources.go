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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

func (pr *ProxyResources) makeClusters(name string) (map[string]proto.Message, error) {
	clusters := make(map[string]proto.Message)
	proxyStateCluster, ok := pr.proxyState.Clusters[name]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", name)
	}

	switch proxyStateCluster.Group.(type) {
	case *pbproxystate.Cluster_FailoverGroup:
		fg := proxyStateCluster.GetFailoverGroup()
		clusters, err := pr.makeEnvoyAggregateCluster(name, proxyStateCluster.Protocol, fg)
		if err != nil {
			return nil, err
		}
		for _, c := range clusters {
			clusters[c.Name] = c
		}

	case *pbproxystate.Cluster_EndpointGroup:
		eg := proxyStateCluster.GetEndpointGroup()
		cluster, err := pr.makeEnvoyCluster(name, proxyStateCluster.Protocol, eg)
		if err != nil {
			return nil, err
		}
		clusters[cluster.Name] = cluster

	default:
		return nil, errors.New("cluster group type should be Endpoint Group or Failover Group")
	}
	return clusters, nil
}

func (pr *ProxyResources) makeEnvoyCluster(name string, protocol pbproxystate.Protocol, eg *pbproxystate.EndpointGroup) (*envoy_cluster_v3.Cluster, error) {
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

func (pr *ProxyResources) makeEnvoyDynamicCluster(name string, protocol pbproxystate.Protocol, dynamic *pbproxystate.DynamicEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
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
		if dynamic.Config.DisablePanicThreshold {
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

func (pr *ProxyResources) makeEnvoyStaticCluster(name string, protocol pbproxystate.Protocol, static *pbproxystate.StaticEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	cluster := &envoy_cluster_v3.Cluster{
		Name:                 name,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC},
	}

	// todo (ishustava/v2): we need to be able to handle the case when empty endpoints are allowed on a cluster.
	endpointList, ok := pr.proxyState.Endpoints[name]
	if ok {
		cluster.LoadAssignment = makeEnvoyClusterLoadAssignment(name, endpointList.Endpoints)
	}

	var err error
	if name == xdscommon.LocalAppClusterName {
		err = addLocalAppHttpProtocolOptions(protocol, cluster)
	} else {
		err = addHttpProtocolOptions(protocol, cluster)
	}
	if err != nil {
		return nil, err
	}

	if static.Config != nil {
		cluster.ConnectTimeout = static.Config.ConnectTimeout
		addEnvoyCircuitBreakers(static.GetConfig().CircuitBreakers, cluster)
	}
	return cluster, nil
}

func (pr *ProxyResources) makeEnvoyDnsCluster(name string, protocol pbproxystate.Protocol, dns *pbproxystate.DNSEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
	return nil, nil
}

func (pr *ProxyResources) makeEnvoyPassthroughCluster(name string, protocol pbproxystate.Protocol, passthrough *pbproxystate.PassthroughEndpointGroup) (*envoy_cluster_v3.Cluster, error) {
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

func (pr *ProxyResources) makeEnvoyAggregateCluster(name string, protocol pbproxystate.Protocol, fg *pbproxystate.FailoverGroup) (map[string]*envoy_cluster_v3.Cluster, error) {
	clusters := make(map[string]*envoy_cluster_v3.Cluster)
	if fg != nil {
		var egNames []string
		for _, eg := range fg.EndpointGroups {
			cluster, err := pr.makeEnvoyCluster(eg.Name, protocol, eg)
			if err != nil {
				return nil, err
			}
			egNames = append(egNames, cluster.Name)
			clusters[cluster.Name] = cluster
		}
		aggregateClusterConfig, err := anypb.New(&envoy_aggregate_cluster_v3.ClusterConfig{
			Clusters: egNames,
		})

		if err != nil {
			return nil, err
		}

		c := &envoy_cluster_v3.Cluster{
			Name:           name,
			ConnectTimeout: fg.Config.ConnectTimeout,
			LbPolicy:       envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_ClusterType{
				ClusterType: &envoy_cluster_v3.Cluster_CustomClusterType{
					Name:        "envoy.clusters.aggregate",
					TypedConfig: aggregateClusterConfig,
				},
			},
		}
		if fg.Config.UseAltStatName {
			c.AltStatName = name
		}
		err = addHttpProtocolOptions(protocol, c)
		if err != nil {
			return nil, err
		}
		clusters[c.Name] = c
	}
	return clusters, nil
}

func addLocalAppHttpProtocolOptions(protocol pbproxystate.Protocol, c *envoy_cluster_v3.Cluster) error {
	if !(protocol == pbproxystate.Protocol_PROTOCOL_HTTP2 || protocol == pbproxystate.Protocol_PROTOCOL_GRPC) {
		// do not error.  returning nil means it won't get set.
		return nil
	}
	cfg := &envoy_upstreams_v3.HttpProtocolOptions{
		UpstreamProtocolOptions: &envoy_upstreams_v3.HttpProtocolOptions_UseDownstreamProtocolConfig{
			UseDownstreamProtocolConfig: &envoy_upstreams_v3.HttpProtocolOptions_UseDownstreamHttpConfig{
				HttpProtocolOptions:  &envoy_core_v3.Http1ProtocolOptions{},
				Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
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

func addHttpProtocolOptions(protocol pbproxystate.Protocol, c *envoy_cluster_v3.Cluster) error {
	if !(protocol == pbproxystate.Protocol_PROTOCOL_HTTP2 || protocol == pbproxystate.Protocol_PROTOCOL_GRPC) {
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

func (pr *ProxyResources) makeEnvoyClusterFromL4Destination(l4 *pbproxystate.L4Destination) error {
	switch l4.Destination.(type) {
	case *pbproxystate.L4Destination_Cluster:
		clusterName := l4.GetCluster().Name
		clusters, _ := pr.makeClusters(clusterName)
		for name, cluster := range clusters {
			pr.envoyResources[xdscommon.ClusterType][name] = cluster
		}

		eps := pr.proxyState.GetEndpoints()[clusterName]
		if eps != nil {
			protoEndpoint := makeEnvoyClusterLoadAssignment(clusterName, eps.Endpoints)
			pr.envoyResources[xdscommon.EndpointType][protoEndpoint.ClusterName] = protoEndpoint
		}

	case *pbproxystate.L4Destination_WeightedClusters:
		psWeightedClusters := l4.GetWeightedClusters()
		for _, psCluster := range psWeightedClusters.GetClusters() {
			clusters, _ := pr.makeClusters(psCluster.Name)
			for name, cluster := range clusters {
				pr.envoyResources[xdscommon.ClusterType][name] = cluster
			}

			eps := pr.proxyState.GetEndpoints()[psCluster.Name]
			if eps != nil {
				protoEndpoint := makeEnvoyClusterLoadAssignment(psCluster.Name, eps.Endpoints)
				pr.envoyResources[xdscommon.EndpointType][psCluster.Name] = protoEndpoint
			}
		}
	default:
		return errors.New("cluster group type should be Endpoint Group or Failover Group")
	}

	return nil
}
