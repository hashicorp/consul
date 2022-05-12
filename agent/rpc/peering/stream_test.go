package peering

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbstatus"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestStreamResources_Server_FirstRequest(t *testing.T) {
	type testCase struct {
		name    string
		input   *pbpeering.ReplicationMessage
		wantErr error
	}

	run := func(t *testing.T, tc testCase) {
		publisher := stream.NewEventPublisher(10 * time.Second)
		store := newStateStore(t, publisher)

		srv := NewService(testutil.Logger(t), &testStreamBackend{
			store: store,
			pub:   publisher,
		})

		client := NewMockClient(context.Background())

		errCh := make(chan error, 1)
		client.ErrCh = errCh

		go func() {
			// Pass errors from server handler into ErrCh so that they can be seen by the client on Recv().
			// This matches gRPC's behavior when an error is returned by a server.
			err := srv.StreamResources(client.ReplicationStream)
			if err != nil {
				errCh <- err
			}
		}()

		err := client.Send(tc.input)
		require.NoError(t, err)

		msg, err := client.Recv()
		require.Nil(t, msg)
		require.Error(t, err)
		require.EqualError(t, err, tc.wantErr.Error())
	}

	tt := []testCase{
		{
			name: "unexpected response",
			input: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Response_{
					Response: &pbpeering.ReplicationMessage_Response{
						ResourceURL: pbpeering.TypeURLService,
						ResourceID:  "api-service",
						Nonce:       "2",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "first message when initiating a peering must be a subscription request"),
		},
		{
			name: "missing peer id",
			input: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "initial subscription request must specify a PeerID"),
		},
		{
			name: "unexpected nonce",
			input: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						PeerID: "63b60245-c475-426b-b314-4588d210859d",
						Nonce:  "1",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "initial subscription request must not contain a nonce"),
		},
		{
			name: "unknown resource",
			input: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						PeerID:      "63b60245-c475-426b-b314-4588d210859d",
						ResourceURL: "nomad.Job",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "subscription request to unknown resource URL: nomad.Job"),
		},
		{
			name: "unknown peer",
			input: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						PeerID:      "63b60245-c475-426b-b314-4588d210859d",
						ResourceURL: pbpeering.TypeURLService,
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "initial subscription for unknown PeerID: 63b60245-c475-426b-b314-4588d210859d"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}

}

func TestStreamResources_Server_Terminate(t *testing.T) {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	srv := NewService(testutil.Logger(t), &testStreamBackend{
		store: store,
		pub:   publisher,
	})

	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	srv.streams.timeNow = it.Now

	client := NewMockClient(context.Background())

	errCh := make(chan error, 1)
	client.ErrCh = errCh

	go func() {
		// Pass errors from server handler into ErrCh so that they can be seen by the client on Recv().
		// This matches gRPC's behavior when an error is returned by a server.
		if err := srv.StreamResources(client.ReplicationStream); err != nil {
			errCh <- err
		}
	}()

	peering := pbpeering.Peering{
		Name: "my-peer",
	}
	require.NoError(t, store.PeeringWrite(0, &peering))

	_, p, err := store.PeeringRead(nil, state.Query{Value: "my-peer"})
	require.NoError(t, err)

	// Receive a subscription from a peer
	peerID := p.ID

	sub := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				PeerID:      peerID,
				ResourceURL: pbpeering.TypeURLService,
			},
		},
	}
	err = client.Send(sub)
	require.NoError(t, err)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	// Receive subscription to my-peer-B's resources
	receivedSub, err := client.Recv()
	require.NoError(t, err)

	expect := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				ResourceURL: pbpeering.TypeURLService,
				PeerID:      peerID,
			},
		},
	}
	prototest.AssertDeepEqual(t, expect, receivedSub)

	testutil.RunStep(t, "terminate the stream", func(t *testing.T) {
		done := srv.ConnectedStreams()[peerID]
		close(done)

		retry.Run(t, func(r *retry.R) {
			_, ok := srv.StreamStatus(peerID)
			require.False(r, ok)
		})
	})

	receivedTerm, err := client.Recv()
	require.NoError(t, err)
	expect = &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Terminated_{
			Terminated: &pbpeering.ReplicationMessage_Terminated{},
		},
	}
	prototest.AssertDeepEqual(t, expect, receivedTerm)
}

