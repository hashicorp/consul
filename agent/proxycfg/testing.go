package proxycfg

import (
	"context"
	"fmt"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// TestCacheTypes encapsulates all the different cache types proxycfg.State will
// watch/request for controlling one during testing.
type TestCacheTypes struct {
	roots             *ControllableCacheType
	leaf              *ControllableCacheType
	intentions        *ControllableCacheType
	health            *ControllableCacheType
	query             *ControllableCacheType
	compiledChain     *ControllableCacheType
	serviceHTTPChecks *ControllableCacheType
}

// NewTestCacheTypes creates a set of ControllableCacheTypes for all types that
// proxycfg will watch suitable for testing a proxycfg.State or Manager.
func NewTestCacheTypes(t testing.T) *TestCacheTypes {
	t.Helper()
	ct := &TestCacheTypes{
		roots:             NewControllableCacheType(t),
		leaf:              NewControllableCacheType(t),
		intentions:        NewControllableCacheType(t),
		health:            NewControllableCacheType(t),
		query:             NewControllableCacheType(t),
		compiledChain:     NewControllableCacheType(t),
		serviceHTTPChecks: NewControllableCacheType(t),
	}
	ct.query.blocking = false
	return ct
}

// TestCacheWithTypes registers ControllableCacheTypes for all types that
// proxycfg will watch suitable for testing a proxycfg.State or Manager.
func TestCacheWithTypes(t testing.T, types *TestCacheTypes) *cache.Cache {
	c := cache.TestCache(t)
	c.RegisterType(cachetype.ConnectCARootName, types.roots, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 10 * time.Minute,
	})
	c.RegisterType(cachetype.ConnectCALeafName, types.leaf, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 10 * time.Minute,
	})
	c.RegisterType(cachetype.IntentionMatchName, types.intentions, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 10 * time.Minute,
	})
	c.RegisterType(cachetype.HealthServicesName, types.health, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 10 * time.Minute,
	})
	c.RegisterType(cachetype.PreparedQueryName, types.query, &cache.RegisterOptions{
		Refresh: false,
	})
	c.RegisterType(cachetype.CompiledDiscoveryChainName, types.compiledChain, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 10 * time.Minute,
	})
	c.RegisterType(cachetype.ServiceHTTPChecksName, types.serviceHTTPChecks, &cache.RegisterOptions{})

	return c
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

// TestIntentions returns a sample intentions match result useful to
// mocking service discovery cache results.
func TestIntentions(t testing.T) *structs.IndexedIntentionMatches {
	return &structs.IndexedIntentionMatches{
		Matches: []structs.Intentions{
			[]*structs.Intention{
				&structs.Intention{
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

func TestUpstreamNodesDC3(t testing.T) structs.CheckServiceNodes {
	return structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test1",
				Node:       "test1",
				Address:    "10.30.1.1",
				Datacenter: "dc3",
			},
			Service: structs.TestNodeService(t),
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "test2",
				Node:       "test2",
				Address:    "10.30.1.2",
				Datacenter: "dc3",
			},
			Service: structs.TestNodeService(t),
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
					Upstreams:              structs.TestUpstreams(t),
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
					Upstreams:              structs.TestUpstreams(t),
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
					Upstreams:              structs.TestUpstreams(t),
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
					Upstreams:              structs.TestUpstreams(t),
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
					Upstreams:              structs.TestUpstreams(t),
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
					Upstreams:              structs.TestUpstreams(t),
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
					Upstreams:              structs.TestUpstreams(t),
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

// TestConfigSnapshot returns a fully populated snapshot
func TestConfigSnapshot(t testing.T) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	// no entries implies we'll get a default chain
	dbChain := discoverychain.TestCompileConfigEntries(
		t, "db", "default", "dc1",
		connect.TestClusterID+".consul", "dc1", nil)

	return &ConfigSnapshot{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-sidecar-proxy",
		ProxyID: structs.NewServiceID("web-sidecar-proxy", nil),
		Address: "0.0.0.0",
		Port:    9999,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceID:   "web",
			DestinationServiceName: "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"foo": "bar",
			},
			Upstreams: structs.TestUpstreams(t),
		},
		Roots: roots,
		ConnectProxy: configSnapshotConnectProxy{
			Leaf: leaf,
			DiscoveryChain: map[string]*structs.CompiledDiscoveryChain{
				"db": dbChain,
			},
			PreparedQueryEndpoints: map[string]structs.CheckServiceNodes{
				"prepared_query:geo-cache": TestUpstreamNodes(t),
			},
			WatchedUpstreamEndpoints: map[string]map[string]structs.CheckServiceNodes{
				"db": map[string]structs.CheckServiceNodes{
					"db.default.dc1": TestUpstreamNodes(t),
				},
			},
		},
		Datacenter: "dc1",
	}
}

// TestConfigSnapshotDiscoveryChain returns a fully populated snapshot using a discovery chain
func TestConfigSnapshotDiscoveryChain(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "simple")
}

