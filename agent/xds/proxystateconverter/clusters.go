// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/config"
	"github.com/hashicorp/consul/agent/xds/naming"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

const (
	meshGatewayExportedClusterNamePrefix = "exported~"
)

type namedCluster struct {
	name    string
	cluster *pbproxystate.Cluster
}

// clustersFromSnapshot returns the xDS API representation of the "clusters" in the snapshot.
func (s *Converter) clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) error {
	if cfgSnap == nil {
		return errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.clustersFromSnapshotConnectProxy(cfgSnap)
	// TODO(proxystate): Terminating Gateways will be added in the future.
	//case structs.ServiceKindTerminatingGateway:
	//	err := s.clustersFromSnapshotTerminatingGateway(cfgSnap)
	//	if err != nil {
	//		return err
	//	}
	//	return nil
	// TODO(proxystate): Mesh Gateways will be added in the future.
	//case structs.ServiceKindMeshGateway:
	//	err := s.clustersFromSnapshotMeshGateway(cfgSnap)
	//	if err != nil {
	//		return err
	//	}
	//	return nil
	// TODO(proxystate): Ingress Gateways will be added in the future.
	//case structs.ServiceKindIngressGateway:
	//	err := s.clustersFromSnapshotIngressGateway(cfgSnap)
	//	if err != nil {
	//		return err
	//	}
	//	return nil
	// TODO(proxystate): API Gateways will be added in the future.
	//case structs.ServiceKindAPIGateway:
	//	res, err := s.clustersFromSnapshotAPIGateway(cfgSnap)
	//	if err != nil {
	//		return err
	//	}
	//	return nil
	default:
		return fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// clustersFromSnapshot returns the xDS API representation of the "clusters"
// (upstreams) in the snapshot.
func (s *Converter) clustersFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) error {
	// This is the list of listeners we add to. It will be empty to start.
	clusters := s.proxyState.Clusters
	var err error

	// Include the "app" cluster for the public listener
	appCluster, err := s.makeAppCluster(cfgSnap, xdscommon.LocalAppClusterName, "", cfgSnap.Proxy.LocalServicePort)
	if err != nil {
		return err
	}
	clusters[appCluster.name] = appCluster.cluster

	if cfgSnap.Proxy.Mode == structs.ProxyModeTransparent {
		passthroughs, err := s.makePassthroughClusters(cfgSnap)
		if err != nil {
			return fmt.Errorf("failed to make passthrough clusters for transparent proxy: %v", err)
		}
		for clusterName, cluster := range passthroughs {
			clusters[clusterName] = cluster
		}
	}

	// NOTE: Any time we skip a chain below we MUST also skip that discovery chain in endpoints.go
	// so that the sets of endpoints generated matches the sets of clusters.
	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		upstream, skip := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta)
		if skip {
			continue
		}

		upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(
			uid,
			upstream,
			chain,
			cfgSnap,
			false,
		)
		if err != nil {
			return err
		}

		for name, cluster := range upstreamClusters {
			clusters[name] = cluster
		}
	}

	// TODO(proxystate): peering will be added in the future.
	//// NOTE: Any time we skip an upstream below we MUST also skip that same
	//// upstream in endpoints.go so that the sets of endpoints generated matches
	//// the sets of clusters.
	//for _, uid := range cfgSnap.ConnectProxy.PeeredUpstreamIDs() {
	//	upstream, skip := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta)
	//	if skip {
	//		continue
	//	}
	//
	//	peerMeta, found := cfgSnap.ConnectProxy.UpstreamPeerMeta(uid)
	//	if !found {
	//		s.Logger.Warn("failed to fetch upstream peering metadata for cluster", "uid", uid)
	//	}
	//	cfg := s.getAndModifyUpstreamConfigForPeeredListener(uid, upstream, peerMeta)
	//
	//	upstreamCluster, err := s.makeUpstreamClusterForPeerService(uid, cfg, peerMeta, cfgSnap)
	//	if err != nil {
	//		return nil, err
	//	}
	//	clusters = append(clusters, upstreamCluster)
	//}

	// TODO(proxystate): L7 Intentions and JWT Auth will be added in the future.
	//// add clusters for jwt-providers
	//for _, prov := range cfgSnap.JWTProviders {
	//	//skip cluster creation for local providers
	//	if prov.JSONWebKeySet == nil || prov.JSONWebKeySet.Remote == nil {
	//		continue
	//	}
	//
	//	cluster, err := makeJWTProviderCluster(prov)
	//	if err != nil {
	//		s.Logger.Warn("failed to make jwt-provider cluster", "provider name", prov.Name, "error", err)
	//		continue
	//	}
	//
	//	clusters[cluster.GetName()] = cluster
	//}

	for _, u := range cfgSnap.Proxy.Upstreams {
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			continue
		}

		upstreamCluster, err := s.makeUpstreamClusterForPreparedQuery(u, cfgSnap)
		if err != nil {
			return err
		}
		clusters[upstreamCluster.name] = upstreamCluster.cluster
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
		clusters[c.name] = c.cluster
	}

	return nil
}

