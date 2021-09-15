package xds

import (
	"errors"
	"fmt"
	"sort"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// clustersFromSnapshot returns the xDS API representation of the "clusters" in the snapshot.
func (s *ResourceGenerator) clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.clustersFromSnapshotConnectProxy(cfgSnap)
	case structs.ServiceKindTerminatingGateway:
		res, err := s.makeGatewayServiceClusters(cfgSnap, cfgSnap.TerminatingGateway.ServiceGroups, cfgSnap.TerminatingGateway.ServiceResolvers)
		if err != nil {
			return nil, err
		}
		return s.maybeInjectStubClusterForGateways(res)
	case structs.ServiceKindMeshGateway:
		res, err := s.clustersFromSnapshotMeshGateway(cfgSnap)
		if err != nil {
			return nil, err
		}
		return s.maybeInjectStubClusterForGateways(res)
	case structs.ServiceKindIngressGateway:
		res, err := s.clustersFromSnapshotIngressGateway(cfgSnap)
		if err != nil {
			return nil, err
		}
		return s.maybeInjectStubClusterForGateways(res)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

func (s *ResourceGenerator) maybeInjectStubClusterForGateways(resources []proto.Message) ([]proto.Message, error) {
	switch {
	case !s.IncrementalXDS:
		return resources, nil
	case !s.ProxyFeatures.GatewaysNeedStubClusterWhenEmptyWithIncrementalXDS:
		return resources, nil
	case len(resources) > 0:
		return resources, nil
	}

	// For more justification for this hacky fix, check the comments associated
	// with s.ProxyFeatures.GatewaysNeedStubClusterWhenEmptyWithIncrementalXDS

	const stubName = "consul-stub-cluster-working-around-envoy-bug-ignore"
	return []proto.Message{
		&envoy_cluster_v3.Cluster{
			Name:                 stubName,
			ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC},
			LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: stubName,
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
					{LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{}},
				},
			},
		},
	}, nil
}

