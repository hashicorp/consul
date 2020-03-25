package agentpb

import (
	fmt "fmt"

	"github.com/hashicorp/consul/types"
	"github.com/mitchellh/go-testing-interface"
)

// TestEventEndOfSnapshot returns a valid EndOfSnapshot event on the given topic
// and index.
func TestEventEndOfSnapshot(t testing.T, topic Topic, index uint64) Event {
	return Event{
		Topic: topic,
		Index: index,
		Payload: &Event_EndOfSnapshot{
			EndOfSnapshot: true,
		},
	}
}

// TestEventResetStream returns a valid ResetStream event on the given topic
// and index.
func TestEventResetStream(t testing.T, topic Topic, index uint64) Event {
	return Event{
		Topic: topic,
		Index: index,
		Payload: &Event_ResetStream{
			ResetStream: true,
		},
	}
}

// TestEventResumeStream returns a valid ResumeStream event on the given topic
// and index.
func TestEventResumeStream(t testing.T, topic Topic, index uint64) Event {
	return Event{
		Topic: topic,
		Index: index,
		Payload: &Event_ResumeStream{
			ResumeStream: true,
		},
	}
}

// TestEventBatch returns a valid EventBatch event it assumes service health
// topic, an index of 100 and contains two health registrations.
func TestEventBatch(t testing.T) Event {
	e1 := TestEventServiceHealthRegister(t, 1, "web")
	e2 := TestEventServiceHealthRegister(t, 1, "api")
	return Event{
		Topic: Topic_ServiceHealth,
		Index: 100,
		Payload: &Event_EventBatch{
			EventBatch: &EventBatch{
				Events: []*Event{&e1, &e2},
			},
		},
	}
}

// TestEventACLTokenUpdate returns a valid ACLToken event.
func TestEventACLTokenUpdate(t testing.T) Event {
	return Event{
		Topic: Topic_ACLTokens,
		Index: 100,
		Payload: &Event_ACLToken{
			ACLToken: &ACLTokenUpdate{
				Op: ACLOp_Update,
				Token: &ACLTokenIdentifier{
					AccessorID: "adfa4d37-560f-4824-a121-356064a7a2ea",
					SecretID:   "f58b28f9-42a4-48b2-a08c-eba8ff6560f1",
				},
			},
		},
	}
}

// TestEventACLPolicyUpdate returns a valid ACLPolicy event.
func TestEventACLPolicyUpdate(t testing.T) Event {
	return Event{
		Topic: Topic_ACLPolicies,
		Index: 100,
		Payload: &Event_ACLPolicy{
			ACLPolicy: &ACLPolicyUpdate{
				Op:       ACLOp_Update,
				PolicyID: "f1df7f3e-6732-45e8-9a3d-ada2a22fa336",
			},
		},
	}
}

// TestEventACLRoleUpdate returns a valid ACLRole event.
func TestEventACLRoleUpdate(t testing.T) Event {
	return Event{
		Topic: Topic_ACLRoles,
		Index: 100,
		Payload: &Event_ACLRole{
			ACLRole: &ACLRoleUpdate{
				Op:     ACLOp_Update,
				RoleID: "40fee72a-510f-4de7-8c91-e05d42512b9f",
			},
		},
	}
}

// TestEventServiceHealthRegister returns a realistically populated service
// health registration event for tests in other packages. The nodeNum is a
// logical node and is used to create the node name ("node%d") but also change
// the node ID and IP address to make it a little more realistic for cases that
// need that. nodeNum should be less than 64k to make the IP address look
// realistic. Any other changes can be made on the returned event to avoid
// adding too many options to callers.
func TestEventServiceHealthRegister(t testing.T, nodeNum int, svc string) Event {

	node := fmt.Sprintf("node%d", nodeNum)
	nodeID := types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-%012d", nodeNum))
	addr := fmt.Sprintf("10.10.%d.%d", nodeNum/256, nodeNum%256)

	return Event{
		Topic: Topic_ServiceHealth,
		Key:   svc,
		Index: 100,
		Payload: &Event_ServiceHealth{
			ServiceHealth: &ServiceHealthUpdate{
				Op: CatalogOp_Register,
				CheckServiceNode: &CheckServiceNode{
					Node: &Node{
						ID:         nodeID,
						Node:       node,
						Address:    addr,
						Datacenter: "dc1",
						RaftIndex: RaftIndex{
							CreateIndex: 100,
							ModifyIndex: 100,
						},
					},
					Service: &NodeService{
						ID:      svc,
						Service: svc,
						Port:    8080,
						Weights: &Weights{
							Passing: 1,
							Warning: 1,
						},
						// Empty sadness
						Proxy: ConnectProxyConfig{
							MeshGateway: &MeshGatewayConfig{},
							Expose:      &ExposeConfig{},
						},
						EnterpriseMeta: &EnterpriseMeta{},
						RaftIndex: RaftIndex{
							CreateIndex: 100,
							ModifyIndex: 100,
						},
					},
					Checks: []*HealthCheck{
						&HealthCheck{
							Node:           node,
							CheckID:        "serf-health",
							Name:           "serf-health",
							Status:         "passing",
							EnterpriseMeta: &EnterpriseMeta{},
							RaftIndex: RaftIndex{
								CreateIndex: 100,
								ModifyIndex: 100,
							},
						},
						&HealthCheck{
							Node:           node,
							CheckID:        types.CheckID("service:" + svc),
							Name:           "service:" + svc,
							ServiceID:      svc,
							ServiceName:    svc,
							Type:           "ttl",
							Status:         "passing",
							EnterpriseMeta: &EnterpriseMeta{},
							RaftIndex: RaftIndex{
								CreateIndex: 100,
								ModifyIndex: 100,
							},
						},
					},
				},
			},
		},
	}
}

// TestEventServiceHealthDeregister returns a realistically populated service
// health deregistration event for tests in other packages. The nodeNum is a
// logical node and is used to create the node name ("node%d") but also change
// the node ID and IP address to make it a little more realistic for cases that
// need that. nodeNum should be less than 64k to make the IP address look
// realistic. Any other changes can be made on the returned event to avoid
// adding too many options to callers.
func TestEventServiceHealthDeregister(t testing.T, nodeNum int, svc string) Event {

	node := fmt.Sprintf("node%d", nodeNum)

	return Event{
		Topic: Topic_ServiceHealth,
		Key:   svc,
		Index: 100,
		Payload: &Event_ServiceHealth{
			ServiceHealth: &ServiceHealthUpdate{
				Op: CatalogOp_Deregister,
				CheckServiceNode: &CheckServiceNode{
					Node: &Node{
						Node: node,
					},
					Service: &NodeService{
						ID:      svc,
						Service: svc,
						Port:    8080,
						Weights: &Weights{
							Passing: 1,
							Warning: 1,
						},
						// Empty sadness
						Proxy: ConnectProxyConfig{
							MeshGateway: &MeshGatewayConfig{},
							Expose:      &ExposeConfig{},
						},
						EnterpriseMeta: &EnterpriseMeta{},
						RaftIndex: RaftIndex{
							// The original insertion index since a delete doesn't update this.
							CreateIndex: 10,
							ModifyIndex: 10,
						},
					},
				},
			},
		},
	}
}