// TODO(proxystate): L7 Intentions and JWT Auth will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeJWTProviderCluster
// func makeJWKSDiscoveryClusterType
// func makeJWTCertValidationContext
// func parseJWTRemoteURL

func makeExposeClusterName(destinationPort int) string {
	return fmt.Sprintf("exposed_cluster_%d", destinationPort)
}

// In transparent proxy mode there are potentially multiple passthrough clusters added.
// The first is for destinations outside of Consul's catalog. This is for a plain TCP proxy.
// All of these use Envoy's ORIGINAL_DST listener filter, which forwards to the original
// destination address (before the iptables redirection).
// The rest are for destinations inside the mesh, which require certificates for mTLS.
func (s *Converter) makePassthroughClusters(cfgSnap *proxycfg.ConfigSnapshot) (map[string]*pbproxystate.Cluster, error) {
	// This size is an upper bound.
	clusters := make(map[string]*pbproxystate.Cluster, 0)
	if meshConf := cfgSnap.MeshConfig(); meshConf == nil ||
		!meshConf.TransparentProxy.MeshDestinationsOnly {

		clusters[naming.OriginalDestinationClusterName] = &pbproxystate.Cluster{
			Group: &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Passthrough{
						Passthrough: &pbproxystate.PassthroughEndpointGroup{
							Config: &pbproxystate.PassthroughEndpointGroupConfig{
								ConnectTimeout: durationpb.New(5 * time.Second),
							},
						},
					},
				},
			},
		}
	}

	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		targetMap, ok := cfgSnap.ConnectProxy.PassthroughUpstreams[uid]
		if !ok {
			continue
		}

		for targetID := range targetMap {
			uid := proxycfg.NewUpstreamIDFromTargetID(targetID)

			sni := connect.ServiceSNI(
				uid.Name, "", uid.NamespaceOrDefault(),
				uid.PartitionOrDefault(), cfgSnap.Datacenter,
				cfgSnap.Roots.TrustDomain)

			// Prefixed with passthrough to distinguish from non-passthrough clusters for the same upstream.
			name := "passthrough~" + sni

			c := pbproxystate.Cluster{
				Group: &pbproxystate.Cluster_EndpointGroup{
					EndpointGroup: &pbproxystate.EndpointGroup{
						Group: &pbproxystate.EndpointGroup_Passthrough{
							Passthrough: &pbproxystate.PassthroughEndpointGroup{
								Config: &pbproxystate.PassthroughEndpointGroupConfig{
									ConnectTimeout: durationpb.New(5 * time.Second),
								},
							},
						},
					},
				},
			}

			if discoTarget, ok := chain.Targets[targetID]; ok && discoTarget.ConnectTimeout > 0 {
				c.GetEndpointGroup().GetPassthrough().GetConfig().
					ConnectTimeout = durationpb.New(discoTarget.ConnectTimeout)
			}

			transportSocket, err := s.createOutboundMeshMTLS(cfgSnap, []string{getSpiffeID(cfgSnap, uid)}, sni)
			if err != nil {
				return nil, err
			}
			c.GetEndpointGroup().GetPassthrough().OutboundTls = transportSocket

			clusters[name] = &c
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

			c := &pbproxystate.Cluster{
				AltStatName: name,
				Group: &pbproxystate.Cluster_EndpointGroup{
					EndpointGroup: &pbproxystate.EndpointGroup{
						Group: &pbproxystate.EndpointGroup_Dynamic{
							Dynamic: &pbproxystate.DynamicEndpointGroup{
								Config: &pbproxystate.DynamicEndpointGroupConfig{
									ConnectTimeout: durationpb.New(5 * time.Second),
									// Endpoints are managed separately by EDS
									// Having an empty config enables outlier detection with default config.
									OutlierDetection: &pbproxystate.OutlierDetection{},
								},
							},
						},
					},
				},
			}
			sni := connect.ServiceSNI(
				uid.Name, "", uid.NamespaceOrDefault(),
				uid.PartitionOrDefault(), cfgSnap.Datacenter,
				cfgSnap.Roots.TrustDomain)
			transportSocket, err := s.createOutboundMeshMTLS(cfgSnap, []string{getSpiffeID(cfgSnap, uid)}, sni)
			if err != nil {
				return err
			}
			c.GetEndpointGroup().GetDynamic().OutboundTls = transportSocket
			clusters[name] = c
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func getSpiffeID(cfgSnap *proxycfg.ConfigSnapshot, uid proxycfg.UpstreamID) string {
	spiffeIDService := &connect.SpiffeIDService{
		Host:       cfgSnap.Roots.TrustDomain,
		Partition:  uid.PartitionOrDefault(),
		Namespace:  uid.NamespaceOrDefault(),
		Datacenter: cfgSnap.Datacenter,
		Service:    uid.Name,
	}
	return spiffeIDService.URI().String()
}
func clusterNameForDestination(cfgSnap *proxycfg.ConfigSnapshot, name string,
	address string, namespace string, partition string) string {
	name = destinationSpecificServiceName(name, address)
	sni := connect.ServiceSNI(name, "", namespace, partition,
		cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

	// Prefixed with destination to distinguish from non-passthrough clusters
	// for the same upstream.
	return "destination." + sni
}

func destinationSpecificServiceName(name string, address string) string {
	address = strings.ReplaceAll(address, ":", "-")
	address = strings.ReplaceAll(address, ".", "-")
	return fmt.Sprintf("%s.%s", address, name)
}

// TODO(proxystate): Mesh Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func clustersFromSnapshotMeshGateway
// func haveVoters

// TODO(proxystate): Peering will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makePeerServerClusters

// TODO(proxystate): Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func clustersFromSnapshotTerminatingGateway

// TODO(proxystate): Mesh Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeGatewayServiceClusters

// TODO(proxystate): Cluster Peering will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeGatewayOutgoingClusterPeeringServiceClusters

// TODO(proxystate): Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeDestinationClusters

// TODO(proxystate): Mesh Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func injectGatewayServiceAddons

// TODO(proxystate): Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func injectGatewayDestinationAddons

// TODO(proxystate): Ingress Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func clustersFromSnapshotIngressGateway

// TODO(proxystate): API Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func clustersFromSnapshotAPIGateway

// TODO(proxystate): Ingress Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func configIngressUpstreamCluster

func (s *Converter) makeAppCluster(cfgSnap *proxycfg.ConfigSnapshot, name, pathProtocol string, port int) (*namedCluster, error) {
	var err error
	namedCluster := &namedCluster{}

	cfg, err := config.ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	//// If we have overridden local cluster config try to parse it into an Envoy cluster
	//if cfg.LocalClusterJSON != "" {
	//	return makeClusterFromUserConfig(cfg.LocalClusterJSON)
	//}

	var endpoint *pbproxystate.Endpoint
	if cfgSnap.Proxy.LocalServiceSocketPath != "" {
		endpoint = makeUnixSocketEndpoint(cfgSnap.Proxy.LocalServiceSocketPath)
	} else {
		addr := cfgSnap.Proxy.LocalServiceAddress
		if addr == "" {
			addr = "127.0.0.1"
		}
		endpoint = makeHostPortEndpoint(addr, port)
	}
	s.proxyState.Endpoints[name] = &pbproxystate.Endpoints{
		Endpoints: []*pbproxystate.Endpoint{endpoint},
	}

	namedCluster.name = name
	namedCluster.cluster = &pbproxystate.Cluster{
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Static{
					Static: &pbproxystate.StaticEndpointGroup{
						Config: &pbproxystate.StaticEndpointGroupConfig{
							ConnectTimeout: durationpb.New(time.Duration(cfg.LocalConnectTimeoutMs) * time.Millisecond),
						},
					},
				},
			},
		},
	}
  
	protocol := pathProtocol
	if protocol == "" {
		protocol = cfg.Protocol
	}
	namedCluster.cluster.Protocol = protocol
	if cfg.MaxInboundConnections > 0 {
		namedCluster.cluster.GetEndpointGroup().GetStatic().GetConfig().
			CircuitBreakers = &pbproxystate.CircuitBreakers{
			UpstreamLimits: &pbproxystate.UpstreamLimits{
				MaxConnections: response.MakeUint32Value(cfg.MaxInboundConnections),
			},
		}
	}

	return namedCluster, err
}

