package xds

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoycluster "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// clustersFromSnapshot returns the xDS API representation of the "clusters" in the snapshot.
func (s *Server) clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.clustersFromSnapshotConnectProxy(cfgSnap, token)
	case structs.ServiceKindMeshGateway:
		return s.clustersFromSnapshotMeshGateway(cfgSnap, token)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// clustersFromSnapshot returns the xDS API representation of the "clusters"
// (upstreams) in the snapshot.
func (s *Server) clustersFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	// TODO(rb): this sizing is a low bound.
	clusters := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	// Include the "app" cluster for the public listener
	appCluster, err := s.makeAppCluster(cfgSnap)
	if err != nil {
		return nil, err
	}

	clusters = append(clusters, appCluster)

	for _, u := range cfgSnap.Proxy.Upstreams {
		id := u.Identifier()
		var chain *structs.CompiledDiscoveryChain
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			chain = cfgSnap.ConnectProxy.DiscoveryChain[id]
		}

		if chain == nil || chain.IsDefault() {
			// Either old-school upstream or prepared query.
			upstreamCluster, err := s.makeUpstreamCluster(u, cfgSnap)
			if err != nil {
				return nil, err
			}
			clusters = append(clusters, upstreamCluster)

		} else {
			upstreamClusters, err := s.makeUpstreamClustersForDiscoveryChain(id, chain, cfgSnap)
			if err != nil {
				return nil, err
			}

			for _, cluster := range upstreamClusters {
				clusters = append(clusters, cluster)
			}
		}
	}

	return clusters, nil
}

// clustersFromSnapshotMeshGateway returns the xDS API representation of the "clusters"
// for a mesh gateway. This will include 1 cluster per remote datacenter as well as
// 1 cluster for each service subset.
func (s *Server) clustersFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	// 1 cluster per remote dc + 1 cluster per local service (this is a lower bound - all subset specific clusters will be appended)
	clusters := make([]proto.Message, 0, len(cfgSnap.MeshGateway.GatewayGroups)+len(cfgSnap.MeshGateway.ServiceGroups))

	// generate the remote dc clusters
	for dc, _ := range cfgSnap.MeshGateway.GatewayGroups {
		clusterName := DatacenterSNI(dc, cfgSnap)

		cluster, err := s.makeMeshGatewayCluster(clusterName, cfgSnap)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
	}

	// generate the per-service clusters
	for svc, _ := range cfgSnap.MeshGateway.ServiceGroups {
		clusterName := ServiceSNI(svc, "", "default", cfgSnap.Datacenter, cfgSnap)

		cluster, err := s.makeMeshGatewayCluster(clusterName, cfgSnap)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
	}

	// generate the service subset clusters
	for svc, resolver := range cfgSnap.MeshGateway.ServiceResolvers {
		for subsetName, _ := range resolver.Subsets {
			clusterName := ServiceSNI(svc, subsetName, "default", cfgSnap.Datacenter, cfgSnap)

			cluster, err := s.makeMeshGatewayCluster(clusterName, cfgSnap)
			if err != nil {
				return nil, err
			}
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

func (s *Server) makeAppCluster(cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Cluster, error) {
	var c *envoy.Cluster
	var err error

	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse Connect.Proxy.Config: %s", err)
	}

	// If we have overridden local cluster config try to parse it into an Envoy cluster
	if cfg.LocalClusterJSON != "" {
		return makeClusterFromUserConfig(cfg.LocalClusterJSON)
	}

	if c == nil {
		addr := cfgSnap.Proxy.LocalServiceAddress
		if addr == "" {
			addr = "127.0.0.1"
		}
		c = &envoy.Cluster{
			Name:                 LocalAppClusterName,
			ConnectTimeout:       time.Duration(cfg.LocalConnectTimeoutMs) * time.Millisecond,
			ClusterDiscoveryType: &envoy.Cluster_Type{Type: envoy.Cluster_STATIC},
			LoadAssignment: &envoy.ClusterLoadAssignment{
				ClusterName: LocalAppClusterName,
				Endpoints: []envoyendpoint.LocalityLbEndpoints{
					{
						LbEndpoints: []envoyendpoint.LbEndpoint{
							makeEndpoint(LocalAppClusterName,
								addr,
								cfgSnap.Proxy.LocalServicePort),
						},
					},
				},
			},
		}
		if cfg.Protocol == "http2" || cfg.Protocol == "grpc" {
			c.Http2ProtocolOptions = &envoycore.Http2ProtocolOptions{}
		}
	}

	return c, err
}

func (s *Server) makeUpstreamCluster(upstream structs.Upstream, cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Cluster, error) {
	var c *envoy.Cluster
	var err error

	ns := "default"
	if upstream.DestinationNamespace != "" {
		ns = upstream.DestinationNamespace
	}
	dc := cfgSnap.Datacenter
	if upstream.Datacenter != "" {
		dc = upstream.Datacenter
	}
	sni := ServiceSNI(upstream.DestinationName, "", ns, dc, cfgSnap)

	cfg, err := ParseUpstreamConfig(upstream.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse Upstream[%s].Config: %s",
			upstream.Identifier(), err)
	}
	if cfg.ClusterJSON != "" {
		c, err = makeClusterFromUserConfig(cfg.ClusterJSON)
		if err != nil {
			return c, err
		}
		// In the happy path don't return yet as we need to inject TLS config still.
	}

	if c == nil {
		c = &envoy.Cluster{
			Name:                 upstream.Identifier(),
			ConnectTimeout:       time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond,
			ClusterDiscoveryType: &envoy.Cluster_Type{Type: envoy.Cluster_EDS},
			EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
				EdsConfig: &envoycore.ConfigSource{
					ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
						Ads: &envoycore.AggregatedConfigSource{},
					},
				},
			},
			// Having an empty config enables outlier detection with default config.
			OutlierDetection: &envoycluster.OutlierDetection{},
		}
		if cfg.Protocol == "http2" || cfg.Protocol == "grpc" {
			c.Http2ProtocolOptions = &envoycore.Http2ProtocolOptions{}
		}
	}

	// Enable TLS upstream with the configured client certificate.
	c.TlsContext = &envoyauth.UpstreamTlsContext{
		CommonTlsContext: makeCommonTLSContext(cfgSnap),
		Sni:              sni,
	}

	return c, nil
}