// clustersFromSnapshot returns the xDS API representation of the "clusters"
// (upstreams) in the snapshot.
func (s *ResourceGenerator) clustersFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	// This sizing is a lower bound.
	clusters := make([]proto.Message, 0, len(cfgSnap.ConnectProxy.DiscoveryChain)+1)

	// Include the "app" cluster for the public listener
	appCluster, err := s.makeAppCluster(cfgSnap, LocalAppClusterName, "", cfgSnap.Proxy.LocalServicePort)
	if err != nil {
		return nil, err
	}
	clusters = append(clusters, appCluster)

	if cfgSnap.Proxy.Mode == structs.ProxyModeTransparent {
		passthroughs, err := makePassthroughClusters(cfgSnap)
		if err != nil {
			return nil, fmt.Errorf("failed to make passthrough clusters for transparent proxy: %v", err)
		}
		clusters = append(clusters, passthroughs...)
	}

	for id, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		chainEndpoints, ok := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[id]
		if !ok {
			// this should not happen
			return nil, fmt.Errorf("no endpoint map for upstream %q", id)
		}

		upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(id, cfgSnap.ConnectProxy.UpstreamConfig[id], chain, chainEndpoints, cfgSnap)
		if err != nil {
			return nil, err
		}

		for _, cluster := range upstreamClusters {
			clusters = append(clusters, cluster)
		}
	}

	for _, u := range cfgSnap.Proxy.Upstreams {
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			continue
		}

		upstreamCluster, err := s.makeUpstreamClusterForPreparedQuery(u, cfgSnap)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, upstreamCluster)
	}

	cfgSnap.Proxy.Expose.Finalize()
	paths := cfgSnap.Proxy.Expose.Paths

	// Add service health checks to the list of paths to create clusters for if needed
	if cfgSnap.Proxy.Expose.Checks {
		psid := structs.NewServiceID(cfgSnap.Proxy.DestinationServiceID, &cfgSnap.ProxyID.EnterpriseMeta)
		for _, check := range s.CheckFetcher.ServiceHTTPBasedChecks(psid) {
			p, err := parseCheckPath(check)
			if err != nil {
				s.Logger.Warn("failed to create cluster for", "check", check.CheckID, "error", err)
				continue
			}
			paths = append(paths, p)
		}
	}

	// Create a new cluster if we need to expose a port that is different from the service port
	for _, path := range paths {
		if path.LocalPathPort == cfgSnap.Proxy.LocalServicePort {
			continue
		}
		c, err := s.makeAppCluster(cfgSnap, makeExposeClusterName(path.LocalPathPort), path.Protocol, path.LocalPathPort)
		if err != nil {
			s.Logger.Warn("failed to make local cluster", "path", path.Path, "error", err)
			continue
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}

func makeExposeClusterName(destinationPort int) string {
	return fmt.Sprintf("exposed_cluster_%d", destinationPort)
}

// In transparent proxy mode there are potentially multiple passthrough clusters added.
// The first is for destinations outside of Consul's catalog. This is for a plain TCP proxy.
// All of these use Envoy's ORIGINAL_DST listener filter, which forwards to the original
// destination address (before the iptables redirection).
// The rest are for destinations inside the mesh, which require certificates for mTLS.
func makePassthroughClusters(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	// This size is an upper bound.
	clusters := make([]proto.Message, 0, len(cfgSnap.ConnectProxy.PassthroughUpstreams)+1)

	if cfgSnap.ConnectProxy.MeshConfig == nil ||
		!cfgSnap.ConnectProxy.MeshConfig.TransparentProxy.MeshDestinationsOnly {

		clusters = append(clusters, &envoy_cluster_v3.Cluster{
			Name: OriginalDestinationClusterName,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
			},
			LbPolicy:       envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,
			ConnectTimeout: ptypes.DurationProto(5 * time.Second),
		})
	}

	for _, passthrough := range cfgSnap.ConnectProxy.PassthroughUpstreams {
		// Prefixed with passthrough to distinguish from non-passthrough clusters for the same upstream.
		name := "passthrough~" + passthrough.SNI

		c := envoy_cluster_v3.Cluster{
			Name: name,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
			},
			LbPolicy: envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,

			// TODO(tproxy) This should use the connection timeout configured on the upstream's config entry
			ConnectTimeout: ptypes.DurationProto(5 * time.Second),
		}

		commonTLSContext := makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf())
		err := injectSANMatcher(commonTLSContext, passthrough.SpiffeID)
		if err != nil {
			return nil, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", passthrough.SNI, err)
		}
		tlsContext := envoy_tls_v3.UpstreamTlsContext{
			CommonTlsContext: commonTLSContext,
			Sni:              passthrough.SNI,
		}
		transportSocket, err := makeUpstreamTLSTransportSocket(&tlsContext)
		if err != nil {
			return nil, err
		}
		c.TransportSocket = transportSocket
		clusters = append(clusters, &c)
	}

	return clusters, nil
}

// clustersFromSnapshotMeshGateway returns the xDS API representation of the "clusters"
// for a mesh gateway. This will include 1 cluster per remote datacenter as well as
// 1 cluster for each service subset.
func (s *ResourceGenerator) clustersFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	datacenters := cfgSnap.MeshGateway.Datacenters()

	// 1 cluster per remote dc + 1 cluster per local service (this is a lower bound - all subset specific clusters will be appended)
	clusters := make([]proto.Message, 0, len(datacenters)+len(cfgSnap.MeshGateway.ServiceGroups))

	// generate the remote dc clusters
	for _, dc := range datacenters {
		if dc == cfgSnap.Datacenter {
			continue // skip local
		}

		opts := gatewayClusterOpts{
			name:              connect.DatacenterSNI(dc, cfgSnap.Roots.TrustDomain),
			hostnameEndpoints: cfgSnap.MeshGateway.HostnameDatacenters[dc],
			isRemote:          dc != cfgSnap.Datacenter,
		}
		cluster := s.makeGatewayCluster(cfgSnap, opts)
		clusters = append(clusters, cluster)
	}

	if cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" && cfgSnap.ServerSNIFn != nil {
		// Add all of the remote wildcard datacenter mappings for servers.
		for _, dc := range datacenters {
			hostnameEndpoints := cfgSnap.MeshGateway.HostnameDatacenters[dc]

			// If the DC is our current DC then this cluster is for traffic from a remote DC to a local server.
			// HostnameDatacenters is populated with gateway addresses, so it does not apply here.
			if dc == cfgSnap.Datacenter {
				hostnameEndpoints = nil
			}
			opts := gatewayClusterOpts{
				name:              cfgSnap.ServerSNIFn(dc, ""),
				hostnameEndpoints: hostnameEndpoints,
				isRemote:          dc != cfgSnap.Datacenter,
			}
			cluster := s.makeGatewayCluster(cfgSnap, opts)
			clusters = append(clusters, cluster)
		}

		// And for the current datacenter, send all flavors appropriately.
		for _, srv := range cfgSnap.MeshGateway.ConsulServers {
			opts := gatewayClusterOpts{
				name: cfgSnap.ServerSNIFn(cfgSnap.Datacenter, srv.Node.Node),
			}
			cluster := s.makeGatewayCluster(cfgSnap, opts)
			clusters = append(clusters, cluster)
		}
	}

	// generate the per-service/subset clusters
	c, err := s.makeGatewayServiceClusters(cfgSnap, cfgSnap.MeshGateway.ServiceGroups, cfgSnap.MeshGateway.ServiceResolvers)
	if err != nil {
		return nil, err
	}
	clusters = append(clusters, c...)

	return clusters, nil
}