func (s *Converter) makeUpstreamClusterForPeerService(
	uid proxycfg.UpstreamID,
	upstreamConfig structs.UpstreamConfig,
	peerMeta structs.PeeringServiceMeta,
	cfgSnap *proxycfg.ConfigSnapshot,
) (string, *pbproxystate.Cluster, *pbproxystate.Endpoints, error) {
	var (
		c   *pbproxystate.Cluster
		e   *pbproxystate.Endpoints
		err error
	)

	// TODO(proxystate): escapeHatches will be implemented in the future
	//if upstreamConfig.EnvoyClusterJSON != "" {
	//	c, err = makeClusterFromUserConfig(upstreamConfig.EnvoyClusterJSON)
	//	if err != nil {
	//		return "", c, e, err
	//	}
	//	// In the happy path don't return yet as we need to inject TLS config still.
	//}

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()

	if err != nil {
		return "", c, e, err
	}

	tbs, ok := upstreamsSnapshot.UpstreamPeerTrustBundles.Get(uid.Peer)
	if !ok {
		// this should never happen since we loop through upstreams with
		// set trust bundles
		return "", c, e, fmt.Errorf("trust bundle not ready for peer %s", uid.Peer)
	}

	clusterName := generatePeeredClusterName(uid, tbs)

	outlierDetection := makeOutlierDetection(upstreamConfig.PassiveHealthCheck, nil, true)
	// We can't rely on health checks for services on cluster peers because they
	// don't take into account service resolvers, splitters and routers. Setting
	// MaxEjectionPercent too 100% gives outlier detection the power to eject the
	// entire cluster.
	outlierDetection.MaxEjectionPercent = &wrapperspb.UInt32Value{Value: 100}

	s.Logger.Trace("generating cluster for", "cluster", clusterName)
	if c == nil {
		c = &pbproxystate.Cluster{}

		useEDS := true
		if _, ok := cfgSnap.ConnectProxy.PeerUpstreamEndpointsUseHostnames[uid]; ok {
			// If we're using local mesh gw, the fact that upstreams use hostnames don't matter.
			// If we're not using local mesh gw, then resort to CDS.
			if upstreamConfig.MeshGateway.Mode != structs.MeshGatewayModeLocal {
				useEDS = false
			}
		}

		// If none of the service instances are addressed by a hostname we
		// provide the endpoint IP addresses via EDS
		if useEDS {
			d := &pbproxystate.DynamicEndpointGroup{
				Config: &pbproxystate.DynamicEndpointGroupConfig{
					UseAltStatName:        false,
					ConnectTimeout:        durationpb.New(time.Duration(upstreamConfig.ConnectTimeoutMs) * time.Millisecond),
					DisablePanicThreshold: true,
					CircuitBreakers: &pbproxystate.CircuitBreakers{
						UpstreamLimits: makeUpstreamLimitsIfNeeded(upstreamConfig.Limits),
					},
					OutlierDetection: outlierDetection,
				},
			}
			c.Group = &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Dynamic{
						Dynamic: d,
					},
				},
			}
			transportSocket := &pbproxystate.TransportSocket{
				ConnectionTls: &pbproxystate.TransportSocket_OutboundMesh{
					OutboundMesh: &pbproxystate.OutboundMeshMTLS{
						ValidationContext: &pbproxystate.MeshOutboundValidationContext{
							SpiffeIds:              peerMeta.SpiffeID,
							TrustBundlePeerNameKey: uid.Peer,
						},
						Sni: peerMeta.PrimarySNI(),
					},
				},
			}
			d.OutboundTls = transportSocket
		} else {
			d := &pbproxystate.DNSEndpointGroup{
				Config: &pbproxystate.DNSEndpointGroupConfig{
					UseAltStatName:        false,
					ConnectTimeout:        durationpb.New(time.Duration(upstreamConfig.ConnectTimeoutMs) * time.Millisecond),
					DisablePanicThreshold: true,
					CircuitBreakers: &pbproxystate.CircuitBreakers{
						UpstreamLimits: makeUpstreamLimitsIfNeeded(upstreamConfig.Limits),
					},
					OutlierDetection: outlierDetection,
				},
			}
			c.Group = &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Dns{
						Dns: d,
					},
				},
			}
			e = &pbproxystate.Endpoints{
				Endpoints: make([]*pbproxystate.Endpoint, 0),
			}

			ep, _ := cfgSnap.ConnectProxy.PeerUpstreamEndpoints.Get(uid)
			configureClusterWithHostnames(
				s.Logger,
				d,
				e,
				"", /*TODO:make configurable?*/
				ep,
				true,  /*isRemote*/
				false, /*onlyPassing*/
			)
			transportSocket := &pbproxystate.TransportSocket{
				ConnectionTls: &pbproxystate.TransportSocket_OutboundMesh{
					OutboundMesh: &pbproxystate.OutboundMeshMTLS{
						ValidationContext: &pbproxystate.MeshOutboundValidationContext{
							SpiffeIds:              peerMeta.SpiffeID,
							TrustBundlePeerNameKey: uid.Peer,
						},
						Sni: peerMeta.PrimarySNI(),
					},
				},
			}
			d.OutboundTls = transportSocket
		}
	}

	return clusterName, c, e, nil
}

