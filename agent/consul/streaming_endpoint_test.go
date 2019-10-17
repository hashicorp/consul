package consul

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/pascaldekloe/goe/verify"
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

	// Start a Subscribe call to our streaming endpoint.
	conn, err := client.grpcClient.GRPCConn()
	require.NoError(err)

	streamClient := stream.NewConsulClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
	require.NoError(err)

	// Start a goroutine to read updates off the stream.
	eventCh := make(chan *stream.Event, 0)
	go testSendEvents(t, eventCh, streamHandle)

	var snapshotEvents []*stream.Event
	for i := 0; i < 3; i++ {
		select {
		case event := <-eventCh:
			snapshotEvents = append(snapshotEvents, event)
		case <-time.After(3 * time.Second):
			t.Fatalf("did not receive events past %d", len(snapshotEvents))
		}
	}

	expected := []*stream.Event{
		{
			Key: "redis",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:       "node1",
							Datacenter: "dc1",
							Address:    "3.4.5.6",
						},
						Service: &stream.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "3.4.5.6",
							Port:    8080,
							Weights: &stream.Weights{Passing: 1, Warning: 1},
						},
					},
				},
			},
		},
		{
			Key: "redis",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:       "node2",
							Datacenter: "dc1",
							Address:    "1.2.3.4",
						},
						Service: &stream.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "1.1.1.1",
							Port:    8080,
							Weights: &stream.Weights{Passing: 1, Warning: 1},
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
	for i := 0; i < 2; i++ {
		// Fix up the index
		expected[i].Index = snapshotEvents[i].Index
		node := expected[i].GetServiceHealth().CheckServiceNode
		node.Node.RaftIndex = snapshotEvents[i].GetServiceHealth().CheckServiceNode.Node.RaftIndex
		node.Service.RaftIndex = snapshotEvents[i].GetServiceHealth().CheckServiceNode.Service.RaftIndex
		expected[i].SetACLRules()
	}
	expected[2].Index = snapshotEvents[2].Index
	verify.Values(t, "", snapshotEvents, expected)

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
		expected := &stream.Event{
			Key: "redis",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
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
		}
		expected.SetACLRules()
		// Fix up the index
		expected.Index = event.Index
		node := expected.GetServiceHealth().CheckServiceNode
		node.Node.RaftIndex = event.GetServiceHealth().CheckServiceNode.Node.RaftIndex
		node.Service.RaftIndex = event.GetServiceHealth().CheckServiceNode.Service.RaftIndex
		node.Checks[0].RaftIndex = event.GetServiceHealth().CheckServiceNode.Checks[0].RaftIndex
		verify.Values(t, "", event, expected)
	case <-time.After(3 * time.Second):
		t.Fatal("never got event")
	}

	// Wait and make sure there aren't any more events coming.
	select {
	case event := <-eventCh:
		t.Fatalf("got another event: %v", event)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestStreaming_Subscribe_SkipSnapshot(t *testing.T) {
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

	// Start a Subscribe call to our streaming endpoint.
	conn, err := client.grpcClient.GRPCConn()
	require.NoError(err)

	streamClient := stream.NewConsulClient(conn)

	var index uint64
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*stream.Event
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				snapshotEvents = append(snapshotEvents, event)
			case <-time.After(3 * time.Second):
				t.Fatalf("did not receive events past %d", len(snapshotEvents))
			}
		}

		// Save the index from the event
		index = snapshotEvents[0].Index
	}

	// Start another Subscribe call passing the index from the last event.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{
			Topic: stream.Topic_ServiceHealth,
			Key:   "redis",
			Index: index,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		// We should only get an empty snapshot and a single "end of snapshot"
		// message to denote it.
		select {
		case event := <-eventCh:
			require.True(event.GetEndOfSnapshot())
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("never got event")
		}

		// Wait and make sure there aren't any events coming. The server shouldn't send
		// a snapshot and we haven't made any updates to the catalog that would trigger
		// more events.
		select {
		case event := <-eventCh:
			t.Fatalf("got another event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func TestStreaming_Subscribe_FilterACL(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir, _, server, codec := testACLFilterServerV8(t, true)
	defer os.RemoveAll(dir)
	defer server.Shutdown()
	defer codec.Close()

	dir2, client := testClient(t)
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	// Try to join
	testrpc.WaitForLeader(t, server.RPC, "dc1")
	joinLAN(t, client, server)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1", testrpc.WithToken("root"))

	// Create a policy for the test token.
	policyReq := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			Description: "foobar",
			Name:        "baz",
			Rules: fmt.Sprintf(`
			service "foo" {
				policy = "write"
			}
			node "%s" {
				policy = "write"
			}
			`, server.config.NodeName),
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	resp := structs.ACLPolicy{}
	require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.PolicySet", &policyReq, &resp))

	// Create a new token that only has access to one node.
	var token structs.ACLToken
	arg := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: resp.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &arg, &token))
	auth, err := server.ResolveToken(token.SecretID)
	require.NoError(err)
	require.False(auth.NodeRead("denied"))

	// Register another instance of service foo on a fake node the token doesn't have access to.
	regArg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "denied",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "foo",
			Service: "foo",
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil))

	// Set up the gRPC client.
	conn, err := client.grpcClient.GRPCConn()
	require.NoError(err)
	streamClient := stream.NewConsulClient(conn)

	// Start a Subscribe call to our streaming endpoint for the service we have access to.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{
			Topic: stream.Topic_ServiceHealth,
			Key:   "foo",
			Token: token.SecretID,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		// Read events off the stream. We should not see any events for the filtered node.
		var snapshotEvents []*stream.Event
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				snapshotEvents = append(snapshotEvents, event)
			case <-time.After(5 * time.Second):
				t.Fatalf("did not receive events past %d", len(snapshotEvents))
			}
		}
		require.Len(snapshotEvents, 2)
		require.Equal("foo", snapshotEvents[0].GetServiceHealth().CheckServiceNode.Service.Service)
		require.Equal(server.config.NodeName, snapshotEvents[0].GetServiceHealth().CheckServiceNode.Node.Node)
		require.True(snapshotEvents[1].GetEndOfSnapshot())

		// Update the service with a new port to trigger a new event.
		regArg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       server.config.NodeName,
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "foo",
				Service: "foo",
				Port:    1234,
			},
			Check: &structs.HealthCheck{
				CheckID:   "service:foo",
				Name:      "service:foo",
				ServiceID: "foo",
				Status:    api.HealthPassing,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil))

		select {
		case event := <-eventCh:
			service := event.GetServiceHealth().CheckServiceNode.Service
			require.Equal("foo", service.Service)
			require.Equal(1234, service.Port)
		case <-time.After(5 * time.Second):
			t.Fatalf("did not receive events past %d", len(snapshotEvents))
		}

		// Now update the service on the denied node and make sure we don't see an event.
		regArg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "denied",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "foo",
				Service: "foo",
				Port:    2345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "service:foo",
				Name:      "service:foo",
				ServiceID: "foo",
				Status:    api.HealthPassing,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil))

		select {
		case event := <-eventCh:
			t.Fatalf("should not have received event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Start another subscribe call for bar, which the token shouldn't have access to.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{
			Topic: stream.Topic_ServiceHealth,
			Key:   "bar",
			Token: token.SecretID,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		select {
		case event := <-eventCh:
			require.True(event.GetEndOfSnapshot())
		case <-time.After(3 * time.Second):
			t.Fatal("did not receive event")
		}

		// Update the service and make sure we don't get a new event.
		regArg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       server.config.NodeName,
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "bar",
				Service: "bar",
				Port:    2345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "service:bar",
				Name:      "service:bar",
				ServiceID: "bar",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil))

		select {
		case event := <-eventCh:
			t.Fatalf("should not have received event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func TestStreaming_Subscribe_ACLUpdate(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir, _, server, codec := testACLFilterServerV8(t, true)
	defer os.RemoveAll(dir)
	defer server.Shutdown()
	defer codec.Close()

	dir2, client := testClient(t)
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	// Try to join
	testrpc.WaitForLeader(t, server.RPC, "dc1")
	joinLAN(t, client, server)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1", testrpc.WithToken("root"))

	// Create a new token that only has access to one node.
	var token string
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "Service/node token",
			Type: structs.ACLTokenTypeClient,
			Rules: fmt.Sprintf(`
service "foo" {
	policy = "write"
}
node "%s" {
	policy = "write"
}
`, server.config.NodeName),
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &token))
	auth, err := server.ResolveToken(token)
	require.NoError(err)
	require.False(auth.NodeRead("denied"))

	// Set up the gRPC client.
	conn, err := client.grpcClient.GRPCConn()
	require.NoError(err)
	streamClient := stream.NewConsulClient(conn)

	// Start a Subscribe call to our streaming endpoint for the service we have access to.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{
			Topic: stream.Topic_ServiceHealth,
			Key:   "foo",
			Token: token,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		// Read events off the stream.
		var snapshotEvents []*stream.Event
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				snapshotEvents = append(snapshotEvents, event)
			case <-time.After(5 * time.Second):
				t.Fatalf("did not receive events past %d", len(snapshotEvents))
			}
		}
		require.Len(snapshotEvents, 2)
		require.Equal("foo", snapshotEvents[0].GetServiceHealth().CheckServiceNode.Service.Service)
		require.Equal(server.config.NodeName, snapshotEvents[0].GetServiceHealth().CheckServiceNode.Node.Node)
		require.True(snapshotEvents[1].GetEndOfSnapshot())

		// Update a different token and make sure we don't see an event.
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name: "Ignored token",
				Type: structs.ACLTokenTypeClient,
				Rules: `
service "foo" {
	policy = "read"
}
`,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply string
		require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &reply))

		select {
		case event := <-eventCh:
			t.Fatalf("should not have received event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}

		// Update our token to trigger a refresh event.
		arg = structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				ID:   token,
				Name: "Service/node token",
				Type: structs.ACLTokenTypeClient,
				Rules: fmt.Sprintf(`
	service "foo" {
		policy = "write"
	}
	node "%s" {
		policy = "write"
	}
	node "bar" {
		policy = "read"
	}
	`, server.config.NodeName),
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &reply))

		select {
		case event := <-eventCh:
			require.True(event.GetReloadStream())
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("did not receive reload event")
		}
	}
}

