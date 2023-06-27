// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestStore_ConfigEntry(t *testing.T) {
	s := testConfigStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, expected))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), idx)
	require.Equal(t, expected, config)

	// Update
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "bar",
		},
	}
	require.NoError(t, s.EnsureConfigEntry(1, updated))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(1), idx)
	require.Equal(t, updated, config)

	// Delete
	require.NoError(t, s.DeleteConfigEntry(2, structs.ProxyDefaults, "global", nil))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Nil(t, config)

	// Set up a watch.
	serviceConf := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	require.NoError(t, s.EnsureConfigEntry(3, serviceConf))

	ws := memdb.NewWatchSet()
	_, _, err = s.ConfigEntry(ws, structs.ServiceDefaults, "foo", nil)
	require.NoError(t, err)

	// Make an unrelated modification and make sure the watch doesn't fire.
	require.NoError(t, s.EnsureConfigEntry(4, updated))
	require.False(t, watchFired(ws))

	// Update the watched config and make sure it fires.
	serviceConf.Protocol = "http"
	require.NoError(t, s.EnsureConfigEntry(5, serviceConf))
	require.True(t, watchFired(ws))

}
func TestStore_ConfigEntryCAS(t *testing.T) {
	s := testConfigStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(1, expected))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(1), idx)
	require.Equal(t, expected, config)

	// Update with invalid index
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "bar",
		},
	}
	ok, err := s.EnsureConfigEntryCAS(2, 99, updated)
	require.False(t, ok)
	require.NoError(t, err)

	// Entry should not be changed
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(1), idx)
	require.Equal(t, expected, config)

	// Update with a valid index
	ok, err = s.EnsureConfigEntryCAS(2, 1, updated)
	require.True(t, ok)
	require.NoError(t, err)

	// Entry should be updated
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Equal(t, updated, config)
}

func TestStore_ConfigEntry_DeleteCAS(t *testing.T) {
	s := testConfigStateStore(t)

	entry := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Attempt to delete the entry before it exists.
	ok, err := s.DeleteConfigEntryCAS(1, 0, entry)
	require.NoError(t, err)
	require.False(t, ok)

	// Create the entry.
	require.NoError(t, s.EnsureConfigEntry(1, entry))

	// Attempt to delete with an invalid index.
	ok, err = s.DeleteConfigEntryCAS(2, 99, entry)
	require.NoError(t, err)
	require.False(t, ok)

	// Entry should not be deleted.
	_, config, err := s.ConfigEntry(nil, entry.Kind, entry.Name, nil)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Attempt to delete with a valid index.
	ok, err = s.DeleteConfigEntryCAS(2, 1, entry)
	require.NoError(t, err)
	require.True(t, ok)

	// Entry should be deleted.
	_, config, err = s.ConfigEntry(nil, entry.Kind, entry.Name, nil)
	require.NoError(t, err)
	require.Nil(t, config)
}

func TestStore_ConfigEntry_UpdateOver(t *testing.T) {
	// This test uses ServiceIntentions because they are the only
	// kind that implements UpdateOver() at this time.

	s := testConfigStateStore(t)

	var (
		idA = testUUID()
		idB = testUUID()

		loc   = time.FixedZone("UTC-8", -8*60*60)
		timeA = time.Date(1955, 11, 5, 6, 15, 0, 0, loc)
		timeB = time.Date(1985, 10, 26, 1, 35, 0, 0, loc)
	)
	require.NotEqual(t, idA, idB)

	initial := &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "api",
		Sources: []*structs.SourceIntention{
			{
				LegacyID:         idA,
				Name:             "web",
				Action:           structs.IntentionActionAllow,
				LegacyCreateTime: &timeA,
				LegacyUpdateTime: &timeA,
			},
		},
	}

	// Create
	nextIndex := uint64(1)
	require.NoError(t, s.EnsureConfigEntry(nextIndex, initial.Clone()))

	idx, raw, err := s.ConfigEntry(nil, structs.ServiceIntentions, "api", nil)
	require.NoError(t, err)
	require.Equal(t, nextIndex, idx)

	got, ok := raw.(*structs.ServiceIntentionsConfigEntry)
	require.True(t, ok)
	initial.RaftIndex = got.RaftIndex
	require.Equal(t, initial, got)

	t.Run("update and fail change legacyID", func(t *testing.T) {
		// Update
		updated := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "api",
			Sources: []*structs.SourceIntention{
				{
					LegacyID:         idB,
					Name:             "web",
					Action:           structs.IntentionActionDeny,
					LegacyCreateTime: &timeB,
					LegacyUpdateTime: &timeB,
				},
			},
		}

		nextIndex++
		err := s.EnsureConfigEntry(nextIndex, updated.Clone())
		testutil.RequireErrorContains(t, err, "cannot set this field to a different value")
	})

	t.Run("update and do not update create time", func(t *testing.T) {
		// Update
		updated := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "api",
			Sources: []*structs.SourceIntention{
				{
					LegacyID:         idA,
					Name:             "web",
					Action:           structs.IntentionActionDeny,
					LegacyCreateTime: &timeB,
					LegacyUpdateTime: &timeB,
				},
			},
		}

		nextIndex++
		require.NoError(t, s.EnsureConfigEntry(nextIndex, updated.Clone()))

		// check
		idx, raw, err = s.ConfigEntry(nil, structs.ServiceIntentions, "api", nil)
		require.NoError(t, err)
		require.Equal(t, nextIndex, idx)

		got, ok = raw.(*structs.ServiceIntentionsConfigEntry)
		require.True(t, ok)
		updated.RaftIndex = got.RaftIndex
		updated.Sources[0].LegacyCreateTime = &timeA // UpdateOver will not replace this
		require.Equal(t, updated, got)
	})
}

func TestStore_ConfigEntries(t *testing.T) {
	s := testConfigStateStore(t)

	// Create some config entries.
	entry1 := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "test1",
	}
	entry2 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "test2",
	}
	entry3 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "test3",
	}

	require.NoError(t, s.EnsureConfigEntry(0, entry1))
	require.NoError(t, s.EnsureConfigEntry(1, entry2))
	require.NoError(t, s.EnsureConfigEntry(2, entry3))

	// Get all entries
	idx, entries, err := s.ConfigEntries(nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Equal(t, []structs.ConfigEntry{entry1, entry2, entry3}, entries)

	// Get all proxy entries
	idx, entries, err = s.ConfigEntriesByKind(nil, structs.ProxyDefaults, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Equal(t, []structs.ConfigEntry{entry1}, entries)

	// Get all service entries
	ws := memdb.NewWatchSet()
	idx, entries, err = s.ConfigEntriesByKind(ws, structs.ServiceDefaults, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Equal(t, []structs.ConfigEntry{entry2, entry3}, entries)

	// Watch should not have fired
	require.False(t, watchFired(ws))

	// Now make an update and make sure the watch fires.
	require.NoError(t, s.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "test2",
		Protocol: "tcp",
	}))
	require.True(t, watchFired(ws))

}

func TestStore_ServiceDefaults_Kind_Destination(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "dest1",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	destination := &structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "dest1",
		Destination: &structs.DestinationConfig{},
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, destination))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindDestination)

	_, kindServices, err := s.ServiceNamesOfKind(ws, structs.ServiceKindDestination)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindDestination)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteConfigEntry(6, structs.ServiceDefaults, destination.Name, &destination.EnterpriseMeta))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, structs.GatewayServiceKindUnknown, gatewayServices[0].ServiceKind)

	_, kindServices, err = s.ServiceNamesOfKind(ws, structs.ServiceKindDestination)
	require.NoError(t, err)
	require.Len(t, kindServices, 0)

}

func TestStore_ServiceDefaults_Kind_NotDestination(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "dest1",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	destination := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "dest1",
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, destination))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.False(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteConfigEntry(6, structs.ServiceDefaults, destination.Name, &destination.EnterpriseMeta))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.False(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

}

