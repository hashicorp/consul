package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetACLRules(t *testing.T) {
	event := Event{
		Payload: &Event_ServiceHealth{
			ServiceHealth: &ServiceHealthUpdate{
				Op: CatalogOp_Register,
				ServiceNode: &CheckServiceNode{
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
