package proxycfgtest

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// TestConfigSnapshot returns a fully populated snapshot
func TestConfigSnapshot(t *testing.T) *proxycfg.ConfigSnapshot {
	roots, leaf := proxycfg.TestCerts(t)

	// no entries implies we'll get a default chain
	dbChain := discoverychain.TestCompileConfigEntries(
		t, "db", "default", "dc1",
		connect.TestClusterID+".consul", "dc1", nil)

	return &proxycfg.ConfigSnapshot{
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
		ConnectProxy: proxycfg.ConfigSnapshotConnectProxy{
			ConfigSnapshotUpstreams: proxycfg.ConfigSnapshotUpstreams{
				Leaf: leaf,
				DiscoveryChain: map[string]*structs.CompiledDiscoveryChain{
					"db": dbChain,
				},
				WatchedUpstreamEndpoints: map[string]map[string]structs.CheckServiceNodes{
					"db": {
						"db.default.dc1": proxycfg.TestUpstreamNodes(t),
					},
				},
			},
			PreparedQueryEndpoints: map[string]structs.CheckServiceNodes{
				"prepared_query:geo-cache": proxycfg.TestUpstreamNodes(t),
			},
			Intentions:    nil, // no intentions defined
			IntentionsSet: true,
		},
		Datacenter: "dc1",
	}
}

// TestConfigSnapshotDiscoveryChain returns a fully populated snapshot using a discovery chain
func TestConfigSnapshotDiscoveryChain(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "simple")
}

func TestConfigSnapshotDiscoveryChainExternalSNI(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "external-sni")
}

func TestConfigSnapshotDiscoveryChainWithOverrides(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "simple-with-overrides")
}

func TestConfigSnapshotDiscoveryChainWithFailover(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway")
}

func TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway")
}

func TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway-triggered")
}

func TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc")
}

func TestConfigSnapshotDiscoveryChainWithEntries(t *testing.T, additionalEntries ...structs.ConfigEntry) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "simple", additionalEntries...)
}

func TestConfigSnapshotDiscoveryChainDefault(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "default")
}

func TestConfigSnapshotDiscoveryChainWithSplitter(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "chain-and-splitter")
}

func TestConfigSnapshotDiscoveryChainWithGRPCRouter(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "grpc-router")
}

func TestConfigSnapshotDiscoveryChainWithRouter(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "chain-and-router")
}

func TestConfigSnapshotDiscoveryChainWithLB(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotDiscoveryChain(t, "lb-resolver")
}

func testConfigSnapshotDiscoveryChain(t *testing.T, variation string, additionalEntries ...structs.ConfigEntry) *proxycfg.ConfigSnapshot {
	roots, leaf := proxycfg.TestCerts(t)

	snap := &proxycfg.ConfigSnapshot{
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
		ConnectProxy: proxycfg.ConfigSnapshotConnectProxy{
			ConfigSnapshotUpstreams: setupTestVariationConfigEntriesAndSnapshot(
				t, variation, leaf, additionalEntries...,
			),
		},
		Datacenter: "dc1",
	}

	return snap
}