func TestStore_Service_TerminatingGateway_Kind_Service_Destination(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "web",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	service := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		Service: "web",
	}
	destination := &structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "web",
		Destination: &structs.DestinationConfig{},
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	// Create
	require.NoError(t, s.EnsureNode(0, &structs.Node{Node: "node1"}))
	require.NoError(t, s.EnsureService(0, "node1", service))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindService)

	_, kindServices, err := s.ServiceNamesOfKind(ws, structs.ServiceKindTypical)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindTypical)

	require.NoError(t, s.EnsureConfigEntry(0, destination))

	_, gatewayServices, err = s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindService)

	_, kindServices, err = s.ServiceNamesOfKind(ws, structs.ServiceKindTypical)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindTypical)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteService(6, "node1", service.ID, &service.EnterpriseMeta, ""))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindDestination)

	_, kindServices, err = s.ServiceNamesOfKind(ws, structs.ServiceKindDestination)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindDestination)

}

func TestStore_Service_TerminatingGateway_Kind_Destination_Service(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "web",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	service := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		Service: "web",
	}
	destination := &structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "web",
		Destination: &structs.DestinationConfig{},
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, destination))

	_, gatewayServices, err = s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindDestination)

	_, kindServices, err := s.ServiceNamesOfKind(ws, structs.ServiceKindDestination)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindDestination)

	require.NoError(t, s.EnsureNode(0, &structs.Node{Node: "node1"}))
	require.NoError(t, s.EnsureService(0, "node1", service))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindService)

	_, kindServices, err = s.ServiceNamesOfKind(ws, structs.ServiceKindTypical)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindTypical)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteService(6, "node1", service.ID, &service.EnterpriseMeta, ""))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindDestination)

	_, kindServices, err = s.ServiceNamesOfKind(ws, structs.ServiceKindDestination)
	require.NoError(t, err)
	require.Len(t, kindServices, 1)
	require.Equal(t, kindServices[0].Kind, structs.ServiceKindDestination)

}

func TestStore_Service_TerminatingGateway_Kind_Service(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "web",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	service := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		Service: "web",
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	// Create
	require.NoError(t, s.EnsureNode(0, &structs.Node{Node: "node1"}))
	require.NoError(t, s.EnsureService(0, "node1", service))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindService)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteService(6, "node1", service.ID, &service.EnterpriseMeta, ""))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindUnknown)

}

