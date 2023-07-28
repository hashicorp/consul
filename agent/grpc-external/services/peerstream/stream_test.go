package peerstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	newproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
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

const (
	testPeerID                = "caf067a6-f112-4907-9101-d45857d2b149"
	testPendingStreamSecretID = "522c0daf-2ef2-4dab-bc78-5e04e3daf552"
	testEstablishmentSecretID = "f6569d37-1c5b-4415-aae5-26f4594f7f60"
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

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

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

	// Receive a subscription from a peer. This message arrives while the
	// server is a leader and should work.
	testutil.RunStep(t, "send subscription request to leader and consume its three requests", func(t *testing.T) {
		sub := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Open_{
				Open: &pbpeerstream.ReplicationMessage_Open{
					PeerID:         testPeerID,
					StreamSecretID: testPendingStreamSecretID,
				},
			},
		}
		err := client.Send(sub)
		require.NoError(t, err)

		msg1, err := client.Recv()
		require.NoError(t, err)
		require.NotEmpty(t, msg1)

		msg2, err := client.Recv()
		require.NoError(t, err)
		require.NotEmpty(t, msg2)

		msg3, err := client.Recv()
		require.NoError(t, err)
		require.NotEmpty(t, msg3)
	})

	// The ACK will be a new request but at this point the server is not the
	// leader in the test and this should fail.
	testutil.RunStep(t, "ack fails with non leader", func(t *testing.T) {
		ack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL:   pbpeerstream.TypeURLExportedService,
					ResponseNonce: "1",
				},
			},
		}

		err := client.Send(ack)
		require.NoError(t, err)

		// expect error
		msg, err := client.Recv()
		require.Nil(t, msg)
		require.Error(t, err)
		require.EqualError(t, err, "rpc error: code = FailedPrecondition desc = node is not a leader anymore; cannot continue streaming")

		// expect a status error
		st, ok := status.FromError(err)
		require.True(t, ok, "need to get back a grpc status error")

		// expect a LeaderAddress message
		expect := []interface{}{
			&pbpeerstream.LeaderAddress{Address: "expected:address"},
		}
		prototest.AssertDeepEqual(t, expect, st.Details())
	})
}

