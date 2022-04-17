package discoverychain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

type compileTestCase struct {
	entries        *configentry.DiscoveryChainSet
	setup          func(req *CompileRequest)
	expect         *structs.CompiledDiscoveryChain
	expectCustom   bool
	expectErr      string
	expectGraphErr bool
}

func TestCompile(t *testing.T) {
	t.Parallel()

	cases := map[string]compileTestCase{
		"router with defaults":                             testcase_JustRouterWithDefaults(),
		"router with defaults and resolver":                testcase_RouterWithDefaults_NoSplit_WithResolver(),
		"router with defaults and noop split":              testcase_RouterWithDefaults_WithNoopSplit_DefaultResolver(),
		"router with defaults and noop split and resolver": testcase_RouterWithDefaults_WithNoopSplit_WithResolver(),
		"router with no destination":                       testcase_JustRouterWithNoDestination(),
		"route bypasses splitter":                          testcase_RouteBypassesSplit(),
		"noop split":                                       testcase_NoopSplit_DefaultResolver(),
		"noop split with protocol from proxy defaults":     testcase_NoopSplit_DefaultResolver_ProtocolFromProxyDefaults(),
		"noop split with resolver":                         testcase_NoopSplit_WithResolver(),
		"subset split":                                     testcase_SubsetSplit(),
		"service split":                                    testcase_ServiceSplit(),
		"split bypasses next splitter":                     testcase_SplitBypassesSplit(),
		"service redirect":                                 testcase_ServiceRedirect(),
		"service and subset redirect":                      testcase_ServiceAndSubsetRedirect(),
		"datacenter redirect":                              testcase_DatacenterRedirect(),
		"datacenter redirect with mesh gateways":           testcase_DatacenterRedirect_WithMeshGateways(),
		"service failover":                                 testcase_ServiceFailover(),
		"service failover through redirect":                testcase_ServiceFailoverThroughRedirect(),
		"circular resolver failover":                       testcase_Resolver_CircularFailover(),
		"service and subset failover":                      testcase_ServiceAndSubsetFailover(),
		"datacenter failover":                              testcase_DatacenterFailover(),
		"datacenter failover with mesh gateways":           testcase_DatacenterFailover_WithMeshGateways(),
		"noop split to resolver with default subset":       testcase_NoopSplit_WithDefaultSubset(),
		"resolver with default subset":                     testcase_Resolve_WithDefaultSubset(),
		"default resolver with external sni":               testcase_DefaultResolver_ExternalSNI(),
		"resolver with no entries and inferring defaults":  testcase_DefaultResolver(),
		"default resolver with proxy defaults":             testcase_DefaultResolver_WithProxyDefaults(),
		"loadbalancer splitter and resolver":               testcase_LBSplitterAndResolver(),
		"loadbalancer resolver":                            testcase_LBResolver(),
		"service redirect to service with default resolver is not a default chain": testcase_RedirectToDefaultResolverIsNotDefaultChain(),
		"service meta projection":               testcase_ServiceMetaProjection(),
		"service meta projection with redirect": testcase_ServiceMetaProjectionWithRedirect(),

		"all the bells and whistles": testcase_AllBellsAndWhistles(),
		"multi dc canary":            testcase_MultiDatacenterCanary(),

		// various errors
		"splitter requires valid protocol":        testcase_SplitterRequiresValidProtocol(),
		"router requires valid protocol":          testcase_RouterRequiresValidProtocol(),
		"split to unsplittable protocol":          testcase_SplitToUnsplittableProtocol(),
		"route to unroutable protocol":            testcase_RouteToUnroutableProtocol(),
		"failover crosses protocols":              testcase_FailoverCrossesProtocols(),
		"redirect crosses protocols":              testcase_RedirectCrossesProtocols(),
		"redirect to missing subset":              testcase_RedirectToMissingSubset(),
		"resolver with failover and external sni": testcase_Resolver_ExternalSNI_FailoverNotAllowed(),
		"resolver with subsets and external sni":  testcase_Resolver_ExternalSNI_SubsetsNotAllowed(),
		"resolver with redirect and external sni": testcase_Resolver_ExternalSNI_RedirectNotAllowed(),

		// overrides
		"resolver with protocol from override":         testcase_ResolverProtocolOverride(),
		"resolver with protocol from override ignored": testcase_ResolverProtocolOverrideIgnored(),
		"router ignored due to protocol override":      testcase_RouterIgnored_ResolverProtocolOverride(),

		// circular references
		"circular resolver redirect": testcase_Resolver_CircularRedirect(),
		"circular split":             testcase_CircularSplit(),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// sanity check entries are normalized and valid
			for _, entry := range tc.entries.Routers {
				require.NoError(t, entry.Normalize())
				require.NoError(t, entry.Validate())
			}
			for _, entry := range tc.entries.Splitters {
				require.NoError(t, entry.Normalize())
				require.NoError(t, entry.Validate())
			}
			for _, entry := range tc.entries.Resolvers {
				require.NoError(t, entry.Normalize())
				require.NoError(t, entry.Validate())
			}

			req := CompileRequest{
				ServiceName:           "main",
				EvaluateInNamespace:   "default",
				EvaluateInPartition:   "default",
				EvaluateInDatacenter:  "dc1",
				EvaluateInTrustDomain: "trustdomain.consul",
				Entries:               tc.entries,
			}
			if tc.setup != nil {
				tc.setup(&req)
			}

			res, err := Compile(req)
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
				_, ok := err.(*structs.ConfigEntryGraphError)
				if tc.expectGraphErr {
					require.True(t, ok, "%T is not a *ConfigEntryGraphError", err)
				} else {
					require.False(t, ok, "did not expect a *ConfigEntryGraphError here: %v", err)
				}
			} else {
				require.NoError(t, err)

				// Avoid requiring unnecessary test boilerplate and inject these
				// ourselves.
				tc.expect.ServiceName = "main"
				tc.expect.Namespace = "default"
				tc.expect.Partition = "default"
				tc.expect.Datacenter = "dc1"

				if tc.expectCustom {
					require.NotEmpty(t, res.CustomizationHash)
					res.CustomizationHash = ""
				} else {
					require.Empty(t, res.CustomizationHash)
				}

				require.Equal(t, tc.expect, res)
			}
		})
	}
}

