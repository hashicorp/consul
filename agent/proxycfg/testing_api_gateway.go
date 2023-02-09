package proxycfg

import (
	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/structs"
)

func TestConfigSnapshotAPIGateway(
	t testing.T,
	protocol string,
	variation string,
	nsFn func(ns *structs.NodeService),
	configFn func(entry *structs.APIGatewayConfigEntry, boundEntry *structs.BoundAPIGatewayConfigEntry),
	extraUpdates []UpdateEvent,
	additionalEntries ...structs.ConfigEntry,
) *ConfigSnapshot {
	roots, _ := TestCerts(t)

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
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: entry,
			},
		},
		{
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: boundEntry,
			},
		},
	}

	upstreams := structs.TestUpstreams(t)
	upstreams = structs.Upstreams{upstreams[0]} // just keep 'db'

	baseEvents = testSpliceEvents(baseEvents, setupTestVariationConfigEntriesAndSnapshot(
		t, variation, upstreams, additionalEntries...,
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
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil, // The first watch on a config entry will return nil if the config entry doesn't exist.
			},
		},
		{
			CorrelationID: gatewayConfigWatchID,
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
