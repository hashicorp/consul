// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

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

func (s *ResourceGenerator) mapDiscoChainTargets(
	cfgSnap *proxycfg.ConfigSnapshot,
	uid proxycfg.UpstreamID,
	chain *structs.CompiledDiscoveryChain,
	node *structs.DiscoveryGraphNode,
	upstreamConfig structs.UpstreamConfig,
	forMeshGateway bool,
	destinationPort string,
) (discoChainTargets, error) {
	failoverTargets := discoChainTargets{}

	if node.Resolver == nil {
		return discoChainTargets{}, fmt.Errorf("impossible to process a non-resolver node")
	}

	primaryTargetID := node.Resolver.Target
	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil && !forMeshGateway {
		return discoChainTargets{}, err
	}

	baseClusterName := s.getTargetClusterName(upstreamsSnapshot, chain, primaryTargetID, forMeshGateway)
	failoverTargets.baseClusterName = destinationPortClusterName(baseClusterName, destinationPort)

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
		if alpnProtocols := destinationPortALPN(destinationPort); len(alpnProtocols) > 0 {
			commonTLSContext.AlpnProtocols = alpnProtocols
		}
		err := injectSANMatcher(commonTLSContext, false, spiffeIDs...)
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

	// An aggregate cluster whose members have no EDS assignment is fatal to
	// Envoy rather than merely degraded. Worker startup replays every known
	// cluster into the new thread, and AggregateClusterLoadBalancer::
	// onClusterAddOrUpdate faults on a member whose priority set was never
	// populated (envoyproxy/envoy#35157, ITCO-15826). The identical state in a
	// plain EDS cluster is benign: Envoy warms it, resolves it with zero hosts
	// once initial_fetch_timeout fires and answers 503 UH until endpoints land.
	//
	// So when any member is not ready, render the base cluster as a plain EDS
	// cluster over the primary target instead of as an aggregate. Route
	// destinations are unaffected because they always reference the base
	// cluster name, and failover is restored by the next snapshot once the
	// member endpoints arrive.
	//
	// This complements the first-push bootstrap gate in agent/xds: the gate
	// keeps a wholly incoherent snapshot away from a cold-starting Envoy,
	// while this guard covers what the gate cannot -- a gate that expired on
	// its timeout, and steady-state churn after the first push has opened it.
	if failoverTargets.failover && !forMeshGateway && cfgSnap.Kind == structs.ServiceKindAPIGateway {
		unready := failoverTargets.unreadyFailoverMembers(
			chain,
			upstreamsSnapshot.WatchedUpstreamEndpoints[uid],
			upstreamsSnapshot.WatchedGatewayEndpoints[uid],
			cfgSnap.Locality,
		)
		if len(unready) > 0 {
			degradedTo := failoverTargets.degradeToSingleTarget(primaryTargetID)
			switch {
			case degradedTo == "":
				// Every target was dropped while mapping, so there is nothing
				// to degrade onto. Failover has been cleared, which leaves no
				// targets and therefore no cluster at all -- a 503 NC that
				// self-heals, rather than an aggregate over an empty member
				// list, which is the same fatal shape as an unpopulated member.
				s.Logger.Warn("api-gateway: emitting no cluster for upstream because it has no usable targets",
					"upstream", uid,
					"cluster", failoverTargets.baseClusterName,
					"unready_targets", unready)
			case degradedTo != primaryTargetID:
				// The primary was dropped while mapping (reachable when it is a
				// peered target whose peering metadata has not resolved yet).
				// Falling back to the highest-priority remaining target is both
				// safe -- a plain EDS cluster is benign no matter how ready it
				// is -- and the correct reading of failover intent: the primary
				// is unusable, so the next target in order takes over.
				s.Logger.Warn("api-gateway: rendering upstream without failover over a non-primary target because the primary is unavailable; "+
					"failover is restored automatically once the primary and member endpoints arrive",
					"upstream", uid,
					"cluster", failoverTargets.baseClusterName,
					"primary_target", primaryTargetID,
					"degraded_to", degradedTo,
					"unready_targets", unready)
			default:
				s.Logger.Warn("api-gateway: rendering upstream without failover because member endpoints are not assembled; "+
					"failover is restored automatically once the endpoints arrive",
					"upstream", uid,
					"cluster", failoverTargets.baseClusterName,
					"unready_targets", unready)
			}
		}
	}

	return failoverTargets, nil
}

