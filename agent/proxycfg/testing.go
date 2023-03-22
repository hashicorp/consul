package proxycfg

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func TestPeerTrustBundles(t testing.T) *pbpeering.TrustBundleListByServiceResponse {
	return &pbpeering.TrustBundleListByServiceResponse{
		Bundles: []*pbpeering.PeeringTrustBundle{
			{
				PeerName:    "peer-a",
				TrustDomain: "1c053652-8512-4373-90cf-5a7f6263a994.consul",
				RootPEMs: []string{`-----BEGIN CERTIFICATE-----
MIICczCCAdwCCQC3BLnEmLCrSjANBgkqhkiG9w0BAQsFADB+MQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQVoxEjAQBgNVBAcMCUZsYWdzdGFmZjEMMAoGA1UECgwDRm9v
MRAwDgYDVQQLDAdleGFtcGxlMQ8wDQYDVQQDDAZwZWVyLWExHTAbBgkqhkiG9w0B
CQEWDmZvb0BwZWVyLWEuY29tMB4XDTIyMDUyNjAxMDQ0NFoXDTIzMDUyNjAxMDQ0
NFowfjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkFaMRIwEAYDVQQHDAlGbGFnc3Rh
ZmYxDDAKBgNVBAoMA0ZvbzEQMA4GA1UECwwHZXhhbXBsZTEPMA0GA1UEAwwGcGVl
ci1hMR0wGwYJKoZIhvcNAQkBFg5mb29AcGVlci1hLmNvbTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEA2zFYGTbXDAntT5pLTpZ2+VTiqx4J63VRJH1kdu11f0FV
c2jl1pqCuYDbQXknDU0Pv1Q5y0+nSAihD2KqGS571r+vHQiPtKYPYRqPEe9FzAhR
2KhWH6v/tk5DG1HqOjV9/zWRKB12gdFNZZqnw/e7NjLNq3wZ2UAwxXip5uJ8uwMC
AwEAATANBgkqhkiG9w0BAQsFAAOBgQC/CJ9Syf4aL91wZizKTejwouRYoWv4gRAk
yto45ZcNMHfJ0G2z+XAMl9ZbQsLgXmzAx4IM6y5Jckq8pKC4PEijCjlKTktLHlEy
0ggmFxtNB1tid2NC8dOzcQ3l45+gDjDqdILhAvLDjlAIebdkqVqb2CfFNW/I2CQH
ZAuKN1aoKA==
-----END CERTIFICATE-----`},
			},
			{
				PeerName:    "peer-b",
				TrustDomain: "d89ac423-e95a-475d-94f2-1c557c57bf31.consul",
				RootPEMs: []string{`-----BEGIN CERTIFICATE-----
MIICcTCCAdoCCQDyGxC08cD0BDANBgkqhkiG9w0BAQsFADB9MQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExETAPBgNVBAcMCENhcmxzYmFkMQwwCgYDVQQKDANGb28x
EDAOBgNVBAsMB2V4YW1wbGUxDzANBgNVBAMMBnBlZXItYjEdMBsGCSqGSIb3DQEJ
ARYOZm9vQHBlZXItYi5jb20wHhcNMjIwNTI2MDExNjE2WhcNMjMwNTI2MDExNjE2
WjB9MQswCQYDVQQGEwJVUzELMAkGA1UECAwCQ0ExETAPBgNVBAcMCENhcmxzYmFk
MQwwCgYDVQQKDANGb28xEDAOBgNVBAsMB2V4YW1wbGUxDzANBgNVBAMMBnBlZXIt
YjEdMBsGCSqGSIb3DQEJARYOZm9vQHBlZXItYi5jb20wgZ8wDQYJKoZIhvcNAQEB
BQADgY0AMIGJAoGBAL4i5erdZ5vKk3mzW9Qt6Wvw/WN/IpMDlL0a28wz9oDCtMLN
cD/XQB9yT5jUwb2s4mD1lCDZtee8MHeD8zygICozufWVB+u2KvMaoA50T9GMQD0E
z/0nz/Z703I4q13VHeTpltmEpYcfxw/7nJ3leKA34+Nj3zteJ70iqvD/TNBBAgMB
AAEwDQYJKoZIhvcNAQELBQADgYEAbL04gicH+EIznDNhZJEb1guMBtBBJ8kujPyU
ao8xhlUuorDTLwhLpkKsOhD8619oSS8KynjEBichidQRkwxIaze0a2mrGT+tGBMf
pVz6UeCkqpde6bSJ/ozEe/2seQzKqYvRT1oUjLwYvY7OIh2DzYibOAxh6fewYAmU
5j5qNLc=
-----END CERTIFICATE-----`},
			},
		},
	}
}

