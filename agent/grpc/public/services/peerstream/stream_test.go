package peerstream

// TODO: rename this file to replication_test.go

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbpeerstream"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbstatus"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/types"
)

func TestStreamResources_Server_Follower(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		backend := c.Backend.(*testStreamBackend)
		backend.leader = func() bool {
			return false
		}
		backend.leaderAddr = "expected:address"
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

	// expect error
	msg, err := client.Recv()
	require.Nil(t, msg)
	require.Error(t, err)
	require.EqualError(t, err, "rpc error: code = FailedPrecondition desc = cannot establish a peering stream on a follower node")

	// expect a status error
	st, ok := status.FromError(err)
	require.True(t, ok, "need to get back a grpc status error")
	deets := st.Details()

	// expect a LeaderAddress message
	exp := []interface{}{&pbpeerstream.LeaderAddress{Address: "expected:address"}}
	prototest.AssertDeepEqual(t, exp, deets)
}

// TestStreamResources_Server_LeaderBecomesFollower simulates a srv that is a leader when the
// subscription request is sent but loses leadership status for subsequent messages.
func TestStreamResources_Server_LeaderBecomesFollower(t *testing.T) {
	srv, store := newTestServer(t, func(c *Config) {
		backend := c.Backend.(*testStreamBackend)

		first := true
		backend.leader = func() bool {
			if first {
				first = false
				return true
			}
			return false
		}

		backend.leaderAddr = "expected:address"
	})

	client := NewMockClient(context.Background())

	errCh := make(chan error, 1)
	client.ErrCh = errCh

	go func() {
		err := srv.StreamResources(client.ReplicationStream)
		if err != nil {
			errCh <- err
		}
	}()

	p := writeEstablishedPeering(t, store, 1, "my-peer")
	peerID := p.ID

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	// Receive a subscription from a peer
	sub := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Request_{
			Request: &pbpeerstream.ReplicationMessage_Request{
				PeerID:      peerID,
				ResourceURL: pbpeerstream.TypeURLService,
			},
		},
	}
	err := client.Send(sub)
	require.NoError(t, err)

	msg, err := client.Recv()
	require.NoError(t, err)
	require.NotEmpty(t, msg)

	receiveRoots, err := client.Recv()
	require.NoError(t, err)
	require.NotNil(t, receiveRoots.GetResponse())
	require.Equal(t, pbpeerstream.TypeURLRoots, receiveRoots.GetResponse().ResourceURL)

	input2 := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Request_{
			Request: &pbpeerstream.ReplicationMessage_Request{
				ResourceURL: pbpeerstream.TypeURLService,
				Nonce:       "1",
			},
		},
	}

	err2 := client.Send(input2)
	require.NoError(t, err2)

	// expect error
	msg2, err2 := client.Recv()
	require.Nil(t, msg2)
	require.Error(t, err2)
	require.EqualError(t, err2, "rpc error: code = FailedPrecondition desc = node is not a leader anymore; cannot continue streaming")

	// expect a status error
	st, ok := status.FromError(err2)
	require.True(t, ok, "need to get back a grpc status error")
	deets := st.Details()

	// expect a LeaderAddress message
	exp := []interface{}{&pbpeerstream.LeaderAddress{Address: "expected:address"}}
	prototest.AssertDeepEqual(t, exp, deets)
}