func setupTestVariationConfigEntriesAndSnapshot(
	t *testing.T,
	variation string,
	leaf *structs.IssuedCert,
	additionalEntries ...structs.ConfigEntry,
) proxycfg.ConfigSnapshotUpstreams {
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
					"v1": {
						Filter: "Service.Meta.version == v1",
					},
					"v2": {
						Filter: "Service.Meta.version == v2",
					},
				},
			},
		)
	case "chain-and-splitter":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "db",
				Splits: []structs.ServiceSplit{
					{Weight: 95.5, Service: "big-side"},
					{Weight: 4, Service: "goldilocks-side"},
					{Weight: 0.5, Service: "lil-bit-side"},
				},
			},
		)
	case "grpc-router":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "grpc",
				},
			},
			&structs.ServiceRouterConfigEntry{
				Kind: structs.ServiceRouter,
				Name: "db",
				Routes: []structs.ServiceRoute{
					{
						Match: &structs.ServiceRouteMatch{
							HTTP: &structs.ServiceRouteHTTPMatch{
								PathExact: "/fgrpc.PingServer/Ping",
							},
						},
						Destination: &structs.ServiceRouteDestination{
							Service: "prefix",
						},
					},
				},
			},
		)
	case "chain-and-router":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "split-3-ways",
				Splits: []structs.ServiceSplit{
					{Weight: 95.5, Service: "big-side"},
					{Weight: 4, Service: "goldilocks-side"},
					{Weight: 0.5, Service: "lil-bit-side"},
				},
			},
			&structs.ServiceRouterConfigEntry{
				Kind: structs.ServiceRouter,
				Name: "db",
				Routes: []structs.ServiceRoute{
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/prefix",
						}),
						Destination: toService("prefix"),
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathExact: "/exact",
						}),
						Destination: toService("exact"),
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathRegex: "/regex",
						}),
						Destination: toService("regex"),
					},
					{
						Match: httpMatchHeader(structs.ServiceRouteHTTPMatchHeader{
							Name:    "x-debug",
							Present: true,
						}),
						Destination: toService("hdr-present"),
					},
					{
						Match: httpMatchHeader(structs.ServiceRouteHTTPMatchHeader{
							Name:    "x-debug",
							Present: true,
							Invert:  true,
						}),
						Destination: toService("hdr-not-present"),
					},
					{
						Match: httpMatchHeader(structs.ServiceRouteHTTPMatchHeader{
							Name:  "x-debug",
							Exact: "exact",
						}),
						Destination: toService("hdr-exact"),
					},
					{
						Match: httpMatchHeader(structs.ServiceRouteHTTPMatchHeader{
							Name:   "x-debug",
							Prefix: "prefix",
						}),
						Destination: toService("hdr-prefix"),
					},
					{
						Match: httpMatchHeader(structs.ServiceRouteHTTPMatchHeader{
							Name:   "x-debug",
							Suffix: "suffix",
						}),
						Destination: toService("hdr-suffix"),
					},
					{
						Match: httpMatchHeader(structs.ServiceRouteHTTPMatchHeader{
							Name:  "x-debug",
							Regex: "regex",
						}),
						Destination: toService("hdr-regex"),
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							Methods: []string{"GET", "PUT"},
						}),
						Destination: toService("just-methods"),
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							Header: []structs.ServiceRouteHTTPMatchHeader{
								{
									Name:  "x-debug",
									Exact: "exact",
								},
							},
							Methods: []string{"GET", "PUT"},
						}),
						Destination: toService("hdr-exact-with-method"),
					},
					{
						Match: httpMatchParam(structs.ServiceRouteHTTPMatchQueryParam{
							Name:  "secretparam1",
							Exact: "exact",
						}),
						Destination: toService("prm-exact"),
					},
					{
						Match: httpMatchParam(structs.ServiceRouteHTTPMatchQueryParam{
							Name:  "secretparam2",
							Regex: "regex",
						}),
						Destination: toService("prm-regex"),
					},
					{
						Match: httpMatchParam(structs.ServiceRouteHTTPMatchQueryParam{
							Name:    "secretparam3",
							Present: true,
						}),
						Destination: toService("prm-present"),
					},
					{
						Match:       nil,
						Destination: toService("nil-match"),
					},
					{
						Match:       &structs.ServiceRouteMatch{},
						Destination: toService("empty-match-1"),
					},
					{
						Match: &structs.ServiceRouteMatch{
							HTTP: &structs.ServiceRouteHTTPMatch{},
						},
						Destination: toService("empty-match-2"),
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/prefix",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:       "prefix-rewrite-1",
							PrefixRewrite: "/",
						},
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/prefix",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:       "prefix-rewrite-2",
							PrefixRewrite: "/nested/newlocation",
						},
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/timeout",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:        "req-timeout",
							RequestTimeout: 33 * time.Second,
						},
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/retry-connect",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:               "retry-connect",
							NumRetries:            15,
							RetryOnConnectFailure: true,
						},
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/retry-codes",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:            "retry-codes",
							NumRetries:         15,
							RetryOnStatusCodes: []uint32{401, 409, 451},
						},
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/retry-both",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:               "retry-both",
							RetryOnConnectFailure: true,
							RetryOnStatusCodes:    []uint32{401, 409, 451},
						},
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/split-3-ways",
						}),
						Destination: toService("split-3-ways"),
					},
				},
			},
		)
	case "lb-resolver":
		entries = append(entries,
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "db",
				Splits: []structs.ServiceSplit{
					{Weight: 95.5, Service: "something-else"},
					{Weight: 4.5, Service: "db"},
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db",
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MinimumRingSize: 20,
						MaximumRingSize: 30,
					},
					HashPolicies: []structs.HashPolicy{
						{
							Field:      "cookie",
							FieldValue: "chocolate-chip",
							Terminal:   true,
						},
						{
							Field:        "cookie",
							FieldValue:   "chocolate-chip",
							CookieConfig: &structs.CookieConfig{Session: true},
						},
						{
							Field:      "header",
							FieldValue: "x-user-id",
						},
						{
							SourceIP: true,
							Terminal: true,
						},
					},
				},
			},
		)
	case "http-multiple-services":
	default:
		t.Fatalf("unexpected variation: %q", variation)
		return proxycfg.ConfigSnapshotUpstreams{}
	}

	if len(additionalEntries) > 0 {
		entries = append(entries, additionalEntries...)
	}

	dbChain := discoverychain.TestCompileConfigEntries(t, "db", "default", "dc1", connect.TestClusterID+".consul", "dc1", compileSetup, entries...)

	snap := proxycfg.ConfigSnapshotUpstreams{
		Leaf: leaf,
		DiscoveryChain: map[string]*structs.CompiledDiscoveryChain{
			"db": dbChain,
		},
		WatchedUpstreamEndpoints: map[string]map[string]structs.CheckServiceNodes{
			"db": {
				"db.default.dc1": proxycfg.TestUpstreamNodes(t),
			},
		},
	}

	switch variation {
	case "default":
	case "simple-with-overrides":
	case "simple":
	case "external-sni":
	case "failover":
		snap.WatchedUpstreamEndpoints["db"]["fail.default.dc1"] =
			proxycfg.TestUpstreamNodesAlternate(t)
	case "failover-through-remote-gateway-triggered":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			proxycfg.TestUpstreamNodesInStatus(t, "critical")
		fallthrough
	case "failover-through-remote-gateway":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			proxycfg.TestUpstreamNodesDC2(t)
		snap.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": {
				"dc2": TestGatewayNodesDC2(t),
			},
		}
	case "failover-through-double-remote-gateway-triggered":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			proxycfg.TestUpstreamNodesInStatus(t, "critical")
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			proxycfg.TestUpstreamNodesInStatusDC2(t, "critical")
		fallthrough
	case "failover-through-double-remote-gateway":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc3"] = proxycfg.TestUpstreamNodesDC2(t)
		snap.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": {
				"dc2": TestGatewayNodesDC2(t),
				"dc3": TestGatewayNodesDC3(t),
			},
		}
	case "failover-through-local-gateway-triggered":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			proxycfg.TestUpstreamNodesInStatus(t, "critical")
		fallthrough
	case "failover-through-local-gateway":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			proxycfg.TestUpstreamNodesDC2(t)
		snap.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": {
				"dc1": TestGatewayNodesDC1(t),
			},
		}
	case "failover-through-double-local-gateway-triggered":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc1"] =
			proxycfg.TestUpstreamNodesInStatus(t, "critical")
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc2"] =
			proxycfg.TestUpstreamNodesInStatusDC2(t, "critical")
		fallthrough
	case "failover-through-double-local-gateway":
		snap.WatchedUpstreamEndpoints["db"]["db.default.dc3"] = proxycfg.TestUpstreamNodesDC2(t)
		snap.WatchedGatewayEndpoints = map[string]map[string]structs.CheckServiceNodes{
			"db": {
				"dc1": TestGatewayNodesDC1(t),
			},
		}
	case "splitter-with-resolver-redirect-multidc":
		snap.WatchedUpstreamEndpoints["db"] = map[string]structs.CheckServiceNodes{
			"v1.db.default.dc1": proxycfg.TestUpstreamNodes(t),
			"v2.db.default.dc2": proxycfg.TestUpstreamNodesDC2(t),
		}
	case "chain-and-splitter":
	case "grpc-router":
	case "chain-and-router":
	case "http-multiple-services":
		snap.WatchedUpstreamEndpoints["foo"] = map[string]structs.CheckServiceNodes{
			"foo.default.dc1": proxycfg.TestUpstreamNodes(t),
		}
		snap.WatchedUpstreamEndpoints["bar"] = map[string]structs.CheckServiceNodes{
			"bar.default.dc1": proxycfg.TestUpstreamNodesAlternate(t),
		}
	case "lb-resolver":
	default:
		t.Fatalf("unexpected variation: %q", variation)
		return proxycfg.ConfigSnapshotUpstreams{}
	}

	return snap
}