func testcase_JustRouterWithDefaults() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_JustRouterWithNoDestination() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
			Routes: []structs.ServiceRoute{
				{
					Match: &structs.ServiceRouteMatch{
						HTTP: &structs.ServiceRouteHTTPMatch{
							PathPrefix: "/",
						},
					},
				},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: &structs.ServiceRoute{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathPrefix: "/",
								},
							},
						},
						NextNode: "resolver:main.default.default.dc1",
					},
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_RouterWithDefaults_NoSplit_WithResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind:           "service-resolver",
			Name:           "main",
			ConnectTimeout: 33 * time.Second,
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 33 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": targetWithConnectTimeout(
				newTarget("main", "", "default", "default", "dc1", nil),
				33*time.Second,
			),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_RouterWithDefaults_WithNoopSplit_DefaultResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
		},
	)
	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "splitter:main.default.default",
					},
				},
			},
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight: 100,
						},
						Weight:   100,
						NextNode: "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_NoopSplit_DefaultResolver_ProtocolFromProxyDefaults() compileTestCase {
	entries := newEntries()
	setGlobalProxyProtocol(entries, "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
		},
	)
	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "splitter:main.default.default",
					},
				},
			},
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight: 100,
						},
						Weight:   100,
						NextNode: "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_RouterWithDefaults_WithNoopSplit_WithResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
		},
	)
	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind:           "service-resolver",
			Name:           "main",
			ConnectTimeout: 33 * time.Second,
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "splitter:main.default.default",
					},
				},
			},
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight: 100,
						},
						Weight:   100,
						NextNode: "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 33 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": targetWithConnectTimeout(
				newTarget("main", "", "default", "default", "dc1", nil),
				33*time.Second,
			),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_RouteBypassesSplit() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")
	setServiceProtocol(entries, "other", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
			Routes: []structs.ServiceRoute{
				// route direct subset reference (bypass split)
				newSimpleRoute("other", func(r *structs.ServiceRoute) {
					r.Destination.ServiceSubset = "bypass"
				}),
			},
		},
	)
	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "other",
			Splits: []structs.ServiceSplit{
				{Weight: 100, Service: "ignored"},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "other",
			Subsets: map[string]structs.ServiceResolverSubset{
				"bypass": {
					Filter: "Service.Meta.version == bypass",
				},
			},
		},
	)

	router := entries.GetRouter(structs.NewServiceID("main", nil))

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: &router.Routes[0],
						NextNode:   "resolver:bypass.other.default.default.dc1",
					},
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
			"resolver:bypass.other.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "bypass.other.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "bypass.other.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
			"bypass.other.default.default.dc1": newTarget("other", "bypass", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == bypass",
				}
			}),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_NoopSplit_DefaultResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight: 100,
						},
						Weight:   100,
						NextNode: "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_NoopSplit_WithResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind:           "service-resolver",
			Name:           "main",
			ConnectTimeout: 33 * time.Second,
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight: 100,
						},
						Weight:   100,
						NextNode: "resolver:main.default.default.dc1",
					},
				},
			},
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 33 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": targetWithConnectTimeout(
				newTarget("main", "", "default", "default", "dc1", nil),
				33*time.Second,
			),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_SubsetSplit() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 60, ServiceSubset: "v2"},
				{Weight: 40, ServiceSubset: "v1"},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": {
					Filter: "Service.Meta.version == 1",
				},
				"v2": {
					Filter: "Service.Meta.version == 2",
				},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight:        60,
							ServiceSubset: "v2",
						},
						Weight:   60,
						NextNode: "resolver:v2.main.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:        40,
							ServiceSubset: "v1",
						},
						Weight:   40,
						NextNode: "resolver:v1.main.default.default.dc1",
					},
				},
			},
			"resolver:v2.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v2.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v2.main.default.default.dc1",
				},
			},
			"resolver:v1.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v1.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v1.main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"v2.main.default.default.dc1": newTarget("main", "v2", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 2",
				}
			}),
			"v1.main.default.default.dc1": newTarget("main", "v1", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 1",
				}
			}),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceSplit() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")
	setServiceProtocol(entries, "foo", "http")
	setServiceProtocol(entries, "bar", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 60, Service: "foo"},
				{Weight: 40, Service: "bar"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight:  60,
							Service: "foo",
						},
						Weight:   60,
						NextNode: "resolver:foo.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:  40,
							Service: "bar",
						},
						Weight:   40,
						NextNode: "resolver:bar.default.default.dc1",
					},
				},
			},
			"resolver:foo.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "foo.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "foo.default.default.dc1",
				},
			},
			"resolver:bar.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "bar.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "bar.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"foo.default.default.dc1": newTarget("foo", "", "default", "default", "dc1", nil),
			"bar.default.default.dc1": newTarget("bar", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_SplitBypassesSplit() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")
	setServiceProtocol(entries, "next", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{
					Weight:        100,
					Service:       "next",
					ServiceSubset: "bypassed",
				},
			},
		},
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "next",
			Splits: []structs.ServiceSplit{
				{
					Weight:        100,
					ServiceSubset: "not-bypassed",
				},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "next",
			Subsets: map[string]structs.ServiceResolverSubset{
				"bypassed": {
					Filter: "Service.Meta.version == bypass",
				},
				"not-bypassed": {
					Filter: "Service.Meta.version != bypass",
				},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight:        100,
							Service:       "next",
							ServiceSubset: "bypassed",
						},
						Weight:   100,
						NextNode: "resolver:bypassed.next.default.default.dc1",
					},
				},
			},
			"resolver:bypassed.next.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "bypassed.next.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "bypassed.next.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"bypassed.next.default.default.dc1": newTarget("next", "bypassed", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == bypass",
				}
			}),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceRedirect() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "other",
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:other.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:other.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "other.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "other.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"other.default.default.dc1": newTarget("other", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceAndSubsetRedirect() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service:       "other",
				ServiceSubset: "v2",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "other",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": {
					Filter: "Service.Meta.version == 1",
				},
				"v2": {
					Filter: "Service.Meta.version == 2",
				},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:v2.other.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:v2.other.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v2.other.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v2.other.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"v2.other.default.default.dc1": newTarget("other", "v2", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 2",
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DatacenterRedirect() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Datacenter: "dc9",
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc9",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc9": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc9",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc9",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc9": newTarget("main", "", "default", "default", "dc9", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DatacenterRedirect_WithMeshGateways() compileTestCase {
	entries := newEntries()
	entries.AddProxyDefaults(&structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		MeshGateway: structs.MeshGatewayConfig{
			Mode: structs.MeshGatewayModeRemote,
		},
	})

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Datacenter: "dc9",
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc9",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc9": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc9",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc9",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc9": newTarget("main", "", "default", "default", "dc9", func(t *structs.DiscoveryTarget) {
				t.MeshGateway = structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceFailover() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Service: "backup"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"backup.default.default.dc1"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1":   newTarget("main", "", "default", "default", "dc1", nil),
			"backup.default.default.dc1": newTarget("backup", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceFailoverThroughRedirect() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "backup",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "actual",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Service: "backup"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"actual.default.default.dc1"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1":   newTarget("main", "", "default", "default", "dc1", nil),
			"actual.default.default.dc1": newTarget("actual", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_Resolver_CircularFailover() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "backup",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Service: "main"},
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Service: "backup"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"backup.default.default.dc1"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1":   newTarget("main", "", "default", "default", "dc1", nil),
			"backup.default.default.dc1": newTarget("backup", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceAndSubsetFailover() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Subsets: map[string]structs.ServiceResolverSubset{
				"backup": {
					Filter: "Service.Meta.version == backup",
				},
			},
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {ServiceSubset: "backup"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"backup.main.default.default.dc1"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
			"backup.main.default.default.dc1": newTarget("main", "backup", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == backup",
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DatacenterFailover() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Datacenters: []string{"dc2", "dc4"}},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"main.default.default.dc2", "main.default.default.dc4"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
			"main.default.default.dc2": newTarget("main", "", "default", "default", "dc2", nil),
			"main.default.default.dc4": newTarget("main", "", "default", "default", "dc4", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DatacenterFailover_WithMeshGateways() compileTestCase {
	entries := newEntries()

	entries.AddProxyDefaults(&structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		MeshGateway: structs.MeshGatewayConfig{
			Mode: structs.MeshGatewayModeRemote,
		},
	})

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Datacenters: []string{"dc2", "dc4"}},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{
							"main.default.default.dc2",
							"main.default.default.dc4",
						},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.MeshGateway = structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				}
			}),
			"main.default.default.dc2": newTarget("main", "", "default", "default", "dc2", func(t *structs.DiscoveryTarget) {
				t.MeshGateway = structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				}
			}),
			"main.default.default.dc4": newTarget("main", "", "default", "default", "dc4", func(t *structs.DiscoveryTarget) {
				t.MeshGateway = structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_NoopSplit_WithDefaultSubset() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind:          "service-resolver",
			Name:          "main",
			DefaultSubset: "v2",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": {Filter: "Service.Meta.version == 1"},
				"v2": {Filter: "Service.Meta.version == 2"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight: 100,
						},
						Weight:   100,
						NextNode: "resolver:v2.main.default.default.dc1",
					},
				},
			},
			"resolver:v2.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v2.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v2.main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"v2.main.default.default.dc1": newTarget("main", "v2", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 2",
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DefaultResolver() compileTestCase {
	entries := newEntries()

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		Default:   true,
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			// TODO-TARGET
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DefaultResolver_WithProxyDefaults() compileTestCase {
	entries := newEntries()

	entries.AddProxyDefaults(&structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "grpc",
		},
		MeshGateway: structs.MeshGatewayConfig{
			Mode: structs.MeshGatewayModeRemote,
		},
	})

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "grpc",
		Default:   true,
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.MeshGateway = structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceMetaProjection() compileTestCase {
	entries := newEntries()
	entries.AddServices(
		&structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: "main",
			Meta: map[string]string{
				"foo": "bar",
				"abc": "123",
			},
		},
	)
	expect := &structs.CompiledDiscoveryChain{
		Protocol: "tcp",
		Default:  true,
		ServiceMeta: map[string]string{
			"foo": "bar",
			"abc": "123",
		},
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_ServiceMetaProjectionWithRedirect() compileTestCase {
	entries := newEntries()
	entries.AddServices(
		&structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: "main",
			Meta: map[string]string{
				"foo": "bar",
				"abc": "123",
			},
		},
		&structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: "other",
			Meta: map[string]string{
				"zim": "gir",
				"abc": "456",
				"xyz": "999",
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "other",
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol: "tcp",
		ServiceMeta: map[string]string{
			// Note this is main's meta, not other's.
			"foo": "bar",
			"abc": "123",
		},
		StartNode: "resolver:other.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:other.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "other.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "other.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"other.default.default.dc1": newTarget("other", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_RedirectToDefaultResolverIsNotDefaultChain() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "other",
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:other.default.default.dc1",
		Default:   false, /*being explicit here because this is the whole point of this test*/
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:other.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "other.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "other.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"other.default.default.dc1": newTarget("other", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func testcase_Resolve_WithDefaultSubset() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind:          "service-resolver",
			Name:          "main",
			DefaultSubset: "v2",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": {Filter: "Service.Meta.version == 1"},
				"v2": {Filter: "Service.Meta.version == 2"},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:v2.main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:v2.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v2.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v2.main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"v2.main.default.default.dc1": newTarget("main", "v2", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 2",
				}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_DefaultResolver_ExternalSNI() compileTestCase {
	entries := newEntries()
	entries.AddServices(&structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "main",
		ExternalSNI: "main.some.other.service.mesh",
	})

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		Default:   true,
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.SNI = "main.some.other.service.mesh"
				t.External = true
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_Resolver_ExternalSNI_FailoverNotAllowed() compileTestCase {
	entries := newEntries()
	entries.AddServices(&structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "main",
		ExternalSNI: "main.some.other.service.mesh",
	})
	entries.AddResolvers(&structs.ServiceResolverConfigEntry{
		Kind:           "service-resolver",
		Name:           "main",
		ConnectTimeout: 33 * time.Second,
		Failover: map[string]structs.ServiceResolverFailover{
			"*": {Service: "backup"},
		},
	})

	return compileTestCase{
		entries:        entries,
		expectErr:      `service "main" has an external SNI set; cannot define failover for external services`,
		expectGraphErr: true,
	}
}

func testcase_Resolver_ExternalSNI_SubsetsNotAllowed() compileTestCase {
	entries := newEntries()
	entries.AddServices(&structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "main",
		ExternalSNI: "main.some.other.service.mesh",
	})
	entries.AddResolvers(&structs.ServiceResolverConfigEntry{
		Kind:           "service-resolver",
		Name:           "main",
		ConnectTimeout: 33 * time.Second,
		Subsets: map[string]structs.ServiceResolverSubset{
			"canary": {
				Filter: "Service.Meta.version == canary",
			},
		},
	})

	return compileTestCase{
		entries:        entries,
		expectErr:      `service "main" has an external SNI set; cannot define subsets for external services`,
		expectGraphErr: true,
	}
}

func testcase_Resolver_ExternalSNI_RedirectNotAllowed() compileTestCase {
	entries := newEntries()
	entries.AddServices(&structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "main",
		ExternalSNI: "main.some.other.service.mesh",
	})
	entries.AddResolvers(&structs.ServiceResolverConfigEntry{
		Kind:           "service-resolver",
		Name:           "main",
		ConnectTimeout: 33 * time.Second,
		Redirect: &structs.ServiceResolverRedirect{
			Datacenter: "dc2",
		},
	})

	return compileTestCase{
		entries:        entries,
		expectErr:      `service "main" has an external SNI set; cannot define redirects for external services`,
		expectGraphErr: true,
	}
}

func testcase_MultiDatacenterCanary() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")
	setServiceProtocol(entries, "main-dc2", "http")
	setServiceProtocol(entries, "main-dc3", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 60, Service: "main-dc2"},
				{Weight: 40, Service: "main-dc3"},
			},
		},
	)
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main-dc2",
			Redirect: &structs.ServiceResolverRedirect{
				Service:    "main",
				Datacenter: "dc2",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main-dc3",
			Redirect: &structs.ServiceResolverRedirect{
				Service:    "main",
				Datacenter: "dc3",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind:           "service-resolver",
			Name:           "main",
			ConnectTimeout: 33 * time.Second,
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight:  60,
							Service: "main-dc2",
						},
						Weight:   60,
						NextNode: "resolver:main.default.default.dc2",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:  40,
							Service: "main-dc3",
						},
						Weight:   40,
						NextNode: "resolver:main.default.default.dc3",
					},
				},
			},
			"resolver:main.default.default.dc2": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc2",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 33 * time.Second,
					Target:         "main.default.default.dc2",
				},
			},
			"resolver:main.default.default.dc3": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc3",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 33 * time.Second,
					Target:         "main.default.default.dc3",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc2": targetWithConnectTimeout(
				newTarget("main", "", "default", "default", "dc2", nil),
				33*time.Second,
			),
			"main.default.default.dc3": targetWithConnectTimeout(
				newTarget("main", "", "default", "default", "dc3", nil),
				33*time.Second,
			),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_AllBellsAndWhistles() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")
	setServiceProtocol(entries, "svc-redirect", "http")
	setServiceProtocol(entries, "svc-redirect-again", "http")
	setServiceProtocol(entries, "svc-split", "http")
	setServiceProtocol(entries, "svc-split-again", "http")
	setServiceProtocol(entries, "svc-split-one-more-time", "http")
	setServiceProtocol(entries, "redirected", "http")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
			Routes: []structs.ServiceRoute{
				newSimpleRoute("svc-redirect"), // double redirected to a default subset
				newSimpleRoute("svc-split"),    // one split is split further
			},
		},
	)
	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "svc-split",
			Splits: []structs.ServiceSplit{
				{Weight: 60, Service: "svc-redirect"},    // double redirected to a default subset
				{Weight: 40, Service: "svc-split-again"}, // split again
			},
		},
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "svc-split-again",
			Splits: []structs.ServiceSplit{
				{Weight: 75, Service: "main", ServiceSubset: "v1"},
				{
					Weight:  25,
					Service: "svc-split-one-more-time",
					RequestHeaders: &structs.HTTPHeaderModifiers{
						Set: map[string]string{
							"parent": "1",
							"shared": "from-parent",
						},
					},
					ResponseHeaders: &structs.HTTPHeaderModifiers{
						Set: map[string]string{
							"parent": "2",
							"shared": "from-parent",
						},
					},
				},
			},
		},
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "svc-split-one-more-time",
			Splits: []structs.ServiceSplit{
				{Weight: 80, Service: "main", ServiceSubset: "v2"},
				{
					Weight:        20,
					Service:       "main",
					ServiceSubset: "v3",
					RequestHeaders: &structs.HTTPHeaderModifiers{
						Set: map[string]string{
							"child":  "3",
							"shared": "from-child",
						},
					},
					ResponseHeaders: &structs.HTTPHeaderModifiers{
						Set: map[string]string{
							"child":  "4",
							"shared": "from-parent",
						},
					},
				},
			},
		},
	)

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "svc-redirect",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "svc-redirect-again",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "svc-redirect-again",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "redirected",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind:          "service-resolver",
			Name:          "redirected",
			DefaultSubset: "prod",
			Subsets: map[string]structs.ServiceResolverSubset{
				"prod": {Filter: "ServiceMeta.env == prod"},
				"qa":   {Filter: "ServiceMeta.env == qa"},
			},
			LoadBalancer: &structs.LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: &structs.RingHashConfig{
					MaximumRingSize: 100,
				},
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
					},
				},
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind:          "service-resolver",
			Name:          "main",
			DefaultSubset: "default-subset",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1":             {Filter: "Service.Meta.version == 1"},
				"v2":             {Filter: "Service.Meta.version == 2"},
				"v3":             {Filter: "Service.Meta.version == 3"},
				"default-subset": {OnlyPassing: true},
			},
		},
	)

	var (
		router = entries.GetRouter(structs.NewServiceID("main", nil))
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "router:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"router:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: "main.default.default",
				Routes: []*structs.DiscoveryRoute{
					{
						Definition: &router.Routes[0],
						NextNode:   "resolver:prod.redirected.default.default.dc1",
					},
					{
						Definition: &router.Routes[1],
						NextNode:   "splitter:svc-split.default.default",
					},
					{
						Definition: newDefaultServiceRoute("main", "default", "default"),
						NextNode:   "resolver:default-subset.main.default.default.dc1",
					},
				},
			},
			"splitter:svc-split.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "svc-split.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight:  60,
							Service: "svc-redirect",
						},
						Weight:   60,
						NextNode: "resolver:prod.redirected.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:        75,
							Service:       "main",
							ServiceSubset: "v1",
						},
						Weight:   30,
						NextNode: "resolver:v1.main.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:        80,
							Service:       "main",
							ServiceSubset: "v2",
							// Should inherit these from parent verbatim as there was no
							// child-split header manip.
							RequestHeaders: &structs.HTTPHeaderModifiers{
								Set: map[string]string{
									"parent": "1",
									"shared": "from-parent",
								},
							},
							ResponseHeaders: &structs.HTTPHeaderModifiers{
								Set: map[string]string{
									"parent": "2",
									"shared": "from-parent",
								},
							},
						},
						Weight:   8,
						NextNode: "resolver:v2.main.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:        20,
							Service:       "main",
							ServiceSubset: "v3",
							// Should get a merge of child and parent rules
							RequestHeaders: &structs.HTTPHeaderModifiers{
								Set: map[string]string{
									"parent": "1",
									"child":  "3",
									"shared": "from-child",
								},
							},
							ResponseHeaders: &structs.HTTPHeaderModifiers{
								Set: map[string]string{
									"parent": "2",
									"child":  "4",
									"shared": "from-parent",
								},
							},
						},
						Weight:   2,
						NextNode: "resolver:v3.main.default.default.dc1",
					},
				},
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MaximumRingSize: 100,
					},
					HashPolicies: []structs.HashPolicy{
						{
							SourceIP: true,
						},
					},
				},
			},
			"resolver:prod.redirected.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "prod.redirected.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "prod.redirected.default.default.dc1",
				},
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MaximumRingSize: 100,
					},
					HashPolicies: []structs.HashPolicy{
						{
							SourceIP: true,
						},
					},
				},
			},
			"resolver:v1.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v1.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v1.main.default.default.dc1",
				},
			},
			"resolver:v2.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v2.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v2.main.default.default.dc1",
				},
			},
			"resolver:v3.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "v3.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "v3.main.default.default.dc1",
				},
			},
			"resolver:default-subset.main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "default-subset.main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 5 * time.Second,
					Target:         "default-subset.main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"prod.redirected.default.default.dc1": newTarget("redirected", "prod", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "ServiceMeta.env == prod",
				}
			}),
			"v1.main.default.default.dc1": newTarget("main", "v1", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 1",
				}
			}),
			"v2.main.default.default.dc1": newTarget("main", "v2", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 2",
				}
			}),
			"v3.main.default.default.dc1": newTarget("main", "v3", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{
					Filter: "Service.Meta.version == 3",
				}
			}),
			"default-subset.main.default.default.dc1": newTarget("main", "default-subset", "default", "default", "dc1", func(t *structs.DiscoveryTarget) {
				t.Subset = structs.ServiceResolverSubset{OnlyPassing: true}
			}),
		},
	}
	return compileTestCase{entries: entries, expect: expect}
}