func TestStreamResources_Server_FirstRequest(t *testing.T) {
	type testCase struct {
		name    string
		input   *pbpeerstream.ReplicationMessage
		wantErr error
	}

	run := func(t *testing.T, tc testCase) {
		srv, _ := newTestServer(t, nil)

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
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Response_{
					Response: &pbpeerstream.ReplicationMessage_Response{
						ResourceURL: pbpeerstream.TypeURLService,
						ResourceID:  "api-service",
						Nonce:       "2",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "first message when initiating a peering must be a subscription request"),
		},
		{
			name: "missing peer id",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "initial subscription request must specify a PeerID"),
		},
		{
			name: "unexpected nonce",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						PeerID: "63b60245-c475-426b-b314-4588d210859d",
						Nonce:  "1",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "initial subscription request must not contain a nonce"),
		},
		{
			name: "unknown resource",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						PeerID:      "63b60245-c475-426b-b314-4588d210859d",
						ResourceURL: "nomad.Job",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "subscription request to unknown resource URL: nomad.Job"),
		},
		{
			name: "unknown peer",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						PeerID:      "63b60245-c475-426b-b314-4588d210859d",
						ResourceURL: pbpeerstream.TypeURLService,
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
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	srv, store := newTestServer(t, func(c *Config) {
		c.Tracker.SetClock(it.Now)
	})

	p := writeEstablishedPeering(t, store, 1, "my-peer")
	var (
		peerID       = p.ID     // for Send
		remotePeerID = p.PeerID // for Recv
	)

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, peerID, remotePeerID)

	// TODO(peering): test fails if we don't drain the stream with this call because the
	// server gets blocked sending the termination message. Figure out a way to let
	// messages queue and filter replication messages.
	receiveRoots, err := client.Recv()
	require.NoError(t, err)
	require.NotNil(t, receiveRoots.GetResponse())
	require.Equal(t, pbpeerstream.TypeURLRoots, receiveRoots.GetResponse().ResourceURL)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

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
	expect := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Terminated_{
			Terminated: &pbpeerstream.ReplicationMessage_Terminated{},
		},
	}
	prototest.AssertDeepEqual(t, expect, receivedTerm)
}