func TestStreamResources_Server_ActiveSecretValidation(t *testing.T) {
	type testSeed struct {
		peering *pbpeering.Peering
		secrets []*pbpeering.SecretsWriteRequest
	}
	type testCase struct {
		name    string
		seed    *testSeed
		input   *pbpeerstream.ReplicationMessage
		wantErr error
	}

	peeringWithoutSecrets := "35bf39d2-836c-4f66-945f-85f20b17c3db"

	run := func(t *testing.T, tc testCase) {
		srv, store := newTestServer(t, nil)

		// Write a seed peering.
		if tc.seed != nil {
			require.NoError(t, store.PeeringWrite(1, &pbpeering.PeeringWriteRequest{Peering: tc.seed.peering}))

			for _, s := range tc.seed.secrets {
				require.NoError(t, store.PeeringSecretsWrite(1, s))
			}
		}

		// Set the initial roots and CA configuration.
		_, _ = writeInitialRootsAndCA(t, store)

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

		_, err = client.Recv()
		if tc.wantErr != nil {
			require.Error(t, err)
			require.EqualError(t, err, tc.wantErr.Error())
		} else {
			require.NoError(t, err)
		}

		client.Close()
	}
	tt := []testCase{
		{
			name: "no secret for peering",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   peeringWithoutSecrets,
				},
			},
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{
						PeerID: peeringWithoutSecrets,
					},
				},
			},
			wantErr: status.Error(codes.Internal, "unable to authorize connection, peering must be re-established"),
		},
		{
			name: "unknown secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testPeerID,
				},
				secrets: []*pbpeering.SecretsWriteRequest{
					{
						PeerID: testPeerID,
						Request: &pbpeering.SecretsWriteRequest_GenerateToken{
							GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
								EstablishmentSecret: testEstablishmentSecretID,
							},
						},
					},
				},
			},
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{
						PeerID:         testPeerID,
						StreamSecretID: "unknown-secret",
					},
				},
			},
			wantErr: status.Error(codes.PermissionDenied, "invalid peering stream secret"),
		},
		{
			name: "known pending secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testPeerID,
				},
				secrets: []*pbpeering.SecretsWriteRequest{
					{
						PeerID: testPeerID,
						Request: &pbpeering.SecretsWriteRequest_GenerateToken{
							GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
								EstablishmentSecret: testEstablishmentSecretID,
							},
						},
					},
					{
						PeerID: testPeerID,
						Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
							ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
								EstablishmentSecret: testEstablishmentSecretID,
								PendingStreamSecret: testPendingStreamSecretID,
							},
						},
					},
				},
			},
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{
						PeerID:         testPeerID,
						StreamSecretID: testPendingStreamSecretID,
					},
				},
			},
		},
		{
			name: "known active secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testPeerID,
				},
				secrets: []*pbpeering.SecretsWriteRequest{
					{
						PeerID: testPeerID,
						Request: &pbpeering.SecretsWriteRequest_GenerateToken{
							GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
								EstablishmentSecret: testEstablishmentSecretID,
							},
						},
					},
					{
						PeerID: testPeerID,
						Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
							ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
								EstablishmentSecret: testEstablishmentSecretID,
								PendingStreamSecret: testPendingStreamSecretID,
							},
						},
					},
					{
						PeerID: testPeerID,
						Request: &pbpeering.SecretsWriteRequest_PromotePending{
							PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
								// Pending gets promoted to active.
								ActiveStreamSecret: testPendingStreamSecretID,
							},
						},
					},
				},
			},
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{
						PeerID:         testPeerID,
						StreamSecretID: testPendingStreamSecretID,
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

func TestStreamResources_Server_PendingSecretPromotion(t *testing.T) {
	srv, store := newTestServer(t, nil)
	_ = writePeeringToBeDialed(t, store, 1, "my-peer")

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

	err := client.Send(&pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Open_{
			Open: &pbpeerstream.ReplicationMessage_Open{
				PeerID:         testPeerID,
				StreamSecretID: testPendingStreamSecretID,
			},
		},
	})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		// Upon presenting a known pending secret ID, it should be promoted to active.
		secrets, err := store.PeeringSecretsRead(nil, testPeerID)
		require.NoError(r, err)
		require.Empty(r, secrets.GetStream().GetPendingSecretID())
		require.Equal(r, testPendingStreamSecretID, secrets.GetStream().GetActiveSecretID())
	})
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
						ResourceURL: pbpeerstream.TypeURLExportedService,
						ResourceID:  "api-service",
						Nonce:       "2",
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "first message when initiating a peering must be: Open"),
		},
		{
			name: "unexpected request",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL: pbpeerstream.TypeURLExportedService,
					},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "first message when initiating a peering must be: Open"),
		},
		{
			name: "missing peer id",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{},
				},
			},
			wantErr: status.Error(codes.InvalidArgument, "initial subscription request must specify a PeerID"),
		},
		{
			name: "unknown peer",
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{
						PeerID: "63b60245-c475-426b-b314-4588d210859d",
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

	srv, store := newTestServer(t, nil)
	srv.Tracker.setClock(it.Now)

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, testPeerID)

	client.DrainStream(t)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	testutil.RunStep(t, "terminate the stream", func(t *testing.T) {
		done := srv.ConnectedStreams()[testPeerID]
		close(done)

		retry.Run(t, func(r *retry.R) {
			_, ok := srv.StreamStatus(testPeerID)
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
	waitUntil := it.FutureNow(7)

	srv, store := newTestServer(t, nil)
	srv.Tracker.setClock(it.Now)

	// Set the initial roots and CA configuration.
	writeInitialRootsAndCA(t, store)

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	client := makeClient(t, srv, testPeerID)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	var lastSendAck time.Time
	var lastSendSuccess *time.Time

	client.DrainStream(t)

	// Wait for async workflows to complete.
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, waitUntil, it.FutureNow(1))
	})

	// Manually grab the last success time from sending the trust bundle or exported services list.
	status, ok := srv.StreamStatus(testPeerID)
	require.True(t, ok)
	lastSendSuccess = status.LastSendSuccess

	testutil.RunStep(t, "ack tracked as success", func(t *testing.T) {
		ack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					PeerID:        testPeerID,
					ResourceURL:   pbpeerstream.TypeURLExportedService,
					ResponseNonce: "1",

					// Acks do not have an Error populated in the request
				},
			},
		}

		lastSendAck = it.FutureNow(1)

		err := client.Send(ack)
		require.NoError(t, err)

		expect := Status{
			Connected:        true,
			LastSendSuccess:  lastSendSuccess,
			LastAck:          &lastSendAck,
			ExportedServices: []string{},
		}
		retry.Run(t, func(r *retry.R) {
			rStatus, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.Equal(r, expect, rStatus)
		})
	})

	var lastNack time.Time
	var lastNackMsg string

	testutil.RunStep(t, "nack tracked as error", func(t *testing.T) {
		nack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					PeerID:        testPeerID,
					ResourceURL:   pbpeerstream.TypeURLExportedService,
					ResponseNonce: "2",
					Error: &pbstatus.Status{
						Code:    int32(code.Code_UNAVAILABLE),
						Message: "bad bad not good",
					},
				},
			},
		}

		lastNack = it.FutureNow(1)
		err := client.Send(nack)
		require.NoError(t, err)

		lastNackMsg = "client peer was unable to apply resource: bad bad not good"

		expect := Status{
			Connected:        true,
			LastSendSuccess:  lastSendSuccess,
			LastAck:          &lastSendAck,
			LastNack:         &lastNack,
			LastNackMessage:  lastNackMsg,
			ExportedServices: []string{},
		}

		retry.Run(t, func(r *retry.R) {
			rStatus, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.Equal(r, expect, rStatus)
		})
	})

	var lastRecvResourceSuccess time.Time

	testutil.RunStep(t, "response applied locally", func(t *testing.T) {
		resp := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Response_{
				Response: &pbpeerstream.ReplicationMessage_Response{
					ResourceURL: pbpeerstream.TypeURLExportedService,
					ResourceID:  "api",
					Nonce:       "21",
					Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
					Resource:    makeAnyPB(t, &pbpeerstream.ExportedService{}),
				},
			},
		}
		lastRecvResourceSuccess = it.FutureNow(1)
		err := client.Send(resp)
		require.NoError(t, err)

		expectAck := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL:   pbpeerstream.TypeURLExportedService,
					ResponseNonce: "21",
				},
			},
		}

		retry.Run(t, func(r *retry.R) {
			msg, err := client.Recv()
			require.NoError(r, err)
			req := msg.GetRequest()
			require.NotNil(r, req)
			require.Equal(r, pbpeerstream.TypeURLExportedService, req.ResourceURL)
			prototest.AssertDeepEqual(t, expectAck, msg)
		})

		expect := Status{
			Connected:               true,
			LastSendSuccess:         lastSendSuccess,
			LastAck:                 &lastSendAck,
			LastNack:                &lastNack,
			LastNackMessage:         lastNackMsg,
			LastRecvResourceSuccess: &lastRecvResourceSuccess,
			ExportedServices:        []string{},
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
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
					ResourceURL: pbpeerstream.TypeURLExportedService,
					ResourceID:  "web",
					Nonce:       "24",

					// Unknown operation gets NACKed
					Operation: pbpeerstream.Operation_OPERATION_UNSPECIFIED,
				},
			},
		}
		lastRecvError = it.FutureNow(1)
		err := client.Send(resp)
		require.NoError(t, err)

		ack, err := client.Recv()
		require.NoError(t, err)

		expectNack := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL:   pbpeerstream.TypeURLExportedService,
					ResponseNonce: "24",
					Error: &pbstatus.Status{
						Code:    int32(code.Code_INVALID_ARGUMENT),
						Message: `unsupported operation: "OPERATION_UNSPECIFIED"`,
					},
				},
			},
		}
		prototest.AssertDeepEqual(t, expectNack, ack)

		lastRecvErrorMsg = `unsupported operation: "OPERATION_UNSPECIFIED"`

		expect := Status{
			Connected:               true,
			LastSendSuccess:         lastSendSuccess,
			LastAck:                 &lastSendAck,
			LastNack:                &lastNack,
			LastNackMessage:         lastNackMsg,
			LastRecvResourceSuccess: &lastRecvResourceSuccess,
			LastRecvError:           &lastRecvError,
			LastRecvErrorMessage:    lastRecvErrorMsg,
			ExportedServices:        []string{},
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	var lastRecvHeartbeat time.Time
	testutil.RunStep(t, "receives heartbeat", func(t *testing.T) {
		resp := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Heartbeat_{
				Heartbeat: &pbpeerstream.ReplicationMessage_Heartbeat{},
			},
		}
		lastRecvHeartbeat = it.FutureNow(1)
		err := client.Send(resp)
		require.NoError(t, err)

		expect := Status{
			Connected:               true,
			LastSendSuccess:         lastSendSuccess,
			LastAck:                 &lastSendAck,
			LastNack:                &lastNack,
			LastNackMessage:         lastNackMsg,
			LastRecvResourceSuccess: &lastRecvResourceSuccess,
			LastRecvError:           &lastRecvError,
			LastRecvErrorMessage:    lastRecvErrorMsg,
			LastRecvHeartbeat:       &lastRecvHeartbeat,
			ExportedServices:        []string{},
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})

	testutil.RunStep(t, "client disconnect marks stream as disconnected", func(t *testing.T) {
		lastRecvError = it.FutureNow(1)
		disconnectTime := it.FutureNow(2)
		lastRecvErrorMsg = "stream ended unexpectedly"

		client.Close()

		expect := Status{
			Connected:               false,
			DisconnectErrorMessage:  lastRecvErrorMsg,
			LastSendSuccess:         lastSendSuccess,
			LastAck:                 &lastSendAck,
			LastNack:                &lastNack,
			LastNackMessage:         lastNackMsg,
			DisconnectTime:          &disconnectTime,
			LastRecvResourceSuccess: &lastRecvResourceSuccess,
			LastRecvError:           &lastRecvError,
			LastRecvErrorMessage:    lastRecvErrorMsg,
			LastRecvHeartbeat:       &lastRecvHeartbeat,
			ExportedServices:        []string{},
		}

		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.Equal(r, expect, status)
		})
	})
}