func TestConfigSnapshotIngress(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "simple")
}

func TestConfigSnapshotIngressWithTLSListener(t *testing.T) *proxycfg.ConfigSnapshot {
	snap := testConfigSnapshotIngressGateway(t, true, "tcp", "default")
	snap.IngressGateway.TLSEnabled = true
	return snap
}

func TestConfigSnapshotIngressWithOverrides(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "simple-with-overrides")
}
func TestConfigSnapshotIngress_SplitterWithResolverRedirectMultiDC(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "splitter-with-resolver-redirect-multidc")
}

func TestConfigSnapshotIngress_HTTPMultipleServices(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "http-multiple-services")
}

func TestConfigSnapshotIngressExternalSNI(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "external-sni")
}

func TestConfigSnapshotIngressWithFailover(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover")
}

func TestConfigSnapshotIngressWithFailoverThroughRemoteGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-remote-gateway")
}

func TestConfigSnapshotIngressWithFailoverThroughRemoteGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-remote-gateway-triggered")
}

func TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-double-remote-gateway")
}

func TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-double-remote-gateway-triggered")
}

func TestConfigSnapshotIngressWithFailoverThroughLocalGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-local-gateway")
}

func TestConfigSnapshotIngressWithFailoverThroughLocalGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-local-gateway-triggered")
}

func TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-double-local-gateway")
}

func TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGatewayTriggered(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "failover-through-double-local-gateway-triggered")
}

func TestConfigSnapshotIngressWithSplitter(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "chain-and-splitter")
}

func TestConfigSnapshotIngressWithGRPCRouter(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "grpc-router")
}

func TestConfigSnapshotIngressWithRouter(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "chain-and-router")
}

func TestConfigSnapshotIngressGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "tcp", "default")
}

func TestConfigSnapshotIngressGatewayNoServices(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, false, "tcp", "default")
}

func TestConfigSnapshotIngressWithLB(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "lb-resolver")
}

func TestConfigSnapshotIngressDiscoveryChainWithEntries(t *testing.T, additionalEntries ...structs.ConfigEntry) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotIngressGateway(t, true, "http", "simple", additionalEntries...)
}

func testConfigSnapshotIngressGateway(
	t *testing.T, populateServices bool, protocol, variation string,
	additionalEntries ...structs.ConfigEntry,
) *proxycfg.ConfigSnapshot {
	roots, leaf := proxycfg.TestCerts(t)
	snap := &proxycfg.ConfigSnapshot{
		Kind:       structs.ServiceKindIngressGateway,
		Service:    "ingress-gateway",
		ProxyID:    structs.NewServiceID("ingress-gateway", nil),
		Address:    "1.2.3.4",
		Roots:      roots,
		Datacenter: "dc1",
	}
	if populateServices {
		snap.IngressGateway = proxycfg.ConfigSnapshotIngressGateway{
			ConfigSnapshotUpstreams: setupTestVariationConfigEntriesAndSnapshot(
				t, variation, leaf, additionalEntries...,
			),
			Upstreams: map[proxycfg.IngressListenerKey]structs.Upstreams{
				{Protocol: protocol, Port: 9191}: {
					{
						// We rely on this one having default type in a few tests...
						DestinationName:  "db",
						LocalBindPort:    9191,
						LocalBindAddress: "2.3.4.5",
					},
				},
			},
		}
	}
	return snap
}

