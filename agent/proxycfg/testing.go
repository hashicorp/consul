package proxycfg

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// TestCacheTypes encapsulates all the different cache types proxycfg.State will
// watch/request for controlling one during testing.
type TestCacheTypes struct {
	roots         *ControllableCacheType
	leaf          *ControllableCacheType
	intentions    *ControllableCacheType
	health        *ControllableCacheType
	query         *ControllableCacheType
	compiledChain *ControllableCacheType
}

// NewTestCacheTypes creates a set of ControllableCacheTypes for all types that
// proxycfg will watch suitable for testing a proxycfg.State or Manager.
func NewTestCacheTypes(t testing.T) *TestCacheTypes {
	t.Helper()
	ct := &TestCacheTypes{
		roots:         NewControllableCacheType(t),
		leaf:          NewControllableCacheType(t),
		intentions:    NewControllableCacheType(t),
		health:        NewControllableCacheType(t),
		query:         NewControllableCacheType(t),
		compiledChain: NewControllableCacheType(t),
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

	return c
}

// TestCerts generates a CA and Leaf suitable for returning as mock CA
// root/leaf cache requests.
func TestCerts(t testing.T) (*structs.IndexedCARoots, *structs.IssuedCert) {
	t.Helper()

	ca := connect.TestCA(t, nil)
	roots := &structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  connect.TestClusterID,
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
		SerialNumber:  connect.HexString(leafCert.SerialNumber.Bytes()),
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
	return &ConfigSnapshot{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-sidecar-proxy",
		ProxyID: "web-sidecar-proxy",
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
			UpstreamEndpoints: map[string]structs.CheckServiceNodes{
				"db": TestUpstreamNodes(t),
			},
		},
		Datacenter: "dc1",
	}
}

func TestConfigSnapshotMeshGateway(t testing.T) *ConfigSnapshot {
	roots, _ := TestCerts(t)
	return &ConfigSnapshot{
		Kind:    structs.ServiceKindMeshGateway,
		Service: "mesh-gateway",
		ProxyID: "mesh-gateway",
		Address: "1.2.3.4",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{},
		},
		TaggedAddresses: map[string]structs.ServiceAddress{
			"lan": structs.ServiceAddress{
				Address: "1.2.3.4",
				Port:    8443,
			},
			"wan": structs.ServiceAddress{
				Address: "198.18.0.1",
				Port:    443,
			},
		},
		Roots:      roots,
		Datacenter: "dc1",
		MeshGateway: configSnapshotMeshGateway{
			WatchedServices: map[string]context.CancelFunc{
				"foo": nil,
				"bar": nil,
			},
			WatchedDatacenters: map[string]context.CancelFunc{
				"dc2": nil,
			},
			ServiceGroups: map[string]structs.CheckServiceNodes{
				"foo": TestGatewayServiceGroupFooDC1(t),
				"bar": TestGatewayServiceGroupBarDC1(t),
			},
			GatewayGroups: map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
			},
		},
	}
}

// ControllableCacheType is a cache.Type that simulates a typical blocking RPC
// but lets us control the responses and when they are delivered easily.
type ControllableCacheType struct {
	index uint64
	value atomic.Value
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
func (ct *ControllableCacheType) Set(value interface{}) {
	atomic.AddUint64(&ct.index, 1)
	ct.value.Store(value)
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

	// reload index as it probably got bumped
	index = atomic.LoadUint64(&ct.index)
	val := ct.value.Load()

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