func testcase_SplitterRequiresValidProtocol() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "tcp")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: structs.ServiceSplitter,
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 90, Namespace: "v1"},
				{Weight: 10, Namespace: "v2"},
			},
		},
	)

	return compileTestCase{
		entries:        entries,
		expectErr:      "does not permit advanced routing or splitting behavior",
		expectGraphErr: true,
	}
}

func testcase_RouterRequiresValidProtocol() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "tcp")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: structs.ServiceRouter,
			Name: "main",
			Routes: []structs.ServiceRoute{
				{
					Match: &structs.ServiceRouteMatch{
						HTTP: &structs.ServiceRouteHTTPMatch{
							PathExact: "/other",
						},
					},
					Destination: &structs.ServiceRouteDestination{
						Namespace: "other",
					},
				},
			},
		},
	)
	return compileTestCase{
		entries:        entries,
		expectErr:      "does not permit advanced routing or splitting behavior",
		expectGraphErr: true,
	}
}

func testcase_SplitToUnsplittableProtocol() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "tcp")
	setServiceProtocol(entries, "other", "tcp")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: structs.ServiceSplitter,
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 90},
				{Weight: 10, Service: "other"},
			},
		},
	)
	return compileTestCase{
		entries:        entries,
		expectErr:      "does not permit advanced routing or splitting behavior",
		expectGraphErr: true,
	}
}