// TestCerts generates a CA and Leaf suitable for returning as mock CA
// root/leaf cache requests.
func TestCerts(t testing.T) (*structs.IndexedCARoots, *structs.IssuedCert) {
	t.Helper()

	ca := connect.TestCA(t, nil)
	roots := &structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  fmt.Sprintf("%s.consul", connect.TestClusterID),
		Roots:        []*structs.CARoot{ca},
	}
	return roots, TestLeafForCA(t, ca)
}

// TestLeafForCA generates new Leaf suitable for returning as mock CA
// leaf cache response, signed by an existing CA.
func TestLeafForCA(t testing.T, ca *structs.CARoot) *structs.IssuedCert {
	leafPEM, pkPEM := connect.TestLeaf(t, "web", ca)

	leafCert, err := connect.ParseCert(leafPEM)
	require.NoError(t, err)

	return &structs.IssuedCert{
		SerialNumber:  connect.EncodeSerialNumber(leafCert.SerialNumber),
		CertPEM:       leafPEM,
		PrivateKeyPEM: pkPEM,
		Service:       "web",
		ServiceURI:    leafCert.URIs[0].String(),
		ValidAfter:    leafCert.NotBefore,
		ValidBefore:   leafCert.NotAfter,
	}
}

// TestCertsForMeshGateway generates a CA and Leaf suitable for returning as
// mock CA root/leaf cache requests in a mesh-gateway for peering.
func TestCertsForMeshGateway(t testing.T) (*structs.IndexedCARoots, *structs.IssuedCert) {
	t.Helper()

	ca := connect.TestCA(t, nil)
	roots := &structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  fmt.Sprintf("%s.consul", connect.TestClusterID),
		Roots:        []*structs.CARoot{ca},
	}
	return roots, TestMeshGatewayLeafForCA(t, ca)
}

// TestMeshGatewayLeafForCA generates new mesh-gateway Leaf suitable for returning as mock CA
// leaf cache response, signed by an existing CA.
func TestMeshGatewayLeafForCA(t testing.T, ca *structs.CARoot) *structs.IssuedCert {
	leafPEM, pkPEM := connect.TestMeshGatewayLeaf(t, "default", ca)

	leafCert, err := connect.ParseCert(leafPEM)
	require.NoError(t, err)

	return &structs.IssuedCert{
		SerialNumber:  connect.EncodeSerialNumber(leafCert.SerialNumber),
		CertPEM:       leafPEM,
		PrivateKeyPEM: pkPEM,
		Kind:          structs.ServiceKindMeshGateway,
		KindURI:       leafCert.URIs[0].String(),
		ValidAfter:    leafCert.NotBefore,
		ValidBefore:   leafCert.NotAfter,
	}
}

// TestIntentions returns a sample intentions match result useful to
// mocking service discovery cache results.
func TestIntentions() structs.Intentions {
	return structs.Intentions{
		{
			ID:              "foo",
			SourceNS:        "default",
			SourceName:      "billing",
			DestinationNS:   "default",
			DestinationName: "web",
			Action:          structs.IntentionActionAllow,
		},
	}
}

// TestUpstreamNodes returns a sample service discovery result useful to
// mocking service discovery cache results.
func TestUpstreamNodes(t testing.T, service string) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.10.1.1",
				Datacenter: "dc1",
				Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
			Service: structs.TestNodeServiceWithName(t, service),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.10.1.2",
				Datacenter: "dc1",
				Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
			Service: structs.TestNodeServiceWithName(t, service),
		},
	}
}

