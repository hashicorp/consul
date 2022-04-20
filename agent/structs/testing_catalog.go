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

// TestRegisterIngressGateway returns a RegisterRequest for registering an
// ingress gateway
func TestRegisterIngressGateway(t testing.T) *RegisterRequest {
	return &RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service:    TestNodeServiceIngressGateway(t, ""),
	}
}

// TestNodeService returns a *NodeService representing a valid regular service: "web".
func TestNodeService(t testing.T) *NodeService {
	return TestNodeServiceWithName(t, "web")
}

func TestNodeServiceWithName(t testing.T, name string) *NodeService {
	return &NodeService{
		Kind:    ServiceKindTypical,
		Service: name,
		Port:    8080,
	}
}

// TestNodeServiceProxy returns a *NodeService representing a valid
// Connect proxy.
func TestNodeServiceProxy(t testing.T) *NodeService {
	return TestNodeServiceProxyInPartition(t, "")
}

func TestNodeServiceProxyInPartition(t testing.T, partition string) *NodeService {
	entMeta := DefaultEnterpriseMetaInPartition(partition)
	return &NodeService{
		Kind:           ServiceKindConnectProxy,
		Service:        "web-proxy",
		Address:        "127.0.0.2",
		Port:           2222,
		Proxy:          TestConnectProxyConfig(t),
		EnterpriseMeta: *entMeta,
	}
}

func TestNodeServiceExpose(t testing.T) *NodeService {
	return &NodeService{
		Kind:    ServiceKindConnectProxy,
		Service: "test-svc",
		Address: "localhost",
		Port:    8080,
		Proxy: ConnectProxyConfig{
			DestinationServiceName: "web",
			Expose: ExposeConfig{
				Paths: []ExposePath{
					{
						Path:          "/foo",
						LocalPathPort: 8080,
						ListenerPort:  21500,
					},
					{
						Path:          "/bar",
						LocalPathPort: 8080,
						ListenerPort:  21501,
					},
				},
			},
		},
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

func TestNodeServiceTerminatingGateway(t testing.T, address string) *NodeService {
	return &NodeService{
		Kind:    ServiceKindTerminatingGateway,
		Port:    8443,
		Service: "terminating-gateway",
		Address: address,
	}
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
			TaggedAddressLAN: lanAddr,
			TaggedAddressWAN: wanAddr,
		},
		RaftIndex: RaftIndex{
			ModifyIndex: 1,
		},
	}
}

func TestNodeServiceIngressGateway(t testing.T, address string) *NodeService {
	return &NodeService{
		Kind:    ServiceKindIngressGateway,
		Service: "ingress-gateway",
		Address: address,
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