func TestStore_ServiceDefaults_Kind_Destination_Wildcard(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "*",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	destination := &structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "dest1",
		Destination: &structs.DestinationConfig{},
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 0)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	// Create
	require.NoError(t, s.EnsureConfigEntry(0, destination))
	require.NoError(t, err)

	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindDestination)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteConfigEntry(6, structs.ServiceDefaults, destination.Name, &destination.EnterpriseMeta))

	// Watch is fired because we deleted the destination - now the mapping should be gone.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 0)

	t.Run("delete service instance before config entry", func(t *testing.T) {
		// Set up a service with both a real instance and destination from a config entry.
		require.NoError(t, s.EnsureNode(7, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
		require.NoError(t, s.EnsureService(8, "foo", &structs.NodeService{ID: "dest2", Service: "dest2", Tags: nil, Address: "", Port: 5000}))

		ws = memdb.NewWatchSet()
		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Len(t, gatewayServices, 1)
		require.Equal(t, structs.GatewayServiceKindService, gatewayServices[0].ServiceKind)

		// Register destination; shouldn't change the gateway mapping.
		destination2 := &structs.ServiceConfigEntry{
			Kind:        structs.ServiceDefaults,
			Name:        "dest2",
			Destination: &structs.DestinationConfig{},
		}
		require.NoError(t, s.EnsureConfigEntry(9, destination2))
		require.False(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Len(t, gatewayServices, 1)
		expected := structs.GatewayServices{
			{
				Service:      structs.NewServiceName("dest2", nil),
				Gateway:      structs.NewServiceName("Gtwy1", nil),
				ServiceKind:  structs.GatewayServiceKindService,
				GatewayKind:  structs.ServiceKindTerminatingGateway,
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		}
		require.Equal(t, expected, gatewayServices)

		// Delete the service, mapping should still exist.
		require.NoError(t, s.DeleteService(10, "foo", "dest2", nil, ""))
		require.False(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Len(t, gatewayServices, 1)
		require.Equal(t, expected, gatewayServices)

		// Delete the config entry, mapping should be gone.
		require.NoError(t, s.DeleteConfigEntry(11, structs.ServiceDefaults, "dest2", &destination.EnterpriseMeta))
		require.True(t, watchFired(ws))

		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Empty(t, gatewayServices)
	})

	t.Run("delete config entry before service instance", func(t *testing.T) {
		// Set up a service with both a real instance and destination from a config entry.
		destination2 := &structs.ServiceConfigEntry{
			Kind:        structs.ServiceDefaults,
			Name:        "dest2",
			Destination: &structs.DestinationConfig{},
		}
		require.NoError(t, s.EnsureConfigEntry(7, destination2))

		ws = memdb.NewWatchSet()
		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Len(t, gatewayServices, 1)
		expected := structs.GatewayServices{
			{
				Service:      structs.NewServiceName("dest2", nil),
				Gateway:      structs.NewServiceName("Gtwy1", nil),
				ServiceKind:  structs.GatewayServiceKindDestination,
				GatewayKind:  structs.ServiceKindTerminatingGateway,
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 7,
					ModifyIndex: 7,
				},
			},
		}
		require.Equal(t, expected, gatewayServices)

		// Register service, only ServiceKind should have changed on the gateway mapping.
		require.NoError(t, s.EnsureNode(8, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
		require.NoError(t, s.EnsureService(9, "foo", &structs.NodeService{ID: "dest2", Service: "dest2", Tags: nil, Address: "", Port: 5000}))
		require.True(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Len(t, gatewayServices, 1)
		expected = structs.GatewayServices{
			{
				Service:      structs.NewServiceName("dest2", nil),
				Gateway:      structs.NewServiceName("Gtwy1", nil),
				ServiceKind:  structs.GatewayServiceKindService,
				GatewayKind:  structs.ServiceKindTerminatingGateway,
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 7,
					ModifyIndex: 9,
				},
			},
		}
		require.Equal(t, expected, gatewayServices)

		// Delete the config entry, mapping should still exist.
		require.NoError(t, s.DeleteConfigEntry(10, structs.ServiceDefaults, "dest2", &destination.EnterpriseMeta))
		require.False(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Len(t, gatewayServices, 1)
		require.Equal(t, expected, gatewayServices)

		// Delete the service, mapping should be gone.
		require.NoError(t, s.DeleteService(11, "foo", "dest2", nil, ""))
		require.True(t, watchFired(ws))

		_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
		require.NoError(t, err)
		require.Empty(t, gatewayServices)
	})
}

func TestStore_Service_TerminatingGateway_Kind_Service_Wildcard(t *testing.T) {
	s := testConfigStateStore(t)

	Gtwy := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "Gtwy1",
		Services: []structs.LinkedService{
			{
				Name: "*",
			},
		},
	}

	// Create
	require.NoError(t, s.EnsureConfigEntry(0, Gtwy))

	service := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		Service: "web",
	}

	_, gatewayServices, err := s.GatewayServices(nil, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 0)

	ws := memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	// Create
	require.NoError(t, s.EnsureNode(0, &structs.Node{Node: "node1"}))
	require.NoError(t, s.EnsureService(0, "node1", service))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 1)
	require.Equal(t, gatewayServices[0].ServiceKind, structs.GatewayServiceKindService)

	ws = memdb.NewWatchSet()
	_, _, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)

	require.NoError(t, s.DeleteService(6, "node1", service.ID, &service.EnterpriseMeta, ""))

	//Watch is fired because we transitioned to a destination, by default we assume it's not.
	require.True(t, watchFired(ws))

	_, gatewayServices, err = s.GatewayServices(ws, "Gtwy1", nil)
	require.NoError(t, err)
	require.Len(t, gatewayServices, 0)
}

func TestStore_ConfigEntry_GraphValidation(t *testing.T) {
	ensureConfigEntry := func(s *Store, idx uint64, entry structs.ConfigEntry) error {
		if err := entry.Normalize(); err != nil {
			return err
		}
		if err := entry.Validate(); err != nil {
			return err
		}
		return s.EnsureConfigEntry(idx, entry)
	}

	type tcase struct {
		entries        []structs.ConfigEntry
		opAdd          structs.ConfigEntry
		opDelete       configentry.KindName
		expectErr      string
		expectGraphErr bool
	}

	EMPTY_KN := configentry.KindName{}

	run := func(t *testing.T, tc tcase) {
		s := testConfigStateStore(t)
		for _, entry := range tc.entries {
			require.NoError(t, ensureConfigEntry(s, 0, entry))
		}

		nOps := 0
		if tc.opAdd != nil {
			nOps++
		}
		if tc.opDelete != EMPTY_KN {
			nOps++
		}
		require.Equal(t, 1, nOps, "exactly one operation is required")

		var err error
		switch {
		case tc.opAdd != nil:
			err = ensureConfigEntry(s, 0, tc.opAdd)
		case tc.opDelete != EMPTY_KN:
			kn := tc.opDelete
			err = s.DeleteConfigEntry(0, kn.Kind, kn.Name, &kn.EnterpriseMeta)
		default:
			t.Fatal("not possible")
		}

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
		}
	}

	cases := map[string]tcase{
		"splitter fails without default protocol": {
			entries: []structs.ConfigEntry{},
			opAdd: &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "main",
				Splits: []structs.ServiceSplit{
					{Weight: 100},
				},
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"splitter fails with tcp protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			opAdd: &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "main",
				Splits: []structs.ServiceSplit{
					{Weight: 100},
				},
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"splitter works with http protocol": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "tcp", // loses
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "main",
					Protocol:       "http",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
			},
			opAdd: &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "main",
				Splits: []structs.ServiceSplit{
					{Weight: 90, ServiceSubset: "v1"},
					{Weight: 10, ServiceSubset: "v2"},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		"splitter works with http protocol (from proxy-defaults)": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
			},
			opAdd: &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "main",
				Splits: []structs.ServiceSplit{
					{Weight: 90, ServiceSubset: "v1"},
					{Weight: 10, ServiceSubset: "v2"},
				},
			},
		},
		"router fails with tcp protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
				},
			},
			opAdd: &structs.ServiceRouterConfigEntry{
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
							ServiceSubset: "other",
						},
					},
				},
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"router fails without default protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
				},
			},
			opAdd: &structs.ServiceRouterConfigEntry{
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
							ServiceSubset: "other",
						},
					},
				},
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot remove default protocol after splitter created": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, ServiceSubset: "v1"},
						{Weight: 10, ServiceSubset: "v2"},
					},
				},
			},
			opDelete:       configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"cannot remove global default protocol after splitter created": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, ServiceSubset: "v1"},
						{Weight: 10, ServiceSubset: "v2"},
					},
				},
			},
			opDelete:       configentry.NewKindName(structs.ProxyDefaults, structs.ProxyConfigGlobal, nil),
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"can remove global default protocol after splitter created if service default overrides it": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, ServiceSubset: "v1"},
						{Weight: 10, ServiceSubset: "v2"},
					},
				},
			},
			opDelete: configentry.NewKindName(structs.ProxyDefaults, structs.ProxyConfigGlobal, nil),
		},
		"cannot change to tcp protocol after splitter created": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 90, ServiceSubset: "v1"},
						{Weight: 10, ServiceSubset: "v2"},
					},
				},
			},
			opAdd: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "main",
				Protocol: "tcp",
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"cannot remove default protocol after router created": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
				},
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
								ServiceSubset: "other",
							},
						},
					},
				},
			},
			opDelete:       configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		"cannot change to tcp protocol after router created": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"other": {
							Filter: "Service.Meta.version == other",
						},
					},
				},
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
								ServiceSubset: "other",
							},
						},
					},
				},
			},
			opAdd: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "main",
				Protocol: "tcp",
			},
			expectErr:      "does not permit advanced routing or splitting behavior",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot split to a service using tcp": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
			},
			opAdd: &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "main",
				Splits: []structs.ServiceSplit{
					{Weight: 90},
					{Weight: 10, Service: "other"},
				},
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		"cannot route to a service using tcp": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
			},
			opAdd: &structs.ServiceRouterConfigEntry{
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
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot failover to a service using a different protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Service: "other",
					},
				},
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		"cannot redirect to a service using a different protocol": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "tcp",
				},
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service: "other",
				},
			},
			expectErr:      "uses inconsistent protocols",
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"redirect to a subset that does exist is fine": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "other",
					ConnectTimeout: 33 * time.Second,
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
					},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "other",
					ServiceSubset: "v1",
				},
			},
		},
		"cannot redirect to a subset that does not exist": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "other",
					ConnectTimeout: 33 * time.Second,
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service:       "other",
					ServiceSubset: "v1",
				},
			},
			expectErr:      `does not have a subset named "v1"`,
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot introduce circular resolver redirect": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "other",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "main",
					},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service: "other",
				},
			},
			expectErr:      `detected circular resolver redirect`,
			expectGraphErr: true,
		},
		"cannot introduce circular split": {
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: "service-splitter",
					Name: "other",
					Splits: []structs.ServiceSplit{
						{Weight: 100, Service: "main"},
					},
				},
			},
			opAdd: &structs.ServiceSplitterConfigEntry{
				Kind: "service-splitter",
				Name: "main",
				Splits: []structs.ServiceSplit{
					{Weight: 100, Service: "other"},
				},
			},
			expectErr:      `detected circular reference`,
			expectGraphErr: true,
		},
		/////////////////////////////////////////////////
		"cannot peer export cross-dc redirect": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: "service-resolver",
					Name: "main",
					Redirect: &structs.ServiceResolverRedirect{
						Datacenter: "dc3",
					},
				},
			},
			opAdd: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{{
					Name:      "main",
					Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
				}},
			},
			expectErr: `contains cross-datacenter resolver redirect`,
		},
		"cannot peer export cross-dc redirect via wildcard": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: "service-resolver",
					Name: "main",
					Redirect: &structs.ServiceResolverRedirect{
						Datacenter: "dc3",
					},
				},
			},
			opAdd: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{{
					Name:      "*",
					Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
				}},
			},
			expectErr: `contains cross-datacenter resolver redirect`,
		},
		"cannot peer export cross-dc failover": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: "service-resolver",
					Name: "main",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Datacenters: []string{"dc3"},
						},
					},
				},
			},
			opAdd: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{{
					Name:      "main",
					Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
				}},
			},
			expectErr: `contains cross-datacenter failover`,
		},
		"cannot peer export cross-dc failover via wildcard": {
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: "service-resolver",
					Name: "main",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Datacenters: []string{"dc3"},
						},
					},
				},
			},
			opAdd: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{{
					Name:      "*",
					Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
				}},
			},
			expectErr: `contains cross-datacenter failover`,
		},
		"cannot redirect a peer exported tcp service": {
			entries: []structs.ConfigEntry{
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service: "other",
				},
			},
			expectErr: `cannot introduce new discovery chain targets like`,
		},
		"can redirect a peer exported http service to another service": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service: "other",
				},
			},
		},
		"cannot redirect a peer exported http service to another peer service": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service: "other",
					Peer:    "something",
				},
			},
			expectErr: `contains cross-peer resolver redirect`,
		},
		"cannot redirect a peer exported http service to a service in another datacenter": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Redirect: &structs.ServiceResolverRedirect{
					Service:    "other",
					Datacenter: "dc12",
				},
			},
			expectErr: `contains cross-datacenter resolver redirect`,
		},
		"can failover a peer exported http service to another service": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Service: "other",
					},
				},
			},
		},
		"can failover a peer exported http service to another peer service": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Targets: []structs.ServiceResolverFailoverTarget{
							{
								Service: "other",
								Peer:    "some-peer",
							},
						},
					},
				},
			},
		},
		"can't failover a peer exported http service to another service in a different datacenter": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Service:     "other",
						Datacenters: []string{"dc12"},
					},
				},
			},
			expectErr: `contains cross-datacenter failover`,
		},
		"can't failover a peer exported http service to another service in a different datacenter using targets": {
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "other",
					Protocol: "http",
				},
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Targets: []structs.ServiceResolverFailoverTarget{
							{
								Service:    "other",
								Datacenter: "dc12",
							},
						},
					},
				},
			},
			expectErr: `contains cross-datacenter failover`,
		},
		"can failover a peer exported tcp service": {
			entries: []structs.ConfigEntry{
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Targets: []structs.ServiceResolverFailoverTarget{
							{
								Service: "other",
							},
							{
								Service: "other",
								Peer:    "cluster-01",
							},
						},
					},
				},
			},
		},
		"can failover a peer exported tcp service from a redirect": {
			entries: []structs.ConfigEntry{
				&structs.ExportedServicesConfigEntry{
					Name: "default",
					Services: []structs.ExportedService{{
						Name:      "main",
						Consumers: []structs.ServiceConsumer{{Peer: "my-peer"}},
					}},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "other",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "main",
					},
				},
			},
			opAdd: &structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "main",
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Targets: []structs.ServiceResolverFailoverTarget{
							{
								Service: "another",
							},
							{
								Service: "other",
								Peer:    "cluster-01",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_ReadDiscoveryChainConfigEntries_Overrides(t *testing.T) {
	for _, tc := range []struct {
		name           string
		entries        []structs.ConfigEntry
		expectBefore   []configentry.KindName
		overrides      map[configentry.KindName]structs.ConfigEntry
		expectAfter    []configentry.KindName
		expectAfterErr string
		checkAfter     func(t *testing.T, entrySet *configentry.DiscoveryChainSet)
	}{
		{
			name: "mask service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil): nil,
			},
			expectAfter: []configentry.KindName{
				// nothing
			},
		},
		{
			name: "edit service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "tcp",
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil): &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "grpc",
				},
			},
			expectAfter: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *configentry.DiscoveryChainSet) {
				defaults := entrySet.GetService(structs.NewServiceID("main", nil))
				require.NotNil(t, defaults)
				require.Equal(t, "grpc", defaults.Protocol)
			},
		},

		{
			name: "mask service-router",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
				configentry.NewKindName(structs.ServiceRouter, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceRouter, "main", nil): nil,
			},
			expectAfter: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			},
		},
		{
			name: "edit service-router",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {Filter: "Service.Meta.version == v1"},
						"v2": {Filter: "Service.Meta.version == v2"},
						"v3": {Filter: "Service.Meta.version == v3"},
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/admin",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								ServiceSubset: "v2",
							},
						},
					},
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
				configentry.NewKindName(structs.ServiceResolver, "main", nil),
				configentry.NewKindName(structs.ServiceRouter, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceRouter, "main", nil): &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/admin",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								ServiceSubset: "v3",
							},
						},
					},
				},
			},
			expectAfter: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
				configentry.NewKindName(structs.ServiceResolver, "main", nil),
				configentry.NewKindName(structs.ServiceRouter, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *configentry.DiscoveryChainSet) {
				router := entrySet.GetRouter(structs.NewServiceID("main", nil))
				require.NotNil(t, router)
				require.Len(t, router.Routes, 1)

				expect := structs.ServiceRoute{
					Match: &structs.ServiceRouteMatch{
						HTTP: &structs.ServiceRouteHTTPMatch{
							PathExact: "/admin",
						},
					},
					Destination: &structs.ServiceRouteDestination{
						ServiceSubset: "v3",
					},
				}
				require.Equal(t, expect, router.Routes[0])
			},
		},

		{
			name: "mask service-splitter",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 100},
					},
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
				configentry.NewKindName(structs.ServiceSplitter, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceSplitter, "main", nil): nil,
			},
			expectAfter: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
			},
		},
		{
			name: "edit service-splitter",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "main",
					Protocol: "http",
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 100},
					},
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
				configentry.NewKindName(structs.ServiceSplitter, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceSplitter, "main", nil): &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "main",
					Splits: []structs.ServiceSplit{
						{Weight: 85, ServiceSubset: "v1"},
						{Weight: 15, ServiceSubset: "v2"},
					},
				},
			},
			expectAfter: []configentry.KindName{
				configentry.NewKindName(structs.ServiceDefaults, "main", nil),
				configentry.NewKindName(structs.ServiceSplitter, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *configentry.DiscoveryChainSet) {
				splitter := entrySet.GetSplitter(structs.NewServiceID("main", nil))
				require.NotNil(t, splitter)
				require.Len(t, splitter.Splits, 2)

				expect := []structs.ServiceSplit{
					{Weight: 85, ServiceSubset: "v1"},
					{Weight: 15, ServiceSubset: "v2"},
				}
				require.Equal(t, expect, splitter.Splits)
			},
		},

		{
			name: "mask service-resolver",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceResolver, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceResolver, "main", nil): nil,
			},
			expectAfter: []configentry.KindName{
				// nothing
			},
		},
		{
			name: "edit service-resolver",
			entries: []structs.ConfigEntry{
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "main",
				},
			},
			expectBefore: []configentry.KindName{
				configentry.NewKindName(structs.ServiceResolver, "main", nil),
			},
			overrides: map[configentry.KindName]structs.ConfigEntry{
				configentry.NewKindName(structs.ServiceResolver, "main", nil): &structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "main",
					ConnectTimeout: 33 * time.Second,
				},
			},
			expectAfter: []configentry.KindName{
				configentry.NewKindName(structs.ServiceResolver, "main", nil),
			},
			checkAfter: func(t *testing.T, entrySet *configentry.DiscoveryChainSet) {
				resolver := entrySet.GetResolver(structs.NewServiceID("main", nil))
				require.NotNil(t, resolver)
				require.Equal(t, 33*time.Second, resolver.ConnectTimeout)
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			s := testConfigStateStore(t)
			for _, entry := range tc.entries {
				require.NoError(t, s.EnsureConfigEntry(0, entry))
			}

			t.Run("without override", func(t *testing.T) {
				_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", nil, nil)
				require.NoError(t, err)
				got := entrySetToKindNames(entrySet)
				require.ElementsMatch(t, tc.expectBefore, got)
			})

			t.Run("with override", func(t *testing.T) {
				_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", tc.overrides, nil)

				if tc.expectAfterErr != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), tc.expectAfterErr)
				} else {
					require.NoError(t, err)
					got := entrySetToKindNames(entrySet)
					require.ElementsMatch(t, tc.expectAfter, got)

					if tc.checkAfter != nil {
						tc.checkAfter(t, entrySet)
					}
				}
			})
		})
	}
}

