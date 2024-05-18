// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package fsm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestRestoreFromEnterprise(t *testing.T) {
	logger := testutil.Logger(t)

	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	handle.apply = func(buf []byte) (any, error) { return storageBackend.Apply(buf, 123), nil }

	fsm := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: storageBackend,
	})

	// To verify if a proper message is displayed when Consul CE tries to
	//  unsuccessfully restore entries from a Consul Ent snapshot.
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}

	type EntMock struct {
		ID   int
		Type string
	}

	entMockEntry := EntMock{
		ID:   65,
		Type: "A Consul Ent Log Type",
	}

	// Write the header
	header := SnapshotHeader{
		LastIndex: 0,
	}
	encoder := codec.NewEncoder(sink, structs.MsgpackHandle)
	encoder.Encode(&header)
	sink.Write([]byte{byte(structs.MessageType(entMockEntry.ID))})
	encoder.Encode(entMockEntry)

	require.EqualError(t, fsm.Restore(sink), "msg type <65> is a Consul Enterprise log entry. Consul CE cannot restore it")
	sink.Cancel()
}

func TestRestoreFromEnterprise_CEDowngrade(t *testing.T) {
	logger := testutil.Logger(t)

	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	handle.apply = func(buf []byte) (any, error) { return storageBackend.Apply(buf, 123), nil }

	fsm := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: storageBackend,
	})

	// To verify if a proper message is displayed when Consul CE tries to
	//  unsuccessfully restore entries from a Consul Ent snapshot.
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}

	type EntMock struct {
		ID   int
		Type string
	}

	entMockEntry := EntMock{
		ID:   65,
		Type: "A Consul Ent Log Type",
	}

	// Create one entry to exercise the Go struct marshaller, and one to exercise the
	// Binary Marshaller interface. This verifies that regardless of whether the struct gets
	// encoded as a msgpack byte string (binary marshaller) or msgpack map (other struct),
	// it will still be skipped over correctly.
	registerEntry := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"primary"},
			Port:    8000,
		},
	}
	proxyDefaultsEntry := &structs.ConfigEntryRequest{
		Op: structs.ConfigEntryUpsert,
		Entry: &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: "global",
			Config: map[string]interface{}{
				"foo": "bar",
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}

	// Write the header and records.
	header := SnapshotHeader{
		LastIndex: 0,
	}
	encoder := codec.NewEncoder(sink, structs.MsgpackHandle)
	encoder.Encode(&header)
	sink.Write([]byte{byte(structs.MessageType(entMockEntry.ID))})
	encoder.Encode(entMockEntry)
	sink.Write([]byte{byte(structs.RegisterRequestType)})
	encoder.Encode(registerEntry)
	sink.Write([]byte{byte(structs.ConfigEntryRequestType)})
	encoder.Encode(proxyDefaultsEntry)

	defer func() {
		structs.CEDowngrade = false
	}()
	structs.CEDowngrade = true

	require.NoError(t, fsm.Restore(sink), "failed to decode Ent snapshot to CE")

	// Verify the register request
	_, nodes, err := fsm.state.Nodes(nil, nil, "")
	require.NoError(t, err)
	require.Len(t, nodes, 1, "incorrect number of nodes: %v", nodes)
	require.Equal(t, "foo", nodes[0].Node)
	require.Equal(t, "dc1", nodes[0].Datacenter)
	require.Equal(t, "127.0.0.1", nodes[0].Address)
	_, fooSrv, err := fsm.state.NodeServices(nil, "foo", nil, "")
	require.NoError(t, err)
	require.Len(t, fooSrv.Services, 1)
	require.Contains(t, fooSrv.Services["db"].Tags, "primary")
	require.True(t, stringslice.Contains(fooSrv.Services["db"].Tags, "primary"))
	require.Equal(t, 8000, fooSrv.Services["db"].Port)

	// Verify the proxy defaults request
	_, configEntry, err := fsm.state.ConfigEntry(nil, structs.ProxyDefaults, "global", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	configEntry.SetHash(proxyDefaultsEntry.Entry.GetHash())
	require.Equal(t, proxyDefaultsEntry.Entry, configEntry)
}