func (s *Converter) makeUpstreamClusterForPreparedQuery(upstream structs.Upstream, cfgSnap *proxycfg.ConfigSnapshot) (*namedCluster, error) {
	var c *pbproxystate.Cluster
	var err error

	uid := proxycfg.NewUpstreamID(&upstream)

	dc := upstream.Datacenter
	if dc == "" {
		dc = cfgSnap.Datacenter
	}
	sni := connect.UpstreamSNI(&upstream, "", dc, cfgSnap.Roots.TrustDomain)

	cfg, _ := structs.ParseUpstreamConfig(upstream.Config)
	// TODO(proxystate): add logger and enable this
	//if err != nil {
	// Don't hard fail on a config typo, just warn. The parse func returns
	// default config if there is an error so it's safe to continue.
	//s.Logger.Warn("failed to parse", "upstream", uid, "error", err)
	//}

	// TODO(proxystate): escapeHatches will be implemented in the future
	//if cfg.EnvoyClusterJSON != "" {
	//	c, err = makeClusterFromUserConfig(cfg.EnvoyClusterJSON)
	//	if err != nil {
	//		return c, err
	//	}
	//	// In the happy path don't return yet as we need to inject TLS config still.
	//}

	if c == nil {
		c = &pbproxystate.Cluster{
			Protocol: cfg.Protocol,
			Group: &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Dynamic{
						Dynamic: &pbproxystate.DynamicEndpointGroup{
							Config: &pbproxystate.DynamicEndpointGroupConfig{
								ConnectTimeout: durationpb.New(time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond),
								// Endpoints are managed separately by EDS
								// Having an empty config enables outlier detection with default config.
								OutlierDetection:      makeOutlierDetection(cfg.PassiveHealthCheck, nil, true),
								DisablePanicThreshold: true,
								CircuitBreakers: &pbproxystate.CircuitBreakers{
									UpstreamLimits: makeUpstreamLimitsIfNeeded(cfg.Limits),
								},
							},
						},
					},
				},
			},
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

	transportSocket, err := s.createOutboundMeshMTLS(cfgSnap, spiffeIDs, sni)
	if err != nil {
		return nil, err
	}
	c.GetEndpointGroup().GetDynamic().OutboundTls = transportSocket

	return &namedCluster{name: sni, cluster: c}, nil
}

