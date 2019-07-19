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
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// clustersFromSnapshot returns the xDS API representation of the "clusters"
// (upstreams) in the snapshot.
func (s *Server) clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	// Include the "app" cluster for the public listener
	clusters := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	var err error
	clusters[0], err = s.makeAppCluster(cfgSnap)
	if err != nil {
		return nil, err
	}

	for idx, upstream := range cfgSnap.Proxy.Upstreams {
		clusters[idx+1], err = s.makeUpstreamCluster(upstream, cfgSnap)
		if err != nil {
			return nil, err
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

	return c, err
}

func (s *Server) makeUpstreamCluster(upstream structs.Upstream, cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Cluster, error) {
	var c *envoy.Cluster
	var err error

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
	}

	return c, nil
}

// makeClusterFromUserConfig returns the listener config decoded from an
// arbitrary proto3 json format string or an error if it's invalid.
//
// For now we only support embedding in JSON strings because of the hcl parsing
// pain (see config.go comment above call to patchSliceOfMaps). Until we
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
