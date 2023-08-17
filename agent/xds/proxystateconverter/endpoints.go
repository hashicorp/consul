// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/go-bexpr"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func makeLbEndpoint(addr string, port int, health pbproxystate.HealthStatus, weight int) *pbproxystate.Endpoint {
	ep := &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: addr,
				Port: uint32(port),
			},
		},
	}
	ep.HealthStatus = health
	ep.LoadBalancingWeight = &wrapperspb.UInt32Value{Value: uint32(weight)}
	return ep
}


// endpointsFromSnapshot returns the mesh API representation of the "routes" in the snapshot.
func (s *Converter) endpointsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) error {

	if cfgSnap == nil {
		return errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.endpointsFromSnapshotConnectProxy(cfgSnap)
	//case structs.ServiceKindTerminatingGateway:
	//	return s.endpointsFromSnapshotTerminatingGateway(cfgSnap)
	//case structs.ServiceKindMeshGateway:
	//	return s.endpointsFromSnapshotMeshGateway(cfgSnap)
	//case structs.ServiceKindIngressGateway:
	//	return s.endpointsFromSnapshotIngressGateway(cfgSnap)
	//case structs.ServiceKindAPIGateway:
	//	return s.endpointsFromSnapshotAPIGateway(cfgSnap)
	default:
		return fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// endpointsFromSnapshotConnectProxy returns the xDS API representation of the "endpoints"
// (upstream instances) in the snapshot.
func (s *Converter) endpointsFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) error {
	eps := make(map[string]*pbproxystate.Endpoints)

	// NOTE: Any time we skip a chain below we MUST also skip that discovery chain in clusters.go
	// so that the sets of endpoints generated matches the sets of clusters.
	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		upstream, skip := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta)
		if skip {
			// Discovery chain is not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		var upstreamConfigMap map[string]interface{}
		if upstream != nil {
			upstreamConfigMap = upstream.Config
		}

		es, err := s.endpointsFromDiscoveryChain(
			uid,
			chain,
			cfgSnap,
			cfgSnap.Locality,
			upstreamConfigMap,
			cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[uid],
			cfgSnap.ConnectProxy.WatchedGatewayEndpoints[uid],
			false,
		)
		if err != nil {
			return err
		}

		for clusterName, endpoints := range es {
			eps[clusterName] = &pbproxystate.Endpoints{
				Endpoints: endpoints,
			}

		}
	}

	// NOTE: Any time we skip an upstream below we MUST also skip that same
	// upstream in clusters.go so that the sets of endpoints generated matches
	// the sets of clusters.
	for _, uid := range cfgSnap.ConnectProxy.PeeredUpstreamIDs() {
		upstream, skip := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta)
		if skip {
			// Discovery chain is not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		tbs, ok := cfgSnap.ConnectProxy.UpstreamPeerTrustBundles.Get(uid.Peer)
		if !ok {
			// this should never happen since we loop through upstreams with
			// set trust bundles
			return fmt.Errorf("trust bundle not ready for peer %s", uid.Peer)
		}

		clusterName := generatePeeredClusterName(uid, tbs)

		mgwMode := structs.MeshGatewayModeDefault
		if upstream != nil {
			mgwMode = upstream.MeshGateway.Mode
		}
		peerServiceEndpoints, err := s.makeEndpointsForPeerService(cfgSnap, uid, mgwMode)
		if err != nil {
			return err
		}

		if peerServiceEndpoints != nil {
			pbEndpoints := &pbproxystate.Endpoints{
				Endpoints: peerServiceEndpoints,
			}

			eps[clusterName] = pbEndpoints
		}
	}

	// Looping over explicit upstreams is only needed for prepared queries because they do not have discovery chains
	for _, u := range cfgSnap.Proxy.Upstreams {
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			continue
		}
		uid := proxycfg.NewUpstreamID(&u)

		dc := u.Datacenter
		if dc == "" {
			dc = cfgSnap.Datacenter
		}
		clusterName := connect.UpstreamSNI(&u, "", dc, cfgSnap.Roots.TrustDomain)

		endpoints, ok := cfgSnap.ConnectProxy.PreparedQueryEndpoints[uid]
		if ok {
			epts := makeEndpointsForLoadAssignment(
				cfgSnap,
				nil,
				[]loadAssignmentEndpointGroup{
					{Endpoints: endpoints},
				},
				cfgSnap.Locality,
			)
			pbEndpoints := &pbproxystate.Endpoints{
				Endpoints: epts,
			}

			eps[clusterName] = pbEndpoints
		}
	}

	// Loop over potential destinations in the mesh, then grab the gateway nodes associated with each
	cfgSnap.ConnectProxy.DestinationsUpstream.ForEachKey(func(uid proxycfg.UpstreamID) bool {
		svcConfig, ok := cfgSnap.ConnectProxy.DestinationsUpstream.Get(uid)
		if !ok || svcConfig.Destination == nil {
			return true
		}

		for _, address := range svcConfig.Destination.Addresses {
			clusterName := clusterNameForDestination(cfgSnap, uid.Name, address, uid.NamespaceOrDefault(), uid.PartitionOrDefault())

			endpoints, ok := cfgSnap.ConnectProxy.DestinationGateways.Get(uid)
			if ok {
				epts := makeEndpointsForLoadAssignment(
					cfgSnap,
					nil,
					[]loadAssignmentEndpointGroup{
						{Endpoints: endpoints},
					},
					proxycfg.GatewayKey{ /*empty so it never matches*/ },
				)
				pbEndpoints := &pbproxystate.Endpoints{
					Endpoints: epts,
				}
				eps[clusterName] = pbEndpoints
			}
		}

		return true
	})

	s.proxyState.Endpoints = eps
	return nil
}

