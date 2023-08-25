package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
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

const peerTrustDomain = "1c053652-8512-4373-90cf-5a7f6263a994.consul"

func TestCheckNodeServiceWithNameInPeer(t testing.T, name, dc, peer, ip string, useHostname bool, remoteEntMeta acl.EnterpriseMeta) CheckServiceNode {

	// Non-default partitions have a different spiffe format.
	spiffe := fmt.Sprintf("spiffe://%s/ns/default/dc/%s/svc/%s", peerTrustDomain, dc, name)
	if !remoteEntMeta.InDefaultPartition() {
		spiffe = fmt.Sprintf("spiffe://%s/ap/%s/ns/%s/dc/%s/svc/%s",
			peerTrustDomain, remoteEntMeta.PartitionOrDefault(), remoteEntMeta.NamespaceOrDefault(), dc, name)
	}
	service := &NodeService{
		Kind:    ServiceKindTypical,
		Service: name,
		// We should not see this port number appear in most xds golden tests,
		// because the WAN addr should typically be used.
		Port:     9090,
		PeerName: peer,
		Connect: ServiceConnect{
			PeerMeta: &PeeringServiceMeta{
				SNI: []string{
					fmt.Sprintf("%s.%s.%s.%s.external.%s",
						name, remoteEntMeta.NamespaceOrDefault(), remoteEntMeta.PartitionOrDefault(), peer, peerTrustDomain),
				},
				SpiffeID: []string{spiffe},
				Protocol: "tcp",
			},
		},
		// This value should typically be seen in golden file output, since this is a peered service.
		TaggedAddresses: map[string]ServiceAddress{
			TaggedAddressWAN: {
				Address: ip,
				Port:    8080,
			},
		},
	}

	if useHostname {
		service.TaggedAddresses = map[string]ServiceAddress{
			TaggedAddressLAN: {
				Address: ip,
				Port:    443,
			},
			TaggedAddressWAN: {
				Address: name + ".us-east-1.elb.notaws.com",
				Port:    8443,
			},
		}
	}

	return CheckServiceNode{
		Node: &Node{
			ID:   "test1",
			Node: "test1",
			// We should not see this address appear in most xds golden tests,
			// because the WAN addr should typically be used.
			Address:    "1.23.45.67",
			Datacenter: dc,
		},
		Service: service,
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

func TestNodeServiceAPIGateway(t testing.T) *NodeService {
	return &NodeService{
		Kind:    ServiceKindAPIGateway,
		Service: "api-gateway",
		Address: "1.1.1.1",
	}
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
