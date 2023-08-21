// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"fmt"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

type discoChainTargets struct {
	baseClusterName string
	targets         []targetInfo
	failover        bool
	failoverPolicy  structs.ServiceResolverFailoverPolicy
}

type targetInfo struct {
	TargetID        string
	TransportSocket *pbproxystate.TransportSocket
	SNI             string
	RootPEMs        string
	SpiffeIDs       []string
	Region          *string

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

func (s *Converter) mapDiscoChainTargets(cfgSnap *proxycfg.ConfigSnapshot, chain *structs.CompiledDiscoveryChain, node *structs.DiscoveryGraphNode, upstreamConfig structs.UpstreamConfig, forMeshGateway bool) (discoChainTargets, error) {
	failoverTargets := discoChainTargets{}

	if node.Resolver == nil {
		return discoChainTargets{}, fmt.Errorf("impossible to process a non-resolver node")
	}

	primaryTargetID := node.Resolver.Target
	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil && !forMeshGateway {
		return discoChainTargets{}, err
	}

	failoverTargets.baseClusterName = s.getTargetClusterName(upstreamsSnapshot, chain, primaryTargetID, forMeshGateway)

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

			peerMeta, found := upstreamsSnapshot.UpstreamPeerMeta(targetUID)
			if !found {
				s.Logger.Warn("failed to fetch upstream peering metadata", "target", targetUID)
				continue
			}
			ti.SNI = peerMeta.PrimarySNI()
			ti.SpiffeIDs = peerMeta.SpiffeID
			region := target.Locality.GetRegion()
			ti.Region = &region
			ti.RootPEMs = tbs.ConcatenatedRootPEMs()
		} else {
			ti.SNI = target.SNI
			ti.RootPEMs = cfgSnap.RootPEMs()
			ti.SpiffeIDs = []string{connect.SpiffeIDService{
				Host:       cfgSnap.Roots.TrustDomain,
				Namespace:  target.Namespace,
				Partition:  target.Partition,
				Datacenter: target.Datacenter,
				Service:    target.Service,
			}.URI().String()}
		}
		//commonTLSContext := makeCommonTLSContext(
		//	cfgSnap.Leaf(),
		//	rootPEMs,
		//	makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing()),
		//)
		//
		//err := injectSANMatcher(commonTLSContext, spiffeIDs...)
		//if err != nil {
		//	return failoverTargets, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", sni, err)
		//}

		//tlsContext := &envoy_tls_v3.UpstreamTlsContext{
		//	CommonTlsContext: commonTLSContext,
		//	Sni:              sni,
		//}
		//ti.TLSContext = tlsContext
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
