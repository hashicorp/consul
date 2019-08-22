package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetACLRules(t *testing.T) {
	event := Event{
		Op: Operation_Upsert,
		Payload: &Event_ServiceHealth{
			ServiceHealth: &ServiceHealthUpdate{
				CheckServiceNode: &CheckServiceNode{
					Node: &Node{
						Node: "node1",
					},
					Service: &NodeService{
						Service: "api",
					},
				},
			},
		},
	}

	event.SetACLRules()

	require.Equal(t, []*ACLRule{
		{
			Resource: ACLResource_NodeACL,
			Segment:  "node1",
		},
		{
			Resource: ACLResource_ServiceACL,
			Segment:  "api",
		},
	}, event.RequiredACLs)
}