func entrySetToKindNames(entrySet *configentry.DiscoveryChainSet) []configentry.KindName {
	var out []configentry.KindName
	for _, entry := range entrySet.Routers {
		out = append(out, configentry.NewKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.Splitters {
		out = append(out, configentry.NewKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.Resolvers {
		out = append(out, configentry.NewKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.Services {
		out = append(out, configentry.NewKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	for _, entry := range entrySet.ProxyDefaults {
		out = append(out, configentry.NewKindName(
			entry.Kind,
			entry.Name,
			&entry.EnterpriseMeta,
		))
	}
	return out
}

func TestStore_ReadDiscoveryChainConfigEntries_SubsetSplit(t *testing.T) {
	s := testConfigStateStore(t)

	entries := []structs.ConfigEntry{
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "main",
			Protocol: "http",
		},
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": {
					Filter: "Service.Meta.version == v1",
				},
				"v2": {
					Filter: "Service.Meta.version == v2",
				},
			},
		},
		&structs.ServiceSplitterConfigEntry{
			Kind: structs.ServiceSplitter,
			Name: "main",
			Splits: []structs.ServiceSplit{
				{Weight: 90, ServiceSubset: "v1"},
				{Weight: 10, ServiceSubset: "v2"},
			},
		},
	}

	for _, entry := range entries {
		require.NoError(t, s.EnsureConfigEntry(0, entry))
	}

	_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", nil, nil)
	require.NoError(t, err)

	require.Len(t, entrySet.Routers, 0)
	require.Len(t, entrySet.Splitters, 1)
	require.Len(t, entrySet.Resolvers, 1)
	require.Len(t, entrySet.Services, 1)
}

func TestStore_ReadDiscoveryChainConfigEntries_FetchPeers(t *testing.T) {
	s := testConfigStateStore(t)

	entries := []structs.ConfigEntry{
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "main",
			Protocol: "http",
		},
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "main",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {
					Targets: []structs.ServiceResolverFailoverTarget{
						{Peer: "cluster-01"},
						{Peer: "cluster-02"}, // Non-existant
					},
				},
			},
		},
	}

	for _, entry := range entries {
		require.NoError(t, s.EnsureConfigEntry(0, entry))
	}

	cluster01Peering := &pbpeering.Peering{
		ID:   testFooPeerID,
		Name: "cluster-01",
	}
	err := s.PeeringWrite(0, &pbpeering.PeeringWriteRequest{Peering: cluster01Peering})
	require.NoError(t, err)

	_, entrySet, err := s.readDiscoveryChainConfigEntries(nil, "main", nil, nil)
	require.NoError(t, err)

	require.Len(t, entrySet.Routers, 0)
	require.Len(t, entrySet.Splitters, 0)
	require.Len(t, entrySet.Resolvers, 1)
	require.Len(t, entrySet.Services, 1)
	prototest.AssertDeepEqual(t, entrySet.Peers, map[string]*pbpeering.Peering{
		"cluster-01": cluster01Peering,
		"cluster-02": nil,
	})
}

// TODO(rb): add ServiceIntentions tests

func TestStore_ValidateGatewayNamesCannotBeShared(t *testing.T) {
	s := testConfigStateStore(t)

	ingress := &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "gateway",
	}
	require.NoError(t, s.EnsureConfigEntry(0, ingress))

	terminating := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "gateway",
	}
	// Cannot have 2 gateways with same service name
	require.Error(t, s.EnsureConfigEntry(1, terminating))

	ingress = &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "gateway",
		Listeners: []structs.IngressListener{
			{Port: 8080},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(2, ingress))
	require.NoError(t, s.DeleteConfigEntry(3, structs.IngressGateway, "gateway", nil))

	// Adding the terminating gateway with same name should now work
	require.NoError(t, s.EnsureConfigEntry(4, terminating))

	// Cannot have 2 gateways with same service name
	require.Error(t, s.EnsureConfigEntry(5, ingress))
}

