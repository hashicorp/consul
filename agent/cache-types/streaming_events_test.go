package cachetype

import (
	fmt "fmt"

	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/types"
)

func newEndOfSnapshotEvent(topic pbsubscribe.Topic, index uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Topic:   topic,
		Index:   index,
		Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
	}
}

func newNewSnapshotToFollowEvent(topic pbsubscribe.Topic, index uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Topic:   topic,
		Index:   index,
		Payload: &pbsubscribe.Event_NewSnapshotToFollow{NewSnapshotToFollow: true},
	}
}

// newEventServiceHealthRegister returns a realistically populated service
// health registration event for tests. The nodeNum is a
// logical node and is used to create the node name ("node%d") but also change
// the node ID and IP address to make it a little more realistic for cases that
// need that. nodeNum should be less than 64k to make the IP address look
// realistic. Any other changes can be made on the returned event to avoid
// adding too many options to callers.
func newEventServiceHealthRegister(index uint64, nodeNum int, svc string) pbsubscribe.Event {
	node := fmt.Sprintf("node%d", nodeNum)
	nodeID := types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-%012d", nodeNum))
	addr := fmt.Sprintf("10.10.%d.%d", nodeNum/256, nodeNum%256)

	return pbsubscribe.Event{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   svc,
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
						Weights: &pbservice.Weights{
							Passing: 1,
							Warning: 1,
						},
						// Empty sadness
						Proxy: pbservice.ConnectProxyConfig{
							MeshGateway: pbservice.MeshGatewayConfig{},
							Expose:      pbservice.ExposeConfig{},
						},
						EnterpriseMeta: pbcommon.EnterpriseMeta{},
						RaftIndex: pbcommon.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
					Checks: []*pbservice.HealthCheck{
						{
							Node:           node,
							CheckID:        "serf-health",
							Name:           "serf-health",
							Status:         "passing",
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
							RaftIndex: pbcommon.RaftIndex{
								CreateIndex: index,
								ModifyIndex: index,
							},
						},
						{
							Node:           node,
							CheckID:        types.CheckID("service:" + svc),
							Name:           "service:" + svc,
							ServiceID:      svc,
							ServiceName:    svc,
							Type:           "ttl",
							Status:         "passing",
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
							RaftIndex: pbcommon.RaftIndex{
								CreateIndex: index,
								ModifyIndex: index,
							},
						},
					},
				},
			},
		},
	}
}

// TestEventServiceHealthDeregister returns a realistically populated service
// health deregistration event for tests. The nodeNum is a
// logical node and is used to create the node name ("node%d") but also change
// the node ID and IP address to make it a little more realistic for cases that
// need that. nodeNum should be less than 64k to make the IP address look
// realistic. Any other changes can be made on the returned event to avoid
// adding too many options to callers.
func newEventServiceHealthDeregister(index uint64, nodeNum int, svc string) pbsubscribe.Event {
	node := fmt.Sprintf("node%d", nodeNum)

	return pbsubscribe.Event{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   svc,
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
						// Empty sadness
						Proxy: pbservice.ConnectProxyConfig{
							MeshGateway: pbservice.MeshGatewayConfig{},
							Expose:      pbservice.ExposeConfig{},
						},
						EnterpriseMeta: pbcommon.EnterpriseMeta{},
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

func newEventBatchWithEvents(first pbsubscribe.Event, evs ...pbsubscribe.Event) pbsubscribe.Event {
	events := make([]*pbsubscribe.Event, len(evs)+1)
	events[0] = &first
	for i := range evs {
		events[i+1] = &evs[i]
	}
	return pbsubscribe.Event{
		Topic: first.Topic,
		Index: first.Index,
		Payload: &pbsubscribe.Event_EventBatch{
			EventBatch: &pbsubscribe.EventBatch{Events: events},
		},
	}
}
