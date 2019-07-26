package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/pascaldekloe/goe/verify"

	"github.com/stretchr/testify/require"
)

func TestStreamingHealthServices(t *testing.T) {
	require := require.New(t)
	client := NewTestStreamingClient()
	typ := StreamingHealthServices{Client: client}

	// Set up the events that will form the snapshot
	events := []*stream.Event{
		{
			Topic: stream.Topic_ServiceHealth,
			Index: 1,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:    "node1",
							Address: "1.2.3.4",
						},
						Service: &stream.NodeService{
							Service: "web",
							Port:    8080,
						},
					},
				},
			},
		},
		{
			Topic: stream.Topic_ServiceHealth,
			Index: 2,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:    "node2",
							Address: "2.3.4.5",
						},
						Service: &stream.NodeService{
							Service: "web",
							Port:    9000,
						},
					},
				},
			},
		},
		{
			Topic:   stream.Topic_ServiceHealth,
			Payload: &stream.Event_EndOfSnapshot{EndOfSnapshot: true},
		},
	}
	client.QueueEvents(events...)

	// Do a fetch, should return the view with both nodes
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}, &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	})
	require.NoError(err)
	verify.Values(t, "", &structs.IndexedCheckServiceNodes{
		Nodes: structs.CheckServiceNodes{
			{
				Node: &structs.Node{
					Node:    "node1",
					Address: "1.2.3.4",
				},
				Service: &structs.NodeService{
					Service: "web",
					Port:    8080,
				},
			},
			{
				Node: &structs.Node{
					Node:    "node2",
					Address: "2.3.4.5",
				},
				Service: &structs.NodeService{
					Service: "web",
					Port:    9000,
				},
			},
		},
		QueryMeta: structs.QueryMeta{
			Index: 2,
		},
	}, result.Value)

	// Add another event to the stream
	client.QueueEvents(&stream.Event{
		Topic: stream.Topic_ServiceHealth,
		Index: 3,
		Payload: &stream.Event_ServiceHealth{
			ServiceHealth: &stream.ServiceHealthUpdate{
				Op: stream.CatalogOp_Register,
				ServiceNode: &stream.CheckServiceNode{
					Node: &stream.Node{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					Service: &stream.NodeService{
						Service: "web",
						Port:    8081,
					},
				},
			},
		},
	})

	// Do another fetch, should return the updated view
	result, err = typ.Fetch(cache.FetchOptions{
		MinIndex:   2,
		Timeout:    1 * time.Second,
		LastResult: &result,
	}, &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	})
	require.NoError(err)
	verify.Values(t, "", &structs.IndexedCheckServiceNodes{
		Nodes: structs.CheckServiceNodes{
			{
				Node: &structs.Node{
					Node:    "node1",
					Address: "1.2.3.4",
				},
				Service: &structs.NodeService{
					Service: "web",
					Port:    8081,
				},
			},
			{
				Node: &structs.Node{
					Node:    "node2",
					Address: "2.3.4.5",
				},
				Service: &structs.NodeService{
					Service: "web",
					Port:    9000,
				},
			},
		},
		QueryMeta: structs.QueryMeta{
			Index: 3,
		},
	}, result.Value)
}