// TestPreparedQueryNodes returns instances of a service spread across two datacenters.
// The service instance names use a "-target" suffix to ensure we don't use the
// prepared query's name for SAN validation.
// The name of prepared queries won't always match the name of the service they target.
func TestPreparedQueryNodes(t testing.T, query string) structs.CheckServiceNodes {
	nodes := structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.10.1.1",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: query + "-sidecar-proxy",
				Port:    8080,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: query + "-target",
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.20.1.2",
				Datacenter: "dc2",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				Service: query + "-target",
				Port:    8080,
				Connect: structs.ServiceConnect{Native: true},
			},
		},
	}
	return nodes
}

func TestUpstreamNodesInStatus(t testing.T, status string) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.10.1.1",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeService(t),
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "test1",
					ServiceName: "web",
					Name:        "force",
					Status:      status,
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.10.1.2",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeService(t),
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "test2",
					ServiceName: "web",
					Name:        "force",
					Status:      status,
				},
			},
		},
	}
}

func TestUpstreamNodesDC2(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.20.1.1",
				Datacenter: "dc2",
			},
			Service: structs.TestNodeService(t),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.20.1.2",
				Datacenter: "dc2",
			},
			Service: structs.TestNodeService(t),
		},
	}
}

func TestUpstreamNodesInStatusDC2(t testing.T, status string) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.20.1.1",
				Datacenter: "dc2",
			},
			Service: structs.TestNodeService(t),
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "test1",
					ServiceName: "web",
					Name:        "force",
					Status:      status,
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.20.1.2",
				Datacenter: "dc2",
			},
			Service: structs.TestNodeService(t),
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "test2",
					ServiceName: "web",
					Name:        "force",
					Status:      status,
				},
			},
		},
	}
}

func TestUpstreamNodesAlternate(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "alt-test1",
				Node:       "alt-test1",
				Address:    "10.20.1.1",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeService(t),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "alt-test2",
				Node:       "alt-test2",
				Address:    "10.20.1.2",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeService(t),
		},
	}
}

func TestGatewayNodesDC1(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-1",
				Node:       "mesh-gateway",
				Address:    "10.10.1.1",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.10.1.1", 8443,
				structs.ServiceAddress{Address: "10.10.1.1", Port: 8443},
				structs.ServiceAddress{Address: "198.118.1.1", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-2",
				Node:       "mesh-gateway",
				Address:    "10.10.1.2",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.10.1.2", 8443,
				structs.ServiceAddress{Address: "10.0.1.2", Port: 8443},
				structs.ServiceAddress{Address: "198.118.1.2", Port: 443}),
		},
	}
}

func TestGatewayNodesDC2(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-1",
				Node:       "mesh-gateway",
				Address:    "10.0.1.1",
				Datacenter: "dc2",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.0.1.1", 8443,
				structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
				structs.ServiceAddress{Address: "198.18.1.1", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-2",
				Node:       "mesh-gateway",
				Address:    "10.0.1.2",
				Datacenter: "dc2",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.0.1.2", 8443,
				structs.ServiceAddress{Address: "10.0.1.2", Port: 8443},
				structs.ServiceAddress{Address: "198.18.1.2", Port: 443}),
		},
	}
}

func TestGatewayNodesDC3(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-1",
				Node:       "mesh-gateway",
				Address:    "10.30.1.1",
				Datacenter: "dc3",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.1", 8443,
				structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
				structs.ServiceAddress{Address: "198.38.1.1", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-2",
				Node:       "mesh-gateway",
				Address:    "10.30.1.2",
				Datacenter: "dc3",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.2", 8443,
				structs.ServiceAddress{Address: "10.30.1.2", Port: 8443},
				structs.ServiceAddress{Address: "198.38.1.2", Port: 443}),
		},
	}
}

func TestGatewayNodesDC4Hostname(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-1",
				Node:       "mesh-gateway",
				Address:    "10.30.1.1",
				Datacenter: "dc4",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.1", 8443,
				structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
				structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-2",
				Node:       "mesh-gateway",
				Address:    "10.30.1.2",
				Datacenter: "dc4",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.2", 8443,
				structs.ServiceAddress{Address: "10.30.1.2", Port: 8443},
				structs.ServiceAddress{Address: "456.us-west-2.elb.notaws.com", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-3",
				Node:       "mesh-gateway",
				Address:    "10.30.1.3",
				Datacenter: "dc4",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.3", 8443,
				structs.ServiceAddress{Address: "10.30.1.3", Port: 8443},
				structs.ServiceAddress{Address: "198.38.1.1", Port: 443}),
		},
	}
}

