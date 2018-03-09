package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

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

// TestNodeServiceProxy returns a *NodeService representing a valid
// Connect proxy.
func TestNodeServiceProxy(t testing.T) *NodeService {
	return &NodeService{
		Kind:             ServiceKindConnectProxy,
		Service:          ConnectProxyServiceName,
		Address:          "127.0.0.2",
		Port:             2222,
		ProxyDestination: "web",
	}
}