func TestStreamResources_Server_ServiceUpdates(t *testing.T) {
	srv, store := newTestServer(t, nil)

	// Create a peering
	var lastIdx uint64 = 1
	p := writePeeringToBeDialed(t, store, lastIdx, "my-peering")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, p.ID)

	// Register a service that is not yet exported
	mysql := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
	}
	mysqlSidecar := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "mysql-sidecar-proxy",
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "mysql",
		},
	}

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, mysql.Node))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "foo", mysql.Service))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "foo", mysqlSidecar))

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

	testutil.RunStep(t, "initial stream data is received", func(t *testing.T) {
		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLPeeringTrustBundle, msg.GetResponse().ResourceURL)
				// Roots tested in TestStreamResources_Server_CARootUpdates
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLExportedServiceList, msg.GetResponse().ResourceURL)
				require.Equal(t, subExportedServiceList, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var exportedServices pbpeerstream.ExportedServiceList
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&exportedServices))
				require.ElementsMatch(t, []string{}, exportedServices.Services)
			},
		)
	})

	testutil.RunStep(t, "exporting mysql leads to an UPSERT event", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-peering"},
					},
				},
				{
					// Mongo does not get pushed because it does not have instances registered.
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-peering"},
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
				// no mongo instances exist
				require.Equal(t, pbpeerstream.TypeURLExportedService, msg.GetResponse().ResourceURL)
				require.Equal(t, mongoSN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbpeerstream.ExportedService
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&nodes))
				require.Len(t, nodes.Nodes, 0)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLExportedService, msg.GetResponse().ResourceURL)
				require.Equal(t, mysqlSN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbpeerstream.ExportedService
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&nodes))
				require.Len(t, nodes.Nodes, 1)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				// proxies can't export because no mesh gateway exists yet
				require.Equal(t, pbpeerstream.TypeURLExportedService, msg.GetResponse().ResourceURL)
				require.Equal(t, mysqlProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbpeerstream.ExportedService
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&nodes))
				require.Len(t, nodes.Nodes, 0)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLExportedServiceList, msg.GetResponse().ResourceURL)
				require.Equal(t, subExportedServiceList, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var exportedServices pbpeerstream.ExportedServiceList
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&exportedServices))
				require.ElementsMatch(t,
					[]string{structs.ServiceName{Name: "mongo"}.String(), structs.ServiceName{Name: "mysql"}.String()},
					exportedServices.Services)
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
				require.Equal(t, pbpeerstream.TypeURLExportedService, msg.GetResponse().ResourceURL)
				require.Equal(t, mysqlProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbpeerstream.ExportedService
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&nodes))
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

	testutil.RunStep(t, "register service resolver to send proxy updates", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "mongo",
		}))
		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLExportedService, msg.GetResponse().ResourceURL)
				require.Equal(t, mongoProxySN, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var nodes pbpeerstream.ExportedService
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&nodes))
				require.Len(t, nodes.Nodes, 1)

				pm := nodes.Nodes[0].Service.Connect.PeerMeta
				require.Equal(t, "grpc", pm.Protocol)
				spiffeIDs := []string{
					"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/mongo",
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

			var nodes pbpeerstream.ExportedService
			require.NoError(r, msg.GetResponse().Resource.UnmarshalTo(&nodes))
			require.Len(r, nodes.Nodes, 1)
		})
	})

	testutil.RunStep(t, "un-exporting mysql leads to an exported service list update", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{
							Peer: "my-peering",
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
			require.Equal(r, pbpeerstream.TypeURLExportedServiceList, msg.GetResponse().ResourceURL)
			require.Equal(r, subExportedServiceList, msg.GetResponse().ResourceID)
			require.Equal(r, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

			var exportedServices pbpeerstream.ExportedServiceList
			require.NoError(r, msg.GetResponse().Resource.UnmarshalTo(&exportedServices))
			require.Equal(r, []string{structs.ServiceName{Name: "mongo"}.String()}, exportedServices.Services)
		})
	})

	testutil.RunStep(t, "deleting the config entry leads to a DELETE event for mongo", func(t *testing.T) {
		err := store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", nil)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			msg, err := client.RecvWithTimeout(100 * time.Millisecond)
			require.NoError(r, err)
			require.Equal(r, pbpeerstream.TypeURLExportedServiceList, msg.GetResponse().ResourceURL)
			require.Equal(r, subExportedServiceList, msg.GetResponse().ResourceID)
			require.Equal(r, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

			var exportedServices pbpeerstream.ExportedServiceList
			require.NoError(r, msg.GetResponse().Resource.UnmarshalTo(&exportedServices))
			require.Len(r, exportedServices.Services, 0)
		})
	})
}