func testcase_RouteToUnroutableProtocol() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "tcp")
	setServiceProtocol(entries, "other", "tcp")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: structs.ServiceRouter,
			Name: "main",
			Routes: []structs.ServiceRoute{
				{
					Match: &structs.ServiceRouteMatch{
						HTTP: &structs.ServiceRouteHTTPMatch{
							PathExact: "/other",
						},
					},
					Destination: &structs.ServiceRouteDestination{
						Service: "other",
					},
				},
			},
		},
	)

	return compileTestCase{
		entries:        entries,
		expectErr:      "does not permit advanced routing or splitting behavior",
		expectGraphErr: true,
	}
}

func testcase_FailoverCrossesProtocols() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "grpc")
	setServiceProtocol(entries, "other", "tcp")

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {
					Service: "other",
				},
			},
		},
	)

	return compileTestCase{
		entries:        entries,
		expectErr:      "uses inconsistent protocols",
		expectGraphErr: true,
	}
}

func testcase_RedirectCrossesProtocols() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "grpc")
	setServiceProtocol(entries, "other", "tcp")

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "other",
			},
		},
	)
	return compileTestCase{
		entries:        entries,
		expectErr:      "uses inconsistent protocols",
		expectGraphErr: true,
	}
}

func testcase_RedirectToMissingSubset() compileTestCase {
	entries := newEntries()

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "other",
			ConnectTimeout: 33 * time.Second,
		},
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service:       "other",
				ServiceSubset: "v1",
			},
		},
	)

	return compileTestCase{
		entries:        entries,
		expectErr:      `does not have a subset named "v1"`,
		expectGraphErr: true,
	}
}

