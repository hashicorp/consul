package subscribe

/* TODO
func TestStreaming_Subscribe(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, server := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	dir2, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
	})
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
	conn, err := client.GRPCConn()
	require.NoError(err)

	streamClient := pbsubscribe.NewConsulClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   "redis",
	})
	require.NoError(err)

	// Start a goroutine to read updates off the pbsubscribe.
	eventCh := make(chan *pbsubscribe.Event, 0)
	go testSendEvents(t, eventCh, streamHandle)

	var snapshotEvents []*pbsubscribe.Event
	for i := 0; i < 3; i++ {
		select {
		case event := <-eventCh:
			snapshotEvents = append(snapshotEvents, event)
		case <-time.After(3 * time.Second):
			t.Fatalf("did not receive events past %d", len(snapshotEvents))
		}
	}

	expected := []*pbsubscribe.Event{
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbsubscribe.CheckServiceNode{
						Node: &pbsubscribe.Node{
							Node:       "node1",
							Datacenter: "dc1",
							Address:    "3.4.5.6",
						},
						Service: &pbsubscribe.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "3.4.5.6",
							Port:    8080,
							Weights: &pbsubscribe.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbsubscribe.ConnectProxyConfig{
								MeshGateway: &pbsubscribe.MeshGatewayConfig{},
								Expose:      &pbsubscribe.ExposeConfig{},
							},
							EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
						},
					},
				},
			},
		},
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbsubscribe.CheckServiceNode{
						Node: &pbsubscribe.Node{
							Node:       "node2",
							Datacenter: "dc1",
							Address:    "1.2.3.4",
						},
						Service: &pbsubscribe.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "1.1.1.1",
							Port:    8080,
							Weights: &pbsubscribe.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbsubscribe.ConnectProxyConfig{
								MeshGateway: &pbsubscribe.MeshGatewayConfig{},
								Expose:      &pbsubscribe.ExposeConfig{},
							},
							EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
						},
					},
				},
			},
		},
		{
			Topic:   pbsubscribe.Topic_ServiceHealth,
			Key:     "redis",
			Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
		},
	}

	require.Len(snapshotEvents, 3)
	for i := 0; i < 2; i++ {
		// Fix up the index
		expected[i].Index = snapshotEvents[i].Index
		node := expected[i].GetServiceHealth().CheckServiceNode
		node.Node.RaftIndex = snapshotEvents[i].GetServiceHealth().CheckServiceNode.Node.RaftIndex
		node.Service.RaftIndex = snapshotEvents[i].GetServiceHealth().CheckServiceNode.Service.RaftIndex
	}
	// Fix index on snapshot event
	expected[2].Index = snapshotEvents[2].Index

	requireEqualProtos(t, expected, snapshotEvents)

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
		expected := &pbsubscribe.Event{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbsubscribe.CheckServiceNode{
						Node: &pbsubscribe.Node{
							Node:       "node2",
							Datacenter: "dc1",
							Address:    "1.2.3.4",
							RaftIndex:  pbsubscribe.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
						},
						Service: &pbsubscribe.NodeService{
							ID:        "redis1",
							Service:   "redis",
							Address:   "1.1.1.1",
							Port:      8080,
							RaftIndex: pbsubscribe.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
							Weights:   &pbsubscribe.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbsubscribe.ConnectProxyConfig{
								MeshGateway: &pbsubscribe.MeshGatewayConfig{},
								Expose:      &pbsubscribe.ExposeConfig{},
							},
							EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
						},
						Checks: []*pbsubscribe.HealthCheck{
							{
								CheckID:        "check1",
								Name:           "check 1",
								Node:           "node2",
								Status:         "critical",
								ServiceID:      "redis1",
								ServiceName:    "redis",
								RaftIndex:      pbsubscribe.RaftIndex{CreateIndex: 14, ModifyIndex: 14},
								EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
							},
						},
					},
				},
			},
		}
		// Fix up the index
		expected.Index = event.Index
		node := expected.GetServiceHealth().CheckServiceNode
		node.Node.RaftIndex = event.GetServiceHealth().CheckServiceNode.Node.RaftIndex
		node.Service.RaftIndex = event.GetServiceHealth().CheckServiceNode.Service.RaftIndex
		node.Checks[0].RaftIndex = event.GetServiceHealth().CheckServiceNode.Checks[0].RaftIndex
		requireEqualProtos(t, expected, event)
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

func TestStreaming_Subscribe_MultiDC(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, server1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server1.Shutdown()

	dir2, server2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.Bootstrap = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer server2.Shutdown()
	codec := rpcClient(t, server2)
	defer codec.Close()

	dir3, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir3)
	defer client.Shutdown()

	// Join the servers via WAN
	joinWAN(t, server2, server1)
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	testrpc.WaitForLeader(t, server2.RPC, "dc2")

	joinLAN(t, client, server1)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	// Register a dummy node in dc2 with a service we don't care about,
	// to make sure we don't see updates for it.
	{
		req := &structs.RegisterRequest{
			Node:       "other",
			Address:    "2.3.4.5",
			Datacenter: "dc2",
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

	// Register a dummy node with our service on it, again in dc2.
	{
		req := &structs.RegisterRequest{
			Node:       "node1",
			Address:    "3.4.5.6",
			Datacenter: "dc2",
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

	// Register a test node in dc2 to be updated later.
	req := &structs.RegisterRequest{
		Node:       "node2",
		Address:    "1.2.3.4",
		Datacenter: "dc2",
		Service: &structs.NodeService{
			ID:      "redis1",
			Service: "redis",
			Address: "1.1.1.1",
			Port:    8080,
		},
	}
	var out struct{}
	require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &out))

	// Start a cross-DC Subscribe call to our streaming endpoint, specifying dc2.
	conn, err := client.GRPCConn()
	require.NoError(err)

	streamClient := pbsubscribe.NewConsulClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
		Topic:      pbsubscribe.Topic_ServiceHealth,
		Key:        "redis",
		Datacenter: "dc2",
	})
	require.NoError(err)

	// Start a goroutine to read updates off the pbsubscribe.
	eventCh := make(chan *pbsubscribe.Event, 0)
	go testSendEvents(t, eventCh, streamHandle)

	var snapshotEvents []*pbsubscribe.Event
	for i := 0; i < 3; i++ {
		select {
		case event := <-eventCh:
			snapshotEvents = append(snapshotEvents, event)
		case <-time.After(3 * time.Second):
			t.Fatalf("did not receive events past %d", len(snapshotEvents))
		}
	}

	expected := []*pbsubscribe.Event{
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbsubscribe.CheckServiceNode{
						Node: &pbsubscribe.Node{
							Node:       "node1",
							Datacenter: "dc2",
							Address:    "3.4.5.6",
						},
						Service: &pbsubscribe.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "3.4.5.6",
							Port:    8080,
							Weights: &pbsubscribe.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbsubscribe.ConnectProxyConfig{
								MeshGateway: &pbsubscribe.MeshGatewayConfig{},
								Expose:      &pbsubscribe.ExposeConfig{},
							},
							EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
						},
					},
				},
			},
		},
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbsubscribe.CheckServiceNode{
						Node: &pbsubscribe.Node{
							Node:       "node2",
							Datacenter: "dc2",
							Address:    "1.2.3.4",
						},
						Service: &pbsubscribe.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "1.1.1.1",
							Port:    8080,
							Weights: &pbsubscribe.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbsubscribe.ConnectProxyConfig{
								MeshGateway: &pbsubscribe.MeshGatewayConfig{},
								Expose:      &pbsubscribe.ExposeConfig{},
							},
							EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
						},
					},
				},
			},
		},
		{
			Topic:   pbsubscribe.Topic_ServiceHealth,
			Key:     "redis",
			Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
		},
	}

	require.Len(snapshotEvents, 3)
	for i := 0; i < 2; i++ {
		// Fix up the index
		expected[i].Index = snapshotEvents[i].Index
		node := expected[i].GetServiceHealth().CheckServiceNode
		node.Node.RaftIndex = snapshotEvents[i].GetServiceHealth().CheckServiceNode.Node.RaftIndex
		node.Service.RaftIndex = snapshotEvents[i].GetServiceHealth().CheckServiceNode.Service.RaftIndex
	}
	expected[2].Index = snapshotEvents[2].Index
	requireEqualProtos(t, expected, snapshotEvents)

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
		expected := &pbsubscribe.Event{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbsubscribe.CheckServiceNode{
						Node: &pbsubscribe.Node{
							Node:       "node2",
							Datacenter: "dc2",
							Address:    "1.2.3.4",
							RaftIndex:  pbsubscribe.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
						},
						Service: &pbsubscribe.NodeService{
							ID:        "redis1",
							Service:   "redis",
							Address:   "1.1.1.1",
							Port:      8080,
							RaftIndex: pbsubscribe.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
							Weights:   &pbsubscribe.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbsubscribe.ConnectProxyConfig{
								MeshGateway: &pbsubscribe.MeshGatewayConfig{},
								Expose:      &pbsubscribe.ExposeConfig{},
							},
							EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
						},
						Checks: []*pbsubscribe.HealthCheck{
							{
								CheckID:        "check1",
								Name:           "check 1",
								Node:           "node2",
								Status:         "critical",
								ServiceID:      "redis1",
								ServiceName:    "redis",
								RaftIndex:      pbsubscribe.RaftIndex{CreateIndex: 14, ModifyIndex: 14},
								EnterpriseMeta: &pbsubscribe.EnterpriseMeta{},
							},
						},
					},
				},
			},
		}
		// Fix up the index
		expected.Index = event.Index
		node := expected.GetServiceHealth().CheckServiceNode
		node.Node.RaftIndex = event.GetServiceHealth().CheckServiceNode.Node.RaftIndex
		node.Service.RaftIndex = event.GetServiceHealth().CheckServiceNode.Service.RaftIndex
		node.Checks[0].RaftIndex = event.GetServiceHealth().CheckServiceNode.Checks[0].RaftIndex
		requireEqualProtos(t, expected, event)
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
	dir1, server := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	dir2, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
	})
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
	conn, err := client.GRPCConn()
	require.NoError(err)

	streamClient := pbsubscribe.NewConsulClient(conn)

	var index uint64
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*pbsubscribe.Event
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
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Index: index,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		// We should get no snapshot and the first event should be "resume stream"
		select {
		case event := <-eventCh:
			require.True(event.GetResumeStream())
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
	dir, _, server, codec := testACLFilterServerWithConfigFn(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir)
	defer server.Shutdown()
	defer codec.Close()

	dir2, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
	})
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
	require.Equal(auth.NodeRead("denied", nil), acl.Deny)

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
	conn, err := client.GRPCConn()
	require.NoError(err)
	streamClient := pbsubscribe.NewConsulClient(conn)

	// Start a Subscribe call to our streaming endpoint for the service we have access to.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "foo",
			Token: token.SecretID,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		// Read events off the pbsubscribe. We should not see any events for the filtered node.
		var snapshotEvents []*pbsubscribe.Event
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
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "bar",
			Token: token.SecretID,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
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
	dir, _, server, codec := testACLFilterServerWithConfigFn(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir)
	defer server.Shutdown()
	defer codec.Close()

	dir2, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	// Try to join
	testrpc.WaitForLeader(t, server.RPC, "dc1")
	joinLAN(t, client, server)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1", testrpc.WithToken("root"))

	// Create a new token/policy that only has access to one node.
	var token structs.ACLToken

	policy, err := upsertTestPolicyWithRules(codec, "root", "dc1", fmt.Sprintf(`
		service "foo" {
			policy = "write"
		}
		node "%s" {
			policy = "write"
		}
		`, server.config.NodeName))
	require.NoError(err)

	arg := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			Description: "Service/node token",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: policy.ID,
				},
			},
			Local: false,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &arg, &token))
	auth, err := server.ResolveToken(token.SecretID)
	require.NoError(err)
	require.Equal(auth.NodeRead("denied", nil), acl.Deny)

	// Set up the gRPC client.
	conn, err := client.GRPCConn()
	require.NoError(err)
	streamClient := pbsubscribe.NewConsulClient(conn)

	// Start a Subscribe call to our streaming endpoint for the service we have access to.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "foo",
			Token: token.SecretID,
		})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		// Read events off the pbsubscribe.
		var snapshotEvents []*pbsubscribe.Event
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
		arg2 := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "Ignored token",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID: policy.ID,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var ignoredToken structs.ACLToken
		require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &arg2, &ignoredToken))

		select {
		case event := <-eventCh:
			t.Fatalf("should not have received event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}

		// Update our token to trigger a refresh event.
		token.Policies = []structs.ACLTokenPolicyLink{}
		arg := structs.ACLTokenSetRequest{
			Datacenter:   "dc1",
			ACLToken:     token,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &arg, &token))

		select {
		case event := <-eventCh:
			require.True(event.GetResetStream())
			// 500 ms was not enough in CI apparently...
		case <-time.After(2 * time.Second):
			t.Fatalf("did not receive reload event")
		}
	}
}

// testSendEvents receives pbsubscribe.Events from a given handle and sends them to the provided
// channel. This is meant to be run in a separate goroutine from the main test.
func testSendEvents(t *testing.T, ch chan *pbsubscribe.Event, handle pbsubscribe.StateChangeSubscription_SubscribeClient) {
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
	conf1.GRPCEnabled = true
	configureTLS(conf1)
	server, err := newServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer server.Shutdown()

	dir2, conf2 := testClientConfig(t)
	conf2.VerifyOutgoing = true
	conf2.GRPCEnabled = true
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
		conn, err := client.GRPCConn()
		require.NoError(err)

		streamClient := pbsubscribe.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*pbsubscribe.Event
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

		retryFailedConn(t, conn)

		streamClient := pbsubscribe.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go testSendEvents(t, eventCh, streamHandle)

		var snapshotEvents []*pbsubscribe.Event
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
	conf1.GRPCEnabled = true

	server, err := newServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer server.Shutdown()

	// Set up a client with valid certs and verify_outgoing = true
	dir2, conf2 := testClientConfig(t)
	conf2.VerifyOutgoing = true
	conf2.GRPCEnabled = true
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
	conn, err := client.GRPCConn()
	require.NoError(err)
	{
		streamClient := pbsubscribe.NewConsulClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err = streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealth, Key: "redis"})
		require.Error(err, "tls: bad certificate")
	}

	// Reload the server with valid certs
	newConf := server.config.ToTLSUtilConfig()
	newConf.CertFile = "../../test/key/ourdomain.cer"
	newConf.KeyFile = "../../test/key/ourdomain.key"
	server.tlsConfigurator.Update(newConf)

	// Try the subscribe call again
	{
		retryFailedConn(t, conn)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		streamClient := pbsubscribe.NewConsulClient(conn)
		_, err = streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealth, Key: "redis"})
		require.NoError(err)
	}
}

// retryFailedConn forces the ClientConn to reset its backoff timer and retry the connection,
// to simulate the client eventually retrying after the initial failure. This is used both to simulate
// retrying after an expected failure as well as to avoid flakiness when running many tests in parallel.
func retryFailedConn(t *testing.T, conn *grpc.ClientConn) {
	state := conn.GetState()
	if state.String() != "TRANSIENT_FAILURE" {
		return
	}

	// If the connection has failed, retry and wait for a state change.
	conn.ResetConnectBackoff()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.True(t, conn.WaitForStateChange(ctx, state))
}

func TestStreaming_DeliversAllMessages(t *testing.T) {
	// This is a fuzz/probabilistic test to try to provoke streaming into dropping
	// messages. There is a bug in the initial implementation that should make
	// this fail. While we can't be certain a pass means it's correct, it is
	// useful for finding bugs in our concurrency design.

	// The issue is that when updates are coming in fast such that updates occur
	// in between us making the snapshot and beginning the stream updates, we
	// shouldn't miss anything.

	// To test this, we will run a background goroutine that will write updates as
	// fast as possible while we then try to stream the results and ensure that we
	// see every change. We'll make the updates monotonically increasing so we can
	// easily tell if we missed one.

	require := require.New(t)
	dir1, server := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	dir2, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer client.Shutdown()

	// Try to join
	testrpc.WaitForLeader(t, server.RPC, "dc1")
	joinLAN(t, client, server)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	// Register a whole bunch of service instances so that the initial snapshot on
	// subscribe is big enough to take a bit of time to load giving more
	// opportunity for missed updates if there is a bug.
	for i := 0; i < 1000; i++ {
		req := &structs.RegisterRequest{
			Node:       fmt.Sprintf("node-redis-%03d", i),
			Address:    "3.4.5.6",
			Datacenter: "dc1",
			Service: &structs.NodeService{
				ID:      fmt.Sprintf("redis-%03d", i),
				Service: "redis",
				Port:    11211,
			},
		}
		var out struct{}
		require.NoError(server.RPC("Catalog.Register", &req, &out))
	}

	// Start background writer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		// Update the registration with a monotonically increasing port as fast as
		// we can.
		req := &structs.RegisterRequest{
			Node:       "node1",
			Address:    "3.4.5.6",
			Datacenter: "dc1",
			Service: &structs.NodeService{
				ID:      "redis-canary",
				Service: "redis",
				Port:    0,
			},
		}
		for {
			if ctx.Err() != nil {
				return
			}
			var out struct{}
			require.NoError(server.RPC("Catalog.Register", &req, &out))
			req.Service.Port++
			if req.Service.Port > 100 {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Now start a whole bunch of streamers in parallel to maximise chance of
	// catching a race.
	conn, err := client.GRPCConn()
	require.NoError(err)

	streamClient := pbsubscribe.NewConsulClient(conn)

	n := 5
	var wg sync.WaitGroup
	var updateCount uint64
	// Buffered error chan so that workers can exit and terminate wg without
	// blocking on send. We collect errors this way since t isn't thread safe.
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go verifyMonotonicStreamUpdates(ctx, t, streamClient, &wg, i, &updateCount, errCh)
	}

	// Wait until all subscribers have verified the first bunch of updates all got
	// delivered.
	wg.Wait()

	close(errCh)

	// Require that none of them errored. Since we closed the chan above this loop
	// should terminate immediately if no errors were buffered.
	for err := range errCh {
		require.NoError(err)
	}

	// Sanity check that at least some non-snapshot messages were delivered. We
	// can't know exactly how many because it's timing dependent based on when
	// each subscribers snapshot occurs.
	require.True(atomic.LoadUint64(&updateCount) > 0,
		"at least some of the subscribers should have received non-snapshot updates")
}

type testLogger interface {
	Logf(format string, args ...interface{})
}

func verifyMonotonicStreamUpdates(ctx context.Context, logger testLogger, client pbsubscribe.StateChangeSubscriptionClient, wg *sync.WaitGroup, i int, updateCount *uint64, errCh chan<- error) {
	defer wg.Done()
	streamHandle, err := client.Subscribe(ctx, &pbsubscribe.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealth, Key: "redis"})
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "context canceled") {
			logger.Logf("subscriber %05d: context cancelled before loop")
			return
		}
		errCh <- err
		return
	}

	snapshotDone := false
	expectPort := 0
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
			errCh <- err
			return
		}

		// Ignore snapshot message
		if event.GetEndOfSnapshot() || event.GetResumeStream() {
			snapshotDone = true
			logger.Logf("subscriber %05d: snapshot done, expect next port to be %d", i, expectPort)
		} else if snapshotDone {
			// Verify we get all updates in order
			svc, err := svcOrErr(event)
			if err != nil {
				errCh <- err
				return
			}
			if expectPort != svc.Port {
				errCh <- fmt.Errorf("subscriber %05d: missed %d update(s)!", i, svc.Port-expectPort)
				return
			}
			atomic.AddUint64(updateCount, 1)
			logger.Logf("subscriber %05d: got event with correct port=%d", i, expectPort)
			expectPort++
		} else {
			// This is a snapshot update. Check if it's an update for the canary
			// instance that got applied before our snapshot was sent (likely)
			svc, err := svcOrErr(event)
			if err != nil {
				errCh <- err
				return
			}
			if svc.ID == "redis-canary" {
				// Update the expected port we see in the next update to be one more
				// than the port in the snapshot.
				expectPort = svc.Port + 1
				logger.Logf("subscriber %05d: saw canary in snapshot with port %d", i, svc.Port)
			}
		}
		if expectPort > 100 {
			return
		}
	}
}

func svcOrErr(event *pbsubscribe.Event) (*pbservice.NodeService, error) {
	health := event.GetServiceHealth()
	if health == nil {
		return nil, fmt.Errorf("not a health event: %#v", event)
	}
	csn := health.CheckServiceNode
	if csn == nil {
		return nil, fmt.Errorf("nil CSN: %#v", event)
	}
	if csn.Service == nil {
		return nil, fmt.Errorf("nil service: %#v", event)
	}
	return csn.Service, nil
}

// requireEqualProtos is a helper that runs arrays or structures containing
// proto buf messages through JSON encoding before comparing/diffing them. This
// is necessary because require.Equal doesn't compare them equal and generates
// really unhelpful output in this case for some reason.
func requireEqualProtos(t *testing.T, want, got interface{}) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	require.NoError(t, err)
	expectJSON, err := json.Marshal(want)
	require.NoError(t, err)
	require.JSONEq(t, string(expectJSON), string(gotJSON))
}
*/