func TestStreamResources_Server_CARootUpdates(t *testing.T) {
	srv, store := newTestServer(t, nil)

	// Create a peering
	var lastIdx uint64 = 1
	p := writePeeringToBeDialed(t, store, lastIdx, "my-peering")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	clusterID, rootA := writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, p.ID)

	testutil.RunStep(t, "initial CA Roots replication", func(t *testing.T) {
		expectReplEvents(t, client,
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLPeeringTrustBundle, msg.GetResponse().ResourceURL)
				require.Equal(t, "roots", msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var trustBundle pbpeering.PeeringTrustBundle
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&trustBundle))

				require.ElementsMatch(t, []string{rootA.RootCert}, trustBundle.RootPEMs)
				expect := connect.SpiffeIDSigningForCluster(clusterID).Host()
				require.Equal(t, expect, trustBundle.TrustDomain)
			},
			func(t *testing.T, msg *pbpeerstream.ReplicationMessage) {
				require.Equal(t, pbpeerstream.TypeURLExportedServiceList, msg.GetResponse().ResourceURL)
				require.Equal(t, subExportedServiceList, msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var exportedServices pbpeerstream.ExportedServiceList
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&exportedServices))
				require.ElementsMatch(t, []string{}, exportedServices.Services)
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
				require.Equal(t, pbpeerstream.TypeURLPeeringTrustBundle, msg.GetResponse().ResourceURL)
				require.Equal(t, "roots", msg.GetResponse().ResourceID)
				require.Equal(t, pbpeerstream.Operation_OPERATION_UPSERT, msg.GetResponse().Operation)

				var trustBundle pbpeering.PeeringTrustBundle
				require.NoError(t, msg.GetResponse().Resource.UnmarshalTo(&trustBundle))

				require.ElementsMatch(t, []string{rootB.RootCert, rootC.RootCert}, trustBundle.RootPEMs)
				expect := connect.SpiffeIDSigningForCluster(clusterID).Host()
				require.Equal(t, expect, trustBundle.TrustDomain)
			},
		)
	})
}

func TestStreamResources_Server_AckNackNonce(t *testing.T) {
	srv, store := newTestServer(t, func(c *Config) {
		c.incomingHeartbeatTimeout = 10 * time.Millisecond
	})

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, testPeerID)
	client.DrainStream(t)

	testutil.RunStep(t, "ack contains nonce from response", func(t *testing.T) {
		resp := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Response_{
				Response: &pbpeerstream.ReplicationMessage_Response{
					ResourceURL: pbpeerstream.TypeURLExportedService,
					Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
					Nonce:       "1234",
				},
			},
		}
		require.NoError(t, client.Send(resp))

		msg, err := client.Recv()
		require.NoError(t, err)
		require.Equal(t, "1234", msg.GetRequest().ResponseNonce)
	})

	testutil.RunStep(t, "nack contains nonce from response", func(t *testing.T) {
		resp := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Response_{
				Response: &pbpeerstream.ReplicationMessage_Response{
					ResourceURL: pbpeerstream.TypeURLExportedService,
					Operation:   pbpeerstream.Operation_OPERATION_UNSPECIFIED, // Unspecified gets NACK
					Nonce:       "5678",
				},
			},
		}
		require.NoError(t, client.Send(resp))

		msg, err := client.Recv()
		require.NoError(t, err)
		require.Equal(t, "5678", msg.GetRequest().ResponseNonce)
	})
	// Add in a sleep to prevent the test from flaking.
	// The mock client expects certain calls to be made.
	time.Sleep(50 * time.Millisecond)
}

// Test that when the client doesn't send a heartbeat in time, the stream is disconnected.
func TestStreamResources_Server_DisconnectsOnHeartbeatTimeout(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	srv, store := newTestServer(t, func(c *Config) {
		c.incomingHeartbeatTimeout = 50 * time.Millisecond
	})
	srv.Tracker.setClock(it.Now)

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, testPeerID)

	client.DrainStream(t)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	testutil.RunStep(t, "stream is disconnected due to heartbeat timeout", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			disconnectTime := ptr(it.StaticNow())
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.False(r, status.Connected)
			require.Equal(r, "heartbeat timeout", status.DisconnectErrorMessage)
			require.Equal(r, disconnectTime, status.DisconnectTime)
		})
	})
}

// Test that the server sends heartbeats at the expected interval.
func TestStreamResources_Server_SendsHeartbeats(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	outgoingHeartbeatInterval := 5 * time.Millisecond

	srv, store := newTestServer(t, func(c *Config) {
		c.outgoingHeartbeatInterval = outgoingHeartbeatInterval
	})
	srv.Tracker.setClock(it.Now)

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, testPeerID)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			_, err := client.Recv()
			require.NoError(r, err)
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	testutil.RunStep(t, "sends first heartbeat", func(t *testing.T) {
		retry.RunWith(&retry.Timer{
			Timeout: outgoingHeartbeatInterval * 2,
			Wait:    outgoingHeartbeatInterval / 2,
		}, t, func(r *retry.R) {
			heartbeat, err := client.Recv()
			require.NoError(r, err)
			require.NotNil(r, heartbeat.GetHeartbeat())
		})
	})

	testutil.RunStep(t, "sends second heartbeat", func(t *testing.T) {
		retry.RunWith(&retry.Timer{
			Timeout: outgoingHeartbeatInterval * 2,
			Wait:    outgoingHeartbeatInterval / 2,
		}, t, func(r *retry.R) {
			heartbeat, err := client.Recv()
			require.NoError(r, err)
			require.NotNil(r, heartbeat.GetHeartbeat())
		})
	})
}

