// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"errors"
	"fmt"
	"strconv"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/go-bexpr"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

const (
	UnnamedSubset = ""
)

// endpointsFromSnapshot returns the xDS API representation of the "endpoints"
func (s *ResourceGenerator) endpointsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.endpointsFromSnapshotConnectProxy(cfgSnap)
	case structs.ServiceKindTerminatingGateway:
		return s.endpointsFromSnapshotTerminatingGateway(cfgSnap)
	case structs.ServiceKindMeshGateway:
		return s.endpointsFromSnapshotMeshGateway(cfgSnap)
	case structs.ServiceKindIngressGateway:
		return s.endpointsFromSnapshotIngressGateway(cfgSnap)
	case structs.ServiceKindAPIGateway:
		return s.endpointsFromSnapshotAPIGateway(cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// endpointsFromSnapshotConnectProxy returns the xDS API representation of the "endpoints"
// (upstream instances) in the snapshot.
func (s *ResourceGenerator) endpointsFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	// TODO: this estimate is wrong
	resources := make([]proto.Message, 0,
		len(cfgSnap.ConnectProxy.PreparedQueryEndpoints)+
			cfgSnap.ConnectProxy.PeerUpstreamEndpoints.Len()+
			len(cfgSnap.ConnectProxy.WatchedUpstreamEndpoints))

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
			return nil, err
		}
		resources = append(resources, es...)
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
			return nil, fmt.Errorf("trust bundle not ready for peer %s", uid.Peer)
		}

		clusterName := generatePeeredClusterName(uid, tbs)

		mgwMode := structs.MeshGatewayModeDefault
		if upstream != nil {
			mgwMode = upstream.MeshGateway.Mode
		}
		loadAssignment, err := s.makeUpstreamLoadAssignmentForPeerService(cfgSnap, clusterName, uid, mgwMode)
		if err != nil {
			return nil, err
		}

		if loadAssignment != nil {
			resources = append(resources, loadAssignment)
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
			la := makeLoadAssignment(
				cfgSnap,
				clusterName,
				nil,
				[]loadAssignmentEndpointGroup{
					{Endpoints: endpoints},
				},
				cfgSnap.Locality,
			)
			resources = append(resources, la)
		}
	}

	// Loop over potential destinations in the mesh, then grab the gateway nodes associated with each
	cfgSnap.ConnectProxy.DestinationsUpstream.ForEachKey(func(uid proxycfg.UpstreamID) bool {
		svcConfig, ok := cfgSnap.ConnectProxy.DestinationsUpstream.Get(uid)
		if !ok || svcConfig.Destination == nil {
			return true
		}

		for _, address := range svcConfig.Destination.Addresses {
			name := clusterNameForDestination(cfgSnap, uid.Name, address, uid.NamespaceOrDefault(), uid.PartitionOrDefault())

			endpoints, ok := cfgSnap.ConnectProxy.DestinationGateways.Get(uid)
			if ok {
				la := makeLoadAssignment(
					cfgSnap,
					name,
					nil,
					[]loadAssignmentEndpointGroup{
						{Endpoints: endpoints},
					},
					proxycfg.GatewayKey{ /*empty so it never matches*/ },
				)
				resources = append(resources, la)
			}
		}

		return true
	})

	return resources, nil
}

func (s *ResourceGenerator) filterSubsetEndpoints(subset *structs.ServiceResolverSubset, endpoints structs.CheckServiceNodes) (structs.CheckServiceNodes, error) {
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

func (s *ResourceGenerator) endpointsFromSnapshotTerminatingGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	return s.endpointsFromServicesAndResolvers(cfgSnap, cfgSnap.TerminatingGateway.ServiceGroups, cfgSnap.TerminatingGateway.ServiceResolvers)
}