func testcase_ResolverProtocolOverride() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "grpc")

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http2",
		Default:   true,
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			// TODO-TARGET
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect,
		expectCustom: true,
		setup: func(req *CompileRequest) {
			req.OverrideProtocol = "http2"
		},
	}
}

func testcase_ResolverProtocolOverrideIgnored() compileTestCase {
	// This shows that if you try to override the protocol to its current value
	// the override is completely ignored.
	entries := newEntries()
	setServiceProtocol(entries, "main", "http2")

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http2",
		Default:   true,
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			// TODO-TARGET
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect,
		setup: func(req *CompileRequest) {
			req.OverrideProtocol = "http2"
		},
	}
}

func testcase_RouterIgnored_ResolverProtocolOverride() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "grpc")

	entries.AddRouters(
		&structs.ServiceRouterConfigEntry{
			Kind: "service-router",
			Name: "main",
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "tcp",
		StartNode: "resolver:main.default.default.dc1",
		Default:   true,
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        true,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			// TODO-TARGET
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}
	return compileTestCase{entries: entries, expect: expect,
		expectCustom: true,
		setup: func(req *CompileRequest) {
			req.OverrideProtocol = "tcp"
		},
	}
}

func testcase_Resolver_CircularRedirect() compileTestCase {
	entries := newEntries()
	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "other",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "other",
			Redirect: &structs.ServiceResolverRedirect{
				Service: "main",
			},
		},
	)

	return compileTestCase{entries: entries,
		expectErr:      "detected circular resolver redirect",
		expectGraphErr: true,
	}
}