// Test that as long as the server receives heartbeats it keeps the connection open.
func TestStreamResources_Server_KeepsConnectionOpenWithHeartbeat(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	incomingHeartbeatTimeout := 50 * time.Millisecond

	srv, store := newTestServer(t, func(c *Config) {
		c.incomingHeartbeatTimeout = incomingHeartbeatTimeout
	})
	srv.Tracker.setClock(it.Now)

	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	require.Empty(t, p.PeerID, "should be empty if being dialed")

	// Set the initial roots and CA configuration.
	_, _ = writeInitialRootsAndCA(t, store)

	client := makeClient(t, srv, testPeerID)

	client.DrainStream(t)

	testutil.RunStep(t, "new stream gets tracked", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			status, ok := srv.StreamStatus(testPeerID)
			require.True(r, ok)
			require.True(r, status.Connected)
		})
	})

	heartbeatMsg := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Heartbeat_{
			Heartbeat: &pbpeerstream.ReplicationMessage_Heartbeat{}}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// errCh is used to collect any send errors from within the goroutine.
	errCh := make(chan error)

	// Set up a goroutine to send the heartbeat every 1/2 of the timeout.
	go func() {
		// This is just a do while loop. We want to send the heartbeat right away to start
		// because the test setup above takes some time and we might be close to the heartbeat
		// timeout already.
		for {
			err := client.Send(heartbeatMsg)
			if err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}
			select {
			case <-time.After(incomingHeartbeatTimeout / 10): // Going any slower here triggers flakes when running
			case <-ctx.Done():
				close(errCh)
				return
			}
		}
	}()

	// Assert that the stream remains connected for 5 heartbeat timeouts.
	require.Never(t, func() bool {
		status, ok := srv.StreamStatus(testPeerID)
		if !ok {
			return true
		}
		return !status.Connected
	}, incomingHeartbeatTimeout*5, incomingHeartbeatTimeout)

	// Kill the heartbeat sending goroutine and check if it had any errors.
	cancel()
	err, ok := <-errCh
	if ok {
		require.NoError(t, err)
	}
}

// makeClient sets up a *MockClient with the initial subscription
// message handshake.
func makeClient(t *testing.T, srv *testServer, peerID string) *MockClient {
	t.Helper()

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

	// Send the initial request
	require.NoError(t, client.Send(&pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Open_{
			Open: &pbpeerstream.ReplicationMessage_Open{
				PeerID:         testPeerID,
				StreamSecretID: testPendingStreamSecretID,
			},
		},
	}))

	// Receive ExportedService, ExportedServiceList, and PeeringTrustBundle subscription requests from server
	receivedSub1, err := client.Recv()
	require.NoError(t, err)
	receivedSub2, err := client.Recv()
	require.NoError(t, err)
	receivedSub3, err := client.Recv()
	require.NoError(t, err)

	// Issue services, roots, and server address subscription to server.
	// Note that server address may not come as an initial message
	for _, resourceURL := range []string{
		pbpeerstream.TypeURLExportedService,
		pbpeerstream.TypeURLExportedServiceList,
		pbpeerstream.TypeURLPeeringTrustBundle,
		// only dialers request, which is why this is absent below
		pbpeerstream.TypeURLPeeringServerAddresses,
	} {
		init := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					PeerID:      peerID,
					ResourceURL: resourceURL,
				},
			},
		}
		require.NoError(t, client.Send(init))
	}

	expect := []*pbpeerstream.ReplicationMessage{
		{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL: pbpeerstream.TypeURLExportedService,
					// The PeerID field is only set for the messages coming FROM
					// the establishing side and are going to be empty from the
					// other side.
					PeerID: "",
				},
			},
		},
		{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL: pbpeerstream.TypeURLExportedServiceList,
					// The PeerID field is only set for the messages coming FROM
					// the establishing side and are going to be empty from the
					// other side.
					PeerID: "",
				},
			},
		},
		{
			Payload: &pbpeerstream.ReplicationMessage_Request_{
				Request: &pbpeerstream.ReplicationMessage_Request{
					ResourceURL: pbpeerstream.TypeURLPeeringTrustBundle,
					// The PeerID field is only set for the messages coming FROM
					// the establishing side and are going to be empty from the
					// other side.
					PeerID: "",
				},
			},
		},
	}
	got := []*pbpeerstream.ReplicationMessage{
		receivedSub1,
		receivedSub2,
		receivedSub3,
	}
	prototest.AssertElementsMatch(t, expect, got)

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

func (b *testStreamBackend) ValidateProposedPeeringSecret(id string) (bool, error) {
	return true, nil
}

func (b *testStreamBackend) PeeringSecretsWrite(req *pbpeering.SecretsWriteRequest) error {
	return b.store.PeeringSecretsWrite(1, req)
}

func (b *testStreamBackend) PeeringWrite(req *pbpeering.PeeringWriteRequest) error {
	return b.store.PeeringWrite(1, req)
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

func Test_ExportedServicesCount(t *testing.T) {
	peerName := "billing"
	peerID := "1fabcd52-1d46-49b0-b1d8-71559aee47f5"

	srv, store := newTestServer(t, nil)
	require.NoError(t, store.PeeringWrite(31, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   peerID,
			Name: peerName,
		},
	}))

	// connect the stream
	mst, err := srv.Tracker.Connected(peerID)
	require.NoError(t, err)

	services := []string{
		structs.NewServiceName("web", nil).String(),
		structs.NewServiceName("api", nil).String(),
		structs.NewServiceName("mongo", nil).String(),
	}
	update := cache.UpdateEvent{
		CorrelationID: subExportedServiceList,
		Result: &pbpeerstream.ExportedServiceList{
			Services: services,
		}}
	_, err = makeExportedServiceListResponse(mst, update)
	require.NoError(t, err)
	// Test the count and contents separately to ensure the count code path is hit.
	require.Equal(t, 3, mst.GetExportedServicesCount())
	require.ElementsMatch(t, services, mst.ExportedServices)
}