func (s *ResourceGenerator) endpointsFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	keys := cfgSnap.MeshGateway.GatewayKeys()

	// Allocation count (this is a lower bound - all subset specific clusters will be appended):
	// 1 cluster per remote dc/partition
	// 1 cluster per local service
	// 1 cluster per unique peer server (control plane traffic)
	resources := make([]proto.Message, 0, len(keys)+len(cfgSnap.MeshGateway.ServiceGroups)+len(cfgSnap.MeshGateway.PeerServers))

	for _, key := range keys {
		if key.Matches(cfgSnap.Datacenter, cfgSnap.ProxyID.PartitionOrDefault()) {
			continue // skip local
		}
		// Also skip gateways with a hostname as their address. EDS cannot resolve hostnames,
		// so we provide them through CDS instead.
		if len(cfgSnap.MeshGateway.HostnameDatacenters[key.String()]) > 0 {
			continue
		}

		endpoints := cfgSnap.GetMeshGatewayEndpoints(key)
		if len(endpoints) == 0 {
			s.Logger.Error("skipping mesh gateway endpoints because no definition found", "datacenter", key)
			continue
		}

		{ // standard connect
			clusterName := connect.GatewaySNI(key.Datacenter, key.Partition, cfgSnap.Roots.TrustDomain)

			la := makeLoadAssignment(
				cfgSnap,
				clusterName,
				nil,
				[]loadAssignmentEndpointGroup{
					{Endpoints: endpoints},
				},
				cfgSnap.Locality,
			)
			resources = append(resources, la)
		}

		if cfgSnap.ProxyID.InDefaultPartition() &&
			cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" &&
			cfgSnap.ServerSNIFn != nil {

			clusterName := cfgSnap.ServerSNIFn(key.Datacenter, "")
			la := makeLoadAssignment(
				cfgSnap,
				clusterName,
				nil,
				[]loadAssignmentEndpointGroup{
					{Endpoints: endpoints},
				},
				cfgSnap.Locality,
			)
			resources = append(resources, la)
		}
	}

	// generate endpoints for our servers if WAN federation is enabled
	if cfgSnap.ProxyID.InDefaultPartition() &&
		cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" &&
		cfgSnap.ServerSNIFn != nil {
		var allServersLbEndpoints []*envoy_endpoint_v3.LbEndpoint

		servers, _ := cfgSnap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)
		for _, srv := range servers {
			clusterName := cfgSnap.ServerSNIFn(cfgSnap.Datacenter, srv.Node.Node)

			_, addr, port := srv.BestAddress(false /*wan*/)

			lbEndpoint := &envoy_endpoint_v3.LbEndpoint{
				HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
					Endpoint: &envoy_endpoint_v3.Endpoint{
						Address: makeAddress(addr, port),
					},
				},
				HealthStatus: envoy_core_v3.HealthStatus_UNKNOWN,
			}

			cla := &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: clusterName,
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{lbEndpoint},
				}},
			}
			allServersLbEndpoints = append(allServersLbEndpoints, lbEndpoint)

			resources = append(resources, cla)
		}

		// And add one catch all so that remote datacenters can dial ANY server
		// in this datacenter without knowing its name.
		resources = append(resources, &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: cfgSnap.ServerSNIFn(cfgSnap.Datacenter, ""),
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
				LbEndpoints: allServersLbEndpoints,
			}},
		})
	}

	// Create endpoints for the cluster where local servers will be dialed by peers.
	// When peering through gateways we load balance across the local servers. They cannot be addressed individually.
	if cfg := cfgSnap.MeshConfig(); cfg.PeerThroughMeshGateways() {
		var serverEndpoints []*envoy_endpoint_v3.LbEndpoint

		servers, _ := cfgSnap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)
		for _, srv := range servers {
			if isReplica := srv.Service.Meta["read_replica"]; isReplica == "true" {
				// Peering control-plane traffic can only ever be handled by the local leader.
				// We avoid routing to read replicas since they will never be Raft voters.
				continue
			}

			_, addr, _ := srv.BestAddress(false)
			portStr, ok := srv.Service.Meta["grpc_tls_port"]
			if !ok {
				s.Logger.Warn("peering is enabled but local server %q does not have the required gRPC TLS port configured",
					"server", srv.Node.Node)
				continue
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				s.Logger.Error("peering is enabled but local server has invalid gRPC TLS port",
					"server", srv.Node.Node, "port", portStr, "error", err)
				continue
			}

			serverEndpoints = append(serverEndpoints, &envoy_endpoint_v3.LbEndpoint{
				HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
					Endpoint: &envoy_endpoint_v3.Endpoint{
						Address: makeAddress(addr, port),
					},
				},
			})
		}
		if len(serverEndpoints) > 0 {
			resources = append(resources, &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: connect.PeeringServerSAN(cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain),
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: serverEndpoints,
				}},
			})
		}
	}

	// Generate the endpoints for each service and its subsets
	e, err := s.endpointsFromServicesAndResolvers(cfgSnap, cfgSnap.MeshGateway.ServiceGroups, cfgSnap.MeshGateway.ServiceResolvers)
	if err != nil {
		return nil, err
	}
	resources = append(resources, e...)

	// Generate the endpoints for exported discovery chain targets.
	e, err = s.makeExportedUpstreamEndpointsForMeshGateway(cfgSnap)
	if err != nil {
		return nil, err
	}
	resources = append(resources, e...)

	// generate the outgoing endpoints for imported peer services.
	e, err = s.makeEndpointsForOutgoingPeeredServices(cfgSnap)
	if err != nil {
		return nil, err
	}
	resources = append(resources, e...)

	// Generate the endpoints for peer server control planes.
	e, err = s.makePeerServerEndpointsForMeshGateway(cfgSnap)
	if err != nil {
		return nil, err
	}
	resources = append(resources, e...)

	return resources, nil
}