func httpMatch(http *structs.ServiceRouteHTTPMatch) *structs.ServiceRouteMatch {
	return &structs.ServiceRouteMatch{HTTP: http}
}
func httpMatchHeader(headers ...structs.ServiceRouteHTTPMatchHeader) *structs.ServiceRouteMatch {
	return httpMatch(&structs.ServiceRouteHTTPMatch{
		Header: headers,
	})
}
func httpMatchParam(params ...structs.ServiceRouteHTTPMatchQueryParam) *structs.ServiceRouteMatch {
	return httpMatch(&structs.ServiceRouteHTTPMatch{
		QueryParam: params,
	})
}
func toService(svc string) *structs.ServiceRouteDestination {
	return &structs.ServiceRouteDestination{Service: svc}
}

func TestConfigSnapshotIngress_MultipleListenersDuplicateService(t *testing.T) *proxycfg.ConfigSnapshot {
	snap := TestConfigSnapshotIngress_HTTPMultipleServices(t)

	snap.IngressGateway.Upstreams = map[proxycfg.IngressListenerKey]structs.Upstreams{
		{Protocol: "http", Port: 8080}: {
			{
				DestinationName: "foo",
				LocalBindPort:   8080,
			},
			{
				DestinationName: "bar",
				LocalBindPort:   8080,
			},
		},
		{Protocol: "http", Port: 443}: {
			{
				DestinationName: "foo",
				LocalBindPort:   443,
			},
		},
	}

	fooChain := discoverychain.TestCompileConfigEntries(t, "foo", "default", "dc1", connect.TestClusterID+".consul", "dc1", nil)
	barChain := discoverychain.TestCompileConfigEntries(t, "bar", "default", "dc1", connect.TestClusterID+".consul", "dc1", nil)

	snap.IngressGateway.DiscoveryChain = map[string]*structs.CompiledDiscoveryChain{
		"foo": fooChain,
		"bar": barChain,
	}

	return snap
}

func TestConfigSnapshotExposeConfig(t *testing.T) *proxycfg.ConfigSnapshot {
	return &proxycfg.ConfigSnapshot{
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

func TestConfigSnapshotGRPCExposeHTTP1(t *testing.T) *proxycfg.ConfigSnapshot {
	return &proxycfg.ConfigSnapshot{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "grpc-proxy",
		ProxyID: structs.NewServiceID("grpc-proxy", nil),
		Address: "1.2.3.4",
		Port:    8080,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "grpc",
			DestinationServiceID:   "grpc",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"protocol": "grpc",
			},
			Expose: structs.ExposeConfig{
				Checks: false,
				Paths: []structs.ExposePath{
					{
						LocalPathPort: 8090,
						Path:          "/healthz",
						ListenerPort:  21500,
						Protocol:      "http",
					},
				},
			},
		},
		Datacenter: "dc1",
	}
}

func TestConfigSnapshotMeshGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotMeshGateway(t, true, false)
}

func TestConfigSnapshotMeshGatewayUsingFederationStates(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotMeshGateway(t, true, true)
}

func TestConfigSnapshotMeshGatewayNoServices(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotMeshGateway(t, false, false)
}