func (s *Server) makeUpstreamClustersForDiscoveryChain(
	upstreamID string,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
) ([]*envoy.Cluster, error) {
	if chain == nil {
		panic("chain must be provided")
	}

	// TODO(rb): make escape hatches work with chains

	var out []*envoy.Cluster
	for target, node := range chain.GroupResolverNodes {
		groupResolver := node.GroupResolver
		// TODO(rb): failover
		// Failover *DiscoveryFailover `json:",omitempty"` // sad path

		clusterName := makeClusterName(upstreamID, target, cfgSnap.Datacenter)
		c := &envoy.Cluster{
			Name:                 clusterName,
			AltStatName:          clusterName, // TODO(rb): change this?
			ConnectTimeout:       groupResolver.ConnectTimeout,
			ClusterDiscoveryType: &envoy.Cluster_Type{Type: envoy.Cluster_EDS},
			CommonLbConfig: &envoy.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoytype.Percent{
					Value: 0, // disable panic threshold
				},
			},
			// TODO(rb): adjust load assignment
			EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
				EdsConfig: &envoycore.ConfigSource{
					ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
						Ads: &envoycore.AggregatedConfigSource{},
					},
				},
			},
			// Having an empty config enables outlier detection with default config.
			OutlierDetection: &envoycluster.OutlierDetection{},
		}
		if chain.Protocol == "http2" || chain.Protocol == "grpc" {
			c.Http2ProtocolOptions = &envoycore.Http2ProtocolOptions{}
		}

		// Enable TLS upstream with the configured client certificate.
		c.TlsContext = &envoyauth.UpstreamTlsContext{
			CommonTlsContext: makeCommonTLSContext(cfgSnap),
		}

		out = append(out, c)
	}

	return out, nil
}

// makeClusterName returns a string representation that uniquely identifies the
// cluster in a canonical but human readable way.
func makeClusterName(upstreamID string, target structs.DiscoveryTarget, currentDatacenter string) string {
	var name string
	if target.ServiceSubset != "" {
		name = target.Service + "/" + target.ServiceSubset
	} else {
		name = target.Service
	}

	if target.Namespace != "" && target.Namespace != "default" {
		name = target.Namespace + "/" + name
	}
	if target.Datacenter != "" && target.Datacenter != currentDatacenter {
		name += "?dc=" + target.Datacenter
	}

	if upstreamID == target.Service {
		// In the common case don't stutter.
		return name
	}

	return upstreamID + "//" + name
}

// makeClusterFromUserConfig returns the listener config decoded from an
// arbitrary proto3 json format string or an error if it's invalid.
//
// For now we only support embedding in JSON strings because of the hcl parsing
// pain (see config.go comment above call to PatchSliceOfMaps). Until we
// refactor config parser a _lot_ user's opaque config that contains arrays will
// be mangled. We could actually fix that up in mapstructure which knows the
// type of the target so could resolve the slices to singletons unambiguously
// and it would work for us here... but we still have the problem that the
// config would render incorrectly in general in our HTTP API responses so we
// really need to fix it "properly".
//
// When we do that we can support just nesting the config directly into the
// JSON/hcl naturally but this is a stop-gap that gets us an escape hatch
// immediately. It's also probably not a bad thing to support long-term since
// any config generated by other systems will likely be in canonical protobuf
// from rather than our slight variant in JSON/hcl.
func makeClusterFromUserConfig(configJSON string) (*envoy.Cluster, error) {
	var jsonFields map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &jsonFields); err != nil {
		fmt.Println("Custom error", err, configJSON)
		return nil, err
	}

	var c envoy.Cluster

	if _, ok := jsonFields["@type"]; ok {
		// Type field is present so decode it as a types.Any
		var any types.Any
		err := jsonpb.UnmarshalString(configJSON, &any)
		if err != nil {
			return nil, err
		}
		// And then unmarshal the listener again...
		err = proto.Unmarshal(any.Value, &c)
		if err != nil {
			return nil, err
		}
		return &c, err
	}

	// No @type so try decoding as a straight listener.
	err := jsonpb.UnmarshalString(configJSON, &c)
	return &c, err
}

func (s *Server) makeMeshGatewayCluster(clusterName string, cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Cluster, error) {
	cfg, err := ParseMeshGatewayConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse mesh gateway config: %s", err)
	}

	return &envoy.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       time.Duration(cfg.ConnectTimeoutMs) * time.Millisecond,
		ClusterDiscoveryType: &envoy.Cluster_Type{Type: envoy.Cluster_EDS},
		EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
			EdsConfig: &envoycore.ConfigSource{
				ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
					Ads: &envoycore.AggregatedConfigSource{},
				},
			},
		},
		// Having an empty config enables outlier detection with default config.
		OutlierDetection: &envoycluster.OutlierDetection{},
	}, nil
}