func TestStreamResources_Server_StreamTracker(t *testing.T) {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	srv := NewService(testutil.Logger(t), &testStreamBackend{
		store: store,
		pub:   publisher,
	})

	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	srv.streams.timeNow = it.Now

	client := NewMockClient(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.StreamResources(client.ReplicationStream)
	}()

	peering := pbpeering.Peering{
		Name: "my-peer",
	}
	require.NoError(t, store.PeeringWrite(0, &peering))

	_, p, err := store.PeeringRead(nil, state.Query{Value: "my-peer"})
	require.NoError(t, err)

	peerID := p.ID

	sub := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				PeerID:      peerID,
				ResourceURL: pbpeering.TypeURLService,
			},
		},
	}
	err = client.Send(sub)
	require.NoError(t, err)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	testutil.RunStep(t, "client receives initial subscription", func(t *testing.T) {
		ack, err := client.Recv()
		require.NoError(t, err)

		expectAck := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Request_{
				Request: &pbpeering.ReplicationMessage_Request{
					ResourceURL: pbpeering.TypeURLService,
					PeerID:      peerID,
					Nonce:       "",
				},
			},
		}
		prototest.AssertDeepEqual(t, expectAck, ack)
	})

	var sequence uint64
	var lastSendSuccess time.Time

	testutil.RunStep(t, "ack tracked as success", func(t *testing.T) {
		ack := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Request_{
				Request: &pbpeering.ReplicationMessage_Request{
					PeerID:      peerID,
					ResourceURL: pbpeering.TypeURLService,
					Nonce:       "1",

					// Acks do not have an Error populated in the request
				},
			},
		}
		err := client.Send(ack)
		require.NoError(t, err)
		sequence++

		lastSendSuccess = it.base.Add(time.Duration(sequence) * time.Second).UTC()

		expect := StreamStatus{
			Connected: true,
			LastAck:   lastSendSuccess,
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	var lastNack time.Time
	var lastNackMsg string

	testutil.RunStep(t, "nack tracked as error", func(t *testing.T) {
		nack := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Request_{
				Request: &pbpeering.ReplicationMessage_Request{
					PeerID:      peerID,
					ResourceURL: pbpeering.TypeURLService,
					Nonce:       "2",
					Error: &pbstatus.Status{
						Code:    int32(code.Code_UNAVAILABLE),
						Message: "bad bad not good",
					},
				},
			},
		}
		err := client.Send(nack)
		require.NoError(t, err)
		sequence++

		lastNackMsg = "client peer was unable to apply resource: bad bad not good"
		lastNack = it.base.Add(time.Duration(sequence) * time.Second).UTC()

		expect := StreamStatus{
			Connected:       true,
			LastAck:         lastSendSuccess,
			LastNack:        lastNack,
			LastNackMessage: lastNackMsg,
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	var lastRecvSuccess time.Time

	testutil.RunStep(t, "response applied locally", func(t *testing.T) {
		resp := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Response_{
				Response: &pbpeering.ReplicationMessage_Response{
					ResourceURL: pbpeering.TypeURLService,
					ResourceID:  "api",
					Nonce:       "21",
					Operation:   pbpeering.ReplicationMessage_Response_UPSERT,
				},
			},
		}
		err := client.Send(resp)
		require.NoError(t, err)
		sequence++

		ack, err := client.Recv()
		require.NoError(t, err)

		expectAck := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Request_{
				Request: &pbpeering.ReplicationMessage_Request{
					ResourceURL: pbpeering.TypeURLService,
					Nonce:       "21",
				},
			},
		}
		prototest.AssertDeepEqual(t, expectAck, ack)

		lastRecvSuccess = it.base.Add(time.Duration(sequence) * time.Second).UTC()

		expect := StreamStatus{
			Connected:          true,
			LastAck:            lastSendSuccess,
			LastNack:           lastNack,
			LastNackMessage:    lastNackMsg,
			LastReceiveSuccess: lastRecvSuccess,
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	var lastRecvError time.Time
	var lastRecvErrorMsg string

	testutil.RunStep(t, "response fails to apply locally", func(t *testing.T) {
		resp := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Response_{
				Response: &pbpeering.ReplicationMessage_Response{
					ResourceURL: pbpeering.TypeURLService,
					ResourceID:  "web",
					Nonce:       "24",

					// Unknown operation gets NACKed
					Operation: pbpeering.ReplicationMessage_Response_Unknown,
				},
			},
		}
		err := client.Send(resp)
		require.NoError(t, err)
		sequence++

		ack, err := client.Recv()
		require.NoError(t, err)

		expectNack := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Request_{
				Request: &pbpeering.ReplicationMessage_Request{
					ResourceURL: pbpeering.TypeURLService,
					Nonce:       "24",
					Error: &pbstatus.Status{
						Code:    int32(code.Code_INVALID_ARGUMENT),
						Message: `unsupported operation: "Unknown"`,
					},
				},
			},
		}
		prototest.AssertDeepEqual(t, expectNack, ack)

		lastRecvError = it.base.Add(time.Duration(sequence) * time.Second).UTC()
		lastRecvErrorMsg = `unsupported operation: "Unknown"`

		expect := StreamStatus{
			Connected:               true,
			LastAck:                 lastSendSuccess,
			LastNack:                lastNack,
			LastNackMessage:         lastNackMsg,
			LastReceiveSuccess:      lastRecvSuccess,
			LastReceiveError:        lastRecvError,
			LastReceiveErrorMessage: lastRecvErrorMsg,
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	testutil.RunStep(t, "client disconnect marks stream as disconnected", func(t *testing.T) {
		client.Close()

		sequence++
		lastRecvError := it.base.Add(time.Duration(sequence) * time.Second).UTC()

		sequence++
		disconnectTime := it.base.Add(time.Duration(sequence) * time.Second).UTC()

		expect := StreamStatus{
			Connected:               false,
			LastAck:                 lastSendSuccess,
			LastNack:                lastNack,
			LastNackMessage:         lastNackMsg,
			DisconnectTime:          disconnectTime,
			LastReceiveSuccess:      lastRecvSuccess,
			LastReceiveErrorMessage: io.EOF.Error(),
			LastReceiveError:        lastRecvError,
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	select {
	case err := <-errCh:
		// Client disconnect is not an error, but should make the handler return.
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestStreamResources_Server_ServiceUpdates(t *testing.T) {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	// Create a peering
	var lastIdx uint64 = 1
	err := store.PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: "my-peering",
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: "my-peering"})
	require.NoError(t, err)
	require.NotNil(t, p)

	srv := NewService(testutil.Logger(t), &testStreamBackend{
		store: store,
		pub:   publisher,
	})

	client := NewMockClient(context.Background())

	errCh := make(chan error, 1)
	client.ErrCh = errCh

	go func() {
		// Pass errors from server handler into ErrCh so that they can be seen by the client on Recv().
		// This matches gRPC's behavior when an error is returned by a server.
		if err := srv.StreamResources(client.ReplicationStream); err != nil {
			errCh <- err
		}
	}()

	// Issue a services subscription to server
	init := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				PeerID:      p.ID,
				ResourceURL: pbpeering.TypeURLService,
			},
		},
	}
	require.NoError(t, client.Send(init))

	// Receive a services subscription from server
	receivedSub, err := client.Recv()
	require.NoError(t, err)

	expect := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				ResourceURL: pbpeering.TypeURLService,
				PeerID:      p.ID,
			},
		},
	}
	prototest.AssertDeepEqual(t, expect, receivedSub)

	// Register a service that is not yet exported
	mysql := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
	}

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, mysql.Node))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "foo", mysql.Service))

	testutil.RunStep(t, "exporting mysql leads to an UPSERT event", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "my-peering",
						},
					},
				},
				{
					// Mongo does not get pushed because it does not have instances registered.
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "my-peering",
						},
					},
				},
			},
		}
		lastIdx++
		err = store.EnsureConfigEntry(lastIdx, entry)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeering.ReplicationMessage_Response_UPSERT, msg.GetResponse().Operation)
			require.Equal(r, mysql.Service.CompoundServiceName().String(), msg.GetResponse().ResourceID)

			var nodes pbservice.IndexedCheckServiceNodes
			require.NoError(r, ptypes.UnmarshalAny(msg.GetResponse().Resource, &nodes))
			require.Len(r, nodes.Nodes, 1)
		})
	})

	mongo := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "zip", Address: "10.0.0.3"},
		Service: &structs.NodeService{ID: "mongo-1", Service: "mongo", Port: 5000},
	}

	testutil.RunStep(t, "registering mongo instance leads to an UPSERT event", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, mongo.Node))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "zip", mongo.Service))

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeering.ReplicationMessage_Response_UPSERT, msg.GetResponse().Operation)
			require.Equal(r, mongo.Service.CompoundServiceName().String(), msg.GetResponse().ResourceID)

			var nodes pbservice.IndexedCheckServiceNodes
			require.NoError(r, ptypes.UnmarshalAny(msg.GetResponse().Resource, &nodes))
			require.Len(r, nodes.Nodes, 1)
		})
	})

	testutil.RunStep(t, "un-exporting mysql leads to a DELETE event for mysql", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "my-peering",
						},
					},
				},
			},
		}
		lastIdx++
		err = store.EnsureConfigEntry(lastIdx, entry)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeering.ReplicationMessage_Response_DELETE, msg.GetResponse().Operation)
			require.Equal(r, mysql.Service.CompoundServiceName().String(), msg.GetResponse().ResourceID)
			require.Nil(r, msg.GetResponse().Resource)
		})
	})

	testutil.RunStep(t, "deleting the config entry leads to a DELETE event for mongo", func(t *testing.T) {
		lastIdx++
		err = store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", nil)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeering.ReplicationMessage_Response_DELETE, msg.GetResponse().Operation)
			require.Equal(r, mongo.Service.CompoundServiceName().String(), msg.GetResponse().ResourceID)
			require.Nil(r, msg.GetResponse().Resource)
		})
	})
}

