package proxycfg

import (
	"time"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
)

func setupTestVariationConfigEntriesAndSnapshot(
	t testing.T,
	variation string,
	upstreams structs.Upstreams,
	additionalEntries ...structs.ConfigEntry,
) []UpdateEvent {
	var (
		dbUpstream = upstreams[0]

		dbUID = NewUpstreamID(&dbUpstream)
	)

	dbChain := setupTestVariationDiscoveryChain(t, variation, additionalEntries...)

	events := []UpdateEvent{
		{
			CorrelationID: "discovery-chain:" + dbUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: dbChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + dbChain.ID() + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "db"),
			},
		},
	}

	switch variation {
	case "default":
	case "simple-with-overrides":
	case "simple":
	case "external-sni":
	case "failover":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:fail.default.default.dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesAlternate(t),
			},
		})
	case "failover-through-remote-gateway-triggered":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		})
		fallthrough
	case "failover-through-remote-gateway":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc2:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc2:" + dbUID.String(),
			Result: &structs.IndexedNodesWithGateways{
				Nodes: TestGatewayNodesDC2(t),
			},
		})
	case "failover-through-double-remote-gateway-triggered":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc2:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatusDC2(t, "critical"),
			},
		})
		fallthrough
	case "failover-through-double-remote-gateway":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc3:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc2:" + dbUID.String(),
			Result: &structs.IndexedNodesWithGateways{
				Nodes: TestGatewayNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc3:" + dbUID.String(),
			Result: &structs.IndexedNodesWithGateways{
				Nodes: TestGatewayNodesDC3(t),
			},
		})
	case "failover-through-local-gateway-triggered":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		})
		fallthrough
	case "failover-through-local-gateway":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc2:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc1:" + dbUID.String(),
			Result: &structs.IndexedNodesWithGateways{
				Nodes: TestGatewayNodesDC1(t),
			},
		})
	case "failover-through-double-local-gateway-triggered":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc2:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatusDC2(t, "critical"),
			},
		})
		fallthrough
	case "failover-through-double-local-gateway":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:db.default.default.dc3:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc1:" + dbUID.String(),
			Result: &structs.IndexedNodesWithGateways{
				Nodes: TestGatewayNodesDC1(t),
			},
		})
	case "splitter-with-resolver-redirect-multidc":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:v1.db.default.default.dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "db"),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:v2.db.default.default.dc2:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
	case "chain-and-splitter":
	case "grpc-router":
	case "chain-and-router":
	case "lb-resolver":
	default:
		t.Fatalf("unexpected variation: %q", variation)
		return nil
	}

	return events
}

func setupTestVariationDiscoveryChain(
	t testing.T,
	variation string,
	additionalEntries ...structs.ConfigEntry,
) *structs.CompiledDiscoveryChain {
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
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
			},
		)
	case "failover":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
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
					{
						Weight:  95.5,
						Service: "big-side",
						RequestHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "big"},
						},
						ResponseHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "big"},
						},
					},
					{
						Weight:  4,
						Service: "goldilocks-side",
						RequestHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "goldilocks"},
						},
						ResponseHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "goldilocks"},
						},
					},
					{
						Weight:  0.5,
						Service: "lil-bit-side",
						RequestHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "small"},
						},
						ResponseHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "small"},
						},
					},
				},
			},
		)
	case "grpc-router":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
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
				RequestTimeout: 33 * time.Second,
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
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathExact: "/header-manip",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service: "header-manip",
							RequestHeaders: &structs.HTTPHeaderModifiers{
								Add: map[string]string{
									"request": "bar",
								},
								Set: map[string]string{
									"bar": "baz",
								},
								Remove: []string{"qux"},
							},
							ResponseHeaders: &structs.HTTPHeaderModifiers{
								Add: map[string]string{
									"response": "bar",
								},
								Set: map[string]string{
									"bar": "baz",
								},
								Remove: []string{"qux"},
							},
						},
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
	default:
		t.Fatalf("unexpected variation: %q", variation)
		return nil
	}

	if len(additionalEntries) > 0 {
		entries = append(entries, additionalEntries...)
	}

	return discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc1", connect.TestClusterID+".consul", compileSetup, entries...)
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
