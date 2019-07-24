package consul

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
)

func TestStreaming_Subscribe(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, server := testServer(t)
	defer os.RemoveAll(dir1)
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	dir2, client := testClient(t)
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	// Try to join
	testrpc.WaitForLeader(t, server.RPC, "dc1")
	joinLAN(t, client, server)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	serverMeta := client.routers.FindServer()
	require.NotNil(serverMeta)

	// Register a dummy node with a service we don't care about, to make sure
	// we don't see updates for it.
	{
		req := &structs.RegisterRequest{
			Node:       "other",
			Address:    "2.3.4.5",
			Datacenter: "dc1",
			Service: &structs.NodeService{
				ID:      "api1",
				Service: "api",
				Address: "2.3.4.5",
				Port:    9000,
			},
		}
		var out struct{}
		require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &out))
	}

	// Register a dummy node with our service on it.
	{
		req := &structs.RegisterRequest{
			Node:       "node1",
			Address:    "3.4.5.6",
			Datacenter: "dc1",
			Service: &structs.NodeService{
				ID:      "redis1",
				Service: "redis",
				Address: "3.4.5.6",
				Port:    8080,
			},
		}
		var out struct{}
		require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &out))
	}

	// Register a test node to be updated later.
	req := &structs.RegisterRequest{
		Node:       "node2",
		Address:    "1.2.3.4",
		Datacenter: "dc1",
		Service: &structs.NodeService{
			ID:      "redis1",
			Service: "redis",
			Address: "1.1.1.1",
			Port:    8080,
		},
	}
	var out struct{}
	require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &out))

	// Make a basic RPC call to our streaming endpoint.
	conn, err := client.grpcClient.GRPCConn(nil)
	require.NoError(err)

	streamClient := stream.NewConsulClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
	require.NoError(err)

	// Start a goroutine to read updates off the stream.
	eventCh := make(chan *stream.Event, 0)
	go func() {
		for {
			event, err := streamHandle.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") ||
					strings.Contains(err.Error(), "context canceled") {
					break
				}
				t.Log(err)
			}
			eventCh <- event
		}
	}()

	var snapshotEvents []*stream.Event
	for i := 0; i < 3; i++ {
		select {
		case event := <-eventCh:
			snapshotEvents = append(snapshotEvents, event)
		case <-time.After(5 * time.Second):
			t.Fatalf("did not receive events past %d", len(snapshotEvents))
		}
	}

	expected := []*stream.Event{
		{
			Key:   "redis",
			Index: 13,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:       "node1",
							Datacenter: "dc1",
							Address:    "3.4.5.6",
							RaftIndex:  stream.RaftIndex{CreateIndex: 12, ModifyIndex: 12},
						},
						Service: &stream.NodeService{
							ID:        "redis1",
							Service:   "redis",
							Address:   "3.4.5.6",
							Port:      8080,
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
							RaftIndex: stream.RaftIndex{CreateIndex: 12, ModifyIndex: 12},
						},
					},
				},
			},
		},
		{
			Key:   "redis",
			Index: 13,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:       "node2",
							Datacenter: "dc1",
							Address:    "1.2.3.4",
							RaftIndex:  stream.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
						},
						Service: &stream.NodeService{
							ID:        "redis1",
							Service:   "redis",
							Address:   "1.1.1.1",
							Port:      8080,
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
							RaftIndex: stream.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
						},
					},
				},
			},
		},
		{
			Topic: stream.Topic_EndOfSnapshot,
		},
	}
	require.Equal(expected, snapshotEvents)

	// Update the registration by adding a check.
	req.Check = &structs.HealthCheck{
		Node:        "node2",
		CheckID:     types.CheckID("check1"),
		ServiceID:   "redis1",
		ServiceName: "redis",
		Name:        "check 1",
	}
	require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &out))

	// Make sure we get the event for the diff.
	select {
	case event := <-eventCh:
		require.Equal(&stream.Event{
			Key:   "redis",
			Index: 14,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:       "node2",
							Datacenter: "dc1",
							Address:    "1.2.3.4",
							RaftIndex:  stream.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
						},
						Service: &stream.NodeService{
							ID:        "redis1",
							Service:   "redis",
							Address:   "1.1.1.1",
							Port:      8080,
							RaftIndex: stream.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
						},
						Checks: []*stream.HealthCheck{
							{
								CheckID:     "check1",
								Name:        "check 1",
								Node:        "node2",
								Status:      "critical",
								ServiceID:   "redis1",
								ServiceName: "redis",
								RaftIndex:   stream.RaftIndex{CreateIndex: 14, ModifyIndex: 14},
							},
						},
					},
				},
			},
		}, event)
	case <-time.After(3 * time.Second):
		t.Fatal("never got event")
	}

	// Wait and make sure there aren't any more events coming.
	select {
	case event := <-eventCh:
		t.Fatalf("got another event: %v", event)
	case <-time.After(3 * time.Second):
	}
}
