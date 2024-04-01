// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"time"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func setupTestVariationConfigEntriesAndSnapshot(
	t testing.T,
	variation string,
	enterprise bool,
	upstreams structs.Upstreams,
	additionalEntries ...structs.ConfigEntry,
) []UpdateEvent {
	var (
		dbUpstream = upstreams[0]

		dbUID = NewUpstreamID(&dbUpstream)
	)

	dbChain := setupTestVariationDiscoveryChain(t, variation, enterprise, dbUID.EnterpriseMeta, additionalEntries...)

	nodes := TestUpstreamNodes(t, "db")
	if variation == "register-to-terminating-gateway" {
		for _, node := range nodes {
			node.Service.Kind = structs.ServiceKindTerminatingGateway
		}
	}
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
				Nodes: nodes,
			},
		},
	}

	dbOpts := structs.DiscoveryTargetOpts{
		Service:    dbUID.Name,
		Namespace:  dbUID.NamespaceOrDefault(),
		Partition:  dbUID.PartitionOrDefault(),
		Datacenter: "dc1",
	}
	dbChainID := structs.ChainID(dbOpts)
	makeChainID := func(opts structs.DiscoveryTargetOpts) string {
		finalOpts := structs.MergeDiscoveryTargetOpts(dbOpts, opts)
		return structs.ChainID(finalOpts)
	}

	switch variation {
	case "default":
	case "simple-with-overrides":
	case "simple":
	case "external-sni":
	case "failover":
		chainID := makeChainID(structs.DiscoveryTargetOpts{Service: "fail"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + chainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesAlternate(t),
			},
		})
	case "failover-through-remote-gateway-triggered":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + dbChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		})
		fallthrough
	case "failover-through-remote-gateway":
		chainID := makeChainID(structs.DiscoveryTargetOpts{Datacenter: "dc2"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + chainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc2:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestGatewayNodesDC2(t),
			},
		})
	case "failover-to-cluster-peer":
		uid := UpstreamID{
			Name:           "db",
			Peer:           "cluster-01",
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(dbUID.PartitionOrDefault(), ""),
		}
		events = append(events, UpdateEvent{
			CorrelationID: "peer-trust-bundle:cluster-01",
			Result: &pbpeering.TrustBundleReadResponse{
				Bundle: &pbpeering.PeeringTrustBundle{
					PeerName:          "peer1",
					TrustDomain:       "peer1.domain",
					ExportedPartition: "peer1ap",
					RootPEMs:          []string{"peer1-root-1"},
				},
			},
		})
		if enterprise {
			uid.EnterpriseMeta = acl.NewEnterpriseMetaWithPartition(dbUID.PartitionOrDefault(), "ns9")
		}
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-peer:" + uid.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: structs.CheckServiceNodes{structs.TestCheckNodeServiceWithNameInPeer(t, "db", "dc2", "cluster-01", "10.40.1.1", false, uid.EnterpriseMeta)},
			},
		})
	case "redirect-to-cluster-peer":
		events = append(events, UpdateEvent{
			CorrelationID: "peer-trust-bundle:cluster-01",
			Result: &pbpeering.TrustBundleReadResponse{
				Bundle: &pbpeering.PeeringTrustBundle{
					PeerName:          "peer1",
					TrustDomain:       "peer1.domain",
					ExportedPartition: "peer1ap",
					RootPEMs:          []string{"peer1-root-1"},
				},
			},
		})
		uid := UpstreamID{
			Name: "db",
			Peer: "cluster-01",
		}
		if enterprise {
			uid.EnterpriseMeta = acl.NewEnterpriseMetaWithPartition(dbUID.PartitionOrDefault(), "ns9")
		}
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-peer:" + uid.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: structs.CheckServiceNodes{structs.TestCheckNodeServiceWithNameInPeer(t, "db", "dc2", "cluster-01", "10.40.1.1", false, uid.EnterpriseMeta)},
			},
		})
	case "failover-through-double-remote-gateway-triggered":
		chainID := makeChainID(structs.DiscoveryTargetOpts{Datacenter: "dc2"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + dbChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		},
			UpdateEvent{
				CorrelationID: "upstream-target:" + chainID + ":" + dbUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodesInStatusDC2(t, "critical"),
				},
			})
		fallthrough
	case "failover-through-double-remote-gateway":
		chainID := makeChainID(structs.DiscoveryTargetOpts{Datacenter: "dc3"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + chainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		},
			UpdateEvent{
				CorrelationID: "mesh-gateway:dc2:" + dbUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestGatewayNodesDC2(t),
				},
			},
			UpdateEvent{
				CorrelationID: "mesh-gateway:dc3:" + dbUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestGatewayNodesDC3(t),
				},
			})
	case "failover-through-local-gateway-triggered":
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + dbChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		})
		fallthrough
	case "failover-through-local-gateway":
		chainID := makeChainID(structs.DiscoveryTargetOpts{Datacenter: "dc2"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + chainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		},
			UpdateEvent{
				CorrelationID: "mesh-gateway:dc1:" + dbUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestGatewayNodesDC1(t),
				},
			})
	case "failover-through-double-local-gateway-triggered":
		db2ChainID := makeChainID(structs.DiscoveryTargetOpts{Datacenter: "dc2"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + dbChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesInStatus(t, "critical"),
			},
		},
			UpdateEvent{
				CorrelationID: "upstream-target:" + db2ChainID + ":" + dbUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodesInStatusDC2(t, "critical"),
				},
			})
		fallthrough
	case "failover-through-double-local-gateway":
		db3ChainID := makeChainID(structs.DiscoveryTargetOpts{Datacenter: "dc3"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + db3ChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "mesh-gateway:dc1:" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestGatewayNodesDC1(t),
			},
		})
	case "splitter-with-resolver-redirect-multidc":
		v1ChainID := makeChainID(structs.DiscoveryTargetOpts{ServiceSubset: "v1"})
		v2ChainID := makeChainID(structs.DiscoveryTargetOpts{ServiceSubset: "v2", Datacenter: "dc2"})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + v1ChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "db"),
			},
		})
		events = append(events, UpdateEvent{
			CorrelationID: "upstream-target:" + v2ChainID + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesDC2(t),
			},
		})
	case "chain-and-splitter":
	case "grpc-router":
	case "chain-and-router":
	case "lb-resolver":
	case "register-to-terminating-gateway":
	case "redirect-to-lb-node":
	case "resolver-with-lb":
	case "splitter-overweight":
	default:
		extraEvents := extraUpdateEvents(t, variation, dbUID)
		events = append(events, extraEvents...)
	}

	return events
}

