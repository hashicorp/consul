// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

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

	// Mirror what newGatewayMeta() does in the controller: copy Port/Protocol/
	// Hostname and other config fields from APIGatewayListener into the
	// corresponding BoundAPIGatewayListener. This keeps test fixtures that only
	// set entry.Listeners (and leave the bound fields blank) working correctly
	// after we removed snap.APIGateway.Listeners and made BoundListeners the
	// single source of truth.
	if len(entry.Listeners) > 0 && len(boundEntry.Listeners) > 0 {
		listenersByName := make(map[string]structs.APIGatewayListener, len(entry.Listeners))
		for _, l := range entry.Listeners {
			listenersByName[l.Name] = l
		}
		for i, bl := range boundEntry.Listeners {
			if l, ok := listenersByName[bl.Name]; ok {
				if bl.Port == 0 {
					boundEntry.Listeners[i].Port = l.Port
				}
				if bl.Protocol == "" {
					boundEntry.Listeners[i].Protocol = l.Protocol
				}
				if bl.Hostname == "" {
					boundEntry.Listeners[i].Hostname = l.Hostname
				}
				if bl.TLS.IsEmpty() {
					boundEntry.Listeners[i].TLS = l.TLS
				}
				if bl.Override == nil {
					boundEntry.Listeners[i].Override = l.Override
				}
				if bl.Default == nil {
					boundEntry.Listeners[i].Default = l.Default
				}
				if bl.MaxRequestHeadersKB == nil {
					boundEntry.Listeners[i].MaxRequestHeadersKB = l.MaxRequestHeadersKB
				}
			}
		}
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
				CorrelationID: fmt.Sprintf("discovery-chain:%s", UpstreamIDString("", "", serviceName.Name, &serviceName.EnterpriseMeta, "", "")),
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