func (s *ResourceGenerator) makeGatewayServiceClusters(
	cfgSnap *proxycfg.ConfigSnapshot,
	services map[structs.ServiceName]structs.CheckServiceNodes,
	resolvers map[structs.ServiceName]*structs.ServiceResolverConfigEntry,
) ([]proto.Message, error) {
	var hostnameEndpoints structs.CheckServiceNodes

	switch cfgSnap.Kind {
	case structs.ServiceKindTerminatingGateway, structs.ServiceKindMeshGateway:
	default:
		return nil, fmt.Errorf("unsupported gateway kind %q", cfgSnap.Kind)
	}

	clusters := make([]proto.Message, 0, len(services))

	for svc := range services {
		clusterName := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
		resolver, hasResolver := resolvers[svc]

		var loadBalancer *structs.LoadBalancer

		if !hasResolver {
			// Use a zero value resolver with no timeout and no subsets
			resolver = &structs.ServiceResolverConfigEntry{}
		}
		if resolver.LoadBalancer != nil {
			loadBalancer = resolver.LoadBalancer
		}

		// When making service clusters we only pass endpoints with hostnames if the kind is a terminating gateway
		// This is because the services a mesh gateway will route to are not external services and are not addressed by a hostname.
		if cfgSnap.Kind == structs.ServiceKindTerminatingGateway {
			hostnameEndpoints = cfgSnap.TerminatingGateway.HostnameServices[svc]
		}

		opts := gatewayClusterOpts{
			name:              clusterName,
			hostnameEndpoints: hostnameEndpoints,
			connectTimeout:    resolver.ConnectTimeout,
		}
		cluster := s.makeGatewayCluster(cfgSnap, opts)

		if err := s.injectGatewayServiceAddons(cfgSnap, cluster, svc, loadBalancer); err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)

		// If there is a service-resolver for this service then also setup a cluster for each subset
		for name, subset := range resolver.Subsets {
			subsetHostnameEndpoints, err := s.filterSubsetEndpoints(&subset, hostnameEndpoints)
			if err != nil {
				return nil, err
			}

			opts := gatewayClusterOpts{
				name:              connect.ServiceSNI(svc.Name, name, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain),
				hostnameEndpoints: subsetHostnameEndpoints,
				onlyPassing:       subset.OnlyPassing,
				connectTimeout:    resolver.ConnectTimeout,
			}
			cluster := s.makeGatewayCluster(cfgSnap, opts)

			if err := s.injectGatewayServiceAddons(cfgSnap, cluster, svc, loadBalancer); err != nil {
				return nil, err
			}
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

func (s *ResourceGenerator) injectGatewayServiceAddons(cfgSnap *proxycfg.ConfigSnapshot, c *envoy_cluster_v3.Cluster, svc structs.ServiceName, lb *structs.LoadBalancer) error {
	switch cfgSnap.Kind {
	case structs.ServiceKindMeshGateway:
		// We can't apply hash based LB config to mesh gateways because they rely on inspecting HTTP attributes
		// and mesh gateways do not decrypt traffic
		if !lb.IsHashBased() {
			if err := injectLBToCluster(lb, c); err != nil {
				return fmt.Errorf("failed to apply load balancer configuration to cluster %q: %v", c.Name, err)
			}
		}
	case structs.ServiceKindTerminatingGateway:
		// Context used for TLS origination to the cluster
		if mapping, ok := cfgSnap.TerminatingGateway.GatewayServices[svc]; ok && mapping.CAFile != "" {
			tlsContext := &envoy_tls_v3.UpstreamTlsContext{
				CommonTlsContext: makeCommonTLSContextFromFiles(mapping.CAFile, mapping.CertFile, mapping.KeyFile),
			}
			if mapping.SNI != "" {
				tlsContext.Sni = mapping.SNI
			}

			transportSocket, err := makeUpstreamTLSTransportSocket(tlsContext)
			if err != nil {
				return err
			}
			c.TransportSocket = transportSocket
		}
		if err := injectLBToCluster(lb, c); err != nil {
			return fmt.Errorf("failed to apply load balancer configuration to cluster %q: %v", c.Name, err)
		}

	}
	return nil
}

func (s *ResourceGenerator) clustersFromSnapshotIngressGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var clusters []proto.Message
	createdClusters := make(map[string]bool)
	for _, upstreams := range cfgSnap.IngressGateway.Upstreams {
		for _, u := range upstreams {
			id := u.Identifier()

			// If we've already created a cluster for this upstream, skip it. Multiple listeners may
			// reference the same upstream, so we don't need to create duplicate clusters in that case.
			if createdClusters[id] {
				continue
			}

			chain, ok := cfgSnap.IngressGateway.DiscoveryChain[id]
			if !ok {
				// this should not happen
				return nil, fmt.Errorf("no discovery chain for upstream %q", id)
			}

			chainEndpoints, ok := cfgSnap.IngressGateway.WatchedUpstreamEndpoints[id]
			if !ok {
				// this should not happen
				return nil, fmt.Errorf("no endpoint map for upstream %q", id)
			}

			upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(id, &u, chain, chainEndpoints, cfgSnap)
			if err != nil {
				return nil, err
			}

			for _, c := range upstreamClusters {
				clusters = append(clusters, c)
			}
			createdClusters[id] = true
		}
	}
	return clusters, nil
}

func (s *ResourceGenerator) makeAppCluster(cfgSnap *proxycfg.ConfigSnapshot, name, pathProtocol string, port int) (*envoy_cluster_v3.Cluster, error) {
	var c *envoy_cluster_v3.Cluster
	var err error

	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// If we have overridden local cluster config try to parse it into an Envoy cluster
	if cfg.LocalClusterJSON != "" {
		return makeClusterFromUserConfig(cfg.LocalClusterJSON)
	}

	var endpoint *envoy_endpoint_v3.LbEndpoint
	if cfgSnap.Proxy.LocalServiceSocketPath != "" {
		endpoint = makePipeEndpoint(cfgSnap.Proxy.LocalServiceSocketPath)
	} else {
		addr := cfgSnap.Proxy.LocalServiceAddress
		if addr == "" {
			addr = "127.0.0.1"
		}
		endpoint = makeEndpoint(addr, port)
	}

	c = &envoy_cluster_v3.Cluster{
		Name:                 name,
		ConnectTimeout:       ptypes.DurationProto(time.Duration(cfg.LocalConnectTimeoutMs) * time.Millisecond),
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC},
		LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						endpoint,
					},
				},
			},
		},
	}
	protocol := pathProtocol
	if protocol == "" {
		protocol = cfg.Protocol
	}
	if protocol == "http2" || protocol == "grpc" {
		c.Http2ProtocolOptions = &envoy_core_v3.Http2ProtocolOptions{}
	}

	return c, err
}

