// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package fsm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
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