// testSendEvents receives stream.Events from a given handle and sends them to the provided
// channel. This is meant to be run in a separate goroutine from the main test.
func testSendEvents(t *testing.T, ch chan *stream.Event, handle stream.Consul_SubscribeClient) {
	for {
		event, err := handle.Recv()
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
		ch <- event
	}
}

func TestStreaming_TLSEnabled(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, conf1 := testServerConfig(t)
	conf1.VerifyIncoming = true
	conf1.VerifyOutgoing = true
	configureTLS(conf1)
	server, err := newServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer server.Shutdown()

	dir2, conf2 := testClientConfig(t)
	conf2.VerifyOutgoing = true
	configureTLS(conf2)
	client, err := NewClient(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	// Try to join
	testrpc.WaitForLeader(t, server.RPC, "dc1")
	joinLAN(t, client, server)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

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
		require.NoError(server.RPC("Catalog.Register", &req, &out))
	}

	// Start a Subscribe call to our streaming endpoint from the client.
	{
		conn, err := client.grpcClient.GRPCConn()
		require.NoError(err)

		streamClient := stream.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*stream.Event
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				snapshotEvents = append(snapshotEvents, event)
			case <-time.After(3 * time.Second):
				t.Fatalf("did not receive events past %d", len(snapshotEvents))
			}
		}

		// Make sure the snapshot events come back with no issues.
		require.Len(snapshotEvents, 2)
	}

	// Start a Subscribe call to our streaming endpoint from the server's loopback client.
	{
		conn, err := server.GRPCConn()
		require.NoError(err)

		streamClient := stream.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*stream.Event
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				snapshotEvents = append(snapshotEvents, event)
			case <-time.After(3 * time.Second):
				t.Fatalf("did not receive events past %d", len(snapshotEvents))
			}
		}

		// Make sure the snapshot events come back with no issues.
		require.Len(snapshotEvents, 2)
	}
}