func (s *Converter) makeEndpointsForPeerService(
	cfgSnap *proxycfg.ConfigSnapshot,
	uid proxycfg.UpstreamID,
	upstreamGatewayMode structs.MeshGatewayMode,
) ([]*pbproxystate.Endpoint, error) {
	var eps []*pbproxystate.Endpoint

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil {
		return eps, err
	}

	if upstreamGatewayMode == structs.MeshGatewayModeNone {
		s.Logger.Warn(fmt.Sprintf("invalid mesh gateway mode 'none', defaulting to 'remote' for %q", uid))
	}

	// If an upstream is configured with local mesh gw mode, we make a load assignment
	// from the gateway endpoints instead of those of the upstreams.
	if upstreamGatewayMode == structs.MeshGatewayModeLocal {
		localGw, ok := cfgSnap.ConnectProxy.WatchedLocalGWEndpoints.Get(cfgSnap.Locality.String())
		if !ok {
			// local GW is not ready; return early
			return eps, nil
		}
		eps = makeEndpointsForLoadAssignment(
			cfgSnap,
			nil,
			[]loadAssignmentEndpointGroup{
				{Endpoints: localGw},
			},
			cfgSnap.Locality,
		)
		return eps, nil
	}

	// Also skip peer instances with a hostname as their address. EDS
	// cannot resolve hostnames, so we provide them through CDS instead.
	if _, ok := upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames[uid]; ok {
		return eps, nil
	}

	endpoints, ok := upstreamsSnapshot.PeerUpstreamEndpoints.Get(uid)
	if !ok {
		return nil, nil
	}
	eps = makeEndpointsForLoadAssignment(
		cfgSnap,
		nil,
		[]loadAssignmentEndpointGroup{
			{Endpoints: endpoints},
		},
		proxycfg.GatewayKey{ /*empty so it never matches*/ },
	)
	return eps, nil
}

func (s *Converter) filterSubsetEndpoints(subset *structs.ServiceResolverSubset, endpoints structs.CheckServiceNodes) (structs.CheckServiceNodes, error) {
	// locally execute the subsets filter
	if subset.Filter != "" {
		filter, err := bexpr.CreateFilter(subset.Filter, nil, endpoints)
		if err != nil {
			return nil, err
		}

		raw, err := filter.Execute(endpoints)
		if err != nil {
			return nil, err
		}
		return raw.(structs.CheckServiceNodes), nil
	}
	return endpoints, nil
}

// TODO(proxystate): Terminating Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func endpointsFromSnapshotTerminatingGateway

// TODO(proxystate): Mesh Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func endpointsFromSnapshotMeshGateway

// TODO(proxystate): Cluster Peering will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func makeEndpointsForOutgoingPeeredServices

// TODO(proxystate): Mesh Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func endpointsFromServicesAndResolvers

// TODO(proxystate): Mesh Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func makePeerServerEndpointsForMeshGateway