func (s *ResourceGenerator) makeUpstreamClusterForPreparedQuery(upstream structs.Upstream, cfgSnap *proxycfg.ConfigSnapshot) (*envoy_cluster_v3.Cluster, error) {
	var c *envoy_cluster_v3.Cluster
	var err error

	dc := upstream.Datacenter
	if dc == "" {
		dc = cfgSnap.Datacenter
	}
	sni := connect.UpstreamSNI(&upstream, "", dc, cfgSnap.Roots.TrustDomain)

	cfg, err := structs.ParseUpstreamConfig(upstream.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", upstream.Identifier(), "error", err)
	}
	if cfg.EnvoyClusterJSON != "" {
		c, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
		if err != nil {
			return c, err
		}
		// In the happy path don't return yet as we need to inject TLS config still.
	}

	if c == nil {
		c = &envoy_cluster_v3.Cluster{
			Name:                 sni,
			ConnectTimeout:       ptypes.DurationProto(time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond),
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: &envoy_core_v3.ConfigSource{
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
					ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
						Ads: &envoy_core_v3.AggregatedConfigSource{},
					},
				},
			},
			CircuitBreakers: &envoy_cluster_v3.CircuitBreakers{
				Thresholds: makeThresholdsIfNeeded(cfg.Limits),
			},
			OutlierDetection: ToOutlierDetection(cfg.PassiveHealthCheck),
		}
		if cfg.Protocol == "http2" || cfg.Protocol == "grpc" {
			c.Http2ProtocolOptions = &envoy_core_v3.Http2ProtocolOptions{}
		}
	}

	endpoints := cfgSnap.ConnectProxy.PreparedQueryEndpoints[upstream.Identifier()]
	var (
		spiffeIDs = make([]connect.SpiffeIDService, 0)
		seen      = make(map[string]struct{})
	)
	for _, e := range endpoints {
		id := fmt.Sprintf("%s/%s", e.Node.Datacenter, e.Service.CompoundServiceName())
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		name := e.Service.Proxy.DestinationServiceName
		if e.Service.Connect.Native {
			name = e.Service.Service
		}
		spiffeIDs = append(spiffeIDs, connect.SpiffeIDService{
			Host:       cfgSnap.Roots.TrustDomain,
			Namespace:  e.Service.NamespaceOrDefault(),
			Partition:  e.Service.PartitionOrDefault(),
			Datacenter: e.Node.Datacenter,
			Service:    name,
		})
	}

	// Enable TLS upstream with the configured client certificate.
	commonTLSContext := makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf())
	err = injectSANMatcher(commonTLSContext, spiffeIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", sni, err)
	}

	tlsContext := &envoy_tls_v3.UpstreamTlsContext{
		CommonTlsContext: commonTLSContext,
		Sni:              sni,
	}

	transportSocket, err := makeUpstreamTLSTransportSocket(tlsContext)
	if err != nil {
		return nil, err
	}
	c.TransportSocket = transportSocket

	return c, nil
}