func TestStore_ValidateIngressGatewayErrorOnMismatchedProtocols(t *testing.T) {
	newIngress := func(protocol, name string) *structs.IngressGatewayConfigEntry {
		return &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "gateway",
			Listeners: []structs.IngressListener{
				{
					Port:     8080,
					Protocol: protocol,
					Services: []structs.IngressService{
						{Name: name},
					},
				},
			},
		}
	}

	t.Run("http ingress fails with http upstream later changed to tcp", func(t *testing.T) {
		s := testConfigStateStore(t)

		// First set the target service as http
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(0, expected))

		// Next configure http ingress to route to the http service
		require.NoError(t, s.EnsureConfigEntry(1, newIngress("http", "web")))

		t.Run("via modification", func(t *testing.T) {
			// Now redefine the target service as tcp
			expected = &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "web",
				Protocol: "tcp",
			}

			err := s.EnsureConfigEntry(2, expected)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "tcp"`)
		})
		t.Run("via deletion", func(t *testing.T) {
			// This will fall back to the default tcp.
			err := s.DeleteConfigEntry(2, structs.ServiceDefaults, "web", nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "tcp"`)
		})
	})

	t.Run("tcp ingress ok with tcp upstream (defaulted) later changed to http", func(t *testing.T) {
		s := testConfigStateStore(t)

		// First configure tcp ingress to route to a defaulted tcp service
		require.NoError(t, s.EnsureConfigEntry(0, newIngress("tcp", "web")))

		// Now redefine the target service as http
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(1, expected))
	})

	t.Run("tcp ingress fails with tcp upstream (defaulted) later changed to http", func(t *testing.T) {
		s := testConfigStateStore(t)

		// First configure tcp ingress to route to a defaulted tcp service
		require.NoError(t, s.EnsureConfigEntry(0, newIngress("tcp", "web")))

		// Now redefine the target service as http
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(1, expected))

		t.Run("and a router defined", func(t *testing.T) {
			// This part should fail.
			expected2 := &structs.ServiceRouterConfigEntry{
				Kind: structs.ServiceRouter,
				Name: "web",
			}
			err := s.EnsureConfigEntry(2, expected2)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "http"`)
		})

		t.Run("and a splitter defined", func(t *testing.T) {
			// This part should fail.
			expected2 := &structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "web",
				Splits: []structs.ServiceSplit{
					{Weight: 100},
				},
			}
			err := s.EnsureConfigEntry(2, expected2)
			require.Error(t, err)
			require.Contains(t, err.Error(), `has protocol "http"`)
		})
	})

	t.Run("http ingress fails with tcp upstream (defaulted)", func(t *testing.T) {
		s := testConfigStateStore(t)
		err := s.EnsureConfigEntry(0, newIngress("http", "web"))
		require.Error(t, err)
		require.Contains(t, err.Error(), `has protocol "tcp"`)
	})

	t.Run("http ingress fails with http2 upstream (via proxy-defaults)", func(t *testing.T) {
		s := testConfigStateStore(t)
		expected := &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: "global",
			Config: map[string]interface{}{
				"protocol": "http2",
			},
		}
		require.NoError(t, s.EnsureConfigEntry(0, expected))

		err := s.EnsureConfigEntry(1, newIngress("http", "web"))
		require.Error(t, err)
		require.Contains(t, err.Error(), `has protocol "http2"`)
	})

	t.Run("http ingress fails with grpc upstream (via service-defaults)", func(t *testing.T) {
		s := testConfigStateStore(t)
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "grpc",
		}
		require.NoError(t, s.EnsureConfigEntry(1, expected))
		err := s.EnsureConfigEntry(2, newIngress("http", "web"))
		require.Error(t, err)
		require.Contains(t, err.Error(), `has protocol "grpc"`)
	})

	t.Run("http ingress ok with http upstream (via service-defaults)", func(t *testing.T) {
		s := testConfigStateStore(t)
		expected := &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}
		require.NoError(t, s.EnsureConfigEntry(2, expected))
		require.NoError(t, s.EnsureConfigEntry(3, newIngress("http", "web")))
	})

	t.Run("http ingress ignores wildcard specifier", func(t *testing.T) {
		s := testConfigStateStore(t)
		require.NoError(t, s.EnsureConfigEntry(4, newIngress("http", "*")))
	})

	t.Run("deleting ingress config entry ok", func(t *testing.T) {
		s := testConfigStateStore(t)
		require.NoError(t, s.EnsureConfigEntry(1, newIngress("tcp", "web")))
		require.NoError(t, s.DeleteConfigEntry(5, structs.IngressGateway, "gateway", nil))
	})
}

func TestSourcesForTarget(t *testing.T) {
	defaultMeta := *structs.DefaultEnterpriseMetaInDefaultPartition()

	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}
	tt := []struct {
		name    string
		entries []structs.ConfigEntry
		expect  expect
	}{
		{
			name:    "no relevant config entries",
			entries: []structs.ConfigEntry{},
			expect: expect{
				idx: 1,
				names: []structs.ServiceName{
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from route match",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "web",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/sink",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "sink",
							},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from failover",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Service:     "sink",
							Datacenters: []string{"dc2", "dc3"},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from splitter",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "web",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "web"},
						{Weight: 10, Service: "sink"},
					},
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "chained route redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "source",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/route",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "routed",
							},
						},
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "routed",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 3,
				names: []structs.ServiceName{
					{Name: "source", EnterpriseMeta: defaultMeta},
					{Name: "routed", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "kitchen sink with multiple services referencing sink directly",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "routed",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/sink",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "sink",
							},
						},
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "redirected",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "failed-over",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Service:     "sink",
							Datacenters: []string{"dc2", "dc3"},
						},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "split",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "no-op"},
						{Weight: 10, Service: "sink"},
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "unrelated",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "zip"},
						{Weight: 10, Service: "zop"},
					},
				},
			},
			expect: expect{
				idx: 6,
				names: []structs.ServiceName{
					{Name: "split", EnterpriseMeta: defaultMeta},
					{Name: "failed-over", EnterpriseMeta: defaultMeta},
					{Name: "redirected", EnterpriseMeta: defaultMeta},
					{Name: "routed", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			ws := memdb.NewWatchSet()

			ca := &structs.CAConfiguration{
				Provider: "consul",
			}
			err := s.CASetConfig(0, ca)
			require.NoError(t, err)

			var i uint64 = 1
			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, s.EnsureConfigEntry(i, entry))
				i++
			}

			tx := s.db.ReadTxn()
			defer tx.Abort()

			sn := structs.NewServiceName("sink", structs.DefaultEnterpriseMetaInDefaultPartition())
			idx, names, err := s.discoveryChainSourcesTxn(tx, ws, "dc1", sn)
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.names, names)
		})
	}
}

func TestTargetsForSource(t *testing.T) {
	defaultMeta := *structs.DefaultEnterpriseMetaInDefaultPartition()

	type expect struct {
		idx uint64
		ids []structs.ServiceName
	}
	tt := []struct {
		name    string
		entries []structs.ConfigEntry
		expect  expect
	}{
		{
			name: "from route match",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "web",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/sink",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "sink",
							},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from failover",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Failover: map[string]structs.ServiceResolverFailover{
						"*": {
							Service:     "remote-web",
							Datacenters: []string{"dc2", "dc3"},
						},
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "from splitter",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "web",
					Splits: []structs.ServiceSplit{
						{Weight: 90, Service: "web"},
						{Weight: 10, Service: "sink"},
					},
				},
			},
			expect: expect{
				idx: 2,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
		{
			name: "chained route redirect",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "web",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/route",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "routed",
							},
						},
					},
				},
				&structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "routed",
					Redirect: &structs.ServiceResolverRedirect{
						Service: "sink",
					},
				},
			},
			expect: expect{
				idx: 3,
				ids: []structs.ServiceName{
					{Name: "web", EnterpriseMeta: defaultMeta},
					{Name: "sink", EnterpriseMeta: defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			ws := memdb.NewWatchSet()

			ca := &structs.CAConfiguration{
				Provider: "consul",
			}
			err := s.CASetConfig(0, ca)
			require.NoError(t, err)

			var i uint64 = 1
			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, s.EnsureConfigEntry(i, entry))
				i++
			}

			tx := s.db.ReadTxn()
			defer tx.Abort()

			idx, ids, err := s.discoveryChainTargetsTxn(tx, ws, "dc1", "web", nil)
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.ids, ids)
		})
	}
}

func TestStore_ValidateServiceIntentionsErrorOnIncompatibleProtocols(t *testing.T) {
	l7perms := []*structs.IntentionPermission{
		{
			Action: structs.IntentionActionAllow,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/v2/",
			},
		},
	}

	serviceDefaults := func(service, protocol string) *structs.ServiceConfigEntry {
		return &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     service,
			Protocol: protocol,
		}
	}

	proxyDefaults := func(protocol string) *structs.ProxyConfigEntry {
		return &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": protocol,
			},
		}
	}

	type operation struct {
		entry    structs.ConfigEntry
		deletion bool
	}

	type testcase struct {
		ops           []operation
		expectLastErr string
	}

	cases := map[string]testcase{
		"L4 intention cannot upgrade to L7 when tcp": {
			ops: []operation{
				{ // set the target service as tcp
					entry: serviceDefaults("api", "tcp"),
				},
				{ // create an L4 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Action: structs.IntentionActionAllow},
						},
					},
				},
				{ // Should fail if converted to L7
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L4 intention can upgrade to L7 when made http via service-defaults": {
			ops: []operation{
				{ // set the target service as tcp
					entry: serviceDefaults("api", "tcp"),
				},
				{ // create an L4 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Action: structs.IntentionActionAllow},
						},
					},
				},
				{ // set the target service as http
					entry: serviceDefaults("api", "http"),
				},
				{ // Should succeed if converted to L7
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
			},
		},
		"L4 intention can upgrade to L7 when made http via proxy-defaults": {
			ops: []operation{
				{ // set the target service as tcp
					entry: proxyDefaults("tcp"),
				},
				{ // create an L4 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Action: structs.IntentionActionAllow},
						},
					},
				},
				{ // set the target service as http
					entry: proxyDefaults("http"),
				},
				{ // Should succeed if converted to L7
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
			},
		},
		"L7 intention cannot have protocol downgraded to tcp via modification via service-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: serviceDefaults("api", "http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry: serviceDefaults("api", "tcp"),
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L7 intention cannot have protocol downgraded to tcp via modification via proxy-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: proxyDefaults("http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry: proxyDefaults("tcp"),
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L7 intention cannot have protocol downgraded to tcp via deletion of service-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: serviceDefaults("api", "http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry:    serviceDefaults("api", "tcp"),
					deletion: true,
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
		"L7 intention cannot have protocol downgraded to tcp via deletion of proxy-defaults": {
			ops: []operation{
				{ // set the target service as http
					entry: proxyDefaults("http"),
				},
				{ // create an L7 intention
					entry: &structs.ServiceIntentionsConfigEntry{
						Kind: structs.ServiceIntentions,
						Name: "api",
						Sources: []*structs.SourceIntention{
							{Name: "web", Permissions: l7perms},
						},
					},
				},
				{ // setting the target service as tcp should fail
					entry:    proxyDefaults("tcp"),
					deletion: true,
				},
			},
			expectLastErr: `has protocol "tcp"`,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			s := testStateStore(t)

			var nextIndex = uint64(1)

			for i, op := range tc.ops {
				isLast := (i == len(tc.ops)-1)

				var err error
				if op.deletion {
					err = s.DeleteConfigEntry(nextIndex, op.entry.GetKind(), op.entry.GetName(), nil)
				} else {
					err = s.EnsureConfigEntry(nextIndex, op.entry)
				}

				if isLast && tc.expectLastErr != "" {
					testutil.RequireErrorContains(t, err, `has protocol "tcp"`)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestStateStore_ConfigEntry_VirtualIP(t *testing.T) {
	createServiceInstance := func(t *testing.T, s *Store, name string) {
		ns1 := &structs.NodeService{
			ID:      name,
			Service: name,
			Address: "1.1.1.1",
			Port:    1111,
			Connect: structs.ServiceConnect{Native: true},
		}
		require.NoError(t, s.EnsureService(0, "node1", ns1))
	}
	deleteServiceInstance := func(t *testing.T, s *Store, name string) {
		require.NoError(t, s.DeleteService(0, "node1", name, nil, ""))
	}
	createServiceResolver := func(t *testing.T, s *Store, name string) {
		require.NoError(t, s.EnsureConfigEntry(0, &structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: name,
		}))
	}
	createServiceRouter := func(t *testing.T, s *Store, name string) {
		require.NoError(t, s.EnsureConfigEntry(0, &structs.ServiceRouterConfigEntry{
			Kind: structs.ServiceRouter,
			Name: name,
		}))
	}
	createServiceSplitter := func(t *testing.T, s *Store, name string) {
		require.NoError(t, s.EnsureConfigEntry(0, &structs.ServiceSplitterConfigEntry{
			Kind: structs.ServiceSplitter,
			Name: name,
			Splits: []structs.ServiceSplit{
				{Weight: 100},
			},
		}))
	}
	deleteConfigEntry := func(t *testing.T, s *Store, kind, name string) {
		require.NoError(t, s.DeleteConfigEntry(0, kind, name, nil))
	}
	ensureVirtualIP := func(t *testing.T, s *Store, service string, value string) {
		vip, err := s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: service}})
		require.NoError(t, err)
		require.Equal(t, value, vip)
	}

	testVIPStateStore := func(t *testing.T) *Store {
		s := testStateStore(t)
		setVirtualIPFlags(t, s)
		testRegisterNode(t, s, 0, "node1")
		s.EnsureConfigEntry(0, &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
		})
		return s
	}

	cases := []struct {
		kind       string
		createFunc func(*testing.T, *Store, string)
	}{
		{
			kind:       structs.ServiceResolver,
			createFunc: createServiceResolver,
		},
		{
			kind:       structs.ServiceRouter,
			createFunc: createServiceRouter,
		},
		{
			kind:       structs.ServiceSplitter,
			createFunc: createServiceSplitter,
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("create and delete %s with no service instances", tc.kind), func(t *testing.T) {
			s := testVIPStateStore(t)

			// Create unrelated service instance
			createServiceInstance(t, s, "unrelated")

			// Create the config entry and make sure a virtual ip is allocated
			ensureVirtualIP(t, s, "foo", "")
			tc.createFunc(t, s, "foo")
			ensureVirtualIP(t, s, "foo", "240.0.0.2")

			// Delete the config entry and make sure the virtual ip is freed and reused
			ensureVirtualIP(t, s, "bar", "")
			deleteConfigEntry(t, s, tc.kind, "foo")
			ensureVirtualIP(t, s, "foo", "")
			tc.createFunc(t, s, "bar")
			ensureVirtualIP(t, s, "bar", "240.0.0.2")
		})

		t.Run(fmt.Sprintf("create and delete %s with service instances", tc.kind), func(t *testing.T) {
			s := testVIPStateStore(t)

			// Create a foo service instance and an unrelated service instance
			createServiceInstance(t, s, "foo")

			// Creating the config entry should not affect the service virtual IP
			ensureVirtualIP(t, s, "foo", "240.0.0.1")
			tc.createFunc(t, s, "foo")
			ensureVirtualIP(t, s, "foo", "240.0.0.1")

			// Deleting should also not affect the service virtual IP because there are still existing
			// service instances that need the VIP.
			deleteConfigEntry(t, s, tc.kind, "foo")
			ensureVirtualIP(t, s, "foo", "240.0.0.1")

			// Now delete the service instance, which should free up the virtual IP
			deleteServiceInstance(t, s, "foo")
			ensureVirtualIP(t, s, "foo", "")

			// Make sure the free address can be reused
			tc.createFunc(t, s, "bar")
			ensureVirtualIP(t, s, "bar", "240.0.0.1")
		})

		t.Run(fmt.Sprintf("create and delete service instance while %s still exists", tc.kind), func(t *testing.T) {
			s := testVIPStateStore(t)

			// Create the config entry to get the virtual IP
			tc.createFunc(t, s, "foo")
			ensureVirtualIP(t, s, "foo", "240.0.0.1")

			// Creating service instance should not affect virtual IP
			createServiceInstance(t, s, "foo")
			ensureVirtualIP(t, s, "foo", "240.0.0.1")

			// Deleting should also not affect the service virtual IP because the config entry still exists.
			deleteServiceInstance(t, s, "foo")
			ensureVirtualIP(t, s, "foo", "240.0.0.1")

			// Now delete the config entry, which should free up the ip
			deleteConfigEntry(t, s, tc.kind, "foo")
			ensureVirtualIP(t, s, "foo", "")

			// Make sure the free address can be reused
			tc.createFunc(t, s, "bar")
			ensureVirtualIP(t, s, "bar", "240.0.0.1")
		})
	}
}

func TestStore_MutualTLSMode_Validation_InitialWrite(t *testing.T) {
	cases := []struct {
		// setup
		mesh *structs.MeshConfigEntry

		mtlsMode structs.MutualTLSMode
		expErr   error
	}{
		// Mesh config entry does not exist. Should default to AllowEnablingPermissiveMutualTLS=false.
		{
			mtlsMode: structs.MutualTLSModeDefault,
		},
		{
			mtlsMode: structs.MutualTLSModeStrict,
		},
		{
			mtlsMode: structs.MutualTLSModePermissive,
			expErr:   permissiveModeNotAllowedError,
		},

		// Mesh config entry contains AllowEnablingPermissiveMutualTLS=false
		{
			mesh:     &structs.MeshConfigEntry{},
			mtlsMode: structs.MutualTLSModeDefault,
		},
		{
			mesh:     &structs.MeshConfigEntry{},
			mtlsMode: structs.MutualTLSModeStrict,
		},
		{
			mesh:     &structs.MeshConfigEntry{},
			mtlsMode: structs.MutualTLSModePermissive,
			expErr:   permissiveModeNotAllowedError,
		},

		// Mesh config entry exists with AllowEnablingPermissiveMutualTLS=true.
		{
			mesh:     &structs.MeshConfigEntry{AllowEnablingPermissiveMutualTLS: true},
			mtlsMode: structs.MutualTLSModeDefault,
		},
		{
			mesh:     &structs.MeshConfigEntry{AllowEnablingPermissiveMutualTLS: true},
			mtlsMode: structs.MutualTLSModeStrict,
		},
		{
			mesh:     &structs.MeshConfigEntry{AllowEnablingPermissiveMutualTLS: true},
			mtlsMode: structs.MutualTLSModePermissive,
		},
	}
	for _, c := range cases {
		c := c
		var name string
		if c.mesh == nil {
			name = fmt.Sprintf("when mesh config entry not found")
		} else {
			name = fmt.Sprintf("when AllowEnablingPermissiveMutualTLS=%v", c.mesh.AllowEnablingPermissiveMutualTLS)
		}
		if c.expErr != nil {
			name += " cannot"
		} else {
			name += " can"
		}
		name += fmt.Sprintf(" set MutualTLSMode=%q", c.mtlsMode)
		t.Run(name, func(t *testing.T) {
			s := testConfigStateStore(t)

			var err error
			var idx uint64
			if c.mesh != nil {
				idx, err = writeConfigAndBumpIndexForTest(s, idx, c.mesh)
				require.NoError(t, err)
			}

			idx, err = writeConfigAndBumpIndexForTest(s, idx, &structs.ProxyConfigEntry{
				Kind:          structs.ProxyDefaults,
				Name:          structs.ProxyConfigGlobal,
				MutualTLSMode: c.mtlsMode,
			})
			require.Equal(t, c.expErr, err)

			_, err = writeConfigAndBumpIndexForTest(s, idx, &structs.ServiceConfigEntry{
				Kind:          structs.ServiceDefaults,
				Name:          "test-svc",
				MutualTLSMode: c.mtlsMode,
			})
			require.Equal(t, c.expErr, err)
		})
	}
}

func TestStore_MutualTLSMode_Validation_SubsequentWrite(t *testing.T) {
	cases := []struct {
		allowPermissive bool
		initialModes    []structs.MutualTLSMode
		transitions     map[structs.MutualTLSMode]error
	}{
		{
			allowPermissive: false,
			initialModes: []structs.MutualTLSMode{
				structs.MutualTLSModeDefault,
				structs.MutualTLSModeStrict,
			},
			transitions: map[structs.MutualTLSMode]error{
				structs.MutualTLSModeDefault: nil,
				structs.MutualTLSModeStrict:  nil,
				// Cannot transition from "" -> "permissive"
				// Cannot transition from "strict" -> "permissive"
				structs.MutualTLSModePermissive: permissiveModeNotAllowedError,
			},
		},
		{
			allowPermissive: false,
			initialModes: []structs.MutualTLSMode{
				structs.MutualTLSModePermissive,
			},
			transitions: map[structs.MutualTLSMode]error{
				structs.MutualTLSModeDefault: nil,
				structs.MutualTLSModeStrict:  nil,
				// Can transition from "permissive" -> "permissive"
				structs.MutualTLSModePermissive: nil,
			},
		},
		{
			allowPermissive: true,
			initialModes: []structs.MutualTLSMode{
				structs.MutualTLSModeDefault,
				structs.MutualTLSModeStrict,
				structs.MutualTLSModePermissive,
			},
			transitions: map[structs.MutualTLSMode]error{
				// Can transition from any mode to any other mode when allowPermissive=true
				structs.MutualTLSModeDefault:    nil,
				structs.MutualTLSModeStrict:     nil,
				structs.MutualTLSModePermissive: nil,
			},
		},
	}
	for _, c := range cases {
		c := c

		for _, initialMode := range c.initialModes {
			for newMode, expErr := range c.transitions {
				name := fmt.Sprintf("when AllowEnablingPermissiveMutualTLS=%v", c.allowPermissive)
				if expErr != nil {
					name += " cannot"
				} else {
					name += " can"
				}
				name += fmt.Sprintf(" transition MutualTLSMode from %q to %q", initialMode, newMode)
				t.Run(name, func(t *testing.T) {
					s := testConfigStateStore(t)

					// Setup initial state.
					idx, err := writeConfigAndBumpIndexForTest(s, 0, &structs.MeshConfigEntry{
						AllowEnablingPermissiveMutualTLS: true, // set to true to allow writing any initial mode.
					})
					require.NoError(t, err)

					idx, err = writeConfigAndBumpIndexForTest(s, idx, &structs.ProxyConfigEntry{
						Kind:          structs.ProxyDefaults,
						Name:          structs.ProxyConfigGlobal,
						MutualTLSMode: initialMode,
					})
					require.NoError(t, err)

					idx, err = writeConfigAndBumpIndexForTest(s, idx, &structs.ServiceConfigEntry{
						Kind:          structs.ServiceDefaults,
						Name:          "test-svc",
						MutualTLSMode: initialMode,
					})
					require.NoError(t, err)

					// Set AllowEnablingPermissiveMutualTLS for the test case.
					idx, err = writeConfigAndBumpIndexForTest(s, idx, &structs.MeshConfigEntry{
						AllowEnablingPermissiveMutualTLS: c.allowPermissive,
					})
					require.NoError(t, err)

					// Test switching to the other mode.
					idx, err = writeConfigAndBumpIndexForTest(s, idx, &structs.ProxyConfigEntry{
						Kind:          structs.ProxyDefaults,
						Name:          structs.ProxyConfigGlobal,
						MutualTLSMode: newMode,
					})
					require.Equal(t, expErr, err)

					_, err = writeConfigAndBumpIndexForTest(s, idx, &structs.ServiceConfigEntry{
						Kind:          structs.ServiceDefaults,
						Name:          "test-svc",
						MutualTLSMode: newMode,
					})
					require.Equal(t, expErr, err)
				})

			}
		}
	}
}

func writeConfigAndBumpIndexForTest(s *Store, idx uint64, entry structs.ConfigEntry) (uint64, error) {
	err := s.EnsureConfigEntry(idx, entry)
	if err == nil {
		idx++
	}
	return idx, err
}

func TestStateStore_DiscoveryChain_AttachVirtualIPs(t *testing.T) {
	s := testStateStore(t)
	setVirtualIPFlags(t, s)

	ca := &structs.CAConfiguration{
		Provider: "consul",
	}
	err := s.CASetConfig(0, ca)
	require.NoError(t, err)

	// Attempt to assign manual virtual IPs to a service that doesn't exist - should be a no-op.
	psn := structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "foo", EnterpriseMeta: *acl.DefaultEnterpriseMeta()}}

	// Register a service via config entry.
	s.EnsureConfigEntry(1, &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "foo",
	})

	vip, err := s.VirtualIPForService(psn)
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.1", vip)

	// Assign manual virtual IPs for foo
	found, _, err := s.AssignManualServiceVIPs(2, psn, []string{"2.2.2.2", "3.3.3.3"})
	require.NoError(t, err)
	require.True(t, found)

	serviceVIP, err := s.ServiceManualVIPs(psn)
	require.NoError(t, err)
	require.Equal(t, uint64(2), serviceVIP.ModifyIndex)
	require.Equal(t, "0.0.0.1", serviceVIP.IP.String())
	require.Equal(t, []string{"2.2.2.2", "3.3.3.3"}, serviceVIP.ManualIPs)

	req := discoverychain.CompileRequest{
		ServiceName:          "foo",
		EvaluateInNamespace:  psn.ServiceName.NamespaceOrDefault(),
		EvaluateInPartition:  psn.ServiceName.PartitionOrDefault(),
		EvaluateInDatacenter: "dc1",
	}
	idx, chain, _, err := s.ServiceDiscoveryChain(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), req)
	require.NoError(t, err)
	require.Equal(t, uint64(1), idx)
	require.Equal(t, []string{"240.0.0.1"}, chain.AutoVirtualIPs)
	require.Equal(t, []string{"2.2.2.2", "3.3.3.3"}, chain.ManualVirtualIPs)

}

func TestFindJWTProviderNameReferences(t *testing.T) {
	oktaProvider := structs.IntentionJWTProvider{Name: "okta"}
	auth0Provider := structs.IntentionJWTProvider{Name: "auth0"}
	cases := map[string]struct {
		entries       []structs.ConfigEntry
		providerName  string
		expectedError string
	}{
		"no jwt at any level": {
			entries:      []structs.ConfigEntry{},
			providerName: "okta",
		},
		"provider not referenced": {
			entries: []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: "service-intentions",
					Name: "api-intention",
					JWT: &structs.IntentionJWTRequirement{
						Providers: []*structs.IntentionJWTProvider{&oktaProvider, &auth0Provider},
					},
				},
			},
			providerName: "fake-provider",
		},
		"only top level jwt with no permissions": {
			entries: []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: "service-intentions",
					Name: "api-intention",
					JWT: &structs.IntentionJWTRequirement{
						Providers: []*structs.IntentionJWTProvider{&oktaProvider, &auth0Provider},
					},
				},
			},
			providerName:  "okta",
			expectedError: "cannot delete jwt provider config entry referenced by an intention. Provider name: okta, intention name: api-intention",
		},
		"top level jwt with permissions": {
			entries: []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: "service-intentions",
					Name: "api-intention",
					JWT: &structs.IntentionJWTRequirement{
						Providers: []*structs.IntentionJWTProvider{&oktaProvider},
					},
					Sources: []*structs.SourceIntention{
						{
							Name:   "api",
							Action: "allow",
							Permissions: []*structs.IntentionPermission{
								{
									Action: "allow",
									JWT: &structs.IntentionJWTRequirement{
										Providers: []*structs.IntentionJWTProvider{&oktaProvider},
									},
								},
							},
						},
						{
							Name:   "serv",
							Action: "allow",
							Permissions: []*structs.IntentionPermission{
								{
									Action: "allow",
									JWT: &structs.IntentionJWTRequirement{
										Providers: []*structs.IntentionJWTProvider{&auth0Provider},
									},
								},
							},
						},
						{
							Name:   "web",
							Action: "allow",
							Permissions: []*structs.IntentionPermission{
								{Action: "allow"},
							},
						},
					},
				},
			},
			providerName:  "auth0",
			expectedError: "cannot delete jwt provider config entry referenced by an intention. Provider name: auth0, intention name: api-intention",
		},
		"no top level jwt and existing permissions": {
			entries: []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: "service-intentions",
					Name: "api-intention",
					Sources: []*structs.SourceIntention{
						{
							Name:   "api",
							Action: "allow",
							Permissions: []*structs.IntentionPermission{
								{
									Action: "allow",
									JWT: &structs.IntentionJWTRequirement{
										Providers: []*structs.IntentionJWTProvider{&oktaProvider},
									},
								},
							},
						},
						{
							Name:   "serv",
							Action: "allow",
							Permissions: []*structs.IntentionPermission{
								{
									Action: "allow",
									JWT: &structs.IntentionJWTRequirement{
										Providers: []*structs.IntentionJWTProvider{&auth0Provider},
									},
								},
							},
						},
						{
							Name:   "web",
							Action: "allow",
							Permissions: []*structs.IntentionPermission{
								{Action: "allow"},
							},
						},
					},
				},
			},
			providerName:  "okta",
			expectedError: "cannot delete jwt provider config entry referenced by an intention. Provider name: okta, intention name: api-intention",
		},
	}

	for name, tt := range cases {
		tt := tt
		t.Run(name, func(t *testing.T) {
			err := findJWTProviderNameReferences(tt.entries, tt.providerName)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStore_ValidateJWTProviderIsReferenced(t *testing.T) {
	s := testStateStore(t)

	// First create a config entry
	provider := &structs.JWTProviderConfigEntry{
		Kind: structs.JWTProvider,
		Name: "okta",
	}
	require.NoError(t, s.EnsureConfigEntry(0, provider))

	// create a service intention referencing the config entry
	ixn := &structs.ServiceIntentionsConfigEntry{
		Name: "api",
		JWT: &structs.IntentionJWTRequirement{
			Providers: []*structs.IntentionJWTProvider{
				{Name: provider.Name},
			},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(1, ixn))

	// attempt deleting a referenced provider
	err := s.DeleteConfigEntry(0, structs.JWTProvider, provider.Name, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `cannot delete jwt provider config entry referenced by an intention. Provider name: okta, intention name: api`)

	// delete the intention
	require.NoError(t, s.DeleteConfigEntry(1, structs.ServiceIntentions, ixn.Name, nil))
	// successfully delete the provider after deleting the intention
	require.NoError(t, s.DeleteConfigEntry(0, structs.JWTProvider, provider.Name, nil))
}