func (s *ResourceGenerator) endpointsFromServicesAndResolvers(
	cfgSnap *proxycfg.ConfigSnapshot,
	services map[structs.ServiceName]structs.CheckServiceNodes,
	resolvers map[structs.ServiceName]*structs.ServiceResolverConfigEntry,
) ([]proto.Message, error) {
	resources := make([]proto.Message, 0, len(services))

	// generate the endpoints for the linked service groups
	for svc, endpoints := range services {
		// Skip creating endpoints for services that have hostnames as addresses
		// EDS cannot resolve hostnames so we provide them through CDS instead
		if cfgSnap.Kind == structs.ServiceKindTerminatingGateway && len(cfgSnap.TerminatingGateway.HostnameServices[svc]) > 0 {
			continue
		}

		clusterEndpoints := make(map[string][]loadAssignmentEndpointGroup)
		clusterEndpoints[UnnamedSubset] = []loadAssignmentEndpointGroup{{Endpoints: endpoints, OnlyPassing: false}}

		// Collect all of the loadAssignmentEndpointGroups for the various subsets. We do this before generating
		// the endpoints for the default/unnamed subset so that we can take into account the DefaultSubset on the
		// service-resolver which may prevent the default/unnamed cluster from creating endpoints for all service
		// instances.
		if resolver, hasResolver := resolvers[svc]; hasResolver {
			for subsetName, subset := range resolver.Subsets {
				subsetEndpoints, err := s.filterSubsetEndpoints(&subset, endpoints)
				if err != nil {
					return nil, err
				}
				groups := []loadAssignmentEndpointGroup{{Endpoints: subsetEndpoints, OnlyPassing: subset.OnlyPassing}}
				clusterEndpoints[subsetName] = groups

				// if this subset is the default then override the unnamed subset with this configuration
				if subsetName == resolver.DefaultSubset {
					clusterEndpoints[UnnamedSubset] = groups
				}
			}
		}

		// now generate the load assignment for all subsets
		for subsetName, groups := range clusterEndpoints {
			clusterName := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
			la := makeLoadAssignment(
				cfgSnap,
				clusterName,
				nil,
				groups,
				cfgSnap.Locality,
			)
			resources = append(resources, la)
		}
	}

	return resources, nil
}

func (s *ResourceGenerator) makeEndpointsForOutgoingPeeredServices(
	cfgSnap *proxycfg.ConfigSnapshot,
) ([]proto.Message, error) {
	var resources []proto.Message

	// generate the endpoints for the linked service groups
	for _, serviceGroups := range cfgSnap.MeshGateway.PeeringServices {
		for sn, serviceGroup := range serviceGroups {
			if serviceGroup.UseCDS || len(serviceGroup.Nodes) == 0 {
				continue
			}

			node := serviceGroup.Nodes[0]
			if node.Service == nil {
				return nil, fmt.Errorf("couldn't get SNI for peered service %s", sn.String())
			}
			// This uses the SNI in the accepting cluster peer so the remote mesh
			// gateway can distinguish between an exported service as opposed to the
			// usual mesh gateway route for a service.
			clusterName := node.Service.Connect.PeerMeta.PrimarySNI()

			groups := []loadAssignmentEndpointGroup{{Endpoints: serviceGroup.Nodes, OnlyPassing: false}}

			la := makeLoadAssignment(
				cfgSnap,
				clusterName,
				nil,
				groups,
				// Use an empty key here so that it never matches. This will force the mesh gateway to always
				// reference the remote mesh gateway's wan addr.
				proxycfg.GatewayKey{},
			)
			resources = append(resources, la)
		}
	}

	return resources, nil
}

