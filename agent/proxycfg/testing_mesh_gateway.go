package proxycfg

import (
	"math"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func TestConfigSnapshotMeshGateway(t testing.T, variant string, nsFn func(ns *structs.NodeService), extraUpdates []UpdateEvent) *ConfigSnapshot {
	roots, _ := TestCertsForMeshGateway(t)

	var (
		populateServices    = true
		useFederationStates = false
		deleteCrossDCEntry  = false
	)

	switch variant {
	case "default":
	case "federation-states":
		populateServices = true
		useFederationStates = true
		deleteCrossDCEntry = true
	case "newer-info-in-federation-states":
		populateServices = true
		useFederationStates = true
		deleteCrossDCEntry = false
	case "older-info-in-federation-states":
		populateServices = true
		useFederationStates = true
		deleteCrossDCEntry = false
	case "no-services":
		populateServices = false
		useFederationStates = false
		deleteCrossDCEntry = false
	case "service-subsets":
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				},
			},
		})
	case "service-subsets2": // TODO(rb): make this merge with 'service-subsets'
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					&structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "foo",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
				},
			},
		})
	case "default-service-subsets2": // TODO(rb): rename to strip the 2 when the prior is merged with 'service-subsets'
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind:          structs.ServiceResolver,
						Name:          "bar",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					&structs.ServiceResolverConfigEntry{
						Kind:          structs.ServiceResolver,
						Name:          "foo",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
				},
			},
		})
	case "ignore-extra-resolvers":
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind:          structs.ServiceResolver,
						Name:          "bar",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					&structs.ServiceResolverConfigEntry{
						Kind:          structs.ServiceResolver,
						Name:          "notfound",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				},
			},
		})
	case "service-timeouts":
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind:           structs.ServiceResolver,
						Name:           "bar",
						ConnectTimeout: 10 * time.Second,
						RequestTimeout: 10 * time.Second,
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				},
			},
		})
	case "non-hash-lb-injected":
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: "service-resolvers", // serviceResolversWatchID
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "least_request",
							LeastRequestConfig: &structs.LeastRequestConfig{
								ChoiceCount: 5,
							},
						},
					},
				},
			},
		})
	case "hash-lb-ignored":
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: "service-resolvers", // serviceResolversWatchID
			Result: &structs.IndexedConfigEntries{
				Kind: structs.ServiceResolver,
				Entries: []structs.ConfigEntry{
					&structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "ring_hash",
							RingHashConfig: &structs.RingHashConfig{
								MinimumRingSize: 20,
								MaximumRingSize: 50,
							},
						},
					},
				},
			},
		})
	default:
		t.Fatalf("unknown variant: %s", variant)
		return nil
	}

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: exportedServiceListWatchID,
			Result: &structs.IndexedExportedServiceList{
				Services: nil,
			},
		},
		{
			CorrelationID: serviceListWatchID,
			Result: &structs.IndexedServiceList{
				Services: nil,
			},
		},
		{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind:    structs.ServiceResolver,
				Entries: nil,
			},
		},
		{
			CorrelationID: datacentersWatchID,
			Result:        &[]string{"dc1"},
		},
		{
			CorrelationID: peeringTrustBundlesWatchID,
			Result: &pbpeering.TrustBundleListByServiceResponse{
				Bundles: nil,
			},
		},
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
	}

	if populateServices || useFederationStates {
		baseEvents = testSpliceEvents(baseEvents, []UpdateEvent{
			{
				CorrelationID: datacentersWatchID,
				Result:        &[]string{"dc1", "dc2", "dc4", "dc6"},
			},
		})
	}

	if populateServices {
		var (
			foo = structs.NewServiceName("foo", nil)
			bar = structs.NewServiceName("bar", nil)
		)
		baseEvents = testSpliceEvents(baseEvents, []UpdateEvent{
			{
				CorrelationID: "mesh-gateway:dc2",
				Result: &structs.IndexedNodesWithGateways{
					Nodes: TestGatewayNodesDC2(t),
				},
			},
			{
				CorrelationID: "mesh-gateway:dc4",
				Result: &structs.IndexedNodesWithGateways{
					Nodes: TestGatewayNodesDC4Hostname(t),
				},
			},
			{
				CorrelationID: "mesh-gateway:dc6",
				Result: &structs.IndexedNodesWithGateways{
					Nodes: TestGatewayNodesDC6Hostname(t),
				},
			},
			{
				CorrelationID: serviceListWatchID,
				Result: &structs.IndexedServiceList{
					Services: []structs.ServiceName{
						foo,
						bar,
					},
				},
			},
			{
				CorrelationID: "connect-service:" + foo.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestGatewayServiceGroupFooDC1(t),
				},
			},
			{
				CorrelationID: "connect-service:" + bar.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestGatewayServiceGroupBarDC1(t),
				},
			},
			{
				CorrelationID: serviceResolversWatchID,
				Result: &structs.IndexedConfigEntries{
					Kind:    structs.ServiceResolver,
					Entries: []structs.ConfigEntry{
						//
					},
				},
			},
		})
	}

	if useFederationStates {
		nsFn = testSpliceNodeServiceFunc(nsFn, func(ns *structs.NodeService) {
			ns.Meta[structs.MetaWANFederationKey] = "1"
		})

		if deleteCrossDCEntry {
			baseEvents = testSpliceEvents(baseEvents, []UpdateEvent{
				{
					// Have the cross-dc query mechanism not work for dc2 so
					// fedstates will infill.
					CorrelationID: "mesh-gateway:dc2",
					Result: &structs.IndexedNodesWithGateways{
						Nodes: nil,
					},
				},
			})
		}

		dc2Nodes := TestGatewayNodesDC2(t)
		switch variant {
		case "newer-info-in-federation-states":
			// Create a duplicate entry in FedStateGateways, with a high ModifyIndex, to
			// verify that fresh data in the federation state is preferred over stale data
			// in GatewayGroups.
			svc := structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.0.1.3", 8443,
				structs.ServiceAddress{Address: "10.0.1.3", Port: 8443},
				structs.ServiceAddress{Address: "198.18.1.3", Port: 443},
			)
			svc.RaftIndex.ModifyIndex = math.MaxUint64

			dc2Nodes = structs.CheckServiceNodes{
				{
					Node:    dc2Nodes[0].Node,
					Service: svc,
				},
			}
		case "older-info-in-federation-states":
			// Create a duplicate entry in FedStateGateways, with a low ModifyIndex, to
			// verify that stale data in the federation state is ignored in favor of the
			// fresher data in GatewayGroups.
			svc := structs.TestNodeServiceMeshGatewayWithAddrs(t,
				"10.0.1.3", 8443,
				structs.ServiceAddress{Address: "10.0.1.3", Port: 8443},
				structs.ServiceAddress{Address: "198.18.1.3", Port: 443},
			)
			svc.RaftIndex.ModifyIndex = 0

			dc2Nodes = structs.CheckServiceNodes{
				{
					Node:    dc2Nodes[0].Node,
					Service: svc,
				},
			}
		}

		baseEvents = testSpliceEvents(baseEvents, []UpdateEvent{
			{
				CorrelationID: federationStateListGatewaysWatchID,
				Result: &structs.DatacenterIndexedCheckServiceNodes{
					DatacenterNodes: map[string]structs.CheckServiceNodes{
						"dc2": dc2Nodes,
						"dc4": TestGatewayNodesDC4Hostname(t),
						"dc6": TestGatewayNodesDC6Hostname(t),
					},
				},
			},
			{
				CorrelationID: consulServerListWatchID,
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: nil, // TODO
				},
			},
		})
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindMeshGateway,
		Service: "mesh-gateway",
		Address: "1.2.3.4",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{},
		},
		Meta: make(map[string]string),
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
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates), false)
}

