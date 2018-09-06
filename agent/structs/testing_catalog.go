package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestRegisterRequest returns a RegisterRequest for registering a typical service.
func TestRegisterRequest(t testing.T) *RegisterRequest {
	return &RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &NodeService{
			Service: "web",
			Address: "",
			Port:    80,
		},
	}
}

// TestRegisterRequestProxy returns a RegisterRequest for registering a
// Connect proxy.
func TestRegisterRequestProxy(t testing.T) *RegisterRequest {
	return &RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service:    TestNodeServiceProxy(t),
	}
}

// TestNodeService returns a *NodeService representing a valid regular service.
func TestNodeService(t testing.T) *NodeService {
	return &NodeService{
		Kind:    ServiceKindTypical,
		Service: "web",
	}
}

// TestNodeServiceProxy returns a *NodeService representing a valid
// Connect proxy.
func TestNodeServiceProxy(t testing.T) *NodeService {
	return &NodeService{
		Kind:    ServiceKindConnectProxy,
		Service: "web-proxy",
		Address: "127.0.0.2",
		Port:    2222,
		Proxy:   TestConnectProxyConfig(t),
	}
}

// TestConnectProxyConfig returns a ConnectProxyConfig representing a valid
// Connect proxy.
func TestConnectProxyConfig(t testing.T) ConnectProxyConfig {
	return ConnectProxyConfig{
		DestinationServiceName: "web",
		Upstreams: Upstreams{
			{
				DestinationType: UpstreamDestTypeService,
				DestinationName: "db",
				LocalBindPort:   9191,
			},
			{
				DestinationType: UpstreamDestTypePreparedQuery,
				DestinationName: "geo-cache",
				LocalBindPort:   8181,
			},
		},
	}
}