func Test_processResponse_Validation(t *testing.T) {
	peerName := "billing"
	peerID := "1fabcd52-1d46-49b0-b1d8-71559aee47f5"

	type testCase struct {
		name       string
		in         *pbpeerstream.ReplicationMessage_Response
		expect     *pbpeerstream.ReplicationMessage
		extraTests func(t *testing.T, s *state.Store)
		wantErr    bool
	}

	srv, store := newTestServer(t, nil)
	require.NoError(t, store.PeeringWrite(31, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                  peerName,
			ID:                    peerID,
			ManualServerAddresses: []string{"manual"},
			PeerServerAddresses:   []string{"one", "two"},
		},
	}))

	// connect the stream
	mst, err := srv.Tracker.Connected(peerID)
	require.NoError(t, err)

	run := func(t *testing.T, tc testCase) {
		reply, err := srv.processResponse(peerName, "", mst, tc.in)
		if tc.wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, tc.expect, reply)
		if tc.extraTests != nil {
			tc.extraTests(t, store)
		}
	}

	tt := []testCase{
		{
			name: "valid upsert",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLExportedService,
				ResourceID:  "api",
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
				Resource:    makeAnyPB(t, &pbpeerstream.ExportedService{}),
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL:   pbpeerstream.TypeURLExportedService,
						ResponseNonce: "1",
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
						ResourceURL:   "nomad.Job",
						ResponseNonce: "1",
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
			name: "missing a nonce",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLExportedService,
				ResourceID:  "web",
				Nonce:       "",
				Operation:   pbpeerstream.Operation_OPERATION_UNSPECIFIED,
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL:   pbpeerstream.TypeURLExportedService,
						ResponseNonce: "",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: fmt.Sprintf(`received response without a nonce for: %s:web`, pbpeerstream.TypeURLExportedService),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unknown operation",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLExportedService,
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_UNSPECIFIED,
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL:   pbpeerstream.TypeURLExportedService,
						ResponseNonce: "1",
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
				ResourceURL: pbpeerstream.TypeURLExportedService,
				Nonce:       "1",
				Operation:   pbpeerstream.Operation(100000),
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL:   pbpeerstream.TypeURLExportedService,
						ResponseNonce: "1",
						Error: &pbstatus.Status{
							Code:    int32(code.Code_INVALID_ARGUMENT),
							Message: `unsupported operation: 100000`,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "manual server addresses are not overwritten",
			in: &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLPeeringServerAddresses,
				Nonce:       "1",
				Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
				Resource: makeAnyPB(t, &pbpeering.PeeringServerAddresses{
					Addresses: []string{"three"},
				}),
			},
			expect: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Request_{
					Request: &pbpeerstream.ReplicationMessage_Request{
						ResourceURL:   pbpeerstream.TypeURLPeeringServerAddresses,
						ResponseNonce: "1",
					},
				},
			},
			extraTests: func(t *testing.T, s *state.Store) {
				_, peer, err := s.PeeringReadByID(nil, peerID)
				require.NoError(t, err)
				require.Equal(t, []string{"manual"}, peer.ManualServerAddresses)
				require.Equal(t, []string{"three"}, peer.PeerServerAddresses)
			},
			wantErr: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// writePeeringToDialFrom creates a peering with the provided name and ensures
// the PeerID field is set for the ID of the remote peer.
func writePeeringToDialFrom(t *testing.T, store *state.Store, idx uint64, peerName string) *pbpeering.Peering {
	remotePeerID, err := uuid.GenerateUUID()
	require.NoError(t, err)
	return writeTestPeering(t, store, idx, peerName, remotePeerID)
}

// writePeeringToBeDialed creates a peering with the provided name and ensures
// the PeerID field is NOT set for the ID of the remote peer.
func writePeeringToBeDialed(t *testing.T, store *state.Store, idx uint64, peerName string) *pbpeering.Peering {
	return writeTestPeering(t, store, idx, peerName, "")
}

func writeTestPeering(t *testing.T, store *state.Store, idx uint64, peerName, remotePeerID string) *pbpeering.Peering {
	peering := pbpeering.Peering{
		ID:     testPeerID,
		Name:   peerName,
		PeerID: remotePeerID,
	}
	if remotePeerID != "" {
		peering.PeerServerAddresses = []string{"127.0.0.1:5300"}
	}

	require.NoError(t, store.PeeringWrite(idx, &pbpeering.PeeringWriteRequest{
		Peering: &peering,
		SecretsRequest: &pbpeering.SecretsWriteRequest{
			PeerID: testPeerID,
			// Simulate generating a stream secret by first generating a token then exchanging for a stream secret.
			Request: &pbpeering.SecretsWriteRequest_GenerateToken{
				GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
					EstablishmentSecret: testEstablishmentSecretID,
				},
			},
		},
	}))
	require.NoError(t, store.PeeringSecretsWrite(idx, &pbpeering.SecretsWriteRequest{
		PeerID: testPeerID,
		Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
			ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
				EstablishmentSecret: testEstablishmentSecretID,
				PendingStreamSecret: testPendingStreamSecretID,
			},
		},
	}))

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

func makeAnyPB(t *testing.T, pb newproto.Message) *anypb.Any {
	any, err := anypb.New(pb)
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
			return reqA.ResponseNonce < reqB.ResponseNonce

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

	nonces := make(map[string]struct{})
	for i := 0; i < num; i++ {
		checkFns[i](t, out[i])

		// Ensure every nonce was unique.
		if resp := out[i].GetResponse(); resp != nil {
			require.NotContains(t, nonces, resp.Nonce)
			nonces[resp.Nonce] = struct{}{}
		}
	}
}

type PeeringProcessResponse_testCase struct {
	name             string
	seed             []*structs.RegisterRequest
	inputServiceName structs.ServiceName
	input            *pbpeerstream.ExportedService
	expect           map[structs.ServiceName]structs.CheckServiceNodes
	exportedServices []string
}

