// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"fmt"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
)

func TestConfigSnapshotAPIGateway(
	t testing.T,
	variation string,
	nsFn func(ns *structs.NodeService),
	configFn func(entry *structs.APIGatewayConfigEntry, boundEntry *structs.BoundAPIGatewayConfigEntry),
	routes []structs.BoundRoute,
	certificates []structs.InlineCertificateConfigEntry,
	extraUpdates []UpdateEvent,
	additionalEntries ...structs.ConfigEntry,
) *ConfigSnapshot {
	roots, placeholderLeaf := TestCerts(t)

	entry := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "api-gateway",
	}
	boundEntry := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "api-gateway",
	}

	if configFn != nil {
		configFn(entry, boundEntry)
	}

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID,
			Result:        placeholderLeaf,
		},
		{
			CorrelationID: apiGatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: entry,
			},
		},
		{
			CorrelationID: boundGatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: boundEntry,
			},
		},
	}

	for _, route := range routes {
		// Add the watch event for the route.
		watch := UpdateEvent{
			CorrelationID: routeConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: route,
			},
		}
		baseEvents = append(baseEvents, watch)

		// Add the watch event for the discovery chain.
		entries := []structs.ConfigEntry{
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": route.GetProtocol(),
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "api-gateway",
			},
		}

		set := configentry.NewDiscoveryChainSet()
		set.AddEntries(entries...)

		// Add a discovery chain watch event for each service.
		for _, serviceName := range route.GetServiceNames() {
			discoChain := UpdateEvent{
				CorrelationID: fmt.Sprintf("discovery-chain:%s", UpstreamIDString("", "", serviceName.Name, &serviceName.EnterpriseMeta, "")),
				Result: &structs.DiscoveryChainResponse{
					Chain: discoverychain.TestCompileConfigEntries(t, serviceName.Name, "default", "default", "dc1", connect.TestClusterID+".consul", nil, set),
				},
			}
			baseEvents = append(baseEvents, discoChain)
		}
	}

	for _, certificate := range certificates {
		inlineCertificate := certificate
		baseEvents = append(baseEvents, UpdateEvent{
			CorrelationID: inlineCertificateConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: &inlineCertificate,
			},
		})
	}

	upstreams := structs.TestUpstreams(t, false)

	baseEvents = testSpliceEvents(baseEvents, setupTestVariationConfigEntriesAndSnapshot(
		t, variation, false, upstreams, additionalEntries...,
	))

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:            structs.ServiceKindAPIGateway,
		Service:         "api-gateway",
		Address:         "1.2.3.4",
		Meta:            nil,
		TaggedAddresses: nil,
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}

// TestConfigSnapshotAPIGateway_NilConfigEntry is used to test when
// the update event for the config entry returns nil
// since this always happens on the first watch if it doesn't exist.
func TestConfigSnapshotAPIGateway_NilConfigEntry(
	t testing.T,
) *ConfigSnapshot {
	roots, _ := TestCerts(t)

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: apiGatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil, // The first watch on a config entry will return nil if the config entry doesn't exist.
			},
		},
		{
			CorrelationID: boundGatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil, // The first watch on a config entry will return nil if the config entry doesn't exist.
			},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:            structs.ServiceKindAPIGateway,
		Service:         "api-gateway",
		Address:         "1.2.3.4",
		Meta:            nil,
		TaggedAddresses: nil,
	}, nil, nil, testSpliceEvents(baseEvents, nil))
}
