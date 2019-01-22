package xds

import (
	"errors"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
)

// clustersFromSnapshot returns the xDS API representation of the "clusters"
// (upstreams) in the snapshot.
func clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	// Include the "app" cluster for the public listener
	clusters := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	clusters[0] = makeAppCluster(cfgSnap)

	for idx, upstream := range cfgSnap.Proxy.Upstreams {
		clusters[idx+1] = makeUpstreamCluster(upstream.Identifier(), cfgSnap)
	}

	return clusters, nil
}

func makeAppCluster(cfgSnap *proxycfg.ConfigSnapshot) *envoy.Cluster {
	addr := cfgSnap.Proxy.LocalServiceAddress
	if addr == "" {
		addr = "127.0.0.1"
	}
	return &envoy.Cluster{
		Name: LocalAppClusterName,
		// TODO(banks): make this configurable from the proxy config
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

func makeUpstreamCluster(name string, cfgSnap *proxycfg.ConfigSnapshot) *envoy.Cluster {
	return &envoy.Cluster{
		Name: name,
		// TODO(banks): make this configurable from the upstream config
		ConnectTimeout: 5 * time.Second,
		Type:           envoy.Cluster_EDS,
		EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
			EdsConfig: &envoycore.ConfigSource{
				ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
					Ads: &envoycore.AggregatedConfigSource{},
				},
			},
		},
		// Enable TLS upstream with the configured client certificate.
		TlsContext: &envoyauth.UpstreamTlsContext{
			CommonTlsContext: makeCommonTLSContext(cfgSnap),
		},
	}
}
