// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"fmt"

	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

type discoChainTargets struct {
	baseClusterName string
	targets         []targetInfo
	failover        bool
	failoverPolicy  structs.ServiceResolverFailoverPolicy
}

type targetInfo struct {
	TargetID   string
	TLSContext *envoy_tls_v3.UpstreamTlsContext
	// Region is the region from the failover target's Locality. nil means the
	// target is in the local Consul cluster.
	Region *string

	PrioritizeByLocality *structs.DiscoveryPrioritizeByLocality
}

type discoChainTargetGroup struct {
	Targets     []targetInfo
	ClusterName string
}

func (ft discoChainTargets) groupedTargets() ([]discoChainTargetGroup, error) {
	var targetGroups []discoChainTargetGroup

	if !ft.failover {
		targetGroups = append(targetGroups, discoChainTargetGroup{
			ClusterName: ft.baseClusterName,
			Targets:     ft.targets,
		})
		return targetGroups, nil
	}

	switch ft.failoverPolicy.Mode {
	case "sequential", "":
		return ft.sequential()
	case "order-by-locality":
		return ft.orderByLocality()
	default:
		return targetGroups, fmt.Errorf("unexpected failover policy")
	}
}

func (s *ResourceGenerator) mapDiscoChainTargets(cfgSnap *proxycfg.ConfigSnapshot, chain *structs.CompiledDiscoveryChain, node *structs.DiscoveryGraphNode, upstreamConfig structs.UpstreamConfig, forMeshGateway bool) (discoChainTargets, error) {
	failoverTargets := discoChainTargets{}

	if node.Resolver == nil {
		return discoChainTargets{}, fmt.Errorf("impossible to process a non-resolver node")
	}

	primaryTargetID := node.Resolver.Target
	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil && !forMeshGateway {
		return discoChainTargets{}, err
	}

	failoverTargets.baseClusterName = s.getTargetClusterName(upstreamsSnapshot, chain, primaryTargetID, forMeshGateway, false)

	tids := []string{primaryTargetID}
	failover := node.Resolver.Failover
	if failover != nil && !forMeshGateway {
		tids = append(tids, failover.Targets...)
		failoverTargets.failover = true
		if failover.Policy == nil {
			failoverTargets.failoverPolicy = structs.ServiceResolverFailoverPolicy{}
		} else {
			failoverTargets.failoverPolicy = *failover.Policy
		}
	}

	for _, tid := range tids {
		target := chain.Targets[tid]
		var sni, rootPEMs string
		var spiffeIDs []string
		targetUID := proxycfg.NewUpstreamIDFromTargetID(tid)
		ti := targetInfo{TargetID: tid, PrioritizeByLocality: target.PrioritizeByLocality}

		configureTLS := true
		if forMeshGateway {
			// We only initiate TLS if we're doing an L7 proxy.
			configureTLS = structs.IsProtocolHTTPLike(upstreamConfig.Protocol)
		}

		if !configureTLS {
			failoverTargets.targets = append(failoverTargets.targets, ti)
			continue
		}

		if targetUID.Peer != "" {
			tbs, _ := upstreamsSnapshot.UpstreamPeerTrustBundles.Get(targetUID.Peer)
			rootPEMs = tbs.ConcatenatedRootPEMs()

			peerMeta, found := upstreamsSnapshot.UpstreamPeerMeta(targetUID)
			if !found {
				s.Logger.Warn("failed to fetch upstream peering metadata", "target", targetUID)
				continue
			}
			sni = peerMeta.PrimarySNI()
			spiffeIDs = peerMeta.SpiffeID
			region := target.Locality.GetRegion()
			ti.Region = &region
		} else {
			sni = target.SNI
			rootPEMs = cfgSnap.RootPEMs()
			spiffeIDs = []string{connect.SpiffeIDService{
				Host:       cfgSnap.Roots.TrustDomain,
				Namespace:  target.Namespace,
				Partition:  target.Partition,
				Datacenter: target.Datacenter,
				Service:    target.Service,
			}.URI().String()}
		}
		commonTLSContext := makeCommonTLSContext(
			cfgSnap.Leaf(),
			rootPEMs,
			makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing()),
		)

		err := injectSANMatcher(commonTLSContext, spiffeIDs...)
		if err != nil {
			return failoverTargets, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", sni, err)
		}

		tlsContext := &envoy_tls_v3.UpstreamTlsContext{
			CommonTlsContext: commonTLSContext,
			Sni:              sni,
		}
		ti.TLSContext = tlsContext
		failoverTargets.targets = append(failoverTargets.targets, ti)
	}

	return failoverTargets, nil
}

func (ft discoChainTargets) sequential() ([]discoChainTargetGroup, error) {
	var targetGroups []discoChainTargetGroup
	for i, t := range ft.targets {
		targetGroups = append(targetGroups, discoChainTargetGroup{
			ClusterName: fmt.Sprintf("%s%d~%s", xdscommon.FailoverClusterNamePrefix, i, ft.baseClusterName),
			Targets:     []targetInfo{t},
		})
	}
	return targetGroups, nil
}