// TODO(proxystate): Ingress Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func endpointsFromSnapshotIngressGateway

// TODO(proxystate): API Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func endpointsFromSnapshotAPIGateway

// used in clusters.go
func makeHostPortEndpoint(host string, port int) *pbproxystate.Endpoint {
	return &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: host,
				Port: uint32(port),
			},
		},
	}
}

func makeUnixSocketEndpoint(path string) *pbproxystate.Endpoint {
	return &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_UnixSocket{
			UnixSocket: &pbproxystate.UnixSocketAddress{
				Path: path,
				// envoy's mode is particular to a pipe address and is uint32.
				// it also says "The mode for the Pipe. Not applicable for abstract sockets."
				// https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/address.proto#config-core-v3-pipe
				Mode: "0",
			},
		},
	}
}


func (s *Converter) makeUpstreamLoadAssignmentEndpointForPeerService(
	cfgSnap *proxycfg.ConfigSnapshot,
	uid proxycfg.UpstreamID,
	upstreamGatewayMode structs.MeshGatewayMode,
) ([]*pbproxystate.Endpoint, error) {
	var eps []*pbproxystate.Endpoint

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil {
		return eps, err
	}

	if upstreamGatewayMode == structs.MeshGatewayModeNone {
		s.Logger.Warn(fmt.Sprintf("invalid mesh gateway mode 'none', defaulting to 'remote' for %q", uid))
	}

	// If an upstream is configured with local mesh gw mode, we make a load assignment
	// from the gateway endpoints instead of those of the upstreams.
	if upstreamGatewayMode == structs.MeshGatewayModeLocal {
		localGw, ok := cfgSnap.ConnectProxy.WatchedLocalGWEndpoints.Get(cfgSnap.Locality.String())
		if !ok {
			// local GW is not ready; return early
			return eps, nil
		}
		eps = makeEndpointsForLoadAssignment(
			cfgSnap,
			nil,
			[]loadAssignmentEndpointGroup{
				{Endpoints: localGw},
			},
			cfgSnap.Locality,
		)
		return eps, nil
	}

	// Also skip peer instances with a hostname as their address. EDS
	// cannot resolve hostnames, so we provide them through CDS instead.
	if _, ok := upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames[uid]; ok {
		return eps, nil
	}

	endpoints, ok := upstreamsSnapshot.PeerUpstreamEndpoints.Get(uid)
	if !ok {
		return nil, nil
	}
	eps = makeEndpointsForLoadAssignment(
		cfgSnap,
		nil,
		[]loadAssignmentEndpointGroup{
			{Endpoints: endpoints},
		},
		proxycfg.GatewayKey{ /*empty so it never matches*/ },
	)
	return eps, nil
}