func testcase_CircularSplit() compileTestCase {
	entries := newEntries()
	setGlobalProxyProtocol(entries, "http")
	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 100, Service: "other"},
			},
		},
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "other",
			Splits: []structs.ServiceSplit{
				{Weight: 100, Service: "main"},
			},
		},
	)

	return compileTestCase{entries: entries,
		expectErr:      "detected circular reference",
		expectGraphErr: true,
	}
}

func testcase_LBSplitterAndResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "foo", "http")
	setServiceProtocol(entries, "bar", "http")
	setServiceProtocol(entries, "baz", "http")

	entries.AddSplitters(
		&structs.ServiceSplitterConfigEntry{
			Kind: "service-splitter",
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 60, Service: "foo"},
				{Weight: 20, Service: "bar"},
				{Weight: 20, Service: "baz"},
			},
		},
	)

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "foo",
			LoadBalancer: &structs.LoadBalancer{
				Policy: "least_request",
				LeastRequestConfig: &structs.LeastRequestConfig{
					ChoiceCount: 3,
				},
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "bar",
			LoadBalancer: &structs.LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: &structs.RingHashConfig{
					MaximumRingSize: 101,
				},
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
					},
				},
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "baz",
			LoadBalancer: &structs.LoadBalancer{
				Policy: "maglev",
				HashPolicies: []structs.HashPolicy{
					{
						Field:      "cookie",
						FieldValue: "chocolate-chip",
						CookieConfig: &structs.CookieConfig{
							TTL:  2 * time.Minute,
							Path: "/bowl",
						},
						Terminal: true,
					},
				},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "splitter:main.default.default",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"splitter:main.default.default": {
				Type: structs.DiscoveryGraphNodeTypeSplitter,
				Name: "main.default.default",
				Splits: []*structs.DiscoverySplit{
					{
						Definition: &structs.ServiceSplit{
							Weight:  60,
							Service: "foo",
						},
						Weight:   60,
						NextNode: "resolver:foo.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:  20,
							Service: "bar",
						},
						Weight:   20,
						NextNode: "resolver:bar.default.default.dc1",
					},
					{
						Definition: &structs.ServiceSplit{
							Weight:  20,
							Service: "baz",
						},
						Weight:   20,
						NextNode: "resolver:baz.default.default.dc1",
					},
				},
				// The LB config from bar is attached because splitters only care about hash-based policies,
				// and it's the config from bar not baz because we pick the first one we encounter in the Splits.
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MaximumRingSize: 101,
					},
					HashPolicies: []structs.HashPolicy{
						{
							SourceIP: true,
						},
					},
				},
			},
			// Each service's LB config is passed down from the service-resolver to the resolver node
			"resolver:foo.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "foo.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        false,
					ConnectTimeout: 5 * time.Second,
					Target:         "foo.default.default.dc1",
				},
				LoadBalancer: &structs.LoadBalancer{
					Policy: "least_request",
					LeastRequestConfig: &structs.LeastRequestConfig{
						ChoiceCount: 3,
					},
				},
			},
			"resolver:bar.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "bar.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        false,
					ConnectTimeout: 5 * time.Second,
					Target:         "bar.default.default.dc1",
				},
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MaximumRingSize: 101,
					},
					HashPolicies: []structs.HashPolicy{
						{
							SourceIP: true,
						},
					},
				},
			},
			"resolver:baz.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "baz.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        false,
					ConnectTimeout: 5 * time.Second,
					Target:         "baz.default.default.dc1",
				},
				LoadBalancer: &structs.LoadBalancer{
					Policy: "maglev",
					HashPolicies: []structs.HashPolicy{
						{
							Field:      "cookie",
							FieldValue: "chocolate-chip",
							CookieConfig: &structs.CookieConfig{
								TTL:  2 * time.Minute,
								Path: "/bowl",
							},
							Terminal: true,
						},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"foo.default.default.dc1": newTarget("foo", "", "default", "default", "dc1", nil),
			"bar.default.default.dc1": newTarget("bar", "", "default", "default", "dc1", nil),
			"baz.default.default.dc1": newTarget("baz", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

// ensure chain with LB cfg in resolver isn't a default chain (!IsDefault)
func testcase_LBResolver() compileTestCase {
	entries := newEntries()
	setServiceProtocol(entries, "main", "http")

	entries.AddResolvers(
		&structs.ServiceResolverConfigEntry{
			Kind: "service-resolver",
			Name: "main",
			LoadBalancer: &structs.LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: &structs.RingHashConfig{
					MaximumRingSize: 101,
				},
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
					},
				},
			},
		},
	)

	expect := &structs.CompiledDiscoveryChain{
		Protocol:  "http",
		StartNode: "resolver:main.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:main.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "main.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					Default:        false,
					ConnectTimeout: 5 * time.Second,
					Target:         "main.default.default.dc1",
				},
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MaximumRingSize: 101,
					},
					HashPolicies: []structs.HashPolicy{
						{
							SourceIP: true,
						},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"main.default.default.dc1": newTarget("main", "", "default", "default", "dc1", nil),
		},
	}

	return compileTestCase{entries: entries, expect: expect}
}

