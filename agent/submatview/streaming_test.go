package submatview

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/types"

	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// TestStreamingClient is a mock StreamingClient for testing that allows
// for queueing up custom events to a subscriber.
type TestStreamingClient struct {
	pbsubscribe.StateChangeSubscription_SubscribeClient
	events            chan eventOrErr
	ctx               context.Context
	expectedNamespace string
}

type eventOrErr struct {
	Err   error
	Event *pbsubscribe.Event
}

func NewTestStreamingClient(ns string) *TestStreamingClient {
	return &TestStreamingClient{
		events:            make(chan eventOrErr, 32),
		expectedNamespace: ns,
	}
}

func (t *TestStreamingClient) Subscribe(
	ctx context.Context,
	req *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	if req.Namespace != t.expectedNamespace {
		return nil, fmt.Errorf("wrong SubscribeRequest.Namespace %v, expected %v",
			req.Namespace, t.expectedNamespace)
	}
	t.ctx = ctx
	return t, nil
}

func (t *TestStreamingClient) QueueEvents(events ...*pbsubscribe.Event) {
	for _, e := range events {
		t.events <- eventOrErr{Event: e}
	}
}

func (t *TestStreamingClient) QueueErr(err error) {
	t.events <- eventOrErr{Err: err}
}

func (t *TestStreamingClient) Recv() (*pbsubscribe.Event, error) {
	select {
	case eoe := <-t.events:
		if eoe.Err != nil {
			return nil, eoe.Err
		}
		return eoe.Event, nil
	case <-t.ctx.Done():
		return nil, t.ctx.Err()
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
						ID:         nodeID,
						Node:       node,
						Address:    addr,
						Datacenter: "dc1",
						RaftIndex: pbcommon.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
					Service: &pbservice.NodeService{
						ID:      svc,
						Service: svc,
						Port:    8080,
						RaftIndex: pbcommon.RaftIndex{
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
						RaftIndex: pbcommon.RaftIndex{
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