func (s *Converter) endpointsFromDiscoveryChain(
	uid proxycfg.UpstreamID,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
	gatewayKey proxycfg.GatewayKey,
	upstreamConfigMap map[string]interface{},
	upstreamEndpoints map[string]structs.CheckServiceNodes,
	gatewayEndpoints map[string]structs.CheckServiceNodes,
	forMeshGateway bool,
) (map[string][]*pbproxystate.Endpoint, error) {
	if chain == nil {
		if forMeshGateway {
			return nil, fmt.Errorf("missing discovery chain for %s", uid)
		}
		return nil, nil
	}

	if upstreamConfigMap == nil {
		upstreamConfigMap = make(map[string]interface{}) // TODO:needed?
	}

	clusterEndpoints := make(map[string][]*pbproxystate.Endpoint)

	// TODO(jm): escape hatches will be implemented in the future
	//var escapeHatchCluster *pbproxystate.Cluster
	//if !forMeshGateway {

	//cfg, err := structs.ParseUpstreamConfigNoDefaults(upstreamConfigMap)
	//if err != nil {
	//	// Don't hard fail on a config typo, just warn. The parse func returns
	//	// default config if there is an error so it's safe to continue.
	//	s.Logger.Warn("failed to parse", "upstream", uid,
	//		"error", err)
	//}

	//if cfg.EnvoyClusterJSON != "" {
	//	if chain.Default {
	//		// If you haven't done anything to setup the discovery chain, then
	//		// you can use the envoy_cluster_json escape hatch.
	//		escapeHatchCluster, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
	//		if err != nil {
	//			return ce, nil
	//		}
	//	} else {
	//		s.Logger.Warn("ignoring escape hatch setting, because a discovery chain is configued for",
	//			"discovery chain", chain.ServiceName, "upstream", uid,
	//			"envoy_cluster_json", chain.ServiceName)
	//	}
	//}
	//}

	mgwMode := structs.MeshGatewayModeDefault
	if upstream, _ := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta); upstream != nil {
		mgwMode = upstream.MeshGateway.Mode
	}

	// Find all resolver nodes.
	for _, node := range chain.Nodes {
		switch {
		case node == nil:
			return nil, fmt.Errorf("impossible to process a nil node")
		case node.Type != structs.DiscoveryGraphNodeTypeResolver:
			continue
		case node.Resolver == nil:
			return nil, fmt.Errorf("impossible to process a non-resolver node")
		}
		rawUpstreamConfig, err := structs.ParseUpstreamConfigNoDefaults(upstreamConfigMap)
		if err != nil {
			return nil, err
		}
		upstreamConfig := finalizeUpstreamConfig(rawUpstreamConfig, chain, node.Resolver.ConnectTimeout)

		mappedTargets, err := s.mapDiscoChainTargets(cfgSnap, chain, node, upstreamConfig, forMeshGateway)
		if err != nil {
			return nil, err
		}

		targetGroups, err := mappedTargets.groupedTargets()
		if err != nil {
			return nil, err
		}

		for _, groupedTarget := range targetGroups {
			clusterName := groupedTarget.ClusterName
			// TODO(jm): escape hatches will be implemented in the future
			//if escapeHatchCluster != nil {
			//	clusterName = escapeHatchCluster.Name
			//}
			switch len(groupedTarget.Targets) {
			case 0:
				continue
			case 1:
				// We expect one target so this passes through to continue setting the load assignment up.
			default:
				return nil, fmt.Errorf("cannot have more than one target")
			}
			ti := groupedTarget.Targets[0]
			s.Logger.Debug("generating endpoints for", "cluster", clusterName, "targetID", ti.TargetID)
			targetUID := proxycfg.NewUpstreamIDFromTargetID(ti.TargetID)
			if targetUID.Peer != "" {
				peerServiceEndpoints, err := s.makeEndpointsForPeerService(cfgSnap, targetUID, mgwMode)
				if err != nil {
					return nil, err
				}
				if peerServiceEndpoints != nil {
					clusterEndpoints[clusterName] = peerServiceEndpoints
				}
				continue
			}

			endpointGroup, valid := makeLoadAssignmentEndpointGroup(
				chain.Targets,
				upstreamEndpoints,
				gatewayEndpoints,
				ti.TargetID,
				gatewayKey,
				forMeshGateway,
			)
			if !valid {
				continue // skip the cluster if we're still populating the snapshot
			}

			epts := makeEndpointsForLoadAssignment(
				cfgSnap,
				ti.PrioritizeByLocality,
				[]loadAssignmentEndpointGroup{endpointGroup},
				gatewayKey,
			)
			clusterEndpoints[clusterName] = epts
		}
	}

	return clusterEndpoints, nil
}

// TODO(proxystate): Mesh Gateway will be added in the future.
// Functions to add from agent/xds/endpoints.go:
// func makeExportedUpstreamEndpointsForMeshGateway

type loadAssignmentEndpointGroup struct {
	Endpoints      structs.CheckServiceNodes
	OnlyPassing    bool
	OverrideHealth pbproxystate.HealthStatus
}

