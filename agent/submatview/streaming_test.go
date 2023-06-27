// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package submatview

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/types"
)

// TestStreamingClient is a mock StreamingClient for testing that allows
// for queueing up custom events to a subscriber.
type TestStreamingClient struct {
	expectedNamespace string
	subClients        []*subscribeClient
	lock              sync.RWMutex
	events            []eventOrErr
}

type eventOrErr struct {
	Err   error
	Event *pbsubscribe.Event
}

func NewTestStreamingClient(ns string) *TestStreamingClient {
	return &TestStreamingClient{expectedNamespace: ns}
}

func (s *TestStreamingClient) Subscribe(
	ctx context.Context,
	req *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	if ns := req.GetNamedSubject().GetNamespace(); ns != s.expectedNamespace {
		return nil, fmt.Errorf("wrong SubscribeRequest.NamedSubject.Namespace %v, expected %v",
			ns, s.expectedNamespace)
	}
	c := &subscribeClient{
		events: make(chan eventOrErr, 32),
		ctx:    ctx,
	}
	s.lock.Lock()
	s.subClients = append(s.subClients, c)
	for _, event := range s.events {
		c.events <- event
	}
	s.lock.Unlock()
	return c, nil
}

type subscribeClient struct {
	grpc.ClientStream
	events chan eventOrErr
	ctx    context.Context
}

func (s *TestStreamingClient) QueueEvents(events ...*pbsubscribe.Event) {
	s.lock.Lock()
	for _, e := range events {
		s.events = append(s.events, eventOrErr{Event: e})
		for _, c := range s.subClients {
			c.events <- eventOrErr{Event: e}
		}
	}
	s.lock.Unlock()
}

func (s *TestStreamingClient) QueueErr(err error) {
	s.lock.Lock()
	s.events = append(s.events, eventOrErr{Err: err})
	for _, c := range s.subClients {
		c.events <- eventOrErr{Err: err}
	}
	s.lock.Unlock()
}

func (c *subscribeClient) Recv() (*pbsubscribe.Event, error) {
	select {
	case eoe := <-c.events:
		if eoe.Err != nil {
			return nil, eoe.Err
		}
		return eoe.Event, nil
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

func newEndOfSnapshotEvent(index uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index:   index,
		Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
	}
}

func newNewSnapshotToFollowEvent() *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Payload: &pbsubscribe.Event_NewSnapshotToFollow{NewSnapshotToFollow: true},
	}
}

func newEventServiceHealthRegister(index uint64, nodeNum int, svc string) *pbsubscribe.Event {
	node := fmt.Sprintf("node%d", nodeNum)
	nodeID := types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-%012d", nodeNum))
	addr := fmt.Sprintf("10.10.%d.%d", nodeNum/256, nodeNum%256)

	return &pbsubscribe.Event{
		Index: index,
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Register,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Node: &pbservice.Node{
						ID:         string(nodeID),
						Node:       node,
						Address:    addr,
						Datacenter: "dc1",
						RaftIndex: &pbcommon.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
					Service: &pbservice.NodeService{
						ID:      svc,
						Service: svc,
						Port:    8080,
						RaftIndex: &pbcommon.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
				},
			},
		},
	}
}

func newEventServiceHealthDeregister(index uint64, nodeNum int, svc string) *pbsubscribe.Event {
	node := fmt.Sprintf("node%d", nodeNum)

	return &pbsubscribe.Event{
		Index: index,
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Deregister,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Node: &pbservice.Node{
						Node: node,
					},
					Service: &pbservice.NodeService{
						ID:      svc,
						Service: svc,
						Port:    8080,
						Weights: &pbservice.Weights{
							Passing: 1,
							Warning: 1,
						},
						RaftIndex: &pbcommon.RaftIndex{
							// The original insertion index since a delete doesn't update
							// this. This magic value came from state store tests where we
							// setup at index 10 and then mutate at index 100. It can be
							// modified by the caller later and makes it easier than having
							// yet another argument in the common case.
							CreateIndex: 10,
							ModifyIndex: 10,
						},
					},
				},
			},
		},
	}
}

func newEventBatchWithEvents(first *pbsubscribe.Event, evs ...*pbsubscribe.Event) *pbsubscribe.Event {
	events := make([]*pbsubscribe.Event, len(evs)+1)
	events[0] = first
	for i := range evs {
		events[i+1] = evs[i]
	}
	return &pbsubscribe.Event{
		Index: first.Index,
		Payload: &pbsubscribe.Event_EventBatch{
			EventBatch: &pbsubscribe.EventBatch{Events: events},
		},
	}
}