func finalizeUpstreamConfig(cfg structs.UpstreamConfig, chain *structs.CompiledDiscoveryChain, connectTimeout time.Duration) structs.UpstreamConfig {
	if cfg.Protocol == "" {
		cfg.Protocol = chain.Protocol
	}

	if cfg.Protocol == "" {
		cfg.Protocol = "tcp"
	}

	if cfg.ConnectTimeoutMs == 0 {
		cfg.ConnectTimeoutMs = int(connectTimeout / time.Millisecond)
	}
	return cfg
}

func (s *Converter) createOutboundMeshMTLS(cfgSnap *proxycfg.ConfigSnapshot, spiffeIDs []string, sni string) (*pbproxystate.TransportSocket, error) {
	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
	case structs.ServiceKindMeshGateway:
	default:
		return nil, fmt.Errorf("cannot inject peering trust bundles for kind %q", cfgSnap.Kind)
	}

	cfg, err := config.ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// Add all trust bundle peer names, including local.
	trustBundlePeerNames := []string{"local"}
	for _, tb := range cfgSnap.PeeringTrustBundles() {
		trustBundlePeerNames = append(trustBundlePeerNames, tb.PeerName)
	}
	// Arbitrary UUID to reference the identity by.
	uuid, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	// Create the transport socket
	ts := &pbproxystate.TransportSocket{}

	ts.ConnectionTls = &pbproxystate.TransportSocket_OutboundMesh{
		OutboundMesh: &pbproxystate.OutboundMeshMTLS{
			IdentityKey: uuid,
			ValidationContext: &pbproxystate.MeshOutboundValidationContext{
				TrustBundlePeerNameKey: trustBundlePeerNames[0],
				SpiffeIds:              spiffeIDs,
			},
			Sni: sni,
		},
	}
	s.proxyState.LeafCertificates[uuid] = &pbproxystate.LeafCertificate{
		Cert: cfgSnap.Leaf().CertPEM,
		Key:  cfgSnap.Leaf().PrivateKeyPEM,
	}
	ts.TlsParameters = makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSOutgoing())
	ts.AlpnProtocols = getAlpnProtocols(cfg.Protocol)

	return ts, nil
}
func (s *Converter) makeUpstreamClustersForDiscoveryChain(
	uid proxycfg.UpstreamID,
	upstream *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
	forMeshGateway bool,
) (map[string]*pbproxystate.Cluster, error) {
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

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()

	// Mesh gateways are exempt because upstreamsSnapshot is only used for
	// cluster peering targets and transative failover/redirects are unsupported.
	if err != nil && !forMeshGateway {
		return nil, err
	}

	rawUpstreamConfig, err := structs.ParseUpstreamConfigNoDefaults(upstreamConfigMap)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", uid,
			"error", err)
	}

	// TODO(proxystate): escapeHatches will be implemented in the future
	//var escapeHatchCluster *pbproxystate.Cluster
	//if !forMeshGateway {
	//	if rawUpstreamConfig.EnvoyClusterJSON != "" {
	//		if chain.Default {
	//			// If you haven't done anything to setup the discovery chain, then
	//			// you can use the envoy_cluster_json escape hatch.
	//			escapeHatchCluster = &pbproxystate.Cluster{
	//				EscapeHatchClusterJson: rawUpstreamConfig.EnvoyClusterJSON,
	//			}
	//		} else {
	//			s.Logger.Warn("ignoring escape hatch setting, because a discovery chain is configured for",
	//				"discovery chain", chain.ServiceName, "upstream", uid,
	//				"envoy_cluster_json", chain.ServiceName)
	//		}
	//	}
	//}

	out := make(map[string]*pbproxystate.Cluster)
	for _, node := range chain.Nodes {
		switch {
		case node == nil:
			return nil, fmt.Errorf("impossible to process a nil node")
		case node.Type != structs.DiscoveryGraphNodeTypeResolver:
			continue
		case node.Resolver == nil:
			return nil, fmt.Errorf("impossible to process a non-resolver node")
		}
		// These variables are prefixed with primary to avoid shaddowing bugs.
		primaryTargetID := node.Resolver.Target
		primaryTarget := chain.Targets[primaryTargetID]
		primaryTargetClusterName := s.getTargetClusterName(upstreamsSnapshot, chain, primaryTargetID, forMeshGateway)
		if primaryTargetClusterName == "" {
			continue
		}
		if forMeshGateway && !cfgSnap.Locality.Matches(primaryTarget.Datacenter, primaryTarget.Partition) {
			s.Logger.Warn("ignoring discovery chain target that crosses a datacenter or partition boundary in a mesh gateway",
				"target", primaryTarget,
				"gatewayLocality", cfgSnap.Locality,
			)
			continue
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

		var failoverGroup *pbproxystate.FailoverGroup
		endpointGroups := make([]*pbproxystate.EndpointGroup, 0)
		if mappedTargets.failover {
			// Create a failover group. The endpoint groups that are part of this failover group are created by the loop
			// below.
			failoverGroup = &pbproxystate.FailoverGroup{
				Config: &pbproxystate.FailoverGroupConfig{
					ConnectTimeout: durationpb.New(node.Resolver.ConnectTimeout),
				},
			}
		}

		// Construct the target dynamic endpoint groups. If these are not part of a failover group, they will get added
		// directly to the map of pbproxystate.Cluster, if they are a part of a failover group, they will be added to
		// the failover group.
		for _, groupedTarget := range targetGroups {
			s.Logger.Debug("generating cluster for", "cluster", groupedTarget.ClusterName)
			dynamic := &pbproxystate.DynamicEndpointGroup{
				Config: &pbproxystate.DynamicEndpointGroupConfig{
					UseAltStatName: true,
					ConnectTimeout: durationpb.New(node.Resolver.ConnectTimeout),
					// TODO(peering): make circuit breakers or outlier detection work?
					CircuitBreakers: &pbproxystate.CircuitBreakers{
						UpstreamLimits: makeUpstreamLimitsIfNeeded(upstreamConfig.Limits),
					},
					OutlierDetection: makeOutlierDetection(upstreamConfig.PassiveHealthCheck, nil, true),
				},
			}
			ti := groupedTarget.Targets[0]
			transportSocket, err := s.createOutboundMeshMTLS(cfgSnap, ti.SpiffeIDs, ti.SNI)
			if err != nil {
				return nil, err
			}
			dynamic.OutboundTls = transportSocket

			var lb *structs.LoadBalancer
			if node.LoadBalancer != nil {
				lb = node.LoadBalancer
			}
			if err := injectLBToCluster(lb, dynamic.Config); err != nil {
				return nil, fmt.Errorf("failed to apply load balancer configuration to cluster %q: %v", groupedTarget.ClusterName, err)
			}

			// TODO: IR: http2 options not currently supported
			//if upstreamConfig.Protocol == "http2" || upstreamConfig.Protocol == "grpc" {
			//	if err := s.setHttp2ProtocolOptions(c); err != nil {
			//		return nil, err
			//	}
			//}

			switch len(groupedTarget.Targets) {
			case 0:
				continue
			case 1:
				// We expect one target so this passes through to continue setting the cluster up.
			default:
				return nil, fmt.Errorf("cannot have more than one target")
			}

			if targetInfo := groupedTarget.Targets[0]; targetInfo.TransportSocket != nil {
				dynamic.OutboundTls = targetInfo.TransportSocket
			}

			// If the endpoint group is part of a failover group, add it to the failover group. Otherwise add it
			// directly to the clusters.
			if failoverGroup != nil {
				eg := &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Dynamic{
						Dynamic: dynamic,
					},
				}
				endpointGroups = append(endpointGroups, eg)
			} else {
				cluster := &pbproxystate.Cluster{
					AltStatName: mappedTargets.baseClusterName,
					Protocol:    upstreamConfig.Protocol,
					Group: &pbproxystate.Cluster_EndpointGroup{
						EndpointGroup: &pbproxystate.EndpointGroup{
							Group: &pbproxystate.EndpointGroup_Dynamic{
								Dynamic: dynamic,
							},
						},
					},
				}

				out[mappedTargets.baseClusterName] = cluster
			}
		}

		// If there's a failover group, we only add the failover group to the top level list of clusters. Its endpoint
		// groups are inlined.
		if failoverGroup != nil {
			failoverGroup.EndpointGroups = endpointGroups
			cluster := &pbproxystate.Cluster{
				AltStatName: mappedTargets.baseClusterName,
				Protocol:    upstreamConfig.Protocol,
				Group: &pbproxystate.Cluster_FailoverGroup{
					FailoverGroup: failoverGroup,
				},
			}
			out[mappedTargets.baseClusterName] = cluster
		}
	}

	//if escapeHatchCluster != nil {
	//	if len(out) != 1 {
	//		return nil, fmt.Errorf("cannot inject escape hatch cluster when discovery chain had no nodes")
	//	}
	//	var defaultCluster *pbproxystate.Cluster
	//	for _, k := range out {
	//		defaultCluster = k
	//		break
	//	}
	//
	//	// Overlay what the user provided.
	//	escapeHatchCluster.GetEndpointGroup().GetDynamic().OutboundTls.ConnectionTls =
	//		defaultCluster.GetEndpointGroup().GetDynamic().OutboundTls.ConnectionTls
	//
	//	out = append(out, escapeHatchCluster)
	//}

	return out, nil
}