func TestConfigSnapshotPeeredMeshGateway(t testing.T, variant string, nsFn func(ns *structs.NodeService), extraUpdates []UpdateEvent) *ConfigSnapshot {
	roots, leaf := TestCertsForMeshGateway(t)

	var (
		needPeerA   bool
		needPeerB   bool
		needLeaf    bool
		discoChains = make(map[structs.ServiceName]*structs.CompiledDiscoveryChain)
		endpoints   = make(map[structs.ServiceName]structs.CheckServiceNodes)
		entries     []structs.ConfigEntry
	)

	switch variant {
	case "default-services-http":
		proxyDefaults := &structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"protocol": "http",
			},
		}
		require.NoError(t, proxyDefaults.Normalize())
		require.NoError(t, proxyDefaults.Validate())
		entries = append(entries, proxyDefaults)
		fallthrough // to-case: "default-services-tcp"
	case "default-services-tcp":
		var (
			fooSN = structs.NewServiceName("foo", nil)
			barSN = structs.NewServiceName("bar", nil)
			girSN = structs.NewServiceName("gir", nil)

			fooChain = discoverychain.TestCompileConfigEntries(t, "foo", "default", "default", "dc1", connect.TestClusterID+".consul", nil, entries...)
			barChain = discoverychain.TestCompileConfigEntries(t, "bar", "default", "default", "dc1", connect.TestClusterID+".consul", nil, entries...)
			girChain = discoverychain.TestCompileConfigEntries(t, "gir", "default", "default", "dc1", connect.TestClusterID+".consul", nil, entries...)
		)

		assert.True(t, fooChain.Default)
		assert.True(t, barChain.Default)
		assert.True(t, girChain.Default)

		needPeerA = true
		needPeerB = true
		needLeaf = true
		discoChains[fooSN] = fooChain
		discoChains[barSN] = barChain
		discoChains[girSN] = girChain
		endpoints[fooSN] = TestUpstreamNodes(t, "foo")
		endpoints[barSN] = TestUpstreamNodes(t, "bar")
		endpoints[girSN] = TestUpstreamNodes(t, "gir")

		extraUpdates = append(extraUpdates,
			UpdateEvent{
				CorrelationID: exportedServiceListWatchID,
				Result: &structs.IndexedExportedServiceList{
					Services: map[string]structs.ServiceList{
						"peer-a": []structs.ServiceName{fooSN, barSN},
						"peer-b": []structs.ServiceName{girSN},
					},
				},
			},
			UpdateEvent{
				CorrelationID: serviceListWatchID,
				Result: &structs.IndexedServiceList{
					Services: []structs.ServiceName{
						fooSN,
						barSN,
						girSN,
					},
				},
			},
		)
	case "chain-and-l7-stuff":
		entries = []structs.ConfigEntry{
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "db",
				ConnectTimeout: 33 * time.Second,
				RequestTimeout: 33 * time.Second,
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "api",
				Subsets: map[string]structs.ServiceResolverSubset{
					"v2": {
						Filter: "Service.Meta.version == v2",
					},
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "api-v2",
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "api",
					ServiceSubset: "v2",
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "split",
				Splits: []structs.ServiceSplit{
					{Weight: 60, Service: "alt"},
					{Weight: 40, Service: "db"},
				},
			},
			&structs.ServiceRouterConfigEntry{
				Kind: structs.ServiceRouter,
				Name: "db",
				Routes: []structs.ServiceRoute{
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/split",
						}),
						Destination: toService("split"),
					},
					{
						Match: httpMatch(&structs.ServiceRouteHTTPMatch{
							PathPrefix: "/api",
						}),
						Destination: toService("api-v2"),
					},
				},
			},
		}
		for _, entry := range entries {
			require.NoError(t, entry.Normalize())
			require.NoError(t, entry.Validate())
		}

		var (
			dbSN  = structs.NewServiceName("db", nil)
			altSN = structs.NewServiceName("alt", nil)

			dbChain = discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc1", connect.TestClusterID+".consul", nil, entries...)
		)

		needPeerA = true
		needLeaf = true
		discoChains[dbSN] = dbChain
		endpoints[dbSN] = TestUpstreamNodes(t, "db")
		endpoints[altSN] = TestUpstreamNodes(t, "alt")

		extraUpdates = append(extraUpdates,
			UpdateEvent{
				CorrelationID: datacentersWatchID,
				Result:        &[]string{"dc1"},
			},
			UpdateEvent{
				CorrelationID: exportedServiceListWatchID,
				Result: &structs.IndexedExportedServiceList{
					Services: map[string]structs.ServiceList{
						"peer-a": []structs.ServiceName{dbSN},
					},
				},
			},
			UpdateEvent{
				CorrelationID: serviceListWatchID,
				Result: &structs.IndexedServiceList{
					Services: []structs.ServiceName{
						dbSN,
						altSN,
					},
				},
			},
		)

	default:
		t.Fatalf("unknown variant: %s", variant)
		return nil
	}

	var peerTrustBundles []*pbpeering.PeeringTrustBundle
	switch {
	case needPeerA && needPeerB:
		peerTrustBundles = TestPeerTrustBundles(t).Bundles
	case needPeerA:
		ptb := TestPeerTrustBundles(t)
		peerTrustBundles = ptb.Bundles[0:1]
	case needPeerB:
		ptb := TestPeerTrustBundles(t)
		peerTrustBundles = ptb.Bundles[1:2]
	}

	if needLeaf {
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: leafWatchID,
			Result:        leaf,
		})
	}

	for suffix, chain := range discoChains {
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: "discovery-chain:" + suffix.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: chain,
			},
		})
	}

	for suffix, nodes := range endpoints {
		extraUpdates = append(extraUpdates, UpdateEvent{
			CorrelationID: "connect-service:" + suffix.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: nodes,
			},
		})
	}

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: peeringTrustBundlesWatchID,
			Result: &pbpeering.TrustBundleListByServiceResponse{
				Bundles: peerTrustBundles,
			},
		},
		{
			CorrelationID: serviceListWatchID,
			Result: &structs.IndexedServiceList{
				Services: nil,
			},
		},
		{
			CorrelationID: serviceResolversWatchID,
			Result: &structs.IndexedConfigEntries{
				Kind:    structs.ServiceResolver,
				Entries: nil,
			},
		},
		{
			CorrelationID: datacentersWatchID,
			Result:        &[]string{"dc1"},
		},
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: exportedServiceListWatchID,
			Result: &structs.IndexedExportedServiceList{
				Services: nil,
			},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindMeshGateway,
		Service: "mesh-gateway",
		Address: "1.2.3.4",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{},
		},
		Meta: make(map[string]string),
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
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates), true)
}