func TestGatewayNodesDC5Hostname(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-1",
				Node:       "mesh-gateway",
				Address:    "10.30.1.1",
				Datacenter: "dc5",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.1", 8443,
				structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
				structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-2",
				Node:       "mesh-gateway",
				Address:    "10.30.1.2",
				Datacenter: "dc5",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.2", 8443,
				structs.ServiceAddress{Address: "10.30.1.2", Port: 8443},
				structs.ServiceAddress{Address: "456.us-west-2.elb.notaws.com", Port: 443}),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-3",
				Node:       "mesh-gateway",
				Address:    "10.30.1.3",
				Datacenter: "dc5",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.3", 8443,
				structs.ServiceAddress{Address: "10.30.1.3", Port: 8443},
				structs.ServiceAddress{Address: "198.38.1.1", Port: 443}),
		},
	}
}

func TestGatewayNodesDC6Hostname(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "mesh-gateway-1",
				Node:       "mesh-gateway",
				Address:    "10.30.1.1",
				Datacenter: "dc6",
			},
			Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.30.1.1", 8443,
				structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
				structs.ServiceAddress{Address: "123.us-east-1.elb.notaws.com", Port: 443}),
			Checks: structs.HealthChecks{
				{
					Status: api.HealthCritical,
				},
			},
		},
	}
}

