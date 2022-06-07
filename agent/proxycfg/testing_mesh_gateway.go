package proxycfg

import (
	"math"
	"time"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/structs"
)

func TestConfigSnapshotMeshGateway(t testing.T, variant string, nsFn func(ns *structs.NodeService), extraUpdates []UpdateEvent) *ConfigSnapshot {
	roots, _ := TestCerts(t)

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
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}