func processResponse_ExportedServiceUpdates(
	t *testing.T,
	srv *testServer,
	store *state.Store,
	localEntMeta acl.EnterpriseMeta,
	peerName string,
	tests []PeeringProcessResponse_testCase,
) *MutableStatus {
	// create a peering in the state store
	peerID := "1fabcd52-1d46-49b0-b1d8-71559aee47f5"
	require.NoError(t, store.PeeringWrite(31, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:        peerID,
			Name:      peerName,
			Partition: localEntMeta.PartitionOrDefault(),
		},
	}))

	// connect the stream
	mst, err := srv.Tracker.Connected(peerID)
	require.NoError(t, err)

	run := func(t *testing.T, tc PeeringProcessResponse_testCase) {
		// Seed the local catalog with some data to reconcile against.
		// and increment the tracker's imported services count
		var serviceNames []structs.ServiceName
		for _, reg := range tc.seed {
			require.NoError(t, srv.Backend.CatalogRegister(reg))

			sn := reg.Service.CompoundServiceName()
			serviceNames = append(serviceNames, sn)
		}
		mst.SetImportedServices(serviceNames)

		in := &pbpeerstream.ReplicationMessage_Response{
			ResourceURL: pbpeerstream.TypeURLExportedService,
			ResourceID:  tc.inputServiceName.String(),
			Nonce:       "1",
			Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
			Resource:    makeAnyPB(t, tc.input),
		}

		// Simulate an update arriving for billing/api.
		_, err = srv.processResponse(peerName, localEntMeta.PartitionOrDefault(), mst, in)
		require.NoError(t, err)

		if len(tc.exportedServices) > 0 {
			resp := &pbpeerstream.ReplicationMessage_Response{
				ResourceURL: pbpeerstream.TypeURLExportedServiceList,
				ResourceID:  subExportedServiceList,
				Nonce:       "2",
				Operation:   pbpeerstream.Operation_OPERATION_UPSERT,
				Resource:    makeAnyPB(t, &pbpeerstream.ExportedServiceList{Services: tc.exportedServices}),
			}

			// Simulate an update arriving for billing/api.
			_, err = srv.processResponse(peerName, localEntMeta.PartitionOrDefault(), mst, resp)
			require.NoError(t, err)
			// Test the count and contents separately to ensure the count code path is hit.
			require.Equal(t, mst.GetImportedServicesCount(), len(tc.exportedServices))
			require.ElementsMatch(t, mst.ImportedServices, tc.exportedServices)
		}

		wildcardNS := acl.NewEnterpriseMetaWithPartition(localEntMeta.PartitionOrDefault(), acl.WildcardName)
		_, allServices, err := srv.GetStore().ServiceList(nil, &wildcardNS, peerName)
		require.NoError(t, err)

		// This ensures that only services specified under tc.expect are stored. It includes
		// all exported services plus their sidecar proxies.
		for _, svc := range allServices {
			_, ok := tc.expect[svc]
			require.True(t, ok)
		}

		for svc, expect := range tc.expect {
			t.Run(svc.String(), func(t *testing.T) {
				_, got, err := srv.GetStore().CheckServiceNodes(nil, svc.Name, &svc.EnterpriseMeta, peerName)
				require.NoError(t, err)
				requireEqualInstances(t, expect, got)
			})
		}
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
	return mst
}