func TestGatewayServiceGroupBarDC1(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "bar-node-1",
				Node:       "bar-node-1",
				Address:    "10.1.1.4",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "bar-sidecar-proxy",
				Address: "172.16.1.6",
				Port:    2222,
				Meta: map[string]string{
					"version": "1",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "bar",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "bar-node-2",
				Node:       "bar-node-2",
				Address:    "10.1.1.5",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "bar-sidecar-proxy",
				Address: "172.16.1.7",
				Port:    2222,
				Meta: map[string]string{
					"version": "1",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "bar",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "bar-node-3",
				Node:       "bar-node-3",
				Address:    "10.1.1.6",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "bar-sidecar-proxy",
				Address: "172.16.1.8",
				Port:    2222,
				Meta: map[string]string{
					"version": "2",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "bar",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
		},
	}
}

func TestGatewayServiceGroupFooDC1(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "foo-node-1",
				Node:       "foo-node-1",
				Address:    "10.1.1.1",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "foo-sidecar-proxy",
				Address: "172.16.1.3",
				Port:    2222,
				Meta: map[string]string{
					"version": "1",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "foo-node-2",
				Node:       "foo-node-2",
				Address:    "10.1.1.2",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "foo-sidecar-proxy",
				Address: "172.16.1.4",
				Port:    2222,
				Meta: map[string]string{
					"version": "1",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "foo-node-3",
				Node:       "foo-node-3",
				Address:    "10.1.1.3",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "foo-sidecar-proxy",
				Address: "172.16.1.5",
				Port:    2222,
				Meta: map[string]string{
					"version": "2",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "foo-node-4",
				Node:       "foo-node-4",
				Address:    "10.1.1.7",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "foo-sidecar-proxy",
				Address: "172.16.1.9",
				Port:    2222,
				Meta: map[string]string{
					"version": "2",
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					Upstreams:              structs.TestUpstreams(t, false),
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo-node-4",
					ServiceName: "foo-sidecar-proxy",
					Name:        "proxy-alive",
					Status:      "warning",
				},
			},
		},
	}
}

type noopDataSource[ReqType any] struct{}

func (*noopDataSource[ReqType]) Notify(context.Context, ReqType, string, chan<- UpdateEvent) error {
	return nil
}

// testConfigSnapshotFixture helps you execute normal proxycfg event machinery
// to assemble a ConfigSnapshot via standard means to ensure test data used in
// any tests is actually a valid configuration.
//
// The provided ns argument will be manipulated by the nsFn callback if present
// before it is used.
//
// The events provided in the updates slice will be fed into the event
// machinery.
func testConfigSnapshotFixture(
	t testing.T,
	ns *structs.NodeService,
	nsFn func(ns *structs.NodeService),
	serverSNIFn ServerSNIFunc,
	updates []UpdateEvent,
) *ConfigSnapshot {
	const token = ""

	if nsFn != nil {
		nsFn(ns)
	}

	config := stateConfig{
		logger: hclog.NewNullLogger(),
		source: &structs.QuerySource{
			Datacenter: "dc1",
		},
		dataSources: DataSources{
			CARoots:                         &noopDataSource[*structs.DCSpecificRequest]{},
			CompiledDiscoveryChain:          &noopDataSource[*structs.DiscoveryChainRequest]{},
			ConfigEntry:                     &noopDataSource[*structs.ConfigEntryQuery]{},
			ConfigEntryList:                 &noopDataSource[*structs.ConfigEntryQuery]{},
			Datacenters:                     &noopDataSource[*structs.DatacentersRequest]{},
			FederationStateListMeshGateways: &noopDataSource[*structs.DCSpecificRequest]{},
			GatewayServices:                 &noopDataSource[*structs.ServiceSpecificRequest]{},
			ServiceGateways:                 &noopDataSource[*structs.ServiceSpecificRequest]{},
			Health:                          &noopDataSource[*structs.ServiceSpecificRequest]{},
			HTTPChecks:                      &noopDataSource[*cachetype.ServiceHTTPChecksRequest]{},
			Intentions:                      &noopDataSource[*structs.ServiceSpecificRequest]{},
			IntentionUpstreams:              &noopDataSource[*structs.ServiceSpecificRequest]{},
			IntentionUpstreamsDestination:   &noopDataSource[*structs.ServiceSpecificRequest]{},
			InternalServiceDump:             &noopDataSource[*structs.ServiceDumpRequest]{},
			LeafCertificate:                 &noopDataSource[*cachetype.ConnectCALeafRequest]{},
			PeeringList:                     &noopDataSource[*cachetype.PeeringListRequest]{},
			PeeredUpstreams:                 &noopDataSource[*structs.PartitionSpecificRequest]{},
			PreparedQuery:                   &noopDataSource[*structs.PreparedQueryExecuteRequest]{},
			ResolvedServiceConfig:           &noopDataSource[*structs.ServiceConfigRequest]{},
			ServiceList:                     &noopDataSource[*structs.DCSpecificRequest]{},
			TrustBundle:                     &noopDataSource[*cachetype.TrustBundleReadRequest]{},
			TrustBundleList:                 &noopDataSource[*cachetype.TrustBundleListRequest]{},
			ExportedPeeredServices:          &noopDataSource[*structs.DCSpecificRequest]{},
		},
		dnsConfig: DNSConfig{ // TODO: make configurable
			Domain:    "consul",
			AltDomain: "",
		},
		serverSNIFn:           serverSNIFn,
		intentionDefaultAllow: false, // TODO: make configurable
	}
	testConfigSnapshotFixtureEnterprise(&config)
	s, err := newServiceInstanceFromNodeService(ProxyID{ServiceID: ns.CompoundServiceID()}, ns, token)
	if err != nil {
		t.Fatalf("err: %v", err)
		return nil
	}

	handler, err := newKindHandler(config, s, nil) // NOTE: nil channel
	if err != nil {
		t.Fatalf("err: %v", err)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snap, err := handler.initialize(ctx)
	if err != nil {
		t.Fatalf("err: %v", err)
		return nil
	}

	for _, u := range updates {
		if err := handler.handleUpdate(ctx, u, &snap); err != nil {
			t.Fatalf("Failed to handle update from watch %q: %v", u.CorrelationID, err)
			return nil
		}
	}
	return &snap
}

func testSpliceEvents(base, extra []UpdateEvent) []UpdateEvent {
	if len(extra) == 0 {
		return base
	}
	var (
		hasExtra      = make(map[string]UpdateEvent)
		completeExtra = make(map[string]struct{})

		allEvents []UpdateEvent
	)

	for _, e := range extra {
		hasExtra[e.CorrelationID] = e
	}

	// Override base events with extras if they share the same correlationID,
	// then put the rest of the extras at the end.
	for _, e := range base {
		if extraEvt, ok := hasExtra[e.CorrelationID]; ok {
			if extraEvt.Result != nil { // nil results are tombstones
				allEvents = append(allEvents, extraEvt)
			}
			completeExtra[e.CorrelationID] = struct{}{}
		} else {
			allEvents = append(allEvents, e)
		}
	}
	for _, e := range extra {
		if _, ok := completeExtra[e.CorrelationID]; !ok {
			allEvents = append(allEvents, e)
		}
	}
	return allEvents
}

func testSpliceNodeServiceFunc(prev, next func(ns *structs.NodeService)) func(ns *structs.NodeService) {
	return func(ns *structs.NodeService) {
		if prev != nil {
			prev(ns)
		}
		next(ns)
	}
}

// ControllableCacheType is a cache.Type that simulates a typical blocking RPC
// but lets us control the responses and when they are delivered easily.
type ControllableCacheType struct {
	index uint64
	value sync.Map
	// Need a condvar to trigger all blocking requests (there might be multiple
	// for same type due to background refresh and timing issues) when values
	// change. Chans make it nondeterministic which one triggers or need extra
	// locking to coordinate replacing after close etc.
	triggerMu sync.Mutex
	trigger   *sync.Cond
	blocking  bool
	lastReq   atomic.Value
}

// NewControllableCacheType returns a cache.Type that can be controlled for
// testing.
func NewControllableCacheType(t testing.T) *ControllableCacheType {
	c := &ControllableCacheType{
		index:    5,
		blocking: true,
	}
	c.trigger = sync.NewCond(&c.triggerMu)
	return c
}

// Set sets the response value to be returned from subsequent cache gets for the
// type.
func (ct *ControllableCacheType) Set(key string, value interface{}) {
	atomic.AddUint64(&ct.index, 1)
	ct.value.Store(key, value)
	ct.triggerMu.Lock()
	ct.trigger.Broadcast()
	ct.triggerMu.Unlock()
}

// Fetch implements cache.Type. It simulates blocking or non-blocking queries.
func (ct *ControllableCacheType) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	index := atomic.LoadUint64(&ct.index)

	ct.lastReq.Store(req)

	shouldBlock := ct.blocking && opts.MinIndex > 0 && opts.MinIndex == index
	if shouldBlock {
		// Wait for return to be triggered. We ignore timeouts based on opts.Timeout
		// since in practice they will always be way longer than our tests run for
		// and the caller can simulate timeout by triggering return without changing
		// index or value.
		ct.triggerMu.Lock()
		ct.trigger.Wait()
		ct.triggerMu.Unlock()
	}

	info := req.CacheInfo()
	key := path.Join(info.Key, info.Datacenter) // omit token for testing purposes

	// reload index as it probably got bumped
	index = atomic.LoadUint64(&ct.index)
	val, _ := ct.value.Load(key)

	if err, ok := val.(error); ok {
		return cache.FetchResult{
			Value: nil,
			Index: index,
		}, err
	}
	return cache.FetchResult{
		Value: val,
		Index: index,
	}, nil
}

func (ct *ControllableCacheType) RegisterOptions() cache.RegisterOptions {
	return cache.RegisterOptions{
		Refresh:          ct.blocking,
		SupportsBlocking: ct.blocking,
		QueryTimeout:     10 * time.Minute,
	}
}

// golden is used to read golden files stores in consul/agent/xds/testdata
func golden(t testing.T, name string) string {
	t.Helper()

	golden := filepath.Join(projectRoot(), "../", "/xds/testdata", name+".golden")
	expected, err := os.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func projectRoot() string {
	_, base, _, _ := runtime.Caller(0)
	return filepath.Dir(base)
}

// NewTestDataSources creates a set of data sources that can be used to provide
// the Manager with data in tests.
func NewTestDataSources() *TestDataSources {
	srcs := &TestDataSources{
		CARoots:                         NewTestDataSource[*structs.DCSpecificRequest, *structs.IndexedCARoots](),
		CompiledDiscoveryChain:          NewTestDataSource[*structs.DiscoveryChainRequest, *structs.DiscoveryChainResponse](),
		ConfigEntry:                     NewTestDataSource[*structs.ConfigEntryQuery, *structs.ConfigEntryResponse](),
		ConfigEntryList:                 NewTestDataSource[*structs.ConfigEntryQuery, *structs.IndexedConfigEntries](),
		Datacenters:                     NewTestDataSource[*structs.DatacentersRequest, *[]string](),
		FederationStateListMeshGateways: NewTestDataSource[*structs.DCSpecificRequest, *structs.DatacenterIndexedCheckServiceNodes](),
		GatewayServices:                 NewTestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedGatewayServices](),
		Health:                          NewTestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedCheckServiceNodes](),
		HTTPChecks:                      NewTestDataSource[*cachetype.ServiceHTTPChecksRequest, []structs.CheckType](),
		Intentions:                      NewTestDataSource[*structs.ServiceSpecificRequest, structs.Intentions](),
		IntentionUpstreams:              NewTestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedServiceList](),
		IntentionUpstreamsDestination:   NewTestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedServiceList](),
		InternalServiceDump:             NewTestDataSource[*structs.ServiceDumpRequest, *structs.IndexedCheckServiceNodes](),
		LeafCertificate:                 NewTestDataSource[*cachetype.ConnectCALeafRequest, *structs.IssuedCert](),
		PeeringList:                     NewTestDataSource[*cachetype.PeeringListRequest, *pbpeering.PeeringListResponse](),
		PreparedQuery:                   NewTestDataSource[*structs.PreparedQueryExecuteRequest, *structs.PreparedQueryExecuteResponse](),
		ResolvedServiceConfig:           NewTestDataSource[*structs.ServiceConfigRequest, *structs.ServiceConfigResponse](),
		ServiceList:                     NewTestDataSource[*structs.DCSpecificRequest, *structs.IndexedServiceList](),
		TrustBundle:                     NewTestDataSource[*cachetype.TrustBundleReadRequest, *pbpeering.TrustBundleReadResponse](),
		TrustBundleList:                 NewTestDataSource[*cachetype.TrustBundleListRequest, *pbpeering.TrustBundleListByServiceResponse](),
	}
	srcs.buildEnterpriseSources()
	return srcs
}

