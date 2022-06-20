package xds

import (
	"errors"
	"fmt"
	"sort"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_cluster_dynamic_forward_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dynamic_forward_proxy/v3"
	envoy_common_dynamic_forward_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/common/dynamic_forward_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_upstreams_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	dynamicForwardProxyClusterName         = "dynamic_forward_proxy_cluster"
	dynamicForwardProxyClusterTypeName     = "envoy.clusters.dynamic_forward_proxy"
	dynamicForwardProxyClusterDNSCacheName = "dynamic_forward_proxy_cache_config"
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
		res, err := s.clustersFromSnapshotTerminatingGateway(cfgSnap)
		if err != nil {
			return nil, err
		}
		return res, nil
	case structs.ServiceKindMeshGateway:
		res, err := s.clustersFromSnapshotMeshGateway(cfgSnap)
		if err != nil {
			return nil, err
		}
		return res, nil
	case structs.ServiceKindIngressGateway:
		res, err := s.clustersFromSnapshotIngressGateway(cfgSnap)
		if err != nil {
			return nil, err
		}
		return res, nil
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
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

	// NOTE: Any time we skip a chain below we MUST also skip that discovery chain in endpoints.go
	// so that the sets of endpoints generated matches the sets of clusters.
	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		upstreamCfg := cfgSnap.ConnectProxy.UpstreamConfig[uid]

		explicit := upstreamCfg.HasLocalPortOrSocket()
		if _, implicit := cfgSnap.ConnectProxy.IntentionUpstreams[uid]; !implicit && !explicit {
			// Discovery chain is not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		chainEndpoints, ok := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[uid]
		if !ok {
			// this should not happen
			return nil, fmt.Errorf("no endpoint map for upstream %q", uid)
		}

		upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(uid, upstreamCfg, chain, chainEndpoints, cfgSnap)
		if err != nil {
			return nil, err
		}

		for _, cluster := range upstreamClusters {
			clusters = append(clusters, cluster)
		}
	}

	// NOTE: Any time we skip an upstream below we MUST also skip that same
	// upstream in endpoints.go so that the sets of endpoints generated matches
	// the sets of clusters.
	//
	// TODO(peering): make this work for tproxy
	for _, uid := range cfgSnap.ConnectProxy.PeeredUpstreamIDs() {
		upstreamCfg := cfgSnap.ConnectProxy.UpstreamConfig[uid]

		explicit := upstreamCfg.HasLocalPortOrSocket()
		if _, implicit := cfgSnap.ConnectProxy.IntentionUpstreams[uid]; !implicit && !explicit {
			// Not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		peerMeta := cfgSnap.ConnectProxy.UpstreamPeerMeta(uid)

		upstreamCluster, err := s.makeUpstreamClusterForPeerService(upstreamCfg, peerMeta, cfgSnap)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, upstreamCluster)
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
		for _, check := range cfgSnap.ConnectProxy.WatchedServiceChecks[psid] {
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

	if meshConf := cfgSnap.MeshConfig(); meshConf == nil ||
		!meshConf.TransparentProxy.MeshDestinationsOnly {

		clusters = append(clusters, &envoy_cluster_v3.Cluster{
			Name: OriginalDestinationClusterName,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
			},
			LbPolicy:       envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,
			ConnectTimeout: durationpb.New(5 * time.Second),
		})
	}

	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		targetMap, ok := cfgSnap.ConnectProxy.PassthroughUpstreams[uid]
		if !ok {
			continue
		}

		for targetID := range targetMap {
			uid := proxycfg.NewUpstreamIDFromTargetID(targetID)

			sni := connect.ServiceSNI(
				uid.Name, "", uid.NamespaceOrDefault(), uid.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

			// Prefixed with passthrough to distinguish from non-passthrough clusters for the same upstream.
			name := "passthrough~" + sni

			c := envoy_cluster_v3.Cluster{
				Name: name,
				ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
					Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
				},
				LbPolicy: envoy_cluster_v3.Cluster_CLUSTER_PROVIDED,

				ConnectTimeout: durationpb.New(5 * time.Second),
			}

			if discoTarget, ok := chain.Targets[targetID]; ok && discoTarget.ConnectTimeout > 0 {
				c.ConnectTimeout = durationpb.New(discoTarget.ConnectTimeout)
			}

			spiffeID := connect.SpiffeIDService{
				Host:       cfgSnap.Roots.TrustDomain,
				Partition:  uid.PartitionOrDefault(),
				Namespace:  uid.NamespaceOrDefault(),
				Datacenter: cfgSnap.Datacenter,
				Service:    uid.Name,
			}

			commonTLSContext := makeCommonTLSContext(
				cfgSnap.Leaf(),
				cfgSnap.RootPEMs(),
				makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing()),
			)
			err := injectSANMatcher(commonTLSContext, spiffeID.URI().String())
			if err != nil {
				return nil, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", sni, err)
			}
			tlsContext := envoy_tls_v3.UpstreamTlsContext{
				CommonTlsContext: commonTLSContext,
				Sni:              sni,
			}
			transportSocket, err := makeUpstreamTLSTransportSocket(&tlsContext)
			if err != nil {
				return nil, err
			}
			c.TransportSocket = transportSocket
			clusters = append(clusters, &c)
		}
	}

	return clusters, nil
}

