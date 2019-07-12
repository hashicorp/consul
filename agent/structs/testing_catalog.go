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
		Port:    8080,
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

// TestNodeServiceMeshGateway returns a *NodeService representing a valid Mesh Gateway
func TestNodeServiceMeshGateway(t testing.T) *NodeService {
	return TestNodeServiceMeshGatewayWithAddrs(t,
		"10.1.2.3",
		8443,
		ServiceAddress{Address: "10.1.2.3", Port: 8443},
		ServiceAddress{Address: "198.18.4.5", Port: 443})
}

func TestNodeServiceMeshGatewayWithAddrs(t testing.T, address string, port int, lanAddr, wanAddr ServiceAddress) *NodeService {
	return &NodeService{
		Kind:    ServiceKindMeshGateway,
		Service: "mesh-gateway",
		Address: address,
		Port:    port,
		Proxy: ConnectProxyConfig{
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
		TaggedAddresses: map[string]ServiceAddress{
			"lan": lanAddr,
			"wan": wanAddr,
		},
	}
}

// TestNodeServiceSidecar returns a *NodeService representing a service
// registration with a nested Sidecar registration.
func TestNodeServiceSidecar(t testing.T) *NodeService {
	return &NodeService{
		Service: "web",
		Port:    2222,
		Connect: ServiceConnect{
			SidecarService: &ServiceDefinition{},
		},
	}
}
