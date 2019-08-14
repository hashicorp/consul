package xds

import (
	"path"
	"sort"
	"testing"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestRoutesFromSnapshot(t *testing.T) {
	httpMatch := func(http *structs.ServiceRouteHTTPMatch) *structs.ServiceRouteMatch {
		return &structs.ServiceRouteMatch{HTTP: http}
	}
	httpMatchHeader := func(headers ...structs.ServiceRouteHTTPMatchHeader) *structs.ServiceRouteMatch {
		return httpMatch(&structs.ServiceRouteHTTPMatch{
			Header: headers,
		})
	}
	httpMatchParam := func(params ...structs.ServiceRouteHTTPMatchQueryParam) *structs.ServiceRouteMatch {
		return httpMatch(&structs.ServiceRouteHTTPMatch{
			QueryParam: params,
		})
	}
	toService := func(svc string) *structs.ServiceRouteDestination {
		return &structs.ServiceRouteDestination{Service: svc}
	}

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
		// Setup is called before the test starts. It is passed the snapshot from
		// create func and is allowed to modify it in any way to setup the
		// test input.
		setup              func(snap *proxycfg.ConfigSnapshot)
		overrideGoldenName string
	}{
		{
			name:   "defaults-no-chain",
			create: proxycfg.TestConfigSnapshot,
			setup:  nil, // Default snapshot
		},
		{
			name:   "connect-proxy-with-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChain,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotDiscoveryChainExternalSNI,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithOverrides,
			setup:  nil,
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name: "connect-proxy-with-chain-and-splitter",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChainWithEntries(t,
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
			},
			setup: nil,
		},
		{
			name: "connect-proxy-with-grpc-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChainWithEntries(t,
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
								Match: httpMatch(&structs.ServiceRouteHTTPMatch{
									PathExact: "/fgrpc.PingServer/Ping",
								}),
								Destination: toService("prefix"),
							},
						},
					},
				)
			},
		},
		{
			name: "connect-proxy-with-chain-and-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChainWithEntries(t,
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
			},
			setup: nil,
		},
		// TODO(rb): test match stanza skipped for grpc
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Sanity check default with no overrides first
			snap := tt.create(t)

			// We need to replace the TLS certs with deterministic ones to make golden
			// files workable. Note we don't update these otherwise they'd change
			// golden files for every test case and so not be any use!
			if snap.ConnectProxy.Leaf != nil {
				snap.ConnectProxy.Leaf.CertPEM = golden(t, "test-leaf-cert", "")
				snap.ConnectProxy.Leaf.PrivateKeyPEM = golden(t, "test-leaf-key", "")
			}
			if snap.Roots != nil {
				snap.Roots.Roots[0].RootCert = golden(t, "test-root-cert", "")
			}

			if tt.setup != nil {
				tt.setup(snap)
			}

			routes, err := routesFromSnapshot(snap, "my-token")
			require.NoError(err)
			sort.Slice(routes, func(i, j int) bool {
				return routes[i].(*envoy.RouteConfiguration).Name < routes[j].(*envoy.RouteConfiguration).Name
			})
			r, err := createResponse(RouteType, "00000001", "00000001", routes)
			require.NoError(err)

			gotJSON := responseToJSON(t, r)

			gName := tt.name
			if tt.overrideGoldenName != "" {
				gName = tt.overrideGoldenName
			}

			require.JSONEq(golden(t, path.Join("routes", gName), gotJSON), gotJSON)
		})
	}
}