type TestDataSources struct {
	CARoots                         *TestDataSource[*structs.DCSpecificRequest, *structs.IndexedCARoots]
	CompiledDiscoveryChain          *TestDataSource[*structs.DiscoveryChainRequest, *structs.DiscoveryChainResponse]
	ConfigEntry                     *TestDataSource[*structs.ConfigEntryQuery, *structs.ConfigEntryResponse]
	ConfigEntryList                 *TestDataSource[*structs.ConfigEntryQuery, *structs.IndexedConfigEntries]
	FederationStateListMeshGateways *TestDataSource[*structs.DCSpecificRequest, *structs.DatacenterIndexedCheckServiceNodes]
	Datacenters                     *TestDataSource[*structs.DatacentersRequest, *[]string]
	GatewayServices                 *TestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedGatewayServices]
	ServiceGateways                 *TestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedServiceNodes]
	Health                          *TestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedCheckServiceNodes]
	HTTPChecks                      *TestDataSource[*cachetype.ServiceHTTPChecksRequest, []structs.CheckType]
	Intentions                      *TestDataSource[*structs.ServiceSpecificRequest, structs.Intentions]
	IntentionUpstreams              *TestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedServiceList]
	IntentionUpstreamsDestination   *TestDataSource[*structs.ServiceSpecificRequest, *structs.IndexedServiceList]
	InternalServiceDump             *TestDataSource[*structs.ServiceDumpRequest, *structs.IndexedCheckServiceNodes]
	LeafCertificate                 *TestDataSource[*cachetype.ConnectCALeafRequest, *structs.IssuedCert]
	PeeringList                     *TestDataSource[*cachetype.PeeringListRequest, *pbpeering.PeeringListResponse]
	PeeredUpstreams                 *TestDataSource[*structs.PartitionSpecificRequest, *structs.IndexedPeeredServiceList]
	PreparedQuery                   *TestDataSource[*structs.PreparedQueryExecuteRequest, *structs.PreparedQueryExecuteResponse]
	ResolvedServiceConfig           *TestDataSource[*structs.ServiceConfigRequest, *structs.ServiceConfigResponse]
	ServiceList                     *TestDataSource[*structs.DCSpecificRequest, *structs.IndexedServiceList]
	TrustBundle                     *TestDataSource[*cachetype.TrustBundleReadRequest, *pbpeering.TrustBundleReadResponse]
	TrustBundleList                 *TestDataSource[*cachetype.TrustBundleListRequest, *pbpeering.TrustBundleListByServiceResponse]

	TestDataSourcesEnterprise
}