// unreadyFailoverMembers returns the IDs of mapped targets whose EDS assignment
// would be skipped by endpoint generation -- that is, the members that would
// leave an aggregate cluster pointing at an unpopulated priority set.
//
// It calls makeLoadAssignmentEndpointGroup, the very function endpoint
// generation uses, rather than re-deriving the readiness rule, so the cluster
// and endpoint paths cannot disagree about which members are ready.
//
// KNOWN LIMITATION: peered members are not covered. Their endpoints come from
// makeUpstreamLoadAssignmentForPeerService, which is checked before
// makeLoadAssignmentEndpointGroup in endpoints.go, so that function -- not this
// one -- decides whether a peered member gets an assignment. It returns a nil
// assignment, while CDS still emits the member cluster, in two cases:
//
//   - mesh gateway mode "local" while the local gateway endpoints are not yet
//     watched (endpoints.go, the !ready early return), and
//   - PeerUpstreamEndpoints not yet populated for the target.
//
// Either leaves an aggregate member unpopulated, which is the fatal shape. A
// third nil case, PeerUpstreamEndpointsUseHostnames, is safe because the
// cluster is then DNS-based and its endpoints come through CDS.
//
// This is not covered here because the decision depends on the mesh gateway
// mode, and the mode endpoints.go uses for API gateways comes from
// upstream.MeshGateway.Mode, whereas the upstreamConfig available at this point
// is parsed from upstream.Config -- a different source. Reproducing the rule
// from the wrong input would reintroduce exactly the CDS/EDS drift this
// function exists to avoid, so covering peered members properly requires
// threading the resolved mode into mapDiscoChainTargets. The bootstrap gate's
// proxycfg predicate skips peered targets for the same reason, so both layers
// share this gap. Reaching it needs an API gateway whose service-resolver fails
// over to a peered target whose peering metadata has resolved but whose
// endpoints have not -- a narrower topology than the one this guard targets.
func (ft discoChainTargets) unreadyFailoverMembers(
	chain *structs.CompiledDiscoveryChain,
	upstreamEndpoints map[string]structs.CheckServiceNodes,
	gatewayEndpoints map[string]structs.CheckServiceNodes,
	localKey proxycfg.GatewayKey,
) []string {
	var unready []string
	for _, ti := range ft.targets {
		target := chain.Targets[ti.TargetID]
		if target == nil {
			continue
		}
		// Peered and external targets are served from a different endpoint path
		// (makeUpstreamLoadAssignmentForPeerService, or no EDS at all), so
		// makeLoadAssignmentEndpointGroup is not the authority on their
		// readiness and gating on it here would be wrong. See the known
		// limitation on this function.
		if target.External || target.Peer != "" {
			continue
		}
		if _, valid := makeLoadAssignmentEndpointGroup(
			chain.Targets,
			upstreamEndpoints,
			gatewayEndpoints,
			ti.TargetID,
			localKey,
			false,
		); !valid {
			unready = append(unready, ti.TargetID)
		}
	}
	return unready
}

// degradeToSingleTarget rewrites the mapped targets so they render as a single
// plain EDS cluster under the base cluster name, instead of an aggregate
// cluster over per-target failover member clusters. This is always safe: an
// unpopulated *aggregate* member is what faults during worker startup, whereas
// a plain EDS cluster with no assignment simply warms, times out and resolves
// with zero hosts.
//
// It prefers the primary target, and otherwise falls back to the
// highest-priority target still present -- the primary can legitimately be
// absent, because a peered target whose peering metadata has not resolved is
// dropped while mapping. It returns the target ID it degraded onto, or the
// empty string if no target remained, in which case failover is still cleared
// so that no aggregate can be emitted over an empty member list.
func (ft *discoChainTargets) degradeToSingleTarget(primaryTargetID string) string {
	ft.failover = false
	ft.failoverPolicy = structs.ServiceResolverFailoverPolicy{}

	if len(ft.targets) == 0 {
		return ""
	}

	chosen := ft.targets[0]
	for _, ti := range ft.targets {
		if ti.TargetID == primaryTargetID {
			chosen = ti
			break
		}
	}

	ft.targets = []targetInfo{chosen}
	return chosen.TargetID
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

// isAggregateCluster returns true if the mapped targets contain failover configuration
// and should be rendered as an aggregate cluster (which delegates to multiple target clusters).
func (ft discoChainTargets) isAggregateCluster() bool {
	return ft.failover
}
