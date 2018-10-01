package xds

import (
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/consul/agent/proxycfg"
)

// clustersFromSnapshot returns the xDS API reprepsentation of the "clusters"
// (upstreams) in the snapshot.
func clustersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	// Inlude the "app" cluster for the public listener
	clusters := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	clusters[0] = makeAppCluster(cfgSnap)

	for idx, upstream := range cfgSnap.Proxy.Upstreams {
		clusters[idx+1] = makeUpstreamCluster(upstream.Identifier(), cfgSnap)
	}

	return clusters, nil
}

func makeAppCluster(cfgSnap *proxycfg.ConfigSnapshot) *v2.Cluster {
	addr := cfgSnap.Proxy.LocalServiceAddress
	if addr == "" {
		addr = "127.0.0.1"
	}
	return &v2.Cluster{
		Name: LocalAppClusterName,
		// TODO(banks): make this configurable from the proxy config
		ConnectTimeout: 5 * time.Second,
		Type:           v2.Cluster_STATIC,
		// API v2 docs say hosts is deprecated and should use LoadAssignment as
		// below.. but it doesn't work for tcp_proxy target for some reason.
		Hosts: []*core.Address{makeAddressPtr(addr, cfgSnap.Proxy.LocalServicePort)},
		// LoadAssignment: &v2.ClusterLoadAssignment{
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

func makeUpstreamCluster(name string, cfgSnap *proxycfg.ConfigSnapshot) *v2.Cluster {
	return &v2.Cluster{
		Name: name,
		// TODO(banks): make this configurable from the upstream config
		ConnectTimeout: 5 * time.Second,
		Type:           v2.Cluster_EDS,
		EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
			EdsConfig: &core.ConfigSource{
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		},
		// Enable TLS upstream with the configured client certificate.
		TlsContext: &auth.UpstreamTlsContext{
			CommonTlsContext: makeCommonTLSContext(cfgSnap),
		},
	}
}