func TestStreamResources_Server_StreamTracker(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	srv, store := newTestServer(t, func(c *Config) {
		c.Tracker.SetClock(it.Now)
	})

	// Set the initial roots and CA configuration.
	_, rootA := writeInitialRootsAndCA(t, store)

	p := writeEstablishedPeering(t, store, 1, "my-peer")
	var (
		peerID       = p.ID     // for Send
		remotePeerID = p.PeerID // for Recv
	)

	client := makeClient(t, srv, peerID, remotePeerID)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(peerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	var sequence uint64
	var lastSendSuccess time.Time

	testutil.RunStep(t, "ack tracked as success", func(t *testing.T) {
		ack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					PeerID:      peerID,
					ResourceURL: pbpeerstream.TypeURLService,
					Nonce:       "1",

					// Acks do not have an Error populated in the request
				},
			},
		}
		err := client.Send(ack)
		require.NoError(t, err)
		sequence++

		lastSendSuccess = it.base.Add(time.Duration(sequence) * time.Second).UTC()

		expect := Status{
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
		nack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					PeerID:      peerID,
					ResourceURL: pbpeerstream.TypeURLService,
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

		expect := Status{
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
		resp := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Response_{
				Response: &pbpeerstream.ReplicationMessage_Response{
					ResourceURL: pbpeerstream.TypeURLService,
					ResourceID:  "api",
					Nonce:       "21",
					Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
					Resource:    makeAnyPB(t, &pbservice.IndexedCheckServiceNodes{}),
				},
			},
		}
		err := client.Send(resp)
		require.NoError(t, err)
		sequence++

		expectRoots := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Response_{
				Response: &pbpeerstream.ReplicationMessage_Response{
					ResourceURL: pbpeerstream.TypeURLRoots,
					ResourceID:  "roots",
					Resource: makeAnyPB(t, &pbpeering.PeeringTrustBundle{
						TrustDomain: connect.TestTrustDomain,
						RootPEMs:    []string{rootA.RootCert},
					}),
					Operation: pbpeerstream.Operation_OPERATION_UPSERT,
				},
			},
		}

		roots, err := client.Recv()
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, expectRoots, roots)

		ack, err := client.Recv()
		require.NoError(t, err)

		expectAck := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL: pbpeerstream.TypeURLService,
					Nonce:       "21",
				},
			},
		}
		prototest.AssertDeepEqual(t, expectAck, ack)

		lastRecvSuccess = it.base.Add(time.Duration(sequence) * time.Second).UTC()

		expect := Status{
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
		resp := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Response_{
				Response: &pbpeerstream.ReplicationMessage_Response{
					ResourceURL: pbpeerstream.TypeURLService,
					ResourceID:  "web",
					Nonce:       "24",

					// Unknown operation gets NACKed
					Operation: pbpeerstream.Operation_OPERATION_UNSPECIFIED,
				},
			},
		}
		err := client.Send(resp)
		require.NoError(t, err)
		sequence++

		ack, err := client.Recv()
		require.NoError(t, err)

		expectNack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL: pbpeerstream.TypeURLService,
					Nonce:       "24",
					Error: &pbstatus.Status{
						Code:    int32(code.Code_INVALID_ARGUMENT),
						Message: `unsupported operation: "OPERATION_UNSPECIFIED"`,
					},
				},
			},
		}
		prototest.AssertDeepEqual(t, expectNack, ack)

		lastRecvError = it.base.Add(time.Duration(sequence) * time.Second).UTC()
		lastRecvErrorMsg = `unsupported operation: "OPERATION_UNSPECIFIED"`

		expect := Status{
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

		expect := Status{
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
}

func TestStreamResources_Server_ServiceUpdates(t *testing.T) {
	srv, store := newTestServer(t, nil)

	// Create a peering
	var lastIdx uint64 = 1
	p := writeEstablishedPeering(t, store, lastIdx, "my-peering")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, p.ID, p.PeerID)

	// Register a service that is not yet exported
	mysql := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
	}

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, mysql.Node))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "foo", mysql.Service))

	mongoSvcDefaults := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "mongo",
		Protocol: "grpc",
	}
	require.NoError(t, mongoSvcDefaults.Normalize())
	require.NoError(t, mongoSvcDefaults.Validate())
	lastIdx++
	require.NoError(t, store.EnsureConfigEntry(lastIdx, mongoSvcDefaults))

	// NOTE: for this test we'll just live in a fantasy realm where we assume
	// that mongo understands gRPC
	var (
		mongoSN      = structs.NewServiceName("mongo", nil).String()
		mongoProxySN = structs.NewServiceName("mongo-sidecar-proxy", nil).String()
		mysqlSN      = structs.NewServiceName("mysql", nil).String()
		mysqlProxySN = structs.NewServiceName("mysql-sidecar-proxy", nil).String()
	)

	testutil.RunStep(t, "exporting mysql leads to an UPSERT event", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					// Mongo does not get pushed because it does not have instances registered.
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
			},
		}
		require.NoError(t, entry.Normalize())
		require.NoError(t, entry.Validate())
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, entry))

		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLRoots, msg.GetResponse().ResourceURL)
				// Roots tested in TestStreamResources_Server_CARootUpdates
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				// no mongo instances exist
				require.Equal(t, pbpeerstream.TypeURLService, msg.GetResponse().ResourceURL)
				require.Equal(t, mongoSN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_DELETE, msg.GetResponse().Operation)
				require.Nil(t, msg.GetResponse().Resource)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				// proxies can't export because no mesh gateway exists yet
				require.Equal(t, pbpeerstream.TypeURLService, msg.GetResponse().ResourceURL)
				require.Equal(t, mongoProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_DELETE, msg.GetResponse().Operation)
				require.Nil(t, msg.GetResponse().Resource)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLService, msg.GetResponse().ResourceURL)
				require.Equal(t, mysqlSN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbservice.IndexedCheckServiceNodes
				require.NoError(t, ptypes.UnmarshalAny(msg.GetResponse().Resource, &nodes))
				require.Len(t, nodes.Nodes, 1)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				// proxies can't export because no mesh gateway exists yet
				require.Equal(t, pbpeerstream.TypeURLService, msg.GetResponse().ResourceURL)
				require.Equal(t, mysqlProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_DELETE, msg.GetResponse().Operation)
				require.Nil(t, msg.GetResponse().Resource)
			},
		)
	})

	testutil.RunStep(t, "register mesh gateway to send proxy updates", func(t *testing.T) {
		gateway := &structs.CheckServiceNode{Node: &structs.Node{Node: "mgw", Address: "10.1.1.1"},
			Service: &structs.NodeService{ID: "gateway-1", Kind: structs.ServiceKindMeshGateway, Service: "gateway", Port: 8443},
			// TODO: checks
		}

		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, gateway.Node))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "mgw", gateway.Service))

		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLService, msg.GetResponse().ResourceURL)
				require.Equal(t, mongoProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbservice.IndexedCheckServiceNodes
				require.NoError(t, ptypes.UnmarshalAny(msg.GetResponse().Resource, &nodes))
				require.Len(t, nodes.Nodes, 1)

				pm := nodes.Nodes[0].Service.Connect.PeerMeta
				require.Equal(t, "grpc", pm.Protocol)
				spiffeIDs := []string{
					"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/mongo",
					"spiffe://11111111-2222-3333-4444-555555555555.consul/gateway/mesh/dc/dc1",
				}
				require.Equal(t, spiffeIDs, pm.SpiffeID)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLService, msg.GetResponse().ResourceURL)
				require.Equal(t, mysqlProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbservice.IndexedCheckServiceNodes
				require.NoError(t, ptypes.UnmarshalAny(msg.GetResponse().Resource, &nodes))
				require.Len(t, nodes.Nodes, 1)

				pm := nodes.Nodes[0].Service.Connect.PeerMeta
				require.Equal(t, "tcp", pm.Protocol)
				spiffeIDs := []string{
					"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/mysql",
					"spiffe://11111111-2222-3333-4444-555555555555.consul/gateway/mesh/dc/dc1",
				}
				require.Equal(t, spiffeIDs, pm.SpiffeID)
			},
		)
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
			require.Equal(r, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)
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
		require.NoError(t, entry.Normalize())
		require.NoError(t, entry.Validate())
		lastIdx++
		err := store.EnsureConfigEntry(lastIdx, entry)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeerstream.Operation_OPERATION_DELETE, msg.GetResponse().Operation)
			require.Equal(r, mysql.Service.CompoundServiceName().String(), msg.GetResponse().ResourceID)
			require.Nil(r, msg.GetResponse().Resource)
		})
	})

	testutil.RunStep(t, "deleting the config entry leads to a DELETE event for mongo", func(t *testing.T) {
		lastIdx++
		err := store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", nil)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeerstream.Operation_OPERATION_DELETE, msg.GetResponse().Operation)
			require.Equal(r, mongo.Service.CompoundServiceName().String(), msg.GetResponse().ResourceID)
			require.Nil(r, msg.GetResponse().Resource)
		})
	})
}