func newSimpleRoute(name string, muts ...func(*structs.ServiceRoute)) structs.ServiceRoute {
	r := structs.ServiceRoute{
		Match: &structs.ServiceRouteMatch{
			HTTP: &structs.ServiceRouteHTTPMatch{PathPrefix: "/" + name},
		},
		Destination: &structs.ServiceRouteDestination{Service: name},
	}

	for _, mut := range muts {
		mut(&r)
	}

	return r
}

func setGlobalProxyProtocol(entries *configentry.DiscoveryChainSet, protocol string) {
	entries.AddProxyDefaults(&structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": protocol,
		},
	})
}

func setServiceProtocol(entries *configentry.DiscoveryChainSet, name, protocol string) {
	entries.AddServices(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     name,
		Protocol: protocol,
	})
}

func newEntries() *configentry.DiscoveryChainSet {
	return &configentry.DiscoveryChainSet{
		Routers:   make(map[structs.ServiceID]*structs.ServiceRouterConfigEntry),
		Splitters: make(map[structs.ServiceID]*structs.ServiceSplitterConfigEntry),
		Resolvers: make(map[structs.ServiceID]*structs.ServiceResolverConfigEntry),
	}
}

func newTarget(service, serviceSubset, namespace, partition, datacenter string, modFn func(t *structs.DiscoveryTarget)) *structs.DiscoveryTarget {
	t := structs.NewDiscoveryTarget(service, serviceSubset, namespace, partition, datacenter)
	t.SNI = connect.TargetSNI(t, "trustdomain.consul")
	t.Name = t.SNI
	t.ConnectTimeout = 5 * time.Second // default
	if modFn != nil {
		modFn(t)
	}
	return t
}

func targetWithConnectTimeout(t *structs.DiscoveryTarget, connectTimeout time.Duration) *structs.DiscoveryTarget {
	t.ConnectTimeout = connectTimeout
	return t
}