func testConfigSnapshotMeshGateway(t *testing.T, populateServices bool, useFederationStates bool) *proxycfg.ConfigSnapshot {
	roots, _ := proxycfg.TestCerts(t)
	snap := &proxycfg.ConfigSnapshot{
		Kind:    structs.ServiceKindMeshGateway,
		Service: "mesh-gateway",
		ProxyID: structs.NewServiceID("mesh-gateway", nil),
		Address: "1.2.3.4",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{},
		},
		TaggedAddresses: map[string]structs.ServiceAddress{
			structs.TaggedAddressLAN: {
				Address: "1.2.3.4",
				Port:    8443,
			},
			structs.TaggedAddressWAN: {
				Address: "198.18.0.1",
				Port:    443,
			},
		},
		Roots:      roots,
		Datacenter: "dc1",
		MeshGateway: proxycfg.ConfigSnapshotMeshGateway{
			WatchedServicesSet: true,
		},
	}

	if populateServices {
		snap.MeshGateway = proxycfg.ConfigSnapshotMeshGateway{
			WatchedServices: map[structs.ServiceName]context.CancelFunc{
				structs.NewServiceName("foo", nil): nil,
				structs.NewServiceName("bar", nil): nil,
			},
			WatchedServicesSet: true,
			WatchedDatacenters: map[string]context.CancelFunc{
				"dc2": nil,
			},
			ServiceGroups: map[structs.ServiceName]structs.CheckServiceNodes{
				structs.NewServiceName("foo", nil): TestGatewayServiceGroupFooDC1(t),
				structs.NewServiceName("bar", nil): TestGatewayServiceGroupBarDC1(t),
			},
			GatewayGroups: map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
				"dc4": proxycfg.TestGatewayNodesDC4Hostname(t),
				"dc6": TestGatewayNodesDC6Hostname(t),
			},
			HostnameDatacenters: map[string]structs.CheckServiceNodes{
				"dc4": {
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
				},
				"dc6": {
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
				},
			},
		}
		if useFederationStates {
			snap.MeshGateway.FedStateGateways = map[string]structs.CheckServiceNodes{
				"dc2": TestGatewayNodesDC2(t),
				"dc4": proxycfg.TestGatewayNodesDC4Hostname(t),
				"dc6": TestGatewayNodesDC6Hostname(t),
			}

			delete(snap.MeshGateway.GatewayGroups, "dc2")
		}
	}

	return snap
}

func TestConfigSnapshotTerminatingGateway(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotTerminatingGateway(t, true)
}

func TestConfigSnapshotTerminatingGatewayNoServices(t *testing.T) *proxycfg.ConfigSnapshot {
	return testConfigSnapshotTerminatingGateway(t, false)
}