func TestStreamResources_Server_CARootUpdates(t *testing.T) {
	srv, store := newTestServer(t, nil)

	// Create a peering
	var lastIdx uint64 = 1
	p := writeEstablishedPeering(t, store, lastIdx, "my-peering")

	// Set the initial roots and CA configuration.
	clusterID, rootA := writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, p.ID, p.PeerID)

	testutil.RunStep(t, "initial CA Roots replication", func(t *testing.T) {
		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLRoots, msg.GetResponse().ResourceURL)
				require.Equal(t, "roots", msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var trustBundle pbpeering.PeeringTrustBundle
				require.NoError(t, ptypes.UnmarshalAny(msg.GetResponse().Resource, &trustBundle))

				require.ElementsMatch(t, []string{rootA.RootCert}, trustBundle.RootPEMs)
				expect := connect.SpiffeIDSigningForCluster(clusterID).Host()
				require.Equal(t, expect, trustBundle.TrustDomain)
			},
		)
	})

	testutil.RunStep(t, "CA root rotation sends upsert event", func(t *testing.T) {
		// get max index for CAS operation
		cidx, _, err := store.CARoots(nil)
		require.NoError(t, err)

		rootB := connect.TestCA(t, nil)
		rootC := connect.TestCA(t, nil)
		rootC.Active = false // there can only be one active root
		lastIdx++
		set, err := store.CARootSetCAS(lastIdx, cidx, []*structs.CARoot{rootB, rootC})
		require.True(t, set)
		require.NoError(t, err)

		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLRoots, msg.GetResponse().ResourceURL)
				require.Equal(t, "roots", msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var trustBundle pbpeering.PeeringTrustBundle
				require.NoError(t, ptypes.UnmarshalAny(msg.GetResponse().Resource, &trustBundle))

				require.ElementsMatch(t, []string{rootB.RootCert, rootC.RootCert}, trustBundle.RootPEMs)
				expect := connect.SpiffeIDSigningForCluster(clusterID).Host()
				require.Equal(t, expect, trustBundle.TrustDomain)
			},
		)
	})
}