func TestConfigSnapshotDiscoveryChainExternalSNI(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "external-sni")
}

func TestConfigSnapshotDiscoveryChainWithOverrides(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "simple-with-overrides")
}

func TestConfigSnapshotDiscoveryChainWithFailover(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGateway(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGatewayTriggered(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGateway(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGatewayTriggered(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGateway(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGatewayTriggered(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGateway(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGatewayTriggered(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc")
}

func TestConfigSnapshotDiscoveryChainWithEntries(t testing.T, additionalEntries ...structs.ConfigEntry) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "simple", additionalEntries...)
}

func TestConfigSnapshotDiscoveryChainDefault(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "default")
}

func testConfigSnapshotDiscoveryChain(t testing.T, variation string, additionalEntries ...structs.ConfigEntry) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	// Compile a chain.
	var (
		entries      []structs.ConfigEntry
		compileSetup func(req *discoverychain.CompileRequest)
	)
	switch variation {
	case "default":
		// no config entries
	case "simple-with-overrides":
		compileSetup = func(req *discoverychain.CompileRequest) {
			req.OverrideMeshGateway.Mode = structs.MeshGatewayModeLocal
			req.OverrideProtocol = "grpc"
			req.OverrideConnectTimeout = 66 * time.Second
		}
		fallthrough
	case "simple":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
			},
		)
	case "external-sni":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind:        structs.ServiceDefaults,
				Name:        "db",
				ExternalSNI: "db.some.other.service.mesh",
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
			},
		)
	case "failover":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Service: "fail",
					},
				},
			},
		)
	case "failover-through-remote-gateway-triggered":
		fallthrough
	case "failover-through-remote-gateway":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "db",
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2"},
					},
				},
			},
		)
	case "failover-through-double-remote-gateway-triggered":
		fallthrough
	case "failover-through-double-remote-gateway":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "db",
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2", "dc3"},
					},
				},
			},
		)
	case "failover-through-local-gateway-triggered":
		fallthrough
	case "failover-through-local-gateway":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "db",
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeLocal,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2"},
					},
				},
			},
		)
	case "failover-through-double-local-gateway-triggered":
		fallthrough
	case "failover-through-double-local-gateway":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "db",
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeLocal,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2", "dc3"},
					},
				},
			},
		)
	case "splitter-with-resolver-redirect-multidc":
		entries = append(entries,
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db",
				Splits: []structs.ServiceSplit{
					{Weight: 50, Service: "db-dc1"},
					{Weight: 50, Service: "db-dc2"},
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db-dc1",
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "db",
					ServiceSubset: "v1",
					Datacenter:    "dc1",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db-dc2",
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "db",
					ServiceSubset: "v2",
					Datacenter:    "dc2",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db",
				Subsets: map[string]structs.ServiceResolverSubset{
					"v1": structs.ServiceResolverSubset{
						Filter: "Service.Meta.version == v1",
					},
					"v2": structs.ServiceResolverSubset{
						Filter: "Service.Meta.version == v2",
					},
				},
			},
		)
	default:
		t.Fatalf("unexpected variation: %q", variation)
		return nil
	}

	if len(additionalEntries) > 0 {
		entries = append(entries, additionalEntries...)
	}

	dbChain := discoverychain.TestCompileConfigEntries(t, "db", "default", "dc1", connect.TestClusterID+".consul", "dc1", compileSetup, entries...)

	snap := &ConfigSnapshot{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-sidecar-proxy",
		ProxyID: structs.NewServiceID("web-sidecar-proxy", nil),
		Address: "0.0.0.0",
		Port:    9999,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceID:   "web",
			DestinationServiceName: "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"foo": "bar",
			},
			Upstreams: structs.TestUpstreams(t),
		},
		Roots: roots,
		ConnectProxy: configSnapshotConnectProxy{
			Leaf: leaf,
			DiscoveryChain: map[string]*structs.CompiledDiscoveryChain{
				"db": dbChain,
			},
			WatchedUpstreamEndpoints: map[string]map[string]structs.CheckServiceNodes{
				"db": map[string]structs.CheckServiceNodes{
					"db.default.dc1": TestUpstreamNodes(t),
				},
			},
		},
		Datacenter: "dc1",
	}

	switch variation {
	case "default":
	case "simple-with-overrides":
	case "simple":
	case "external-sni":
	case "failover":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["fail.default.dc1"] =
			TestUpstreamNodesAlternate(t)
	case "failover-through-remote-gateway-triggered":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			TestUpstreamNodesInStatus(t, "critical")
		fallthrough
	case "failover-through-remote-gateway":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			TestUpstreamNodesDC2(t)
		snap.ConnectProxy.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
			},
		}
	case "failover-through-double-remote-gateway-triggered":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			TestUpstreamNodesInStatus(t, "critical")
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			TestUpstreamNodesInStatusDC2(t, "critical")
		fallthrough
	case "failover-through-double-remote-gateway":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc3"] = TestUpstreamNodesDC2(t)
		snap.ConnectProxy.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
				"dc3": TestGatewayNodesDC3(t),
			},
		}
	case "failover-through-local-gateway-triggered":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			TestUpstreamNodesInStatus(t, "critical")
		fallthrough
	case "failover-through-local-gateway":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			TestUpstreamNodesDC2(t)
		snap.ConnectProxy.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": map[string]structs.CheckServiceNodes{
				"dc1": TestGatewayNodesDC1(t),
			},
		}
	case "failover-through-double-local-gateway-triggered":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			TestUpstreamNodesInStatus(t, "critical")
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			TestUpstreamNodesInStatusDC2(t, "critical")
		fallthrough
	case "failover-through-double-local-gateway":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"]["db.default.dc3"] = TestUpstreamNodesDC2(t)
		snap.ConnectProxy.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": map[string]structs.CheckServiceNodes{
				"dc1": TestGatewayNodesDC1(t),
			},
		}
	case "splitter-with-resolver-redirect-multidc":
		snap.ConnectProxy.WatchedUpstreamEndpoints["db"] = map[string]structs.CheckServiceNodes{
			"v1.db.default.dc1": TestUpstreamNodes(t),
			"v2.db.default.dc2": TestUpstreamNodesDC2(t),
		}
	default:
		t.Fatalf("unexpected variation: %q", variation)
		return nil
	}

	return snap
}

