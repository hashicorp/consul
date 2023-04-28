package xds

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
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
	"github.com/hashicorp/consul/proto/pbpeering"
)

const (
	meshGatewayExportedClusterNamePrefix = "exported~"
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
		upstream := cfgSnap.ConnectProxy.UpstreamConfig[uid]

		explicit := upstream.HasLocalPortOrSocket()
		implicit := cfgSnap.ConnectProxy.IsImplicitUpstream(uid)
		if !implicit && !explicit {
			// Discovery chain is not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		chainEndpoints, ok := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[uid]
		if !ok {
			// this should not happen
			return nil, fmt.Errorf("no endpoint map for upstream %q", uid)
		}

		upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(
			uid,
			upstream,
			chain,
			chainEndpoints,
			cfgSnap,
			false,
		)
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
	for _, uid := range cfgSnap.ConnectProxy.PeeredUpstreamIDs() {
		upstreamCfg := cfgSnap.ConnectProxy.UpstreamConfig[uid]

		explicit := upstreamCfg.HasLocalPortOrSocket()
		implicit := cfgSnap.ConnectProxy.IsImplicitUpstream(uid)
		if !implicit && !explicit {
			// Not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		peerMeta := cfgSnap.ConnectProxy.UpstreamPeerMeta(uid)

		upstreamCluster, err := s.makeUpstreamClusterForPeerService(uid, upstreamCfg, peerMeta, cfgSnap)
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

			transportSocket, err := makeMTLSTransportSocket(cfgSnap, uid, sni)
			if err != nil {
				return nil, err
			}
			c.TransportSocket = transportSocket
			clusters = append(clusters, &c)
		}
	}

	err := cfgSnap.ConnectProxy.DestinationsUpstream.ForEachKeyE(func(uid proxycfg.UpstreamID) error {
		svcConfig, ok := cfgSnap.ConnectProxy.DestinationsUpstream.Get(uid)
		if !ok || svcConfig.Destination == nil {
			return nil
		}

		// One Cluster per Destination Address
		for _, address := range svcConfig.Destination.Addresses {
			name := clusterNameForDestination(cfgSnap, uid.Name, address, uid.NamespaceOrDefault(), uid.PartitionOrDefault())

			c := envoy_cluster_v3.Cluster{
				Name:           name,
				AltStatName:    name,
				ConnectTimeout: durationpb.New(5 * time.Second),
				CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
					HealthyPanicThreshold: &envoy_type_v3.Percent{
						Value: 0, // disable panic threshold
					},
				},
				ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS},
				EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
					EdsConfig: &envoy_core_v3.ConfigSource{
						ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
						ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
							Ads: &envoy_core_v3.AggregatedConfigSource{},
						},
					},
				},
				// Endpoints are managed separately by EDS
				// Having an empty config enables outlier detection with default config.
				OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			}

			// Use the cluster name as the SNI to match on in the terminating gateway
			transportSocket, err := makeMTLSTransportSocket(cfgSnap, uid, name)
			if err != nil {
				return err
			}
			c.TransportSocket = transportSocket
			clusters = append(clusters, &c)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func makeMTLSTransportSocket(cfgSnap *proxycfg.ConfigSnapshot, uid proxycfg.UpstreamID, sni string) (*envoy_core_v3.TransportSocket, error) {
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
	return transportSocket, nil
}

func clusterNameForDestination(cfgSnap *proxycfg.ConfigSnapshot, name string, address string, namespace string, partition string) string {
	name = destinationSpecificServiceName(name, address)
	sni := connect.ServiceSNI(name, "", namespace, partition, cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

	// Prefixed with destination to distinguish from non-passthrough clusters for the same upstream.
	return "destination." + sni
}

func destinationSpecificServiceName(name string, address string) string {
	address = strings.ReplaceAll(address, ":", "-")
	address = strings.ReplaceAll(address, ".", "-")
	return fmt.Sprintf("%s.%s", address, name)
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

	// Generate per-target clusters for all exported discovery chains.
	c, err = s.makeExportedUpstreamClustersForMeshGateway(cfgSnap)
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

		svcConfig, ok := cfgSnap.TerminatingGateway.ServiceConfigs[svc]
		isHTTP2 := false
		if ok {
			upstreamCfg, err := structs.ParseUpstreamConfig(svcConfig.ProxyConfig)
			if err != nil {
				// Don't hard fail on a config typo, just warn. The parse func returns
				// default config if there is an error so it's safe to continue.
				s.Logger.Warn("failed to parse", "upstream", svc, "error", err)
			}
			isHTTP2 = upstreamCfg.Protocol == "http2" || upstreamCfg.Protocol == "grpc"
		}

		if isHTTP2 {
			if err := s.setHttp2ProtocolOptions(cluster); err != nil {
				return nil, err
			}
		}

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
			if isHTTP2 {
				if err := s.setHttp2ProtocolOptions(cluster); err != nil {
					return nil, err
				}
			}
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

func (s *ResourceGenerator) makeDestinationClusters(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	serviceConfigs := cfgSnap.TerminatingGateway.ServiceConfigs

	clusters := make([]proto.Message, 0, len(cfgSnap.TerminatingGateway.DestinationServices))

	for _, svcName := range cfgSnap.TerminatingGateway.ValidDestinations() {
		svcConfig, _ := serviceConfigs[svcName]
		dest := svcConfig.Destination

		for _, address := range dest.Addresses {
			opts := clusterOpts{
				name:    clusterNameForDestination(cfgSnap, svcName.Name, address, svcName.NamespaceOrDefault(), svcName.PartitionOrDefault()),
				address: address,
				port:    dest.Port,
			}

			var cluster *envoy_cluster_v3.Cluster
			if structs.IsIP(address) {
				cluster = s.makeTerminatingIPCluster(cfgSnap, opts)
			} else {
				cluster = s.makeTerminatingHostnameCluster(cfgSnap, opts)
			}
			if err := s.injectGatewayDestinationAddons(cfgSnap, cluster, svcName); err != nil {
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

func (s *ResourceGenerator) injectGatewayDestinationAddons(cfgSnap *proxycfg.ConfigSnapshot, c *envoy_cluster_v3.Cluster, svc structs.ServiceName) error {
	switch cfgSnap.Kind {
	case structs.ServiceKindTerminatingGateway:
		// Context used for TLS origination to the cluster
		if mapping, ok := cfgSnap.TerminatingGateway.DestinationServices[svc]; ok && mapping.CAFile != "" {
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

	}
	return nil
}

func (s *ResourceGenerator) clustersFromSnapshotIngressGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var clusters []proto.Message
	createdClusters := make(map[proxycfg.UpstreamID]bool)
	for listenerKey, upstreams := range cfgSnap.IngressGateway.Upstreams {
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

			upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(
				uid,
				&u,
				chain,
				chainEndpoints,
				cfgSnap,
				false,
			)
			if err != nil {
				return nil, err
			}

			for _, c := range upstreamClusters {
				s.configIngressUpstreamCluster(c, cfgSnap, listenerKey, &u)
				clusters = append(clusters, c)
			}
			createdClusters[uid] = true
		}
	}
	return clusters, nil
}

func (s *ResourceGenerator) configIngressUpstreamCluster(c *envoy_cluster_v3.Cluster, cfgSnap *proxycfg.ConfigSnapshot, listenerKey proxycfg.IngressListenerKey, u *structs.Upstream) {
	var threshold *envoy_cluster_v3.CircuitBreakers_Thresholds
	setThresholdLimit := func(limitType string, limit int) {
		if limit <= 0 {
			return
		}

		if threshold == nil {
			threshold = &envoy_cluster_v3.CircuitBreakers_Thresholds{}
		}

		switch limitType {
		case "max_connections":
			threshold.MaxConnections = makeUint32Value(limit)
		case "max_pending_requests":
			threshold.MaxPendingRequests = makeUint32Value(limit)
		case "max_requests":
			threshold.MaxRequests = makeUint32Value(limit)
		}
	}

	setThresholdLimit("max_connections", int(cfgSnap.IngressGateway.Defaults.MaxConnections))
	setThresholdLimit("max_pending_requests", int(cfgSnap.IngressGateway.Defaults.MaxPendingRequests))
	setThresholdLimit("max_requests", int(cfgSnap.IngressGateway.Defaults.MaxConcurrentRequests))

	// Adjust the limit for upstream service
	// Lookup listener and service config details from ingress gateway
	// definition.
	var svc *structs.IngressService
	if lCfg, ok := cfgSnap.IngressGateway.Listeners[listenerKey]; ok {
		svc = findIngressServiceMatchingUpstream(lCfg, *u)
	}

	if svc != nil {
		setThresholdLimit("max_connections", int(svc.MaxConnections))
		setThresholdLimit("max_pending_requests", int(svc.MaxPendingRequests))
		setThresholdLimit("max_requests", int(svc.MaxConcurrentRequests))
	}

	if threshold != nil {
		c.CircuitBreakers.Thresholds = []*envoy_cluster_v3.CircuitBreakers_Thresholds{threshold}
	}
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
	uid proxycfg.UpstreamID,
	upstream *structs.Upstream,
	peerMeta structs.PeeringServiceMeta,
	cfgSnap *proxycfg.ConfigSnapshot,
) (*envoy_cluster_v3.Cluster, error) {
	var (
		c   *envoy_cluster_v3.Cluster
		err error
	)

	cfg := s.getAndModifyUpstreamConfigForPeeredListener(uid, upstream, peerMeta)
	if cfg.EnvoyClusterJSON != "" {
		c, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
		if err != nil {
			return c, err
		}
		// In the happy path don't return yet as we need to inject TLS config still.
	}

	tbs, ok := cfgSnap.ConnectProxy.UpstreamPeerTrustBundles.Get(uid.Peer)
	if !ok {
		// this should never happen since we loop through upstreams with
		// set trust bundles
		return c, fmt.Errorf("trust bundle not ready for peer %s", uid.Peer)
	}

	clusterName := generatePeeredClusterName(uid, tbs)

	s.Logger.Trace("generating cluster for", "cluster", clusterName)
	if c == nil {
		c = &envoy_cluster_v3.Cluster{
			Name:           clusterName,
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
			ep, _ := cfgSnap.ConnectProxy.PeerUpstreamEndpoints.Get(uid)
			configureClusterWithHostnames(
				s.Logger,
				c,
				"", /*TODO:make configurable?*/
				ep,
				true,  /*isRemote*/
				false, /*onlyPassing*/
			)
		}

	}

	rootPEMs := cfgSnap.RootPEMs()
	if uid.Peer != "" {
		tbs, _ := cfgSnap.ConnectProxy.UpstreamPeerTrustBundles.Get(uid.Peer)
		rootPEMs = tbs.ConcatenatedRootPEMs()
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
	forMeshGateway bool,
) ([]*envoy_cluster_v3.Cluster, error) {
	if chain == nil {
		return nil, fmt.Errorf("cannot create upstream cluster without discovery chain for %s", uid)
	}

	if uid.Peer != "" && forMeshGateway {
		return nil, fmt.Errorf("impossible to get a peer discovery chain in a mesh gateway")
	}

	upstreamConfigMap := make(map[string]interface{})
	if upstream != nil {
		upstreamConfigMap = upstream.Config
	}

	cfg, err := structs.ParseUpstreamConfigNoDefaults(upstreamConfigMap)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", uid,
			"error", err)
	}

	var escapeHatchCluster *envoy_cluster_v3.Cluster
	if !forMeshGateway {
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
	}

	var out []*envoy_cluster_v3.Cluster
	for _, node := range chain.Nodes {
		if node.Type != structs.DiscoveryGraphNodeTypeResolver {
			continue
		}
		failover := node.Resolver.Failover
		targetID := node.Resolver.Target

		target := chain.Targets[targetID]

		if forMeshGateway && !cfgSnap.Locality.Matches(target.Datacenter, target.Partition) {
			s.Logger.Warn("ignoring discovery chain target that crosses a datacenter or partition boundary in a mesh gateway",
				"target", target,
				"gatewayLocality", cfgSnap.Locality,
			)
			continue
		}

		// Determine if we have to generate the entire cluster differently.
		failoverThroughMeshGateway := chain.WillFailoverThroughMeshGateway(node) && !forMeshGateway

		sni := target.SNI
		clusterName := CustomizeClusterName(target.Name, chain)
		if forMeshGateway {
			clusterName = meshGatewayExportedClusterNamePrefix + clusterName
		}

		// Get the SpiffeID for upstream SAN validation.
		//
		// For imported services the SpiffeID is embedded in the proxy instances.
		// Whereas for local services we can construct the SpiffeID from the chain target.
		var targetSpiffeID string
		var additionalSpiffeIDs []string
		if uid.Peer != "" {
			for _, e := range chainEndpoints[targetID] {
				targetSpiffeID = e.Service.Connect.PeerMeta.SpiffeID[0]
				additionalSpiffeIDs = e.Service.Connect.PeerMeta.SpiffeID[1:]

				// Only grab the first instance because it is the same for all instances.
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

		spiffeIDs := append([]string{targetSpiffeID}, additionalSpiffeIDs...)
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
			// TODO(peering): make circuit breakers or outlier detection work?
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

		var proto string
		if !forMeshGateway {
			proto = cfg.Protocol
		}
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

		configureTLS := true
		if forMeshGateway {
			// We only initiate TLS if we're doing an L7 proxy.
			configureTLS = structs.IsProtocolHTTPLike(proto)
		}

		if configureTLS {
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
		}

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

func (s *ResourceGenerator) makeExportedUpstreamClustersForMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	// NOTE: Despite the mesh gateway already having one cluster per service
	// (and subset) in the local datacenter we cannot reliably use those to
	// send inbound peered traffic targeting a discovery chain.
	//
	// For starters, none of those add TLS so they'd be unusable for http-like
	// L7 protocols.
	//
	// Additionally, those other clusters are all thin wrappers around simple
	// catalog resolutions and are largely not impacted by various
	// customizations related to a service-resolver, such as configuring the
	// failover section.
	//
	// Instead we create brand new clusters solely to accept incoming peered
	// traffic and give them a unique cluster prefix name to avoid collisions
	// to keep the two use cases separate.
	var clusters []proto.Message

	createdExportedClusters := make(map[string]struct{}) // key=clusterName
	for _, svc := range cfgSnap.MeshGatewayValidExportedServices() {
		chain := cfgSnap.MeshGateway.DiscoveryChain[svc]

		exportClusters, err := s.makeUpstreamClustersForDiscoveryChain(
			proxycfg.NewUpstreamIDFromServiceName(svc),
			nil,
			chain,
			nil,
			cfgSnap,
			true,
		)
		if err != nil {
			return nil, err
		}

		for _, cluster := range exportClusters {
			if _, ok := createdExportedClusters[cluster.Name]; ok {
				continue
			}
			createdExportedClusters[cluster.Name] = struct{}{}
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

// injectSANMatcher updates a TLS context so that it verifies the upstream SAN.
func injectSANMatcher(tlsContext *envoy_tls_v3.CommonTlsContext, matchStrings ...string) error {
	if tlsContext == nil {
		return fmt.Errorf("invalid type: expected CommonTlsContext_ValidationContext not to be nil")
	}

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

	// Corresponds to a valid ip/port in a Destination
	address string
	port    int
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

	// TCP keepalive settings can be enabled for terminating gateway upstreams or remote mesh gateways.
	remoteUpstream := opts.isRemote || snap.Kind == structs.ServiceKindTerminatingGateway
	if remoteUpstream && cfg.TcpKeepaliveEnable {
		cluster.UpstreamConnectionOptions = &envoy_cluster_v3.UpstreamConnectionOptions{
			TcpKeepalive: &envoy_core_v3.TcpKeepalive{},
		}
		if cfg.TcpKeepaliveTime != 0 {
			cluster.UpstreamConnectionOptions.TcpKeepalive.KeepaliveTime = makeUint32Value(cfg.TcpKeepaliveTime)
		}
		if cfg.TcpKeepaliveInterval != 0 {
			cluster.UpstreamConnectionOptions.TcpKeepalive.KeepaliveInterval = makeUint32Value(cfg.TcpKeepaliveInterval)
		}
		if cfg.TcpKeepaliveProbes != 0 {
			cluster.UpstreamConnectionOptions.TcpKeepalive.KeepaliveProbes = makeUint32Value(cfg.TcpKeepaliveProbes)
		}
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

// makeTerminatingIPCluster creates an Envoy cluster for a terminating gateway with an ip destination
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
		OutlierDetection:     &envoy_cluster_v3.OutlierDetection{},
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC},
	}

	endpoints := []*envoy_endpoint_v3.LbEndpoint{
		makeEndpoint(opts.address, opts.port),
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

// makeTerminatingHostnameCluster creates an Envoy cluster for a terminating gateway with a hostname destination
func (s *ResourceGenerator) makeTerminatingHostnameCluster(snap *proxycfg.ConfigSnapshot, opts clusterOpts) *envoy_cluster_v3.Cluster {
	cfg, err := ParseGatewayConfig(snap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse gateway config", "error", err)
	}
	opts.connectTimeout = time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond

	cluster := &envoy_cluster_v3.Cluster{
		Name:           opts.name,
		ConnectTimeout: durationpb.New(opts.connectTimeout),

		// Having an empty config enables outlier detection with default config.
		OutlierDetection:     &envoy_cluster_v3.OutlierDetection{},
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_LOGICAL_DNS},
		DnsLookupFamily:      envoy_cluster_v3.Cluster_V4_ONLY,
	}

	rate := 10 * time.Second
	cluster.DnsRefreshRate = durationpb.New(rate)

	address := makeAddress(opts.address, opts.port)

	endpoints := []*envoy_endpoint_v3.LbEndpoint{
		{
			HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
				Endpoint: &envoy_endpoint_v3.Endpoint{
					Address: address,
				},
			},
		},
	}

	cluster.LoadAssignment = &envoy_endpoint_v3.ClusterLoadAssignment{
		ClusterName: cluster.Name,
		Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
			LbEndpoints: endpoints,
		}},
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

// generatePeeredClusterName returns an SNI-like cluster name which mimics PeeredServiceSNI
// but excludes partition information which could be ambiguous (local vs remote partition).
func generatePeeredClusterName(uid proxycfg.UpstreamID, tb *pbpeering.PeeringTrustBundle) string {
	return strings.Join([]string{
		uid.Name,
		uid.NamespaceOrDefault(),
		uid.Peer,
		"external",
		tb.TrustDomain,
	}, ".")
}