// makeClient sets up a *MockClient with the initial subscription
// message handshake.
func makeClient(
	t *testing.T,
	srv pbpeerstream.PeerStreamServiceServer,
	peerID string,
	remotePeerID string,
) *MockClient {
	t.Helper()

	client := NewMockClient(context.Background())

	errCh := make(chan error, 1)
	client.ErrCh = errCh

	go func() {
		// Pass errors from server handler into ErrCh so that they can be seen by the client on Recv().
		// This matches gRPC's behavior when an error is returned by a server.
		if err := srv.StreamResources(client.ReplicationStream); err != nil {
			errCh <- srv.StreamResources(client.ReplicationStream)
		}
	}()

	// Issue a services subscription to server
	init := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Request_{
			Request: &pbpeerstream.ReplicationMessage_Request{
				PeerID:      peerID,
				ResourceURL: pbpeerstream.TypeURLService,
			},
		},
	}
	require.NoError(t, client.Send(init))

	// Receive a services subscription from server
	receivedSub, err := client.Recv()
	require.NoError(t, err)

	expect := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Request_{
			Request: &pbpeerstream.ReplicationMessage_Request{
				ResourceURL: pbpeerstream.TypeURLService,
				PeerID:      remotePeerID,
			},
		},
	}
	prototest.AssertDeepEqual(t, expect, receivedSub)

	return client
}

type testStreamBackend struct {
	pub    state.EventPublisher
	store  *state.Store
	leader func() bool

	leaderAddrLock sync.Mutex
	leaderAddr     string
}

var _ Backend = (*testStreamBackend)(nil)

func (b *testStreamBackend) IsLeader() bool {
	if b.leader != nil {
		return b.leader()
	}
	return true
}

func (b *testStreamBackend) SetLeaderAddress(addr string) {
	b.leaderAddrLock.Lock()
	defer b.leaderAddrLock.Unlock()
	b.leaderAddr = addr
}

func (b *testStreamBackend) GetLeaderAddress() string {
	b.leaderAddrLock.Lock()
	defer b.leaderAddrLock.Unlock()
	return b.leaderAddr
}

func (b *testStreamBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return b.pub.Subscribe(req)
}

func (b *testStreamBackend) PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error {
	panic("not implemented")
}

func (b *testStreamBackend) PeeringTrustBundleWrite(req *pbpeering.PeeringTrustBundleWriteRequest) error {
	panic("not implemented")
}

// CatalogRegister mocks catalog registrations through Raft by copying the logic of FSM.applyRegister.
func (b *testStreamBackend) CatalogRegister(req *structs.RegisterRequest) error {
	return b.store.EnsureRegistration(1, req)
}

// CatalogDeregister mocks catalog de-registrations through Raft by copying the logic of FSM.applyDeregister.
func (b *testStreamBackend) CatalogDeregister(req *structs.DeregisterRequest) error {
	if req.ServiceID != "" {
		if err := b.store.DeleteService(1, req.Node, req.ServiceID, &req.EnterpriseMeta, req.PeerName); err != nil {
			return err
		}
	} else if req.CheckID != "" {
		if err := b.store.DeleteCheck(1, req.Node, req.CheckID, &req.EnterpriseMeta, req.PeerName); err != nil {
			return err
		}
	} else {
		if err := b.store.DeleteNode(1, req.Node, &req.EnterpriseMeta, req.PeerName); err != nil {
			return err
		}
	}
	return nil
}

