// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"errors"
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_aggregate_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	envoy_upstreams_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

func (pr *ProxyResources) makeClustersAndEndpoints(name string) (map[string]proto.Message, map[string]proto.Message, error) {
	envoyClusters := make(map[string]proto.Message)
	envoyEndpoints := make(map[string]proto.Message)
	proxyStateCluster, ok := pr.proxyState.Clusters[name]
	if !ok {
		return nil, nil, fmt.Errorf("cluster %q not found", name)
	}

	switch proxyStateCluster.Group.(type) {
	case *pbproxystate.Cluster_FailoverGroup:
		fg := proxyStateCluster.GetFailoverGroup()
		clusters, eps, err := pr.makeEnvoyAggregateClusterAndEndpoint(name, proxyStateCluster.Protocol, fg)
		if err != nil {
			return nil, nil, err
		}
		// for each cluster, add it to clusters map and add endpoint to endpoint map
		for _, c := range clusters {
			envoyClusters[c.Name] = c
			if ep, ok := eps[c.Name]; ok {
				envoyEndpoints[c.Name] = ep
			}
		}

	case *pbproxystate.Cluster_EndpointGroup:
		eg := proxyStateCluster.GetEndpointGroup()
		cluster, eps, err := pr.makeEnvoyClusterAndEndpoint(name, proxyStateCluster.Protocol, eg)
		if err != nil {
			return nil, nil, err
		}

		// for each cluster, add it to clusters map and add endpoint to endpoint map
		envoyClusters[cluster.Name] = cluster
		if ep, ok := eps[cluster.Name]; ok {
			envoyEndpoints[cluster.Name] = ep
		}

	default:
		return nil, nil, errors.New("cluster group type should be Endpoint Group or Failover Group")
	}
	return envoyClusters, envoyEndpoints, nil
}

func (pr *ProxyResources) makeEnvoyClusterAndEndpoint(name string, protocol pbproxystate.Protocol,
	eg *pbproxystate.EndpointGroup) (*envoy_cluster_v3.Cluster, map[string]*envoy_endpoint_v3.ClusterLoadAssignment, error) {
	if eg != nil {
		switch t := eg.Group.(type) {
		case *pbproxystate.EndpointGroup_Dynamic:
			dynamic := eg.GetDynamic()
			return pr.makeEnvoyDynamicClusterAndEndpoint(name, protocol, dynamic)
		case *pbproxystate.EndpointGroup_Static:
			static := eg.GetStatic()
			return pr.makeEnvoyStaticClusterAndEndpoint(name, protocol, static)
		case *pbproxystate.EndpointGroup_Dns:
			dns := eg.GetDns()
			return pr.makeEnvoyDnsCluster(name, protocol, dns)
		case *pbproxystate.EndpointGroup_Passthrough:
			passthrough := eg.GetPassthrough()
			return pr.makeEnvoyPassthroughCluster(name, protocol, passthrough)
		default:
			return nil, nil, fmt.Errorf("unsupported endpoint group type: %s", t)
		}
	}
	return nil, nil, fmt.Errorf("no endpoint group")
}

func (pr *ProxyResources) makeEnvoyDynamicClusterAndEndpoint(name string, protocol pbproxystate.Protocol,
	dynamic *pbproxystate.DynamicEndpointGroup) (*envoy_cluster_v3.Cluster, map[string]*envoy_endpoint_v3.ClusterLoadAssignment, error) {
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
		return nil, nil, err
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
			return nil, nil, err
		}
	}

	if dynamic.OutboundTls != nil {
		envoyTransportSocket, err := pr.makeEnvoyTransportSocket(dynamic.OutboundTls)
		if err != nil {
			return nil, nil, err
		}
		cluster.TransportSocket = envoyTransportSocket
	}

	// Generate Envoy endpoint
	endpointResources := make(map[string]*envoy_endpoint_v3.ClusterLoadAssignment)
	if endpointList, ok := pr.proxyState.Endpoints[cluster.Name]; ok {
		protoEndpoint := makeEnvoyClusterLoadAssignment(cluster.Name, endpointList.Endpoints)
		endpointResources[cluster.Name] = protoEndpoint
	}

	return cluster, endpointResources, nil

}