func setupTestVariationDiscoveryChain(
	t testing.T,
	variation string,
	enterprise bool,
	entMeta acl.EnterpriseMeta,
	additionalEntries ...structs.ConfigEntry,
) *structs.CompiledDiscoveryChain {
	// Compile a chain.
	var (
		peers        []*pbpeering.Peering
		entries      []structs.ConfigEntry
		compileSetup func(req *discoverychain.CompileRequest)
	)

	switch variation {
	case "default":
		// no config entries
	case "register-to-terminating-gateway":
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
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
			},
		)
	case "external-sni":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind:           structs.ServiceDefaults,
				Name:           "db",
				EnterpriseMeta: entMeta,
				ExternalSNI:    "db.some.other.service.mesh",
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
			},
		)
	case "failover":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
				Kind:           structs.ServiceDefaults,
				Name:           "db",
				EnterpriseMeta: entMeta,
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2"},
					},
				},
			},
		)
	case "failover-to-cluster-peer":
		target := structs.ServiceResolverFailoverTarget{
			Peer: "cluster-01",
		}

		if enterprise {
			target.Namespace = "ns9"
		}

		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Targets: []structs.ServiceResolverFailoverTarget{target},
					},
				},
			},
		)
	case "redirect-to-cluster-peer":
		redirect := &structs.ServiceResolverRedirect{
			Peer: "cluster-01",
		}
		if enterprise {
			redirect.Namespace = "ns9"
		}

		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
				Redirect:       redirect,
			},
		)
	case "failover-through-double-remote-gateway-triggered":
		fallthrough
	case "failover-through-double-remote-gateway":
		entries = append(entries,
			&structs.ServiceConfigEntry{
				Kind:           structs.ServiceDefaults,
				Name:           "db",
				EnterpriseMeta: entMeta,
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeRemote,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
				Kind:           structs.ServiceDefaults,
				Name:           "db",
				EnterpriseMeta: entMeta,
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeLocal,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
				Kind:           structs.ServiceDefaults,
				Name:           "db",
				EnterpriseMeta: entMeta,
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeLocal,
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
		em := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), acl.NamespaceOrDefault(""))
		entries = append(entries,
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: em,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				Splits: []structs.ServiceSplit{
					{Weight: 50, Service: "db-dc1"},
					{Weight: 50, Service: "db-dc2"},
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db-dc1",
				EnterpriseMeta: entMeta,
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "db",
					ServiceSubset: "v1",
					Datacenter:    "dc1",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db-dc2",
				EnterpriseMeta: entMeta,
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "db",
					ServiceSubset: "v2",
					Datacenter:    "dc2",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
				EnterpriseMeta: entMeta,
				ConnectTimeout: 25 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			// Adding a ServiceRouter in this case allows testing ServiceRoute.Destination timeouts.
			&structs.ServiceRouterConfigEntry{
				Kind:           structs.ServiceRouter,
				Name:           "db",
				EnterpriseMeta: entMeta,
				Routes: []structs.ServiceRoute{
					{
						Match: &structs.ServiceRouteMatch{
							HTTP: &structs.ServiceRouteHTTPMatch{
								PathPrefix: "/big-side",
							},
						},
						Destination: &structs.ServiceRouteDestination{
							Service: "big-side",
							// Test disabling idle timeout.
							IdleTimeout: -1 * time.Second,
							// Test a positive value for request timeout.
							RequestTimeout: 10 * time.Second,
						},
					},
					{
						Match: &structs.ServiceRouteMatch{
							HTTP: &structs.ServiceRouteHTTPMatch{
								PathPrefix: "/lil-bit-side",
							},
						},
						Destination: &structs.ServiceRouteDestination{
							Service: "lil-bit-side",
							// Test zero values for these timeouts.
							IdleTimeout:    0 * time.Second,
							RequestTimeout: 0 * time.Second,
						},
					},
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind:           structs.ServiceSplitter,
				Name:           "db",
				EnterpriseMeta: entMeta,
				Splits: []structs.ServiceSplit{
					{
						Weight:  1,
						Service: "db",
						RequestHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "db"},
						},
						ResponseHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "db"},
						},
					},
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
						Weight:  3,
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
	case "splitter-overweight":
		entries = append(entries,
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind:           structs.ServiceSplitter,
				Name:           "db",
				EnterpriseMeta: entMeta,
				Splits: []structs.ServiceSplit{
					{
						Weight:  100.0,
						Service: "big-side",
						RequestHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "big"},
						},
						ResponseHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "big"},
						},
					},
					{
						Weight:  100.0,
						Service: "goldilocks-side",
						RequestHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "goldilocks"},
						},
						ResponseHeaders: &structs.HTTPHeaderModifiers{
							Set: map[string]string{"x-split-leg": "goldilocks"},
						},
					},
					{
						Weight:  100.0,
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
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "grpc",
				Config: map[string]interface{}{
					"protocol": "grpc",
				},
			},
			&structs.ServiceRouterConfigEntry{
				Kind:           structs.ServiceRouter,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
				EnterpriseMeta: entMeta,
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
			},
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind:           structs.ServiceSplitter,
				Name:           "split-3-ways",
				EnterpriseMeta: entMeta,
				Splits: []structs.ServiceSplit{
					{Weight: 95.5, Service: "big-side"},
					{Weight: 4, Service: "goldilocks-side"},
					{Weight: 0.5, Service: "lil-bit-side"},
				},
			},
			&structs.ServiceRouterConfigEntry{
				Kind:           structs.ServiceRouter,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
							PathPrefix: "/idle-timeout",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:     "idle-timeout",
							IdleTimeout: 33 * time.Second,
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
							PathPrefix: "/retry-reset",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:    "retry-reset",
							NumRetries: 15,
							RetryOn:    []string{"reset"},
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
							PathPrefix: "/retry-all",
						}),
						Destination: &structs.ServiceRouteDestination{
							Service:               "retry-all",
							RetryOnConnectFailure: true,
							RetryOn:               []string{"5xx", "gateway-error", "reset", "connect-failure", "envoy-ratelimited", "retriable-4xx", "refused-stream", "cancelled", "deadline-exceeded", "internal", "resource-exhausted", "unavailable"},
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
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind:           structs.ServiceSplitter,
				Name:           "db",
				EnterpriseMeta: entMeta,
				Splits: []structs.ServiceSplit{
					{Weight: 95.5, Service: "something-else"},
					{Weight: 4.5, Service: "db"},
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
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
							Field:      "query_parameter",
							FieldValue: "my-pretty-param",
						},
						{
							SourceIP: true,
							Terminal: true,
						},
					},
				},
			})
	case "redirect-to-lb-node":
		entries = append(entries,
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceRouterConfigEntry{
				Kind:           structs.ServiceRouter,
				Name:           "db",
				EnterpriseMeta: entMeta,
				Routes: []structs.ServiceRoute{
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/web",
						}),
						Destination: toService("web"),
					},
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "web",
				EnterpriseMeta: entMeta,
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MinimumRingSize: 20,
						MaximumRingSize: 30,
					},
				},
			},
		)
	case "resolver-with-lb":
		entries = append(entries,
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: entMeta,
				Protocol:       "http",
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				EnterpriseMeta: entMeta,
				LoadBalancer: &structs.LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &structs.RingHashConfig{
						MinimumRingSize: 20,
						MaximumRingSize: 30,
					},
				},
			},
		)
	default:
		e, p := extraDiscoChainConfig(t, variation, entMeta)

		entries = append(entries, e...)
		peers = append(peers, p...)
	}

	if len(additionalEntries) > 0 {
		entries = append(entries, additionalEntries...)
	}

	set := configentry.NewDiscoveryChainSet()

	set.AddEntries(entries...)
	set.AddPeers(peers...)

	return discoverychain.TestCompileConfigEntries(t, "db", entMeta.NamespaceOrDefault(), entMeta.PartitionOrDefault(), "dc1", connect.TestClusterID+".consul", compileSetup, set)
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