func Test_processResponse_Validation(t *testing.T) {
	type testCase struct {
		name    string
		in      *pbpeerstream.ReplicationMessage_Response
		expect  *pbpeerstream.ReplicationMessage
		wantErr bool
	}

	srv, _ := newTestServer(t, nil)

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
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLService,
				ResourceID:  "api",
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
				Resource:    makeAnyPB(t, &pbservice.IndexedCheckServiceNodes{}),
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL: pbpeerstream.TypeURLService,
						Nonce:       "1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid delete",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLService,
				ResourceID:  "api",
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_DELETE,
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL: pbpeerstream.TypeURLService,
						Nonce:       "1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid resource url",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: "nomad.Job",
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_UNSPECIFIED,
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
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
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLService,
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_UNSPECIFIED,
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL: pbpeerstream.TypeURLService,
						Nonce:       "1",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: `unsupported operation: "OPERATION_UNSPECIFIED"`,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "out of range operation",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLService,
				Nonce:       "1",
				Operation:   pbpeerstream.Operation(100000),
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL: pbpeerstream.TypeURLService,
						Nonce:       "1",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: `unsupported operation: 100000`,
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

// writeEstablishedPeering creates a peering with the provided name and ensures
// the PeerID field is set for the ID of the remote peer.
func writeEstablishedPeering(t *testing.T, store *state.Store, idx uint64, peerName string) *pbpeering.Peering {
	remotePeerID, err := uuid.GenerateUUID()
	require.NoError(t, err)

	peering := pbpeering.Peering{
		ID:     testUUID(t),
		Name:   peerName,
		PeerID: remotePeerID,
	}
	require.NoError(t, store.PeeringWrite(idx, &peering))

	_, p, err := store.PeeringRead(nil, state.Query{Value: peerName})
	require.NoError(t, err)

	return p
}

func writeInitialRootsAndCA(t *testing.T, store *state.Store) (string, *structs.CARoot) {
	const clusterID = connect.TestClusterID

	rootA := connect.TestCA(t, nil)
	_, err := store.CARootSetCAS(1, 0, structs.CARoots{rootA})
	require.NoError(t, err)

	err = store.CASetConfig(0, &structs.CAConfiguration{ClusterID: clusterID})
	require.NoError(t, err)

	return clusterID, rootA
}

func makeAnyPB(t *testing.T, pb proto.Message) *any.Any {
	any, err := ptypes.MarshalAny(pb)
	require.NoError(t, err)
	return any
}

func expectReplEvents(t *testing.T, client *MockClient, checkFns ...func(t *testing.T, got *pbpeerstream.ReplicationMessage)) {
	t.Helper()

	num := len(checkFns)

	if num == 0 {
		// No updates should be received.
		msg, err := client.RecvWithTimeout(100 * time.Millisecond)
		if err == io.EOF && msg == nil {
			return
		} else if err != nil {
			t.Fatalf("received unexpected update error: %v", err)
		} else {
			t.Fatalf("received unexpected update: %+v", msg)
		}
	}

	const timeout = 10 * time.Second

	var out []*pbpeerstream.ReplicationMessage
	for len(out) < num {
		msg, err := client.RecvWithTimeout(timeout)
		if err == io.EOF && msg == nil {
			t.Fatalf("timed out with %d of %d events", len(out), num)
		}
		require.NoError(t, err)
		out = append(out, msg)
	}

	if msg, err := client.RecvWithTimeout(100 * time.Millisecond); err != io.EOF || msg != nil {
		t.Fatalf("expected only %d events but got more; prev %+v; next %+v", num, out, msg)
	}

	require.Len(t, out, num)

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]

		typeA := fmt.Sprintf("%T", a.GetPayload())
		typeB := fmt.Sprintf("%T", b.GetPayload())
		if typeA != typeB {
			return typeA < typeB
		}

		switch a.GetPayload().(type) {
		case *pbpeerstream.ReplicationMessage_Request_:
			reqA, reqB := a.GetRequest(), b.GetRequest()
			if reqA.ResourceURL != reqB.ResourceURL {
				return reqA.ResourceURL < reqB.ResourceURL
			}
			return reqA.Nonce < reqB.Nonce

		case *pbpeerstream.ReplicationMessage_Response_:
			respA, respB := a.GetResponse(), b.GetResponse()
			if respA.ResourceURL != respB.ResourceURL {
				return respA.ResourceURL < respB.ResourceURL
			}
			if respA.ResourceID != respB.ResourceID {
				return respA.ResourceID < respB.ResourceID
			}
			return respA.Nonce < respB.Nonce

		case *pbpeerstream.ReplicationMessage_Terminated_:
			return false

		default:
			panic("unknown type")
		}
	})

	for i := 0; i < num; i++ {
		checkFns[i](t, out[i])
	}
}