func (s *ResourceGenerator) makePeerServerEndpointsForMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	resources := make([]proto.Message, 0, len(cfgSnap.MeshGateway.PeerServers))

	// Peer server names are assumed to already be formatted in SNI notation:
	// server.<datacenter>.peering.<trust-domain>
	for name, servers := range cfgSnap.MeshGateway.PeerServers {
		if servers.UseCDS || len(servers.Addresses) == 0 {
			continue
		}

		es := make([]*envoy_endpoint_v3.LbEndpoint, 0, len(servers.Addresses))

		for _, address := range servers.Addresses {
			es = append(es, makeEndpoint(address.Address, address.Port))
		}

		cla := &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: es,
				},
			},
		}

		resources = append(resources, cla)
	}

	return resources, nil
}

func (s *ResourceGenerator) endpointsFromSnapshotIngressGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message
	createdClusters := make(map[proxycfg.UpstreamID]bool)
	for _, upstreams := range cfgSnap.IngressGateway.Upstreams {
		for _, u := range upstreams {
			uid := proxycfg.NewUpstreamID(&u)

			// If we've already created endpoints for this upstream, skip it. Multiple listeners may
			// reference the same upstream, so we don't need to create duplicate endpoints in that case.
			if createdClusters[uid] {
				continue
			}

			es, err := s.endpointsFromDiscoveryChain(
				uid,
				cfgSnap.IngressGateway.DiscoveryChain[uid],
				cfgSnap,
				proxycfg.GatewayKey{Datacenter: cfgSnap.Datacenter, Partition: u.DestinationPartition},
				u.Config,
				cfgSnap.IngressGateway.WatchedUpstreamEndpoints[uid],
				cfgSnap.IngressGateway.WatchedGatewayEndpoints[uid],
				false,
			)
			if err != nil {
				return nil, err
			}
			resources = append(resources, es...)
			createdClusters[uid] = true
		}
	}
	return resources, nil
}

func (s *ResourceGenerator) endpointsFromSnapshotAPIGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message
	createdClusters := make(map[proxycfg.UpstreamID]struct{})

	readyListeners := getReadyListeners(cfgSnap)

	for _, readyListener := range readyListeners {
		for _, u := range readyListener.upstreams {
			uid := proxycfg.NewUpstreamID(&u)

			// If we've already created endpoints for this upstream, skip it. Multiple listeners may
			// reference the same upstream, so we don't need to create duplicate endpoints in that case.
			_, ok := createdClusters[uid]
			if ok {
				continue
			}

			endpoints, err := s.endpointsFromDiscoveryChain(
				uid,
				cfgSnap.APIGateway.DiscoveryChain[uid],
				cfgSnap,
				proxycfg.GatewayKey{Datacenter: cfgSnap.Datacenter, Partition: u.DestinationPartition},
				u.Config,
				cfgSnap.APIGateway.WatchedUpstreamEndpoints[uid],
				cfgSnap.APIGateway.WatchedGatewayEndpoints[uid],
				false,
			)
			if err != nil {
				return nil, err
			}

			resources = append(resources, endpoints...)
			createdClusters[uid] = struct{}{}
		}
	}
	return resources, nil
}

// used in clusters.go
func makeEndpoint(host string, port int) *envoy_endpoint_v3.LbEndpoint {
	return &envoy_endpoint_v3.LbEndpoint{
		HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
			Endpoint: &envoy_endpoint_v3.Endpoint{
				Address: makeAddress(host, port),
			},
		},
	}
}

func makePipeEndpoint(path string) *envoy_endpoint_v3.LbEndpoint {
	return &envoy_endpoint_v3.LbEndpoint{
		HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
			Endpoint: &envoy_endpoint_v3.Endpoint{
				Address: makePipeAddress(path, 0),
			},
		},
	}
}

func (s *ResourceGenerator) makeUpstreamLoadAssignmentForPeerService(
	cfgSnap *proxycfg.ConfigSnapshot,
	clusterName string,
	uid proxycfg.UpstreamID,
	upstreamGatewayMode structs.MeshGatewayMode,
) (*envoy_endpoint_v3.ClusterLoadAssignment, error) {
	var la *envoy_endpoint_v3.ClusterLoadAssignment

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil {
		return la, err
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
			return la, nil
		}
		la = makeLoadAssignment(
			cfgSnap,
			clusterName,
			nil,
			[]loadAssignmentEndpointGroup{
				{Endpoints: localGw},
			},
			cfgSnap.Locality,
		)
		return la, nil
	}

	// Also skip peer instances with a hostname as their address. EDS
	// cannot resolve hostnames, so we provide them through CDS instead.
	if _, ok := upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames[uid]; ok {
		return la, nil
	}

	endpoints, ok := upstreamsSnapshot.PeerUpstreamEndpoints.Get(uid)
	if !ok {
		return nil, nil
	}
	la = makeLoadAssignment(
		cfgSnap,
		clusterName,
		nil,
		[]loadAssignmentEndpointGroup{
			{Endpoints: endpoints},
		},
		proxycfg.GatewayKey{ /*empty so it never matches*/ },
	)
	return la, nil
}