func testConfigSnapshotTerminatingGateway(t *testing.T, populateServices bool) *proxycfg.ConfigSnapshot {
	roots, _ := proxycfg.TestCerts(t)

	snap := &proxycfg.ConfigSnapshot{
		Kind:    structs.ServiceKindTerminatingGateway,
		Service: "terminating-gateway",
		ProxyID: structs.NewServiceID("terminating-gateway", nil),
		Address: "1.2.3.4",
		TaggedAddresses: map[string]structs.ServiceAddress{
			structs.TaggedAddressWAN: {
				Address: "198.18.0.1",
				Port:    443,
			},
		},
		Port:       8443,
		Roots:      roots,
		Datacenter: "dc1",
	}
	if populateServices {
		web := structs.NewServiceName("web", nil)
		webNodes := proxycfg.TestUpstreamNodes(t)
		webNodes[0].Service.Meta = map[string]string{
			"version": "1",
		}
		webNodes[1].Service.Meta = map[string]string{
			"version": "2",
		}

		api := structs.NewServiceName("api", nil)
		apiNodes := structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "api",
					Node:       "test1",
					Address:    "10.10.1.1",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "api.mydomain",
					Port:    8081,
				},
				Checks: structs.HealthChecks{
					{
						Status: "critical",
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
				Service: &structs.NodeService{
					Service: "api",
					Address: "api.altdomain",
					Port:    8081,
					Meta: map[string]string{
						"domain": "alt",
					},
				},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "test3",
					Node:       "test3",
					Address:    "10.10.1.3",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "10.10.1.3",
					Port:    8081,
				},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "test4",
					Node:       "test4",
					Address:    "10.10.1.4",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "api.thirddomain",
					Port:    8081,
				},
			},
		}

		// Has failing instance
		db := structs.NewServiceName("db", nil)
		dbNodes := structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "db",
					Node:       "test4",
					Address:    "10.10.1.4",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "db",
					Address: "db.mydomain",
					Port:    8081,
				},
				Checks: structs.HealthChecks{
					{
						Status: "critical",
					},
				},
			},
		}

		// Has passing instance but failing subset
		cache := structs.NewServiceName("cache", nil)
		cacheNodes := structs.CheckServiceNodes{
			{
				Node: &structs.Node{
					ID:         "cache",
					Node:       "test5",
					Address:    "10.10.1.5",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "cache",
					Address: "cache.mydomain",
					Port:    8081,
				},
			},
			{
				Node: &structs.Node{
					ID:         "cache",
					Node:       "test5",
					Address:    "10.10.1.5",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "cache",
					Address: "cache.mydomain",
					Port:    8081,
					Meta: map[string]string{
						"Env": "prod",
					},
				},
				Checks: structs.HealthChecks{
					{
						Status: "critical",
					},
				},
			},
		}

		snap.TerminatingGateway = proxycfg.ConfigSnapshotTerminatingGateway{
			ServiceGroups: map[structs.ServiceName]structs.CheckServiceNodes{
				web:   webNodes,
				api:   apiNodes,
				db:    dbNodes,
				cache: cacheNodes,
			},
			ServiceResolversSet: map[structs.ServiceName]bool{
				web:   true,
				api:   true,
				db:    true,
				cache: true,
			},
			GatewayServices: map[structs.ServiceName]structs.GatewayService{
				web: {
					Service: web,
					CAFile:  "ca.cert.pem",
				},
				api: {
					Service:  api,
					CAFile:   "ca.cert.pem",
					CertFile: "api.cert.pem",
					KeyFile:  "api.key.pem",
				},
				db: {
					Service: db,
				},
				cache: {
					Service: cache,
				},
			},
			HostnameServices: map[structs.ServiceName]structs.CheckServiceNodes{
				api:   {apiNodes[0], apiNodes[1]},
				db:    {dbNodes[0]},
				cache: {cacheNodes[0], cacheNodes[1]},
			},
		}

		snap.TerminatingGateway.ServiceConfigs = map[structs.ServiceName]*structs.ServiceConfigResponse{
			web: {
				ProxyConfig: map[string]interface{}{"protocol": "tcp"},
			},
			api: {
				ProxyConfig: map[string]interface{}{"protocol": "tcp"},
			},
			db: {
				ProxyConfig: map[string]interface{}{"protocol": "tcp"},
			},
			cache: {
				ProxyConfig: map[string]interface{}{"protocol": "tcp"},
			},
		}
		snap.TerminatingGateway.Intentions = map[structs.ServiceName]structs.Intentions{
			// no intentions defined for thse services
			web:   nil,
			api:   nil,
			db:    nil,
			cache: nil,
		}

		snap.TerminatingGateway.ServiceLeaves = map[structs.ServiceName]*structs.IssuedCert{
			web: {
				CertPEM:       golden(t, "test-leaf-cert"),
				PrivateKeyPEM: golden(t, "test-leaf-key"),
			},
			api: {
				CertPEM:       golden(t, "alt-test-leaf-cert"),
				PrivateKeyPEM: golden(t, "alt-test-leaf-key"),
			},
			db: {
				CertPEM:       golden(t, "db-test-leaf-cert"),
				PrivateKeyPEM: golden(t, "db-test-leaf-key"),
			},
			cache: {
				CertPEM:       golden(t, "cache-test-leaf-cert"),
				PrivateKeyPEM: golden(t, "cache-test-leaf-key"),
			},
		}
	}
	return snap
}

// golden is used to read golden files stores in consul/agent/xds/testdata
func golden(t *testing.T, name string) string {
	t.Helper()

	golden := filepath.Join("../xds/testdata", name+".golden")
	expected, err := ioutil.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func TestGatewayServiceGroupFooDC1(t *testing.T) structs.CheckServiceNodes {
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

func TestGatewayServiceGroupBarDC1(t *testing.T) structs.CheckServiceNodes {
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

func TestGatewayNodesDC6Hostname(t *testing.T) structs.CheckServiceNodes {
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

func TestGatewayNodesDC1(t *testing.T) structs.CheckServiceNodes {
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

func TestGatewayNodesDC2(t *testing.T) structs.CheckServiceNodes {
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

func TestGatewayNodesDC3(t *testing.T) structs.CheckServiceNodes {
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