func (pr *ProxyResources) makeEnvoyStaticClusterAndEndpoint(name string, protocol pbproxystate.Protocol,
	static *pbproxystate.StaticEndpointGroup) (*envoy_cluster_v3.Cluster, map[string]*envoy_endpoint_v3.ClusterLoadAssignment, error) {
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
		return nil, nil, err
	}

	if static.Config != nil {
		cluster.ConnectTimeout = static.Config.ConnectTimeout
		addEnvoyCircuitBreakers(static.GetConfig().CircuitBreakers, cluster)
	}
	return cluster, nil, nil
}

func (pr *ProxyResources) makeEnvoyDnsCluster(name string, protocol pbproxystate.Protocol,
	dns *pbproxystate.DNSEndpointGroup) (*envoy_cluster_v3.Cluster, map[string]*envoy_endpoint_v3.ClusterLoadAssignment, error) {
	return nil, nil, nil
}

func (pr *ProxyResources) makeEnvoyPassthroughCluster(name string, protocol pbproxystate.Protocol,
	passthrough *pbproxystate.PassthroughEndpointGroup) (*envoy_cluster_v3.Cluster, map[string]*envoy_endpoint_v3.ClusterLoadAssignment, error) {
	cluster := &envoy_cluster_v3.Cluster{
		Name:                 name,
		ConnectTimeout:       passthrough.Config.ConnectTimeout,
		LbPolicy:             envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_ORIGINAL_DST},
	}
	if passthrough.OutboundTls != nil {
		envoyTransportSocket, err := pr.makeEnvoyTransportSocket(passthrough.OutboundTls)
		if err != nil {
			return nil, nil, err
		}
		cluster.TransportSocket = envoyTransportSocket
	}
	err := addHttpProtocolOptions(protocol, cluster)
	if err != nil {
		return nil, nil, err
	}
	return cluster, nil, nil
}

func (pr *ProxyResources) makeEnvoyAggregateClusterAndEndpoint(name string, protocol pbproxystate.Protocol,
	fg *pbproxystate.FailoverGroup) (map[string]*envoy_cluster_v3.Cluster, map[string]*envoy_endpoint_v3.ClusterLoadAssignment, error) {
	clusters := make(map[string]*envoy_cluster_v3.Cluster)
	endpointResources := make(map[string]*envoy_endpoint_v3.ClusterLoadAssignment)
	if fg != nil {
		var egNames []string
		for _, eg := range fg.EndpointGroups {
			cluster, eps, err := pr.makeEnvoyClusterAndEndpoint(eg.Name, protocol, eg)
			if err != nil {
				return nil, eps, err
			}
			egNames = append(egNames, cluster.Name)

			// add failover cluster
			clusters[cluster.Name] = cluster

			// add endpoint for failover cluster
			if ep, ok := eps[cluster.Name]; ok {
				endpointResources[cluster.Name] = ep
			}
		}
		aggregateClusterConfig, err := anypb.New(&envoy_aggregate_cluster_v3.ClusterConfig{
			Clusters: egNames,
		})

		if err != nil {
			return nil, nil, err
		}

		// create aggregate cluster
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
			return nil, nil, err
		}

		// add aggregate cluster
		clusters[c.Name] = c

		// add endpoint for aggregate cluster
		if endpointList, ok := pr.proxyState.Endpoints[c.Name]; ok {
			protoEndpoint := makeEnvoyClusterLoadAssignment(c.Name, endpointList.Endpoints)
			endpointResources[c.Name] = protoEndpoint
		}
	}
	return clusters, endpointResources, nil
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

func (pr *ProxyResources) makeEnvoyClustersAndEndpointsFromL4Destination(l4 *pbproxystate.L4Destination) error {
	switch l4.Destination.(type) {
	case *pbproxystate.L4Destination_Cluster:
		pr.addEnvoyClustersAndEndpointsToEnvoyResources(l4.GetCluster().GetName())

	case *pbproxystate.L4Destination_WeightedClusters:
		psWeightedClusters := l4.GetWeightedClusters()
		for _, psCluster := range psWeightedClusters.GetClusters() {
			pr.addEnvoyClustersAndEndpointsToEnvoyResources(psCluster.Name)
		}
	default:
		return errors.New("cluster group type should be Endpoint Group or Failover Group")
	}

	return nil
}
