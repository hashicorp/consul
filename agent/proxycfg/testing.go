package proxycfg

import (
	"fmt"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

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

// TestIntentions returns a sample intentions match result useful to
// mocking service discovery cache results.
func TestIntentions() *structs.IndexedIntentionMatches {
	return &structs.IndexedIntentionMatches{
		Matches: []structs.Intentions{
			[]*structs.Intention{
				{
					ID:              "foo",
					SourceNS:        "default",
					SourceName:      "billing",
					DestinationNS:   "default",
					DestinationName: "web",
					Action:          structs.IntentionActionAllow,
				},
			},
		},
	}
}

// TestUpstreamNodes returns a sample service discovery result useful to
// mocking service discovery cache results.
func TestUpstreamNodes(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.10.1.1",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeService(t),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.10.1.2",
				Datacenter: "dc1",
			},
			Service: structs.TestNodeService(t),
		},
	}
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