func TestConfigSnapshotMeshGateway(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotMeshGateway(t, true, false)
}

func TestConfigSnapshotMeshGatewayUsingFederationStates(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotMeshGateway(t, true, true)
}

func TestConfigSnapshotMeshGatewayNoServices(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotMeshGateway(t, false, false)
}

func testConfigSnapshotMeshGateway(t testing.T, populateServices bool, useFederationStates bool) *ConfigSnapshot {
	roots, _ := TestCerts(t)
	snap := &ConfigSnapshot{
		Kind:    structs.ServiceKindMeshGateway,
		Service: "mesh-gateway",
		ProxyID: structs.NewServiceID("mesh-gateway", nil),
		Address: "1.2.3.4",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{},
		},
		TaggedAddresses: map[string]structs.ServiceAddress{
			structs.TaggedAddressLAN: structs.ServiceAddress{
				Address: "1.2.3.4",
				Port:    8443,
			},
			structs.TaggedAddressWAN: structs.ServiceAddress{
				Address: "198.18.0.1",
				Port:    443,
			},
		},
		Roots:      roots,
		Datacenter: "dc1",
		MeshGateway: configSnapshotMeshGateway{
			WatchedServicesSet: true,
		},
	}

	if populateServices {
		snap.MeshGateway = configSnapshotMeshGateway{
			WatchedServices: map[structs.ServiceID]context.CancelFunc{
				structs.NewServiceID("foo", nil): nil,
				structs.NewServiceID("bar", nil): nil,
			},
			WatchedServicesSet: true,
			WatchedDatacenters: map[string]context.CancelFunc{
				"dc2": nil,
			},
			ServiceGroups: map[structs.ServiceID]structs.CheckServiceNodes{
				structs.NewServiceID("foo", nil): TestGatewayServiceGroupFooDC1(t),
				structs.NewServiceID("bar", nil): TestGatewayServiceGroupBarDC1(t),
			},
			GatewayGroups: map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
			},
		}
		if useFederationStates {
			snap.MeshGateway.FedStateGateways = map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
			}

			delete(snap.MeshGateway.GatewayGroups, "dc2")
		}
	}

	return snap
}

func TestConfigSnapshotExposeConfig(t testing.T) *ConfigSnapshot {
	return &ConfigSnapshot{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-proxy",
		ProxyID: structs.NewServiceID("web-proxy", nil),
		Address: "1.2.3.4",
		Port:    8080,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServicePort:       8080,
			Expose: structs.ExposeConfig{
				Checks: false,
				Paths: []structs.ExposePath{
					{
						LocalPathPort: 8080,
						Path:          "/health1",
						ListenerPort:  21500,
					},
					{
						LocalPathPort: 8080,
						Path:          "/health2",
						ListenerPort:  21501,
					},
				},
			},
		},
		Datacenter: "dc1",
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

// SupportsBlocking implements cache.Type
func (ct *ControllableCacheType) SupportsBlocking() bool {
	return ct.blocking
}