func (s *ResourceGenerator) makeUpstreamClustersForDiscoveryChain(
	id string,
	upstream *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	chainEndpoints map[string]structs.CheckServiceNodes,
	cfgSnap *proxycfg.ConfigSnapshot,
) ([]*envoy_cluster_v3.Cluster, error) {
	if chain == nil {
		return nil, fmt.Errorf("cannot create upstream cluster without discovery chain for %s", id)
	}

	configMap := make(map[string]interface{})
	if upstream != nil {
		configMap = upstream.Config
	}
	cfg, err := structs.ParseUpstreamConfigNoDefaults(configMap)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", id,
			"error", err)
	}

	var escapeHatchCluster *envoy_cluster_v3.Cluster
	if cfg.EnvoyClusterJSON != "" {
		if chain.IsDefault() {
			// If you haven't done anything to setup the discovery chain, then
			// you can use the envoy_cluster_json escape hatch.
			escapeHatchCluster, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
			if err != nil {
				return nil, err
			}
		} else {
			s.Logger.Warn("ignoring escape hatch setting, because a discovery chain is configured for",
				"discovery chain", chain.ServiceName, "upstream", id,
				"envoy_cluster_json", chain.ServiceName)
		}
	}

	var out []*envoy_cluster_v3.Cluster
	for _, node := range chain.Nodes {
		if node.Type != structs.DiscoveryGraphNodeTypeResolver {
			continue
		}
		failover := node.Resolver.Failover
		targetID := node.Resolver.Target

		target := chain.Targets[targetID]

		// Determine if we have to generate the entire cluster differently.
		failoverThroughMeshGateway := chain.WillFailoverThroughMeshGateway(node)

		sni := target.SNI
		clusterName := CustomizeClusterName(target.Name, chain)

		targetSpiffeID := connect.SpiffeIDService{
			Host:       cfgSnap.Roots.TrustDomain,
			Namespace:  target.Namespace,
			Partition:  target.Partition,
			Datacenter: target.Datacenter,
			Service:    target.Service,
		}

		if failoverThroughMeshGateway {
			actualTargetID := firstHealthyTarget(
				chain.Targets,
				chainEndpoints,
				targetID,
				failover.Targets,
			)

			if actualTargetID != targetID {
				actualTarget := chain.Targets[actualTargetID]
				sni = actualTarget.SNI
			}
		}

		spiffeIDs := []connect.SpiffeIDService{targetSpiffeID}
		seenIDs := map[string]struct{}{
			targetSpiffeID.URI().String(): {},
		}

		if failover != nil {
			// When failovers are present we need to add them as valid SANs to validate against.
			// Envoy makes the failover decision independently based on the endpoint health it has available.
			for _, tid := range failover.Targets {
				target, ok := chain.Targets[tid]
				if !ok {
					continue
				}

				id := connect.SpiffeIDService{
					Host:       cfgSnap.Roots.TrustDomain,
					Namespace:  target.Namespace,
					Partition:  target.Partition,
					Datacenter: target.Datacenter,
					Service:    target.Service,
				}

				// Failover targets might be subsets of the same service, so these are deduplicated.
				if _, ok := seenIDs[id.URI().String()]; ok {
					continue
				}
				seenIDs[id.URI().String()] = struct{}{}

				spiffeIDs = append(spiffeIDs, id)
			}
		}
		sort.Slice(spiffeIDs, func(i, j int) bool {
			return spiffeIDs[i].URI().String() < spiffeIDs[j].URI().String()
		})

		s.Logger.Debug("generating cluster for", "cluster", clusterName)
		c := &envoy_cluster_v3.Cluster{
			Name:                 clusterName,
			AltStatName:          clusterName,
			ConnectTimeout:       ptypes.DurationProto(node.Resolver.ConnectTimeout),
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS},
			CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{
					Value: 0, // disable panic threshold
				},
			},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: &envoy_core_v3.ConfigSource{
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
					ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
						Ads: &envoy_core_v3.AggregatedConfigSource{},
					},
				},
			},
			CircuitBreakers: &envoy_cluster_v3.CircuitBreakers{
				Thresholds: makeThresholdsIfNeeded(cfg.Limits),
			},
			OutlierDetection: ToOutlierDetection(cfg.PassiveHealthCheck),
		}

		var lb *structs.LoadBalancer
		if node.LoadBalancer != nil {
			lb = node.LoadBalancer
		}
		if err := injectLBToCluster(lb, c); err != nil {
			return nil, fmt.Errorf("failed to apply load balancer configuration to cluster %q: %v", clusterName, err)
		}

		proto := cfg.Protocol
		if proto == "" {
			proto = chain.Protocol
		}

		if proto == "" {
			proto = "tcp"
		}

		if proto == "http2" || proto == "grpc" {
			c.Http2ProtocolOptions = &envoy_core_v3.Http2ProtocolOptions{}
		}

		commonTLSContext := makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf())
		err = injectSANMatcher(commonTLSContext, spiffeIDs...)
		if err != nil {
			return nil, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", sni, err)
		}

		tlsContext := &envoy_tls_v3.UpstreamTlsContext{
			CommonTlsContext: commonTLSContext,
			Sni:              sni,
		}

		transportSocket, err := makeUpstreamTLSTransportSocket(tlsContext)
		if err != nil {
			return nil, err
		}
		c.TransportSocket = transportSocket

		out = append(out, c)
	}

	if escapeHatchCluster != nil {
		if len(out) != 1 {
			return nil, fmt.Errorf("cannot inject escape hatch cluster when discovery chain had no nodes")
		}
		defaultCluster := out[0]

		// Overlay what the user provided.
		escapeHatchCluster.TransportSocket = defaultCluster.TransportSocket

		out = []*envoy_cluster_v3.Cluster{escapeHatchCluster}
	}

	return out, nil
}