func (s *ResourceGenerator) endpointsFromDiscoveryChain(
	uid proxycfg.UpstreamID,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
	gatewayKey proxycfg.GatewayKey,
	upstreamConfigMap map[string]interface{},
	upstreamEndpoints map[string]structs.CheckServiceNodes,
	gatewayEndpoints map[string]structs.CheckServiceNodes,
	forMeshGateway bool,
) ([]proto.Message, error) {
	if chain == nil {
		if forMeshGateway {
			return nil, fmt.Errorf("missing discovery chain for %s", uid)
		}
		return nil, nil
	}

	if upstreamConfigMap == nil {
		upstreamConfigMap = make(map[string]interface{}) // TODO:needed?
	}

	var resources []proto.Message

	var escapeHatchCluster *envoy_cluster_v3.Cluster
	if !forMeshGateway {

		cfg, err := structs.ParseUpstreamConfigNoDefaults(upstreamConfigMap)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", uid,
				"error", err)
		}

		if cfg.EnvoyClusterJSON != "" {
			if chain.Default {
				// If you haven't done anything to setup the discovery chain, then
				// you can use the envoy_cluster_json escape hatch.
				escapeHatchCluster, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
				if err != nil {
					return resources, nil
				}
			} else {
				s.Logger.Warn("ignoring escape hatch setting, because a discovery chain is configued for",
					"discovery chain", chain.ServiceName, "upstream", uid,
					"envoy_cluster_json", chain.ServiceName)
			}
		}
	}

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
			if escapeHatchCluster != nil {
				clusterName = escapeHatchCluster.Name
			}
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
				loadAssignment, err := s.makeUpstreamLoadAssignmentForPeerService(cfgSnap, clusterName, targetUID, mgwMode)
				if err != nil {
					return nil, err
				}
				if loadAssignment != nil {
					resources = append(resources, loadAssignment)
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

			la := makeLoadAssignment(
				cfgSnap,
				clusterName,
				ti.PrioritizeByLocality,
				[]loadAssignmentEndpointGroup{endpointGroup},
				gatewayKey,
			)
			resources = append(resources, la)
		}
	}

	return resources, nil
}

func (s *ResourceGenerator) makeExportedUpstreamEndpointsForMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	populatedExportedClusters := make(map[string]struct{}) // key=clusterName
	for _, svc := range cfgSnap.MeshGatewayValidExportedServices() {
		chain := cfgSnap.MeshGateway.DiscoveryChain[svc]

		chainEndpoints := make(map[string]structs.CheckServiceNodes)
		for _, target := range chain.Targets {
			if !cfgSnap.Locality.Matches(target.Datacenter, target.Partition) || target.Peer != "" {
				s.Logger.Warn("ignoring discovery chain target that crosses a datacenter, peer, or partition boundary in a mesh gateway",
					"target", target,
					"gatewayLocality", cfgSnap.Locality,
				)
				continue
			}

			targetSvc := target.ServiceName()

			endpoints, ok := cfgSnap.MeshGateway.ServiceGroups[targetSvc]
			if !ok {
				continue // ignore; not ready
			}

			if target.ServiceSubset == "" {
				chainEndpoints[target.ID] = endpoints
			} else {
				resolver, ok := cfgSnap.MeshGateway.ServiceResolvers[targetSvc]
				if !ok {
					continue // ignore; not ready
				}
				subset, ok := resolver.Subsets[target.ServiceSubset]
				if !ok {
					continue // ignore; not ready
				}

				subsetEndpoints, err := s.filterSubsetEndpoints(&subset, endpoints)
				if err != nil {
					return nil, err
				}
				chainEndpoints[target.ID] = subsetEndpoints
			}
		}

		clusterEndpoints, err := s.endpointsFromDiscoveryChain(
			proxycfg.NewUpstreamIDFromServiceName(svc),
			chain,
			cfgSnap,
			cfgSnap.Locality,
			nil,
			chainEndpoints,
			nil,
			true,
		)
		if err != nil {
			return nil, err
		}
		for _, endpoints := range clusterEndpoints {
			clusterName := xdscommon.GetResourceName(endpoints)
			if _, ok := populatedExportedClusters[clusterName]; ok {
				continue
			}
			populatedExportedClusters[clusterName] = struct{}{}
			resources = append(resources, endpoints)
		}
	}
	return resources, nil
}