// TODO(proxystate): Mesh Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeExportedUpstreamClustersForMeshGateway

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

// TODO(proxystate): Mesh and Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeGatewayCluster

func configureClusterWithHostnames(
	logger hclog.Logger,
	dnsEndpointGroup *pbproxystate.DNSEndpointGroup,
	endpointList *pbproxystate.Endpoints,
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
	if dnsEndpointGroup.Config == nil {
		dnsEndpointGroup.Config = &pbproxystate.DNSEndpointGroupConfig{}
	}
	dnsEndpointGroup.Config.DiscoveryType = pbproxystate.DiscoveryType_DISCOVERY_TYPE_LOGICAL
	if dnsDiscoveryType == "strict_dns" {
		dnsEndpointGroup.Config.DiscoveryType = pbproxystate.DiscoveryType_DISCOVERY_TYPE_STRICT
	}

	endpoints := make([]*pbproxystate.Endpoint, 0, 1)
	uniqueHostnames := make(map[string]bool)

	var (
		hostname string
		idx      int
		fallback *pbproxystate.Endpoint
	)
	for i, e := range hostnameEndpoints {
		_, addr, port := e.BestAddress(isRemote)
		uniqueHostnames[addr] = true

		health, weight := calculateEndpointHealthAndWeight(e, onlyPassing)
		if health == pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY {
			fallback = makeLbEndpoint(addr, port, health, weight)
			continue
		}

		if len(endpoints) == 0 {
			endpointList.Endpoints = append(endpointList.Endpoints, makeLbEndpoint(addr, port, health, weight))

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

		//endpoints = append(endpoints, fallback)
		endpointList.Endpoints = append(endpointList.Endpoints, fallback)
	}
	if len(uniqueHostnames) > 1 {
		logger.Warn(fmt.Sprintf("service contains instances with more than one unique hostname; only %q be resolved by Envoy", hostname),
			"dc", dc, "service", service.String())
	}

}

// TODO(proxystate): Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeExternalIPCluster

// TODO(proxystate): Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func makeExternalHostnameCluster

func makeUpstreamLimitsIfNeeded(limits *structs.UpstreamLimits) *pbproxystate.UpstreamLimits {
	if limits == nil {
		return nil
	}

	upstreamLimits := &pbproxystate.UpstreamLimits{}

	// Likewise, make sure to not set any threshold values on the zero-value in
	// order to rely on Envoy defaults
	if limits.MaxConnections != nil {
		upstreamLimits.MaxConnections = response.MakeUint32Value(*limits.MaxConnections)
	}
	if limits.MaxPendingRequests != nil {
		upstreamLimits.MaxPendingRequests = response.MakeUint32Value(*limits.MaxPendingRequests)
	}
	if limits.MaxConcurrentRequests != nil {
		upstreamLimits.MaxConcurrentRequests = response.MakeUint32Value(*limits.MaxConcurrentRequests)
	}

	return upstreamLimits
}

func injectLBToCluster(ec *structs.LoadBalancer, dc *pbproxystate.DynamicEndpointGroupConfig) error {
	if ec == nil {
		return nil
	}

	switch ec.Policy {
	case "":
		return nil
	case structs.LBPolicyLeastRequest:
		lr := &pbproxystate.DynamicEndpointGroupConfig_LeastRequest{
			LeastRequest: &pbproxystate.LBPolicyLeastRequest{},
		}

		dc.LbPolicy = lr

		if ec.LeastRequestConfig != nil {
			lr.LeastRequest.ChoiceCount = &wrapperspb.UInt32Value{Value: ec.LeastRequestConfig.ChoiceCount}
		}
	case structs.LBPolicyRoundRobin:
		dc.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_RoundRobin{
			RoundRobin: &pbproxystate.LBPolicyRoundRobin{},
		}

	case structs.LBPolicyRandom:
		dc.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_Random{
			Random: &pbproxystate.LBPolicyRandom{},
		}

	case structs.LBPolicyRingHash:
		rh := &pbproxystate.DynamicEndpointGroupConfig_RingHash{
			RingHash: &pbproxystate.LBPolicyRingHash{},
		}

		dc.LbPolicy = rh

		if ec.RingHashConfig != nil {
			rh.RingHash.MinimumRingSize = &wrapperspb.UInt64Value{Value: ec.RingHashConfig.MinimumRingSize}
			rh.RingHash.MaximumRingSize = &wrapperspb.UInt64Value{Value: ec.RingHashConfig.MaximumRingSize}
		}
	case structs.LBPolicyMaglev:
		dc.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_Maglev{
			Maglev: &pbproxystate.LBPolicyMaglev{},
		}

	default:
		return fmt.Errorf("unsupported load balancer policy %q", ec.Policy)
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

func (s *Converter) getTargetClusterName(upstreamsSnapshot *proxycfg.ConfigSnapshotUpstreams, chain *structs.CompiledDiscoveryChain, tid string, forMeshGateway bool) string {
	target := chain.Targets[tid]
	clusterName := target.Name
	targetUID := proxycfg.NewUpstreamIDFromTargetID(tid)
	if targetUID.Peer != "" {
		tbs, ok := upstreamsSnapshot.UpstreamPeerTrustBundles.Get(targetUID.Peer)
		// We can't generate cluster on peers without the trust bundle. The
		// trust bundle should be ready soon.
		if !ok {
			s.Logger.Debug("peer trust bundle not ready for discovery chain target",
				"peer", targetUID.Peer,
				"target", tid,
			)
			return ""
		}

		clusterName = generatePeeredClusterName(targetUID, tbs)
	}
	clusterName = naming.CustomizeClusterName(clusterName, chain)
	if forMeshGateway {
		clusterName = meshGatewayExportedClusterNamePrefix + clusterName
	}
	return clusterName
}

// Return an pbproxystate.OutlierDetection populated by the values from structs.PassiveHealthCheck.
// If all values are zero a default empty OutlierDetection will be returned to
// enable outlier detection with default values.
//   - If override is not nil, it will overwrite the values from p, e.g., ingress gateway defaults
//   - allowZero is added to handle the legacy case where connect-proxy and mesh gateway can set 0
//     for EnforcingConsecutive5xx. Due to the definition of proto of PassiveHealthCheck, ingress
//     gateway's EnforcingConsecutive5xx must be > 0.
func makeOutlierDetection(p *structs.PassiveHealthCheck, override *structs.PassiveHealthCheck, allowZero bool) *pbproxystate.OutlierDetection {
	od := &pbproxystate.OutlierDetection{}
	if p != nil {

		if p.Interval != 0 {
			od.Interval = durationpb.New(p.Interval)
		}
		if p.MaxFailures != 0 {
			od.Consecutive_5Xx = &wrapperspb.UInt32Value{Value: p.MaxFailures}
		}

		if p.EnforcingConsecutive5xx != nil {
			// NOTE: EnforcingConsecutive5xx must be greater than 0 for ingress-gateway
			if *p.EnforcingConsecutive5xx != 0 {
				od.EnforcingConsecutive_5Xx = &wrapperspb.UInt32Value{Value: *p.EnforcingConsecutive5xx}
			} else if allowZero {
				od.EnforcingConsecutive_5Xx = &wrapperspb.UInt32Value{Value: *p.EnforcingConsecutive5xx}
			}
		}

		if p.MaxEjectionPercent != nil {
			od.MaxEjectionPercent = &wrapperspb.UInt32Value{Value: *p.MaxEjectionPercent}
		}
		if p.BaseEjectionTime != nil {
			od.BaseEjectionTime = durationpb.New(*p.BaseEjectionTime)
		}
	}

	if override == nil {
		return od
	}

	// override the default outlier detection value
	if override.Interval != 0 {
		od.Interval = durationpb.New(override.Interval)
	}
	if override.MaxFailures != 0 {
		od.Consecutive_5Xx = &wrapperspb.UInt32Value{Value: override.MaxFailures}
	}

	if override.EnforcingConsecutive5xx != nil {
		// NOTE: EnforcingConsecutive5xx must be great than 0 for ingress-gateway
		if *override.EnforcingConsecutive5xx != 0 {
			od.EnforcingConsecutive_5Xx = &wrapperspb.UInt32Value{Value: *override.EnforcingConsecutive5xx}
		}
		// Because only ingress gateways have overrides and they cannot have a value of 0, there is no allowZero
		// override case to handle
	}

	if override.MaxEjectionPercent != nil {
		od.MaxEjectionPercent = &wrapperspb.UInt32Value{Value: *override.MaxEjectionPercent}
	}
	if override.BaseEjectionTime != nil {
		od.BaseEjectionTime = durationpb.New(*override.BaseEjectionTime)
	}

	return od
}