func makeEndpointsForLoadAssignment(cfgSnap *proxycfg.ConfigSnapshot,
	policy *structs.DiscoveryPrioritizeByLocality,
	endpointGroups []loadAssignmentEndpointGroup,
	localKey proxycfg.GatewayKey) []*pbproxystate.Endpoint {
	pbEndpoints := make([]*pbproxystate.Endpoint, 0, len(endpointGroups))

	// TODO(jm): make this work in xdsv2
	//if len(endpointGroups) > 1 {
	//	cla.Policy = &envoy_endpoint_v3.ClusterLoadAssignment_Policy{
	//		// We choose such a large value here that the failover math should
	//		// in effect not happen until zero instances are healthy.
	//		OverprovisioningFactor: response.MakeUint32Value(100000),
	//	}
	//}

	var priority uint32

	for _, endpointGroup := range endpointGroups {
		endpointsByLocality, err := groupedEndpoints(cfgSnap.ServiceLocality, policy, endpointGroup.Endpoints)

		if err != nil {
			continue
		}

		for _, endpoints := range endpointsByLocality {
			for _, ep := range endpoints {
				// TODO (mesh-gateway) - should we respect the translate_wan_addrs configuration here or just always use the wan for cross-dc?
				_, addr, port := ep.BestAddress(!localKey.Matches(ep.Node.Datacenter, ep.Node.PartitionOrDefault()))
				healthStatus, weight := calculateEndpointHealthAndWeight(ep, endpointGroup.OnlyPassing)

				if endpointGroup.OverrideHealth != pbproxystate.HealthStatus_HEALTH_STATUS_UNKNOWN {
					healthStatus = endpointGroup.OverrideHealth
				}

				endpoint := makeHostPortEndpoint(addr, port)
				endpoint.HealthStatus = healthStatus
				endpoint.LoadBalancingWeight = response.MakeUint32Value(weight)

				pbEndpoints = append(pbEndpoints, endpoint)
			}

			// TODO(jm): what do we do about priority downstream?
			//cla.Endpoints = append(cla.Endpoints, &envoy_endpoint_v3.LocalityLbEndpoints{
			//	Priority:    priority,
			//	LbEndpoints: es,
			//})

			priority++
		}
	}

	return pbEndpoints
}

func makeLoadAssignmentEndpointGroup(
	targets map[string]*structs.DiscoveryTarget,
	targetHealth map[string]structs.CheckServiceNodes,
	gatewayHealth map[string]structs.CheckServiceNodes,
	targetID string,
	localKey proxycfg.GatewayKey,
	forMeshGateway bool,
) (loadAssignmentEndpointGroup, bool) {
	realEndpoints, ok := targetHealth[targetID]
	if !ok {
		// skip the cluster if we're still populating the snapshot
		return loadAssignmentEndpointGroup{}, false
	}
	target := targets[targetID]

	var gatewayKey proxycfg.GatewayKey

	switch target.MeshGateway.Mode {
	case structs.MeshGatewayModeRemote:
		gatewayKey.Datacenter = target.Datacenter
		gatewayKey.Partition = target.Partition
	case structs.MeshGatewayModeLocal:
		gatewayKey = localKey
	}

	if forMeshGateway || gatewayKey.IsEmpty() || localKey.Matches(target.Datacenter, target.Partition) {
		// Gateways are not needed if the request isn't for a remote DC or partition.
		return loadAssignmentEndpointGroup{
			Endpoints:   realEndpoints,
			OnlyPassing: target.Subset.OnlyPassing,
		}, true
	}

	// If using a mesh gateway we need to pull those endpoints instead.
	gatewayEndpoints, ok := gatewayHealth[gatewayKey.String()]
	if !ok {
		// skip the cluster if we're still populating the snapshot
		return loadAssignmentEndpointGroup{}, false
	}

	// But we will use the health from the actual backend service.
	overallHealth := pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
	for _, ep := range realEndpoints {
		health, _ := calculateEndpointHealthAndWeight(ep, target.Subset.OnlyPassing)
		if health == pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY {
			overallHealth = pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY
			break
		}
	}

	return loadAssignmentEndpointGroup{
		Endpoints:      gatewayEndpoints,
		OverrideHealth: overallHealth,
	}, true
}

func calculateEndpointHealthAndWeight(
	ep structs.CheckServiceNode,
	onlyPassing bool,
) (pbproxystate.HealthStatus, int) {
	healthStatus := pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY
	weight := 1
	if ep.Service.Weights != nil {
		weight = ep.Service.Weights.Passing
	}

	for _, chk := range ep.Checks {
		if chk.Status == api.HealthCritical {
			healthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
		}
		if onlyPassing && chk.Status != api.HealthPassing {
			healthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
		}
		if chk.Status == api.HealthWarning && ep.Service.Weights != nil {
			weight = ep.Service.Weights.Warning
		}
	}
	// Make weights fit Envoy's limits. A zero weight means that either Warning
	// (likely) or Passing (weirdly) weight has been set to 0 effectively making
	// this instance unhealthy and should not be sent traffic.
	if weight < 1 {
		healthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
		weight = 1
	}
	if weight > 128 {
		weight = 128
	}
	return healthStatus, weight
}