type loadAssignmentEndpointGroup struct {
	Endpoints      structs.CheckServiceNodes
	OnlyPassing    bool
	OverrideHealth envoy_core_v3.HealthStatus
}

func makeLoadAssignment(cfgSnap *proxycfg.ConfigSnapshot, clusterName string, policy *structs.DiscoveryPrioritizeByLocality, endpointGroups []loadAssignmentEndpointGroup, localKey proxycfg.GatewayKey) *envoy_endpoint_v3.ClusterLoadAssignment {
	cla := &envoy_endpoint_v3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   make([]*envoy_endpoint_v3.LocalityLbEndpoints, 0, len(endpointGroups)),
	}

	if len(endpointGroups) > 1 {
		cla.Policy = &envoy_endpoint_v3.ClusterLoadAssignment_Policy{
			// We choose such a large value here that the failover math should
			// in effect not happen until zero instances are healthy.
			OverprovisioningFactor: makeUint32Value(100000),
		}
	}

	var priority uint32

	for _, endpointGroup := range endpointGroups {
		endpointsByLocality, err := groupedEndpoints(cfgSnap.ServiceLocality, policy, endpointGroup.Endpoints)

		if err != nil {
			continue
		}

		for _, endpoints := range endpointsByLocality {
			es := make([]*envoy_endpoint_v3.LbEndpoint, 0, len(endpointGroup.Endpoints))

			for _, ep := range endpoints {
				// TODO (mesh-gateway) - should we respect the translate_wan_addrs configuration here or just always use the wan for cross-dc?
				_, addr, port := ep.BestAddress(!localKey.Matches(ep.Node.Datacenter, ep.Node.PartitionOrDefault()))
				healthStatus, weight := calculateEndpointHealthAndWeight(ep, endpointGroup.OnlyPassing)

				if endpointGroup.OverrideHealth != envoy_core_v3.HealthStatus_UNKNOWN {
					healthStatus = endpointGroup.OverrideHealth
				}

				endpoint := &envoy_endpoint_v3.Endpoint{
					Address: makeAddress(addr, port),
				}
				es = append(es, &envoy_endpoint_v3.LbEndpoint{
					HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
						Endpoint: endpoint,
					},
					HealthStatus:        healthStatus,
					LoadBalancingWeight: makeUint32Value(weight),
				})
			}

			cla.Endpoints = append(cla.Endpoints, &envoy_endpoint_v3.LocalityLbEndpoints{
				Priority:    priority,
				LbEndpoints: es,
			})

			priority++
		}
	}

	return cla
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
	overallHealth := envoy_core_v3.HealthStatus_UNHEALTHY
	for _, ep := range realEndpoints {
		health, _ := calculateEndpointHealthAndWeight(ep, target.Subset.OnlyPassing)
		if health == envoy_core_v3.HealthStatus_HEALTHY {
			overallHealth = envoy_core_v3.HealthStatus_HEALTHY
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
) (envoy_core_v3.HealthStatus, int) {
	healthStatus := envoy_core_v3.HealthStatus_HEALTHY
	weight := 1
	if ep.Service.Weights != nil {
		weight = ep.Service.Weights.Passing
	}

	for _, chk := range ep.Checks {
		if chk.Status == api.HealthCritical {
			healthStatus = envoy_core_v3.HealthStatus_UNHEALTHY
		}
		if onlyPassing && chk.Status != api.HealthPassing {
			healthStatus = envoy_core_v3.HealthStatus_UNHEALTHY
		}
		if chk.Status == api.HealthWarning && ep.Service.Weights != nil {
			weight = ep.Service.Weights.Warning
		}
	}
	// Make weights fit Envoy's limits. A zero weight means that either Warning
	// (likely) or Passing (weirdly) weight has been set to 0 effectively making
	// this instance unhealthy and should not be sent traffic.
	if weight < 1 {
		healthStatus = envoy_core_v3.HealthStatus_UNHEALTHY
		weight = 1
	}
	if weight > 128 {
		weight = 128
	}
	return healthStatus, weight
}