// injectSANMatcher updates a TLS context so that it verifies the upstream SAN.
func injectSANMatcher(tlsContext *envoy_tls_v3.CommonTlsContext, spiffeIDs ...connect.SpiffeIDService) error {
	validationCtx, ok := tlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContext)
	if !ok {
		return fmt.Errorf("invalid type: expected CommonTlsContext_ValidationContext, got %T",
			tlsContext.ValidationContextType)
	}

	var matchers []*envoy_matcher_v3.StringMatcher
	for _, id := range spiffeIDs {
		matchers = append(matchers, &envoy_matcher_v3.StringMatcher{
			MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
				Exact: id.URI().String(),
			},
		})
	}
	validationCtx.ValidationContext.MatchSubjectAltNames = matchers

	return nil
}

// makeClusterFromUserConfig returns the listener config decoded from an
// arbitrary proto3 json format string or an error if it's invalid.
//
// For now we only support embedding in JSON strings because of the hcl parsing
// pain (see Background section in the comment for decode.HookWeakDecodeFromSlice).
// This may be fixed in decode.HookWeakDecodeFromSlice in the future.
//
// When we do that we can support just nesting the config directly into the
// JSON/hcl naturally but this is a stop-gap that gets us an escape hatch
// immediately. It's also probably not a bad thing to support long-term since
// any config generated by other systems will likely be in canonical protobuf
// from rather than our slight variant in JSON/hcl.
func makeClusterFromUserConfig(configJSON string) (*envoy_cluster_v3.Cluster, error) {
	// Type field is present so decode it as a types.Any
	var any any.Any
	err := jsonpb.UnmarshalString(configJSON, &any)
	if err != nil {
		return nil, err
	}

	// And then unmarshal the listener again...
	var c envoy_cluster_v3.Cluster
	err = proto.Unmarshal(any.Value, &c)
	if err != nil {
		return nil, err
	}
	return &c, err
}