func TestHandleUpdateService(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		backend := c.Backend.(*testStreamBackend)
		backend.leader = func() bool {
			return false
		}
	})

	type testCase struct {
		name   string
		seed   []*structs.RegisterRequest
		input  *pbservice.IndexedCheckServiceNodes
		expect map[string]structs.CheckServiceNodes
	}

	peerName := "billing"
	remoteMeta := pbcommon.NewEnterpriseMetaFromStructs(*structs.DefaultEnterpriseMetaInPartition("billing-ap"))

	// "api" service is imported from the billing-ap partition, corresponding to the billing peer.
	// Locally it is stored to the default partition.
	defaultMeta := *acl.DefaultEnterpriseMeta()
	apiSN := structs.NewServiceName("api", &defaultMeta)

	run := func(t *testing.T, tc testCase) {
		// Seed the local catalog with some data to reconcile against.
		for _, reg := range tc.seed {
			require.NoError(t, srv.Backend.CatalogRegister(reg))
		}

		// Simulate an update arriving for billing/api.
		require.NoError(t, srv.handleUpdateService(peerName, acl.DefaultPartitionName, apiSN, tc.input))

		for svc, expect := range tc.expect {
			t.Run(svc, func(t *testing.T) {
				_, got, err := srv.GetStore().CheckServiceNodes(nil, svc, &defaultMeta, peerName)
				require.NoError(t, err)
				requireEqualInstances(t, expect, got)
			})
		}
	}

	tt := []testCase{
		{
			name: "upsert two service instances to the same node",
			input: &pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
			expect: map[string]structs.CheckServiceNodes{
				"api": {
					{
						Node: &structs.Node{
							ID:   "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node: "node-foo",

							// The remote billing-ap partition is overwritten for all resources with the local default.
							Partition: defaultMeta.PartitionOrEmpty(),

							// The name of the peer "billing" is attached as well.
							PeerName: peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name: "upsert two service instances to different nodes",
			input: &pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &pbservice.Node{
							ID:        "c0f97de9-4e1b-4e80-a1c6-cd8725835ab2",
							Node:      "node-bar",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-bar-check",
								Node:           "node-bar",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-bar",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
			expect: map[string]structs.CheckServiceNodes{
				"api": {
					{
						Node: &structs.Node{
							ID:        "c0f97de9-4e1b-4e80-a1c6-cd8725835ab2",
							Node:      "node-bar",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-bar-check",
								Node:           "node-bar",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-bar",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &structs.Node{
							ID:   "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node: "node-foo",

							// The remote billing-ap partition is overwritten for all resources with the local default.
							Partition: defaultMeta.PartitionOrEmpty(),

							// The name of the peer "billing" is attached as well.
							PeerName: peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name: "receiving a nil input leads to deleting data in the catalog",
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("c0f97de9-4e1b-4e80-a1c6-cd8725835ab2"),
					Node:     "node-bar",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-2",
						Service:        "api",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-bar",
							ServiceID: "api-2",
							CheckID:   types.CheckID("api-2-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-bar",
							CheckID:  types.CheckID("node-bar-check"),
							PeerName: peerName,
						},
					},
				},
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1",
						Service:        "api",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-1",
							CheckID:   types.CheckID("api-1-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
			},
			input: nil,
			expect: map[string]structs.CheckServiceNodes{
				"api": {},
			},
		},
		{
			name: "deleting one service name from a node does not delete other service names",
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "redis-2",
							CheckID:   types.CheckID("redis-2-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1",
						Service:        "api",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-1",
							CheckID:   types.CheckID("api-1-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
			},
			// Nil input is for the "api" service.
			input: nil,
			expect: map[string]structs.CheckServiceNodes{
				"api": {},
				// Existing redis service was not affected by deletion.
				"redis": {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "redis-2-check",
								ServiceID:      "redis-2",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name: "service checks are cleaned up when not present in a response",
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1",
						Service:        "api",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-1",
							CheckID:   types.CheckID("api-1-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
			},
			input: &pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							// Service check was deleted
						},
					},
				},
			},
			expect: map[string]structs.CheckServiceNodes{
				// Service check should be gone
				"api": {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{},
					},
				},
			},
		},
		{
			name: "node checks are cleaned up when not present in a response",
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "redis-2",
							CheckID:   types.CheckID("redis-2-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1",
						Service:        "api",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-1",
							CheckID:   types.CheckID("api-1-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
			},
			input: &pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							// Node check was deleted
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: remoteMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
			expect: map[string]structs.CheckServiceNodes{
				// Node check should be gone
				"api": {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
				"redis": {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "redis-2-check",
								ServiceID:      "redis-2",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name: "replacing a service instance on a node cleans up the old instance",
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "redis-2",
							CheckID:   types.CheckID("redis-2-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1",
						Service:        "api",
						EnterpriseMeta: defaultMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-1",
							CheckID:   types.CheckID("api-1-check"),
							PeerName:  peerName,
						},
						{
							Node:     "node-foo",
							CheckID:  types.CheckID("node-foo-check"),
							PeerName: peerName,
						},
					},
				},
			},
			input: &pbservice.IndexedCheckServiceNodes{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: remoteMeta.Partition,
							PeerName:  peerName,
						},
						// New service ID and checks for the api service.
						Service: &pbservice.NodeService{
							ID:             "new-api-v2",
							Service:        "api",
							EnterpriseMeta: remoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								Node:      "node-foo",
								ServiceID: "new-api-v2",
								CheckID:   "new-api-v2-check",
								PeerName:  peerName,
							},
							{
								Node:     "node-foo",
								CheckID:  "node-foo-check",
								PeerName: peerName,
							},
						},
					},
				},
			},
			expect: map[string]structs.CheckServiceNodes{
				"api": {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "new-api-v2",
							Service:        "api",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								Node:     "node-foo",
								CheckID:  "node-foo-check",
								PeerName: peerName,
							},
							{
								CheckID:        "new-api-v2-check",
								ServiceID:      "new-api-v2",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
				"redis": {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: defaultMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: defaultMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								Node:     "node-foo",
								CheckID:  "node-foo-check",
								PeerName: peerName,
							},
							{
								CheckID:        "redis-2-check",
								ServiceID:      "redis-2",
								Node:           "node-foo",
								EnterpriseMeta: defaultMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func requireEqualInstances(t *testing.T, expect, got structs.CheckServiceNodes) {
	t.Helper()

	require.Equal(t, len(expect), len(got), "got differing number of instances")

	for i := range expect {
		// Node equality
		require.Equal(t, expect[i].Node.ID, got[i].Node.ID, "node mismatch")
		require.Equal(t, expect[i].Node.Partition, got[i].Node.Partition, "partition mismatch")
		require.Equal(t, expect[i].Node.PeerName, got[i].Node.PeerName, "peer name mismatch")

		// Service equality
		require.Equal(t, expect[i].Service.ID, got[i].Service.ID, "service id mismatch")
		require.Equal(t, expect[i].Service.PeerName, got[i].Service.PeerName, "peer name mismatch")
		require.Equal(t, expect[i].Service.PartitionOrDefault(), got[i].Service.PartitionOrDefault(), "partition mismatch")

		// Check equality
		require.Equal(t, len(expect[i].Checks), len(got[i].Checks), "got differing number of check")

		for j := range expect[i].Checks {
			require.Equal(t, expect[i].Checks[j].CheckID, got[i].Checks[j].CheckID, "check id mismatch")
			require.Equal(t, expect[i].Checks[j].PeerName, got[i].Checks[j].PeerName, "peer name mismatch")
			require.Equal(t, expect[i].Checks[j].PartitionOrDefault(), got[i].Checks[j].PartitionOrDefault(), "partition mismatch")
		}
	}
}

type testServer struct {
	*Server
}

func newTestServer(t *testing.T, configFn func(c *Config)) (*testServer, *state.Store) {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	ports := freeport.GetN(t, 1) // {grpc}

	cfg := Config{
		Backend: &testStreamBackend{
			store: store,
			pub:   publisher,
		},
		Tracker:        NewTracker(),
		GetStore:       func() StateStore { return store },
		Logger:         testutil.Logger(t),
		ACLResolver:    nil, // TODO(peering): add something for acl testing
		Datacenter:     "dc1",
		ConnectEnabled: true,
	}
	if configFn != nil {
		configFn(&cfg)
	}

	grpcServer := grpc.NewServer()

	srv := NewServer(cfg)
	srv.Register(grpcServer)

	var (
		grpcPort = ports[0]
		grpcAddr = fmt.Sprintf("127.0.0.1:%d", grpcPort)
	)
	ln, err := net.Listen("tcp", grpcAddr)
	require.NoError(t, err)
	go func() {
		_ = grpcServer.Serve(ln)
	}()
	t.Cleanup(grpcServer.Stop)

	return &testServer{
		Server: srv,
	}, store
}

func testUUID(t *testing.T) string {
	v, err := lib.GenerateUUID(nil)
	require.NoError(t, err)
	return v
}

func noopForwardRPC(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error) {
	return false, nil
}