func Test_processResponse_ExportedServiceUpdates(t *testing.T) {
	peerName := "billing"
	localEntMeta := *acl.DefaultEnterpriseMeta()

	remoteMeta := *structs.DefaultEnterpriseMetaInPartition("billing-ap")
	pbRemoteMeta := pbcommon.NewEnterpriseMetaFromStructs(remoteMeta)

	apiLocalSN := structs.NewServiceName("api", &localEntMeta)
	redisLocalSN := structs.NewServiceName("redis", &localEntMeta)
	tests := []PeeringProcessResponse_testCase{
		{
			name:             "upsert two service instances to the same node",
			exportedServices: []string{apiLocalSN.String()},
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			input: &pbpeerstream.ExportedService{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				structs.NewServiceName("api", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:   "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node: "node-foo",

							// The remote billing-ap partition is overwritten for all resources with the local default.
							Partition: localEntMeta.PartitionOrEmpty(),

							// The name of the peer "billing" is attached as well.
							PeerName: peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name:             "deleting a service with an empty exported service event",
			exportedServices: []string{apiLocalSN.String()},
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-2",
						Service:        "api",
						EnterpriseMeta: localEntMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-2",
							CheckID:   types.CheckID("api-2-check"),
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
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			input:            &pbpeerstream.ExportedService{},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				structs.NewServiceName("api", &localEntMeta): {},
			},
		},
		{
			name:             "upsert two service instances to different nodes",
			exportedServices: []string{apiLocalSN.String()},
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			input: &pbpeerstream.ExportedService{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &pbservice.Node{
							ID:        "c0f97de9-4e1b-4e80-a1c6-cd8725835ab2",
							Node:      "node-bar",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							{
								CheckID:        "node-bar-check",
								Node:           "node-bar",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-bar",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				structs.NewServiceName("api", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "c0f97de9-4e1b-4e80-a1c6-cd8725835ab2",
							Node:      "node-bar",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-2",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-bar-check",
								Node:           "node-bar",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-2-check",
								ServiceID:      "api-2",
								Node:           "node-bar",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
					{
						Node: &structs.Node{
							ID:   "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node: "node-foo",

							// The remote billing-ap partition is overwritten for all resources with the local default.
							Partition: localEntMeta.PartitionOrEmpty(),

							// The name of the peer "billing" is attached as well.
							PeerName: peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name:             "deleting one service name from a node does not delete other service names",
			exportedServices: []string{apiLocalSN.String(), redisLocalSN.String()},
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: localEntMeta,
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
						EnterpriseMeta: localEntMeta,
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
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			// Nil input is for the "api" service.
			input: &pbpeerstream.ExportedService{},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				structs.NewServiceName("api", &localEntMeta): {},
				// Existing redis service was not affected by deletion.
				structs.NewServiceName("redis", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "redis-2-check",
								ServiceID:      "redis-2",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name: "unexporting a service does not delete other services",
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: localEntMeta,
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
						ID:             "redis-2-sidecar-proxy",
						Service:        "redis-sidecar-proxy",
						EnterpriseMeta: localEntMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "redis-2-sidecar-proxy",
							CheckID:   types.CheckID("redis-2-sidecar-proxy-check"),
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
						EnterpriseMeta: localEntMeta,
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
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1-sidecar-proxy",
						Service:        "api-sidecar-proxy",
						EnterpriseMeta: localEntMeta,
						PeerName:       peerName,
					},
					Checks: structs.HealthChecks{
						{
							Node:      "node-foo",
							ServiceID: "api-1-sidecar-proxy",
							CheckID:   types.CheckID("api-1-check"),
							PeerName:  peerName,
						},
						{
							Node:      "node-foo",
							CheckID:   types.CheckID("node-foo-sidecar-proxy-check"),
							ServiceID: "api-1-sidecar-proxy",
							PeerName:  peerName,
						},
					},
				},
			},
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			// Nil input is for the "api" service.
			input:            &pbpeerstream.ExportedService{},
			exportedServices: []string{redisLocalSN.String()},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				// Existing redis service was not affected by deletion.
				structs.NewServiceName("redis", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "redis-2-check",
								ServiceID:      "redis-2",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
				structs.NewServiceName("redis-sidecar-proxy", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2-sidecar-proxy",
							Service:        "redis-sidecar-proxy",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "node-foo-check",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
							{
								CheckID:        "redis-2-sidecar-proxy-check",
								ServiceID:      "redis-2-sidecar-proxy",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name:             "service checks are cleaned up when not present in a response",
			exportedServices: []string{apiLocalSN.String()},
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "api-1",
						Service:        "api",
						EnterpriseMeta: localEntMeta,
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
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			input: &pbpeerstream.ExportedService{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							// Service check was deleted
						},
					},
				},
			},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				// Service check should be gone
				structs.NewServiceName("api", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{},
					},
				},
			},
		},
		{
			name:             "node checks are cleaned up when not present in a response",
			exportedServices: []string{apiLocalSN.String(), redisLocalSN.String()},
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: localEntMeta,
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
						EnterpriseMeta: localEntMeta,
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
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			input: &pbpeerstream.ExportedService{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						Service: &pbservice.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
							PeerName:       peerName,
						},
						Checks: []*pbservice.HealthCheck{
							// Node check was deleted
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: pbRemoteMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				// Node check should be gone
				structs.NewServiceName("api", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "api-1",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "api-1-check",
								ServiceID:      "api-1",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
				structs.NewServiceName("redis", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: localEntMeta,
							PeerName:       peerName,
						},
						Checks: []*structs.HealthCheck{
							{
								CheckID:        "redis-2-check",
								ServiceID:      "redis-2",
								Node:           "node-foo",
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
		{
			name:             "replacing a service instance on a node cleans up the old instance",
			exportedServices: []string{apiLocalSN.String(), redisLocalSN.String()},
			seed: []*structs.RegisterRequest{
				{
					ID:       types.NodeID("af913374-68ea-41e5-82e8-6ffd3dffc461"),
					Node:     "node-foo",
					PeerName: peerName,
					Service: &structs.NodeService{
						ID:             "redis-2",
						Service:        "redis",
						EnterpriseMeta: localEntMeta,
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
						EnterpriseMeta: localEntMeta,
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
			inputServiceName: structs.NewServiceName("api", &remoteMeta),
			input: &pbpeerstream.ExportedService{
				Nodes: []*pbservice.CheckServiceNode{
					{
						Node: &pbservice.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: pbRemoteMeta.Partition,
							PeerName:  peerName,
						},
						// New service ID and checks for the api service.
						Service: &pbservice.NodeService{
							ID:             "new-api-v2",
							Service:        "api",
							EnterpriseMeta: pbRemoteMeta,
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
			expect: map[structs.ServiceName]structs.CheckServiceNodes{
				structs.NewServiceName("api", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "new-api-v2",
							Service:        "api",
							EnterpriseMeta: localEntMeta,
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
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
				structs.NewServiceName("redis", &localEntMeta): {
					{
						Node: &structs.Node{
							ID:        "af913374-68ea-41e5-82e8-6ffd3dffc461",
							Node:      "node-foo",
							Partition: localEntMeta.PartitionOrEmpty(),
							PeerName:  peerName,
						},
						Service: &structs.NodeService{
							ID:             "redis-2",
							Service:        "redis",
							EnterpriseMeta: localEntMeta,
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
								EnterpriseMeta: localEntMeta,
								PeerName:       peerName,
							},
						},
					},
				},
			},
		},
	}
	srv, store := newTestServer(t, func(c *Config) {
		backend := c.Backend.(*testStreamBackend)
		backend.leader = func() bool {
			return false
		}
	})
	processResponse_ExportedServiceUpdates(t, srv, store, localEntMeta, peerName, tests)
}

// TestLogTraceProto tests that all PB trace log helpers redact the
// long-lived SecretStreamID.
// We ensure it gets redacted when logging a ReplicationMessage_Open or a ReplicationMessage.
// In the stream handler we only log the ReplicationMessage_Open, but testing both guards against
// a change in that behavior.
func TestLogTraceProto(t *testing.T) {
	type testCase struct {
		input proto.Message
	}

	tt := map[string]testCase{
		"replication message": {
			input: &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Open_{
					Open: &pbpeerstream.ReplicationMessage_Open{
						StreamSecretID: testPendingStreamSecretID,
					},
				},
			},
		},
		"open message": {
			input: &pbpeerstream.ReplicationMessage_Open{
				StreamSecretID: testPendingStreamSecretID,
			},
		},
	}
	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			logger, err := logging.Setup(logging.Config{
				LogLevel: "TRACE",
			}, &b)
			require.NoError(t, err)

			logTraceRecv(logger, tc.input)
			logTraceSend(logger, tc.input)
			logTraceProto(logger, tc.input, false)

			body, err := io.ReadAll(&b)
			require.NoError(t, err)
			require.NotContains(t, string(body), testPendingStreamSecretID)
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

	// readyServersSnapshotHandler is solely used for handling autopilot events
	// which don't come from the state store.
	readyServersSnapshotHandler *dummyReadyServersSnapshotHandler
}

func newTestServer(t *testing.T, configFn func(c *Config)) (*testServer, *state.Store) {
	t.Helper()
	publisher := stream.NewEventPublisher(10 * time.Second)
	store, handler := newStateStore(t, publisher)

	ports := freeport.GetN(t, 1) // {grpc}

	cfg := Config{
		Backend: &testStreamBackend{
			store: store,
			pub:   publisher,
		},
		GetStore:       func() StateStore { return store },
		Logger:         testutil.Logger(t),
		Datacenter:     "dc1",
		ConnectEnabled: true,
		ForwardRPC:     noopForwardRPC,
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
		Server:                      srv,
		readyServersSnapshotHandler: handler,
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