type gatewayClusterOpts struct {
	// name for the cluster
	name string

	// isRemote determines whether the cluster is in a remote DC and we should prefer a WAN address
	isRemote bool

	// onlyPassing determines whether endpoints that do not have a passing status should be considered unhealthy
	onlyPassing bool

	// connectTimeout is the timeout for new network connections to hosts in the cluster
	connectTimeout time.Duration

	// hostnameEndpoints is a list of endpoints with a hostname as their address
	hostnameEndpoints structs.CheckServiceNodes
}

// makeGatewayCluster creates an Envoy cluster for a mesh or terminating gateway
func (s *ResourceGenerator) makeGatewayCluster(snap *proxycfg.ConfigSnapshot, opts gatewayClusterOpts) *envoy_cluster_v3.Cluster {
	cfg, err := ParseGatewayConfig(snap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse gateway config", "error", err)
	}
	if opts.connectTimeout <= 0 {
		opts.connectTimeout = time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond
	}

	cluster := &envoy_cluster_v3.Cluster{
		Name:           opts.name,
		ConnectTimeout: ptypes.DurationProto(opts.connectTimeout),

		// Having an empty config enables outlier detection with default config.
		OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
	}

	useEDS := true
	if len(opts.hostnameEndpoints) > 0 {
		useEDS = false
	}

	// If none of the service instances are addressed by a hostname we provide the endpoint IP addresses via EDS
	if useEDS {
		cluster.ClusterDiscoveryType = &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS}
		cluster.EdsClusterConfig = &envoy_cluster_v3.Cluster_EdsClusterConfig{
			EdsConfig: &envoy_core_v3.ConfigSource{
				ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
				ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
					Ads: &envoy_core_v3.AggregatedConfigSource{},
				},
			},
		}
		return cluster
	}

	// When a service instance is addressed by a hostname we have Envoy do the DNS resolution
	// by setting a DNS cluster type and passing the hostname endpoints via CDS.
	rate := 10 * time.Second
	cluster.DnsRefreshRate = ptypes.DurationProto(rate)
	cluster.DnsLookupFamily = envoy_cluster_v3.Cluster_V4_ONLY

	discoveryType := envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_LOGICAL_DNS}
	if cfg.DNSDiscoveryType == "strict_dns" {
		discoveryType.Type = envoy_cluster_v3.Cluster_STRICT_DNS
	}
	cluster.ClusterDiscoveryType = &discoveryType

	endpoints := make([]*envoy_endpoint_v3.LbEndpoint, 0, 1)
	uniqueHostnames := make(map[string]bool)

	var (
		hostname string
		idx      int
		fallback *envoy_endpoint_v3.LbEndpoint
	)
	for i, e := range opts.hostnameEndpoints {
		addr, port := e.BestAddress(opts.isRemote)
		uniqueHostnames[addr] = true

		health, weight := calculateEndpointHealthAndWeight(e, opts.onlyPassing)
		if health == envoy_core_v3.HealthStatus_UNHEALTHY {
			fallback = makeLbEndpoint(addr, port, health, weight)
			continue
		}

		if len(endpoints) == 0 {
			endpoints = append(endpoints, makeLbEndpoint(addr, port, health, weight))

			hostname = addr
			idx = i
			break
		}
	}

	dc := opts.hostnameEndpoints[idx].Node.Datacenter
	service := opts.hostnameEndpoints[idx].Service.CompoundServiceName()

	// Fall back to last unhealthy endpoint if none were healthy
	if len(endpoints) == 0 {
		s.Logger.Warn("upstream service does not contain any healthy instances",
			"dc", dc, "service", service.String())

		endpoints = append(endpoints, fallback)
	}
	if len(uniqueHostnames) > 1 {
		s.Logger.Warn(fmt.Sprintf("service contains instances with more than one unique hostname; only %q be resolved by Envoy", hostname),
			"dc", dc, "service", service.String())
	}

	cluster.LoadAssignment = &envoy_endpoint_v3.ClusterLoadAssignment{
		ClusterName: cluster.Name,
		Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
			{
				LbEndpoints: endpoints,
			},
		},
	}
	return cluster
}

