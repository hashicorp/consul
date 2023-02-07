package troubleshoot

import (
	"testing"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGetUpstreamIPsFromFilterChain(t *testing.T) {
	filterChainJson := `{
		"name": "outbound_listener:127.0.0.1:15001",
		"address": {
		 "socket_address": {
		  "address": "127.0.0.1",
		  "port_value": 15001
		 }
		},
		"filter_chains":[
	{
	 "filter_chain_match": {
	  "prefix_ranges": [
	   {
		"address_prefix": "10.244.0.63",
		"prefix_len": 32
	   },
	   {
		"address_prefix": "10.244.0.64",
		"prefix_len": 32
	   }
	  ]
	 },
	 "filters": [
	  {
	   "name": "envoy.filters.network.tcp_proxy",
	   "typed_config": {
		"@type": "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
		"stat_prefix": "upstream.foo.default.default.dc1",
		"cluster": "passthrough~foo.default.dc1.internal.dc1.consul"
	   }
	  }
	 ]
	},
	{
	 "filter_chain_match": {
	  "prefix_ranges": [
	   {
		"address_prefix": "10.96.5.96",
		"prefix_len": 32
	   },
	   {
		"address_prefix": "240.0.0.1",
		"prefix_len": 32
	   }
	  ]
	 },
	 "filters": [
	  {
	   "name": "envoy.filters.network.http_connection_manager",
	   "typed_config": {
		"@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
		"stat_prefix": "upstream.foo.default.default.dc1",
		"route_config": {
		 "name": "foo",
		 "virtual_hosts": [
		  {
		   "name": "foo.default.default.dc1",
		   "domains": [
			"*"
		   ],
		   "routes": [
			{
			 "match": {
			  "prefix": "/"
			 },
			 "route": {
			  "cluster": "foo.default.dc1.internal.dc1.consul"
			 }
			}
		   ]
		  }
		 ]
		},
		"http_filters": [
		 {
		  "name": "envoy.filters.http.router",
		  "typed_config": {
		   "@type": "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router"
		  }
		 }
		],
		"tracing": {
		 "random_sampling": {}
		}
	   }
	  }
	 ]
	}
   ]}`

	expected := []UpstreamIP{
		{
			IPs: []string{
				"10.244.0.63",
				"10.244.0.64",
			},
			IsVirtual:    false,
			ClusterNames: map[string]struct{}{"passthrough~foo.default.dc1.internal.dc1.consul": struct{}{}},
		},
		{
			IPs: []string{
				"10.96.5.96",
				"240.0.0.1",
			},
			IsVirtual:    true,
			ClusterNames: map[string]struct{}{"foo.default.dc1.internal.dc1.consul": struct{}{}},
		},
	}

	var listener envoy_listener_v3.Listener
	err := protojson.Unmarshal([]byte(filterChainJson), &listener)
	require.NoError(t, err)

	upstream_ips, err := getUpstreamIPsFromFilterChain(listener.GetFilterChains())
	require.NoError(t, err)

	require.Equal(t, expected, upstream_ips)
}