func (t *TestDataSources) ToDataSources() DataSources {
	ds := DataSources{
		CARoots:                       t.CARoots,
		CompiledDiscoveryChain:        t.CompiledDiscoveryChain,
		ConfigEntry:                   t.ConfigEntry,
		ConfigEntryList:               t.ConfigEntryList,
		Datacenters:                   t.Datacenters,
		GatewayServices:               t.GatewayServices,
		ServiceGateways:               t.ServiceGateways,
		Health:                        t.Health,
		HTTPChecks:                    t.HTTPChecks,
		Intentions:                    t.Intentions,
		IntentionUpstreams:            t.IntentionUpstreams,
		IntentionUpstreamsDestination: t.IntentionUpstreamsDestination,
		InternalServiceDump:           t.InternalServiceDump,
		LeafCertificate:               t.LeafCertificate,
		PeeringList:                   t.PeeringList,
		PeeredUpstreams:               t.PeeredUpstreams,
		PreparedQuery:                 t.PreparedQuery,
		ResolvedServiceConfig:         t.ResolvedServiceConfig,
		ServiceList:                   t.ServiceList,
		TrustBundle:                   t.TrustBundle,
		TrustBundleList:               t.TrustBundleList,
	}
	t.fillEnterpriseDataSources(&ds)
	return ds
}

