// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

func servicePortsFromEndpoints(_ structs.CheckServiceNodes) structs.ServicePorts {
	return nil
}

func portBasedALPN(_ string) []string {
	return nil
}

func (s *ResourceGenerator) makeProxiedAppClusters(cfgSnap *proxycfg.ConfigSnapshot, clusters []proto.Message) ([]proto.Message, error) {
	appCluster, err := s.makeAppCluster(cfgSnap, xdscommon.LocalAppClusterName, "", cfgSnap.Proxy.LocalServicePort)
	if err != nil {
		return nil, err
	}
	clusters = append(clusters, appCluster)
	return clusters, nil
}

func (s *ResourceGenerator) appendEntGatewayServiceClusters(
	clusters []proto.Message,
	_ *proxycfg.ConfigSnapshot,
	_ structs.ServiceName,
	_ string,
	_ structs.CheckServiceNodes,
	_ *structs.ServiceResolverConfigEntry,
	_ *structs.LoadBalancer,
	_ *structs.UpstreamLimits,
	_ bool,
	_ bool,
) ([]proto.Message, bool, error) {
	return clusters, false, nil
}

func (s *ResourceGenerator) appendEntDiscoveryChainTargetClusters(
	out []*envoy_cluster_v3.Cluster,
	_ *proxycfg.ConfigSnapshot,
	_ *proxycfg.ConfigSnapshotUpstreams,
	_ proxycfg.UpstreamID,
	_ *structs.CompiledDiscoveryChain,
	_ discoChainTargetGroup,
	_ *envoy_cluster_v3.Cluster,
	_ bool,
) ([]*envoy_cluster_v3.Cluster, bool, error) {
	return out, false, nil
}

func (s *ResourceGenerator) appendEntTransparentProxyMultiportFilterChains(
	_ *envoy_listener_v3.Listener,
	_ structs.CheckServiceNodes,
	_ string,
	_ string,
	_ filterChainOpts,
) (bool, error) {
	return false, nil
}

func (s *ResourceGenerator) appendEntConnectProxyMultiportFilterChains(
	_ *envoy_listener_v3.Listener,
	_ string,
	_ *proxycfg.ConfigSnapshot,
	_ listenerFilterOpts,
) (bool, error) {
	return false, nil
}

func (s *ResourceGenerator) appendEntMeshGatewayMultiportFilterChains(
	_ *envoy_listener_v3.Listener,
	_ string,
	_ *proxycfg.ConfigSnapshot,
) error {
	return nil
}

func (s *ResourceGenerator) appendEntMeshGatewayServicePortLoadAssignments(
	resources []proto.Message,
	_ *proxycfg.ConfigSnapshot,
	_ structs.ServiceName,
	_ string,
	_ []loadAssignmentEndpointGroup,
) []proto.Message {
	return resources
}