type testStreamBackend struct {
	pub   state.EventPublisher
	store *state.Store
}

func (b *testStreamBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return b.pub.Subscribe(req)
}

func (b *testStreamBackend) Store() Store {
	return b.store
}

func (b *testStreamBackend) Forward(info structs.RPCInfo, f func(conn *grpc.ClientConn) error) (handled bool, err error) {
	return true, nil
}

func (b *testStreamBackend) GetAgentCACertificates() ([]string, error) {
	return []string{}, nil
}

func (b *testStreamBackend) GetServerAddresses() ([]string, error) {
	return []string{}, nil
}

func (b *testStreamBackend) GetServerName() string {
	return ""
}

func (b *testStreamBackend) EncodeToken(tok *structs.PeeringToken) ([]byte, error) {
	return nil, nil
}

func (b *testStreamBackend) DecodeToken([]byte) (*structs.PeeringToken, error) {
	return nil, nil
}

func (b *testStreamBackend) EnterpriseCheckPartitions(partition string) error {
	return nil
}

func (b *testStreamBackend) Apply() Apply {
	return nil
}

func Test_processResponse_Validation(t *testing.T) {
	type testCase struct {
		name    string
		in      *pbpeering.ReplicationMessage_Response
		expect  *pbpeering.ReplicationMessage
		wantErr bool
	}

	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)
	srv := NewService(testutil.Logger(t), &testStreamBackend{
		store: store,
		pub:   publisher,
	})

	run := func(t *testing.T, tc testCase) {
		reply, err := srv.processResponse("", "", tc.in)
		if tc.wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, tc.expect, reply)
	}

	tt := []testCase{
		{
			name: "valid upsert",
			in: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				ResourceID:  "api",
				Nonce:       "1",
				Operation:   pbpeering.ReplicationMessage_Response_UPSERT,
			},
			expect: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						ResourceURL: pbpeering.TypeURLService,
						Nonce:       "1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid delete",
			in: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				ResourceID:  "api",
				Nonce:       "1",
				Operation:   pbpeering.ReplicationMessage_Response_DELETE,
			},
			expect: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						ResourceURL: pbpeering.TypeURLService,
						Nonce:       "1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid resource url",
			in: &pbpeering.ReplicationMessage_Response{
				ResourceURL: "nomad.Job",
				Nonce:       "1",
				Operation:   pbpeering.ReplicationMessage_Response_Unknown,
			},
			expect: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						ResourceURL: "nomad.Job",
						Nonce:       "1",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: `received response for unknown resource type "nomad.Job"`,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unknown operation",
			in: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				Nonce:       "1",
				Operation:   pbpeering.ReplicationMessage_Response_Unknown,
			},
			expect: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						ResourceURL: pbpeering.TypeURLService,
						Nonce:       "1",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: `unsupported operation: "Unknown"`,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "out of range operation",
			in: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				Nonce:       "1",
				Operation:   pbpeering.ReplicationMessage_Response_Operation(100000),
			},
			expect: &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Request_{
					Request: &pbpeering.ReplicationMessage_Request{
						ResourceURL: pbpeering.TypeURLService,
						Nonce:       "1",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: `unsupported operation: "100000"`,
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