// NewTestDataSource creates a test data source that accepts requests to Notify
// of type RequestType and dispatches UpdateEvents with a result of type ValType.
//
// TODO(agentless): we still depend on cache.Request here because it provides the
// CacheInfo method used for hashing the request - this won't work when we extract
// this package into a shared library.
func NewTestDataSource[ReqType cache.Request, ValType any]() *TestDataSource[ReqType, ValType] {
	return &TestDataSource[ReqType, ValType]{
		data:    make(map[string]ValType),
		trigger: make(chan struct{}),
	}
}

type TestDataSource[ReqType cache.Request, ValType any] struct {
	mu      sync.Mutex
	data    map[string]ValType
	lastReq ReqType

	// Note: trigger is currently global for all requests of the given type, so
	// Manager may receive duplicate events - as the dispatch goroutine will be
	// woken up whenever *any* requested data changes.
	trigger chan struct{}
}

// Notify satisfies the interfaces used by Manager to subscribe to data.
func (t *TestDataSource[ReqType, ValType]) Notify(ctx context.Context, req ReqType, correlationID string, ch chan<- UpdateEvent) error {
	t.mu.Lock()
	t.lastReq = req
	t.mu.Unlock()

	go t.dispatch(ctx, correlationID, t.reqKey(req), ch)

	return nil
}

func (t *TestDataSource[ReqType, ValType]) dispatch(ctx context.Context, correlationID, key string, ch chan<- UpdateEvent) {
	for {
		t.mu.Lock()
		val, ok := t.data[key]
		trigger := t.trigger
		t.mu.Unlock()

		if ok {
			event := UpdateEvent{
				CorrelationID: correlationID,
				Result:        val,
			}

			select {
			case ch <- event:
			case <-ctx.Done():
			}
		}

		select {
		case <-trigger:
		case <-ctx.Done():
			return
		}
	}
}

func (t *TestDataSource[ReqType, ValType]) reqKey(req ReqType) string {
	return req.CacheInfo().Key
}

// Set broadcasts the given value to consumers that subscribed with the given
// request.
func (t *TestDataSource[ReqType, ValType]) Set(req ReqType, val ValType) error {
	t.mu.Lock()
	t.data[t.reqKey(req)] = val
	oldTrigger := t.trigger
	t.trigger = make(chan struct{})
	t.mu.Unlock()

	close(oldTrigger)

	return nil
}

// LastReq returns the request from the last call to Notify that was received.
func (t *TestDataSource[ReqType, ValType]) LastReq() ReqType {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.lastReq
}