// clustersFromSnapshotMeshGateway returns the xDS API representation of the "clusters"
// for a mesh gateway. This will include 1 cluster per remote datacenter as well as
// 1 cluster for each service subset.
func (s *ResourceGenerator) clustersFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	keys := cfgSnap.MeshGateway.GatewayKeys()

	// 1 cluster per remote dc/partition + 1 cluster per local service (this is a lower bound - all subset specific clusters will be appended)
	clusters := make([]proto.Message, 0, len(keys)+len(cfgSnap.MeshGateway.ServiceGroups))

	// Generate the remote clusters
	for _, key := range keys {
		if key.Matches(cfgSnap.Datacenter, cfgSnap.ProxyID.PartitionOrDefault()) {
			continue // skip local
		}

		opts := clusterOpts{
			name:              connect.GatewaySNI(key.Datacenter, key.Partition, cfgSnap.Roots.TrustDomain),
			hostnameEndpoints: cfgSnap.MeshGateway.HostnameDatacenters[key.String()],
			isRemote:          true,
		}
		cluster := s.makeGatewayCluster(cfgSnap, opts)
		clusters = append(clusters, cluster)
	}

	if cfgSnap.ProxyID.InDefaultPartition() &&
		cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" &&
		cfgSnap.ServerSNIFn != nil {

		// Add all of the remote wildcard datacenter mappings for servers.
		for _, key := range keys {
			hostnameEndpoints := cfgSnap.MeshGateway.HostnameDatacenters[key.String()]

			// If the DC is our current DC then this cluster is for traffic from a remote DC to a local server.
			// HostnameDatacenters is populated with gateway addresses, so it does not apply here.
			if key.Datacenter == cfgSnap.Datacenter {
				hostnameEndpoints = nil
			}
			opts := clusterOpts{
				name:              cfgSnap.ServerSNIFn(key.Datacenter, ""),
				hostnameEndpoints: hostnameEndpoints,
				isRemote:          !key.Matches(cfgSnap.Datacenter, cfgSnap.ProxyID.PartitionOrDefault()),
			}
			cluster := s.makeGatewayCluster(cfgSnap, opts)
			clusters = append(clusters, cluster)
		}

		// And for the current datacenter, send all flavors appropriately.
		for _, srv := range cfgSnap.MeshGateway.ConsulServers {
			opts := clusterOpts{
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

// clustersFromSnapshotTerminatingGateway returns the xDS API representation of the "clusters"
// for a terminating gateway. This will include 1 cluster per Destination associated with this terminating gateway.
func (s *ResourceGenerator) clustersFromSnapshotTerminatingGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	res := []proto.Message{}
	gwClusters, err := s.makeGatewayServiceClusters(cfgSnap, cfgSnap.TerminatingGateway.ServiceGroups, cfgSnap.TerminatingGateway.ServiceResolvers)
	if err != nil {
		return nil, err
	}
	res = append(res, gwClusters...)

	destClusters, err := s.makeDestinationClusters(cfgSnap)
	if err != nil {
		return nil, err
	}
	res = append(res, destClusters...)

	return res, nil
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

		var isRemote bool
		if len(services[svc]) > 0 {
			isRemote = !cfgSnap.Locality.Matches(services[svc][0].Node.Datacenter, services[svc][0].Node.PartitionOrDefault())
		}

		opts := clusterOpts{
			name:              clusterName,
			hostnameEndpoints: hostnameEndpoints,
			connectTimeout:    resolver.ConnectTimeout,
			isRemote:          isRemote,
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

			opts := clusterOpts{
				name:              connect.ServiceSNI(svc.Name, name, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain),
				hostnameEndpoints: subsetHostnameEndpoints,
				onlyPassing:       subset.OnlyPassing,
				connectTimeout:    resolver.ConnectTimeout,
				isRemote:          isRemote,
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

func (s *ResourceGenerator) makeDestinationClusters(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var createDynamicForwardProxy bool
	serviceConfigs := cfgSnap.TerminatingGateway.ServiceConfigs

	clusters := make([]proto.Message, 0, len(cfgSnap.TerminatingGateway.DestinationServices))

	for _, svcName := range cfgSnap.TerminatingGateway.ValidDestinations() {
		svcConfig, _ := serviceConfigs[svcName]
		dest := svcConfig.Destination

		// If IP, create a cluster with the fake name.
		if dest.HasIP() {
			opts := clusterOpts{
				name:            connect.ServiceSNI(svcName.Name, "", svcName.NamespaceOrDefault(), svcName.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain),
				addressEndpoint: dest,
			}
			cluster := s.makeTerminatingIPCluster(cfgSnap, opts)
			clusters = append(clusters, cluster)
			continue
		}

		// TODO (dans): clusters will need to be customized later when we figure out how to manage a TLS segment from the terminating gateway to the Destination.
		createDynamicForwardProxy = true
	}

	if createDynamicForwardProxy {
		opts := clusterOpts{
			name: dynamicForwardProxyClusterName,
		}
		cluster := s.makeDynamicForwardProxyCluster(cfgSnap, opts)

		// TODO (dans): might be relevant later for TLS addons like CA validation
		//if err := s.injectGatewayServiceAddons(cfgSnap, cluster, svc, loadBalancer); err != nil {
		//	return nil, err
		//}
		clusters = append(clusters, cluster)
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
				if err := injectSANMatcher(tlsContext.CommonTlsContext, mapping.SNI); err != nil {
					return fmt.Errorf("failed to inject SNI matcher into TLS context: %v", err)
				}
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
	createdClusters := make(map[proxycfg.UpstreamID]bool)
	for _, upstreams := range cfgSnap.IngressGateway.Upstreams {
		for _, u := range upstreams {
			uid := proxycfg.NewUpstreamID(&u)

			// If we've already created a cluster for this upstream, skip it. Multiple listeners may
			// reference the same upstream, so we don't need to create duplicate clusters in that case.
			if createdClusters[uid] {
				continue
			}

			chain, ok := cfgSnap.IngressGateway.DiscoveryChain[uid]
			if !ok {
				// this should not happen
				return nil, fmt.Errorf("no discovery chain for upstream %q", uid)
			}

			chainEndpoints, ok := cfgSnap.IngressGateway.WatchedUpstreamEndpoints[uid]
			if !ok {
				// this should not happen
				return nil, fmt.Errorf("no endpoint map for upstream %q", uid)
			}

			upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(uid, &u, chain, chainEndpoints, cfgSnap)
			if err != nil {
				return nil, err
			}

			for _, c := range upstreamClusters {
				clusters = append(clusters, c)
			}
			createdClusters[uid] = true
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
		ConnectTimeout:       durationpb.New(time.Duration(cfg.LocalConnectTimeoutMs) * time.Millisecond),
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
		if err := s.setHttp2ProtocolOptions(c); err != nil {
			return c, err
		}
	}
	if cfg.MaxInboundConnections > 0 {
		c.CircuitBreakers = &envoy_cluster_v3.CircuitBreakers{
			Thresholds: []*envoy_cluster_v3.CircuitBreakers_Thresholds{
				{
					MaxConnections: makeUint32Value(cfg.MaxInboundConnections),
				},
			},
		}
	}

	return c, err
}

func (s *ResourceGenerator) makeUpstreamClusterForPeerService(
	upstream *structs.Upstream,
	peerMeta structs.PeeringServiceMeta,
	cfgSnap *proxycfg.ConfigSnapshot,
) (*envoy_cluster_v3.Cluster, error) {
	var (
		c   *envoy_cluster_v3.Cluster
		err error
	)

	uid := proxycfg.NewUpstreamID(upstream)

	cfg := s.getAndModifyUpstreamConfigForPeeredListener(uid, upstream, peerMeta)
	if cfg.EnvoyClusterJSON != "" {
		c, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
		if err != nil {
			return c, err
		}
		// In the happy path don't return yet as we need to inject TLS config still.
	}

	// TODO(peering): if we replicated service metadata separately from the
	// instances we wouldn't have to flip/flop this cluster name like this.
	clusterName := peerMeta.PrimarySNI()
	if clusterName == "" {
		clusterName = uid.EnvoyID()
	}

	s.Logger.Trace("generating cluster for", "cluster", clusterName)
	if c == nil {
		c = &envoy_cluster_v3.Cluster{
			Name:           clusterName,
			AltStatName:    clusterName,
			ConnectTimeout: durationpb.New(time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond),
			CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{
					Value: 0, // disable panic threshold
				},
			},
			CircuitBreakers: &envoy_cluster_v3.CircuitBreakers{
				Thresholds: makeThresholdsIfNeeded(cfg.Limits),
			},
			OutlierDetection: ToOutlierDetection(cfg.PassiveHealthCheck),
		}
		if cfg.Protocol == "http2" || cfg.Protocol == "grpc" {
			if err := s.setHttp2ProtocolOptions(c); err != nil {
				return c, err
			}
		}

		useEDS := true
		if _, ok := cfgSnap.ConnectProxy.PeerUpstreamEndpointsUseHostnames[uid]; ok {
			useEDS = false
		}

		// If none of the service instances are addressed by a hostname we
		// provide the endpoint IP addresses via EDS
		if useEDS {
			c.ClusterDiscoveryType = &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS}
			c.EdsClusterConfig = &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: &envoy_core_v3.ConfigSource{
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
					ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
						Ads: &envoy_core_v3.AggregatedConfigSource{},
					},
				},
			}
		} else {
			configureClusterWithHostnames(
				s.Logger,
				c,
				"", /*TODO:make configurable?*/
				cfgSnap.ConnectProxy.PeerUpstreamEndpoints[uid],
				true,  /*isRemote*/
				false, /*onlyPassing*/
			)
		}

	}

	rootPEMs := cfgSnap.RootPEMs()
	if uid.Peer != "" {
		rootPEMs = cfgSnap.ConnectProxy.PeerTrustBundles[uid.Peer].ConcatenatedRootPEMs()
	}

	// Enable TLS upstream with the configured client certificate.
	commonTLSContext := makeCommonTLSContext(
		cfgSnap.Leaf(),
		rootPEMs,
		makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing()),
	)
	err = injectSANMatcher(commonTLSContext, peerMeta.SpiffeID...)
	if err != nil {
		return nil, fmt.Errorf("failed to inject SAN matcher rules for cluster %q: %v", clusterName, err)
	}

	tlsContext := &envoy_tls_v3.UpstreamTlsContext{
		CommonTlsContext: commonTLSContext,
		Sni:              peerMeta.PrimarySNI(),
	}

	transportSocket, err := makeUpstreamTLSTransportSocket(tlsContext)
	if err != nil {
		return nil, err
	}
	c.TransportSocket = transportSocket

	return c, nil
}

func (s *ResourceGenerator) makeUpstreamClusterForPreparedQuery(upstream structs.Upstream, cfgSnap *proxycfg.ConfigSnapshot) (*envoy_cluster_v3.Cluster, error) {
	var c *envoy_cluster_v3.Cluster
	var err error

	uid := proxycfg.NewUpstreamID(&upstream)

	dc := upstream.Datacenter
	if dc == "" {
		dc = cfgSnap.Datacenter
	}
	sni := connect.UpstreamSNI(&upstream, "", dc, cfgSnap.Roots.TrustDomain)

	cfg, err := structs.ParseUpstreamConfig(upstream.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", uid, "error", err)
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
			ConnectTimeout:       durationpb.New(time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond),
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
			if err := s.setHttp2ProtocolOptions(c); err != nil {
				return c, err
			}
		}
	}

	endpoints := cfgSnap.ConnectProxy.PreparedQueryEndpoints[uid]
	var (
		spiffeIDs = make([]string, 0)
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
		}.URI().String())
	}

	// Enable TLS upstream with the configured client certificate.
	commonTLSContext := makeCommonTLSContext(
		cfgSnap.Leaf(),
		cfgSnap.RootPEMs(),
		makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing()),
	)
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
	uid proxycfg.UpstreamID,
	upstream *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	chainEndpoints map[string]structs.CheckServiceNodes,
	cfgSnap *proxycfg.ConfigSnapshot,
) ([]*envoy_cluster_v3.Cluster, error) {
	if chain == nil {
		return nil, fmt.Errorf("cannot create upstream cluster without discovery chain for %s", uid)
	}

	configMap := make(map[string]interface{})
	if upstream != nil {
		configMap = upstream.Config
	}
	cfg, err := structs.ParseUpstreamConfigNoDefaults(configMap)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", uid,
			"error", err)
	}

	var escapeHatchCluster *envoy_cluster_v3.Cluster
	if cfg.EnvoyClusterJSON != "" {
		if chain.Default {
			// If you haven't done anything to setup the discovery chain, then
			// you can use the envoy_cluster_json escape hatch.
			escapeHatchCluster, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
			if err != nil {
				return nil, err
			}
		} else {
			s.Logger.Warn("ignoring escape hatch setting, because a discovery chain is configured for",
				"discovery chain", chain.ServiceName, "upstream", uid,
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

		// Get the SpiffeID for upstream SAN validation.
		//
		// For imported services the SpiffeID is embedded in the proxy instances.
		// Whereas for local services we can construct the SpiffeID from the chain target.
		var targetSpiffeID string
		if uid.Peer != "" {
			for _, e := range chainEndpoints[targetID] {
				targetSpiffeID = e.Service.Connect.PeerMeta.SpiffeID[0]

				// Only grab the first because it is the same for all instances.
				break
			}
		} else {
			targetSpiffeID = connect.SpiffeIDService{
				Host:       cfgSnap.Roots.TrustDomain,
				Namespace:  target.Namespace,
				Partition:  target.Partition,
				Datacenter: target.Datacenter,
				Service:    target.Service,
			}.URI().String()
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

		spiffeIDs := []string{targetSpiffeID}
		seenIDs := map[string]struct{}{
			targetSpiffeID: {},
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
				}.URI().String()

				// Failover targets might be subsets of the same service, so these are deduplicated.
				if _, ok := seenIDs[id]; ok {
					continue
				}
				seenIDs[id] = struct{}{}

				spiffeIDs = append(spiffeIDs, id)
			}
		}
		sort.Strings(spiffeIDs)

		s.Logger.Trace("generating cluster for", "cluster", clusterName)
		c := &envoy_cluster_v3.Cluster{
			Name:                 clusterName,
			AltStatName:          clusterName,
			ConnectTimeout:       durationpb.New(node.Resolver.ConnectTimeout),
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
			if err := s.setHttp2ProtocolOptions(c); err != nil {
				return nil, err
			}
		}

		rootPEMs := cfgSnap.RootPEMs()
		if uid.Peer != "" {
			rootPEMs = cfgSnap.ConnectProxy.PeerTrustBundles[uid.Peer].ConcatenatedRootPEMs()
		}
		commonTLSContext := makeCommonTLSContext(
			cfgSnap.Leaf(),
			rootPEMs,
			makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing()),
		)

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
func injectSANMatcher(tlsContext *envoy_tls_v3.CommonTlsContext, matchStrings ...string) error {
	validationCtx, ok := tlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContext)
	if !ok {
		return fmt.Errorf("invalid type: expected CommonTlsContext_ValidationContext, got %T",
			tlsContext.ValidationContextType)
	}

	var matchers []*envoy_matcher_v3.StringMatcher
	for _, m := range matchStrings {
		matchers = append(matchers, &envoy_matcher_v3.StringMatcher{
			MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
				Exact: m,
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

type clusterOpts struct {
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

	// addressEndpoint is a singular ip/port endpoint
	addressEndpoint structs.DestinationConfig
}

// makeGatewayCluster creates an Envoy cluster for a mesh or terminating gateway
func (s *ResourceGenerator) makeGatewayCluster(snap *proxycfg.ConfigSnapshot, opts clusterOpts) *envoy_cluster_v3.Cluster {
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
		ConnectTimeout: durationpb.New(opts.connectTimeout),

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
	} else {
		configureClusterWithHostnames(
			s.Logger,
			cluster,
			cfg.DNSDiscoveryType,
			opts.hostnameEndpoints,
			opts.isRemote,
			opts.onlyPassing,
		)
	}

	return cluster
}

func configureClusterWithHostnames(
	logger hclog.Logger,
	cluster *envoy_cluster_v3.Cluster,
	dnsDiscoveryType string,
	// hostnameEndpoints is a list of endpoints with a hostname as their address
	hostnameEndpoints structs.CheckServiceNodes,
	// isRemote determines whether the cluster is in a remote DC or partition and we should prefer a WAN address
	isRemote bool,
	// onlyPassing determines whether endpoints that do not have a passing status should be considered unhealthy
	onlyPassing bool,
) {
	// When a service instance is addressed by a hostname we have Envoy do the DNS resolution
	// by setting a DNS cluster type and passing the hostname endpoints via CDS.
	rate := 10 * time.Second
	cluster.DnsRefreshRate = durationpb.New(rate)
	cluster.DnsLookupFamily = envoy_cluster_v3.Cluster_V4_ONLY

	discoveryType := envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_LOGICAL_DNS}
	if dnsDiscoveryType == "strict_dns" {
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
	for i, e := range hostnameEndpoints {
		_, addr, port := e.BestAddress(isRemote)
		uniqueHostnames[addr] = true

		health, weight := calculateEndpointHealthAndWeight(e, onlyPassing)
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

	dc := hostnameEndpoints[idx].Node.Datacenter
	service := hostnameEndpoints[idx].Service.CompoundServiceName()

	// Fall back to last unhealthy endpoint if none were healthy
	if len(endpoints) == 0 {
		logger.Warn("upstream service does not contain any healthy instances",
			"dc", dc, "service", service.String())

		endpoints = append(endpoints, fallback)
	}
	if len(uniqueHostnames) > 1 {
		logger.Warn(fmt.Sprintf("service contains instances with more than one unique hostname; only %q be resolved by Envoy", hostname),
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
}

// makeGatewayCluster creates an Envoy cluster for a mesh or terminating gateway
func (s *ResourceGenerator) makeTerminatingIPCluster(snap *proxycfg.ConfigSnapshot, opts clusterOpts) *envoy_cluster_v3.Cluster {
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
		ConnectTimeout: durationpb.New(opts.connectTimeout),

		// Having an empty config enables outlier detection with default config.
		OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
	}

	discoveryType := envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC}
	cluster.ClusterDiscoveryType = &discoveryType

	endpoints := []*envoy_endpoint_v3.LbEndpoint{
		makeEndpoint(opts.addressEndpoint.Address, opts.addressEndpoint.Port),
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

// makeDynamicForwardProxyCluster creates an Envoy cluster for that routes based on the SNI header received at the listener
func (s *ResourceGenerator) makeDynamicForwardProxyCluster(snap *proxycfg.ConfigSnapshot, opts clusterOpts) *envoy_cluster_v3.Cluster {
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
		ConnectTimeout: durationpb.New(opts.connectTimeout),
	}

	dynamicForwardProxyCluster, err := anypb.New(&envoy_cluster_dynamic_forward_proxy_v3.ClusterConfig{
		DnsCacheConfig: getCommonDNSCacheConfiguration(),
	})
	if err != nil {
		// we should never get here since this message is static
		s.Logger.Error("failed serialize dynamic forward proxy cluster config", "error", err)
	}

	cluster.LbPolicy = envoy_cluster_v3.Cluster_CLUSTER_PROVIDED
	cluster.ClusterDiscoveryType = &envoy_cluster_v3.Cluster_ClusterType{
		ClusterType: &envoy_cluster_v3.Cluster_CustomClusterType{
			Name:        dynamicForwardProxyClusterTypeName,
			TypedConfig: dynamicForwardProxyCluster,
		},
	}

	return cluster
}

func getCommonDNSCacheConfiguration() *envoy_common_dynamic_forward_proxy_v3.DnsCacheConfig {
	return &envoy_common_dynamic_forward_proxy_v3.DnsCacheConfig{
		Name:            dynamicForwardProxyClusterDNSCacheName,
		DnsLookupFamily: envoy_cluster_v3.Cluster_AUTO,
	}
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

func (s *ResourceGenerator) setHttp2ProtocolOptions(c *envoy_cluster_v3.Cluster) error {
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