func makeThresholdsIfNeeded(limits *structs.UpstreamLimits) []*envoy_cluster_v3.CircuitBreakers_Thresholds {
	if limits == nil {
		return nil
	}

	threshold := &envoy_cluster_v3.CircuitBreakers_Thresholds{}

	// Likewise, make sure to not set any threshold values on the zero-value in
	// order to rely on Envoy defaults
	if limits.MaxConnections != nil {
		threshold.MaxConnections = makeUint32Value(*limits.MaxConnections)
	}
	if limits.MaxPendingRequests != nil {
		threshold.MaxPendingRequests = makeUint32Value(*limits.MaxPendingRequests)
	}
	if limits.MaxConcurrentRequests != nil {
		threshold.MaxRequests = makeUint32Value(*limits.MaxConcurrentRequests)
	}

	return []*envoy_cluster_v3.CircuitBreakers_Thresholds{threshold}
}

func makeLbEndpoint(addr string, port int, health envoy_core_v3.HealthStatus, weight int) *envoy_endpoint_v3.LbEndpoint {
	return &envoy_endpoint_v3.LbEndpoint{
		HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
			Endpoint: &envoy_endpoint_v3.Endpoint{
				Address: &envoy_core_v3.Address{
					Address: &envoy_core_v3.Address_SocketAddress{
						SocketAddress: &envoy_core_v3.SocketAddress{
							Address: addr,
							PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
								PortValue: uint32(port),
							},
						},
					},
				},
			},
		},
		HealthStatus:        health,
		LoadBalancingWeight: makeUint32Value(weight),
	}
}

func injectLBToCluster(ec *structs.LoadBalancer, c *envoy_cluster_v3.Cluster) error {
	if ec == nil {
		return nil
	}

	switch ec.Policy {
	case "":
		return nil
	case structs.LBPolicyLeastRequest:
		c.LbPolicy = envoy_cluster_v3.Cluster_LEAST_REQUEST

		if ec.LeastRequestConfig != nil {
			c.LbConfig = &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
				LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
					ChoiceCount: &wrappers.UInt32Value{Value: ec.LeastRequestConfig.ChoiceCount},
				},
			}
		}
	case structs.LBPolicyRoundRobin:
		c.LbPolicy = envoy_cluster_v3.Cluster_ROUND_ROBIN

	case structs.LBPolicyRandom:
		c.LbPolicy = envoy_cluster_v3.Cluster_RANDOM

	case structs.LBPolicyRingHash:
		c.LbPolicy = envoy_cluster_v3.Cluster_RING_HASH

		if ec.RingHashConfig != nil {
			c.LbConfig = &envoy_cluster_v3.Cluster_RingHashLbConfig_{
				RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
					MinimumRingSize: &wrappers.UInt64Value{Value: ec.RingHashConfig.MinimumRingSize},
					MaximumRingSize: &wrappers.UInt64Value{Value: ec.RingHashConfig.MaximumRingSize},
				},
			}
		}
	case structs.LBPolicyMaglev:
		c.LbPolicy = envoy_cluster_v3.Cluster_MAGLEV

	default:
		return fmt.Errorf("unsupported load balancer policy %q for cluster %q", ec.Policy, c.Name)
	}
	return nil
}
