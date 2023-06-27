// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peerstream

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestSubscriptionState_Events(t *testing.T) {
	logger := hclog.NewNullLogger()

	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	state := newSubscriptionState("my-peering", partition)

	testutil.RunStep(t, "empty", func(t *testing.T) {
		pending := &pendingPayload{}

		ch := make(chan cache.UpdateEvent, 1)
		state.publicUpdateCh = ch
		go func() {
			state.sendPendingEvents(context.Background(), logger, pending)
			close(ch)
		}()

		got := drainEvents(t, ch)
		require.Len(t, got, 0)
	})

	meshNode1 := &pbservice.CheckServiceNode{
		Node:    &pbservice.Node{Node: "foo"},
		Service: &pbservice.NodeService{ID: "mgw-1", Service: "mgw", Kind: "mesh-gateway"},
	}

	testutil.RunStep(t, "one", func(t *testing.T) {
		pending := &pendingPayload{}
		require.NoError(t, pending.Add(
			meshGatewayPayloadID,
			subMeshGateway+partition,
			&pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					proto.Clone(meshNode1).(*pbservice.CheckServiceNode),
				},
			},
		))

		ch := make(chan cache.UpdateEvent, 1)
		state.publicUpdateCh = ch
		go func() {
			state.sendPendingEvents(context.Background(), logger, pending)
			close(ch)
		}()

		got := drainEvents(t, ch)
		require.Len(t, got, 1)

		evt := got[0]
		require.Equal(t, subMeshGateway+partition, evt.CorrelationID)
		require.Len(t, evt.Result.(*pbservice.IndexedCheckServiceNodes).Nodes, 1)
	})

	testutil.RunStep(t, "a duplicate is omitted", func(t *testing.T) {
		pending := &pendingPayload{}
		require.NoError(t, pending.Add(
			meshGatewayPayloadID,
			subMeshGateway+partition,
			&pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					proto.Clone(meshNode1).(*pbservice.CheckServiceNode),
				},
			},
		))

		ch := make(chan cache.UpdateEvent, 1)
		state.publicUpdateCh = ch
		go func() {
			state.sendPendingEvents(context.Background(), logger, pending)
			close(ch)
		}()

		got := drainEvents(t, ch)
		require.Len(t, got, 0)
	})

	webNode1 := &pbservice.CheckServiceNode{
		Node:    &pbservice.Node{Node: "zim"},
		Service: &pbservice.NodeService{ID: "web-1", Service: "web"},
	}

	webSN := structs.NewServiceName("web", nil)

	testutil.RunStep(t, "a duplicate is omitted even if mixed", func(t *testing.T) {
		pending := &pendingPayload{}
		require.NoError(t, pending.Add(
			meshGatewayPayloadID,
			subMeshGateway+partition,
			&pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					proto.Clone(meshNode1).(*pbservice.CheckServiceNode),
				},
			},
		))
		require.NoError(t, pending.Add(
			servicePayloadIDPrefix+webSN.String(),
			subExportedService+webSN.String(),
			&pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					proto.Clone(webNode1).(*pbservice.CheckServiceNode),
				},
			},
		))

		ch := make(chan cache.UpdateEvent, 1)
		state.publicUpdateCh = ch
		go func() {
			state.sendPendingEvents(context.Background(), logger, pending)
			close(ch)
		}()

		got := drainEvents(t, ch)
		require.Len(t, got, 1)

		evt := got[0]
		require.Equal(t, subExportedService+webSN.String(), evt.CorrelationID)
		require.Len(t, evt.Result.(*pbservice.IndexedCheckServiceNodes).Nodes, 1)
	})

	meshNode2 := &pbservice.CheckServiceNode{
		Node:    &pbservice.Node{Node: "bar"},
		Service: &pbservice.NodeService{ID: "mgw-2", Service: "mgw", Kind: "mesh-gateway"},
	}

	testutil.RunStep(t, "an update to an existing item is published", func(t *testing.T) {
		pending := &pendingPayload{}
		require.NoError(t, pending.Add(
			meshGatewayPayloadID,
			subMeshGateway+partition,
			&pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					proto.Clone(meshNode1).(*pbservice.CheckServiceNode),
					proto.Clone(meshNode2).(*pbservice.CheckServiceNode),
				},
			},
		))

		ch := make(chan cache.UpdateEvent, 1)
		state.publicUpdateCh = ch
		go func() {
			state.sendPendingEvents(context.Background(), logger, pending)
			close(ch)
		}()

		got := drainEvents(t, ch)
		require.Len(t, got, 1)

		evt := got[0]
		require.Equal(t, subMeshGateway+partition, evt.CorrelationID)
		require.Len(t, evt.Result.(*pbservice.IndexedCheckServiceNodes).Nodes, 2)
	})
}

func drainEvents(t *testing.T, ch <-chan cache.UpdateEvent) []cache.UpdateEvent {
	var out []cache.UpdateEvent

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, evt)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("channel did not close in time")
		}
	}
}

func testNewSubscriptionState(partition string) (
	*subscriptionState,
	chan cache.UpdateEvent,
) {
	var (
		publicUpdateCh = make(chan cache.UpdateEvent, 1)
	)

	state := newSubscriptionState("my-peering", partition)
	state.publicUpdateCh = publicUpdateCh

	return state, publicUpdateCh
}
