package xds

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// clustersFromSnapshot returns the xDS API representation of the "clusters"
// (upstreams) in the snapshot.
func clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	// Include the "app" cluster for the public listener
	clusters := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	var err error
	clusters[0], err = makeAppCluster(cfgSnap)
	if err != nil {
		return nil, err
	}

	for idx, upstream := range cfgSnap.Proxy.Upstreams {
		clusters[idx+1], err = makeUpstreamCluster(upstream, cfgSnap)
		if err != nil {
			return nil, err
		}
	}

	return clusters, nil
}

func makeAppCluster(cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Cluster, error) {
	var c *envoy.Cluster
	var err error

	if clusterJSONRaw, ok := cfgSnap.Proxy.Config["envoy_app_cluster_json"]; ok {
		if clusterJSON, ok := clusterJSONRaw.(string); ok {
			c, err = makeClusterFromUserConfig(clusterJSON)
			if err != nil {
				return c, err
			}
		}
	}

	if c == nil {
		addr := cfgSnap.Proxy.LocalServiceAddress
		if addr == "" {
			addr = "127.0.0.1"
		}
		c = &envoy.Cluster{
			Name:           LocalAppClusterName,
			ConnectTimeout: 5 * time.Second,
			Type:           envoy.Cluster_STATIC,
			// API v2 docs say hosts is deprecated and should use LoadAssignment as
			// below.. but it doesn't work for tcp_proxy target for some reason.
			Hosts: []*envoycore.Address{makeAddressPtr(addr, cfgSnap.Proxy.LocalServicePort)},
			// LoadAssignment: &envoy.ClusterLoadAssignment{
			//  ClusterName: LocalAppClusterName,
			//  Endpoints: []endpoint.LocalityLbEndpoints{
			//    {
			//      LbEndpoints: []endpoint.LbEndpoint{
			//        makeEndpoint(LocalAppClusterName,
			//          addr,
			//          cfgSnap.Proxy.LocalServicePort),
			//      },
			//    },
			//  },
			// },
		}
	}

	return c, err
}

func makeUpstreamCluster(upstream structs.Upstream, cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Cluster, error) {
	var c *envoy.Cluster
	var err error

	if clusterJSONRaw, ok := upstream.Config["envoy_cluster_json"]; ok {
		if clusterJSON, ok := clusterJSONRaw.(string); ok {
			c, err = makeClusterFromUserConfig(clusterJSON)
			if err != nil {
				return c, err
			}
		}
	}

	if c == nil {
		c = &envoy.Cluster{
			Name:           upstream.Identifier(),
			ConnectTimeout: 5 * time.Second,
			Type:           envoy.Cluster_EDS,
			EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
				EdsConfig: &envoycore.ConfigSource{
					ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
						Ads: &envoycore.AggregatedConfigSource{},
					},
				},
			},
		}
	}

	// Enable TLS upstream with the configured client certificate.
	c.TlsContext = &envoyauth.UpstreamTlsContext{
		CommonTlsContext: makeCommonTLSContext(cfgSnap),
	}

	return c, nil
}

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
			panic(err)
			//return nil, err
		}
		return &c, err
	}

	// No @type so try decoding as a straight listener.
	err := jsonpb.UnmarshalString(configJSON, &c)
	return &c, err
}