func TestStreaming_TLSReload(t *testing.T) {
	t.Parallel()

	// Set up a server with initially bad certificates.
	require := require.New(t)
	dir1, conf1 := testServerConfig(t)
	conf1.VerifyIncoming = true
	conf1.VerifyOutgoing = true
	conf1.CAFile = "../../test/ca/root.cer"
	conf1.CertFile = "../../test/key/ssl-cert-snakeoil.pem"
	conf1.KeyFile = "../../test/key/ssl-cert-snakeoil.key"

	server, err := newServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer server.Shutdown()

	// Set up a client with valid certs and verify_outgoing = true
	dir2, conf2 := testClientConfig(t)
	conf2.VerifyOutgoing = true
	configureTLS(conf2)
	client, err := NewClient(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	testrpc.WaitForLeader(t, server.RPC, "dc1")

	// Subscribe calls should fail initially
	joinLAN(t, client, server)
	{
		conn, err := client.GRPCConn()
		require.NoError(err)

		streamClient := stream.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err = streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
		require.Error(err, "tls: bad certificate")
	}

	// Reload the server with valid certs
	newConf := server.config.ToTLSUtilConfig()
	newConf.CertFile = "../../test/key/ourdomain.cer"
	newConf.KeyFile = "../../test/key/ourdomain.key"
	server.tlsConfigurator.Update(newConf)

	// Try the subscribe call again
	{
		conn, err := client.GRPCConn()
		require.NoError(err)

		streamClient := stream.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err = streamClient.Subscribe(ctx, &stream.SubscribeRequest{Topic: stream.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)
	}
}

func TestStreaming_Filter(t *testing.T) {
	t.Parallel()

	dir1, server := testServer(t)
	defer os.RemoveAll(dir1)
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	testrpc.WaitForLeader(t, server.RPC, "dc1")

	// Prep the cluster with some data we can use in our filters.
	registerTestCatalogEntries(t, codec)

	// Set up a test function for reading some snapshot events from subscribe to test
	// with different filters.
	testSubscribe := func(t *testing.T, req stream.SubscribeRequest, numEvents int) []*stream.Event {
		conn, err := server.GRPCConn()
		require.NoError(t, err)

		streamClient := stream.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &req)
		require.NoError(t, err)

		// Start a goroutine to read updates off the stream.
		eventCh := make(chan *stream.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*stream.Event
		for i := 0; i < numEvents+1; i++ {
			select {
			case event := <-eventCh:
				if !event.GetEndOfSnapshot() {
					snapshotEvents = append(snapshotEvents, event)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("did not receive events past %d (filter %q)", len(snapshotEvents), req.Filter)
			}
		}

		return snapshotEvents
	}

	t.Run("ServiceNodes", func(t *testing.T) {
		req := stream.SubscribeRequest{
			Topic:  stream.Topic_ServiceHealth,
			Key:    "redis",
			Filter: "Service.Meta.version == 2",
		}

		events := testSubscribe(t, req, 1)
		require.Len(t, events, 1)

		req.Key = "web"
		req.Filter = "Node.Meta.os == linux"
		events = testSubscribe(t, req, 2)
		require.Len(t, events, 2)
		require.Equal(t, "baz", events[0].GetServiceHealth().CheckServiceNode.Node.Node)
		require.Equal(t, "baz", events[1].GetServiceHealth().CheckServiceNode.Node.Node)

		req.Filter = "Node.Meta.os == linux and Service.Meta.version == 1"
		events = testSubscribe(t, req, 1)
		require.Len(t, events, 1)
	})
}
