package serverdiscovery

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	acl "github.com/hashicorp/consul/acl"
	resolver "github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/autopilotevents"
	"github.com/hashicorp/consul/agent/consul/stream"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/proto-public/pbserverdiscovery"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

const testACLToken = "eb61f1ed-65a4-4da6-8d3d-0564bd16c965"

func TestWatchServers_StreamLifeCycle(t *testing.T) {
	// The flow for this test is roughly:
	//
	// 1. Open a WatchServers stream
	// 2. Observe the snapshot message is sent back through
	//    the stream.
	// 3. Publish an event that changes to 2 servers.
	// 4. See the corresponding message sent back through the stream.
	// 5. Send a NewCloseSubscriptionEvent for the token secret.
	// 6. See that a new snapshot is taken and the corresponding message
	//    gets sent back. If there were multiple subscribers for the topic
	//    then this should not happen. However with the current EventPublisher
	//    implementation, whenever the last subscriber for a topic has its
	//    subscription closed then the publisher will delete the whole topic
	//    buffer. When that happens, resubscribing will see no snapshot
	//    cache, or latest event in the buffer and force creating a new snapshot.
	// 7. Publish another event to move to 3 servers.
	// 8. Ensure that the message gets sent through the stream. Also
	//    this will validate that no other 1 or 2 server event is
	//    seen after stream reinitialization.

	srv1 := autopilotevents.ReadyServerInfo{
		ID:      "9aeb73f6-e83e-43c1-bdc9-ca5e43efe3e4",
		Address: "198.18.0.1",
		Version: "1.12.0",
	}
	srv2 := autopilotevents.ReadyServerInfo{
		ID:      "eec8721f-c42b-48da-a5a5-07565158015e",
		Address: "198.18.0.2",
		Version: "1.12.3",
	}
	srv3 := autopilotevents.ReadyServerInfo{
		ID:      "256796f2-3a38-4f80-8cef-375c3cb3aa1f",
		Address: "198.18.0.3",
		Version: "1.12.3",
	}

	oneServerEventPayload := autopilotevents.EventPayloadReadyServers{srv1}
	twoServerEventPayload := autopilotevents.EventPayloadReadyServers{srv1, srv2}
	threeServerEventPayload := autopilotevents.EventPayloadReadyServers{srv1, srv2, srv3}

	oneServerResponse := &pbserverdiscovery.WatchServersResponse{
		Servers: []*pbserverdiscovery.Server{
			{
				Id:      srv1.ID,
				Address: srv1.Address,
				Version: srv1.Version,
			},
		},
	}

	twoServerResponse := &pbserverdiscovery.WatchServersResponse{
		Servers: []*pbserverdiscovery.Server{
			{
				Id:      srv1.ID,
				Address: srv1.Address,
				Version: srv1.Version,
			},
			{
				Id:      srv2.ID,
				Address: srv2.Address,
				Version: srv2.Version,
			},
		},
	}

	threeServerResponse := &pbserverdiscovery.WatchServersResponse{
		Servers: []*pbserverdiscovery.Server{
			{
				Id:      srv1.ID,
				Address: srv1.Address,
				Version: srv1.Version,
			},
			{
				Id:      srv2.ID,
				Address: srv2.Address,
				Version: srv2.Version,
			},
			{
				Id:      srv3.ID,
				Address: srv3.Address,
				Version: srv3.Version,
			},
		},
	}

	// setup the event publisher and snapshot handler
	handler, publisher := setupPublisher(t)
	// we only expect this to be called once. For the rest of the
	// test we ought to be able to resume the stream.
	handler.expect(testACLToken, 0, 1, oneServerEventPayload)
	handler.expect(testACLToken, 2, 3, twoServerEventPayload)

	// setup the mock ACLResolver and its expectations
	// 2 times authorization should succeed and the third should fail.
	resolver := newMockACLResolver(t)
	resolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.TestAuthorizerServiceWriteAny(t), nil).Twice()

	// add the token to the requests context
	ctx := external.ContextWithToken(context.Background(), testACLToken)

	// setup the server
	server := NewServer(Config{
		Publisher:   publisher,
		Logger:      testutil.Logger(t),
		ACLResolver: resolver,
	})

	// Run the server and get a test client for it
	client := testClient(t, server)

	// 1. Open the WatchServers stream
	serverStream, err := client.WatchServers(ctx, &pbserverdiscovery.WatchServersRequest{Wan: false})
	require.NoError(t, err)

	rspCh := handleReadyServersStream(t, serverStream)

	// 2. Observe the snapshot message is sent back through the stream.
	rsp := mustGetServers(t, rspCh)
	require.NotNil(t, rsp)
	prototest.AssertDeepEqual(t, oneServerResponse, rsp)

	// 3. Publish an event that changes to 2 servers.
	publisher.Publish([]stream.Event{
		{
			Topic:   autopilotevents.EventTopicReadyServers,
			Index:   2,
			Payload: twoServerEventPayload,
		},
	})

	// 4. See the corresponding message sent back through the stream.
	rsp = mustGetServers(t, rspCh)
	require.NotNil(t, rsp)
	prototest.AssertDeepEqual(t, twoServerResponse, rsp)

	// 5. Send a NewCloseSubscriptionEvent for the token secret.
	publisher.Publish([]stream.Event{
		stream.NewCloseSubscriptionEvent([]string{testACLToken}),
	})

	// 6. Observe another snapshot message
	rsp = mustGetServers(t, rspCh)
	require.NotNil(t, rsp)
	prototest.AssertDeepEqual(t, twoServerResponse, rsp)

	// 7. Publish another event to move to 3 servers.
	publisher.Publish([]stream.Event{
		{
			Topic:   autopilotevents.EventTopicReadyServers,
			Index:   4,
			Payload: threeServerEventPayload,
		},
	})

	// 8. Ensure that the message gets sent through the stream. Also
	//    this will validate that no other 1 or 2 server event is
	//    seen after stream reinitialization.
	rsp = mustGetServers(t, rspCh)
	require.NotNil(t, rsp)
	prototest.AssertDeepEqual(t, threeServerResponse, rsp)
}

func TestWatchServers_ACLToken_PermissionDenied(t *testing.T) {
	// setup the event publisher and snapshot handler
	_, publisher := setupPublisher(t)

	resolver := newMockACLResolver(t)
	resolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.TestAuthorizerDenyAll(t), nil).Once()

	// add the token to the requests context
	ctx := external.ContextWithToken(context.Background(), testACLToken)

	// setup the server
	server := NewServer(Config{
		Publisher:   publisher,
		Logger:      testutil.Logger(t),
		ACLResolver: resolver,
	})

	// Run the server and get a test client for it
	client := testClient(t, server)

	// 1. Open the WatchServers stream
	serverStream, err := client.WatchServers(ctx, &pbserverdiscovery.WatchServersRequest{Wan: false})
	require.NoError(t, err)
	rspCh := handleReadyServersStream(t, serverStream)

	// Expect to get an Unauthenticated error immediately.
	err = mustGetError(t, rspCh)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
}

func TestWatchServers_ACLToken_Unauthenticated(t *testing.T) {
	// setup the event publisher and snapshot handler
	_, publisher := setupPublisher(t)

	aclResolver := newMockACLResolver(t)
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound).Once()

	// add the token to the requests context
	ctx := external.ContextWithToken(context.Background(), testACLToken)

	// setup the server
	server := NewServer(Config{
		Publisher:   publisher,
		Logger:      testutil.Logger(t),
		ACLResolver: aclResolver,
	})

	// Run the server and get a test client for it
	client := testClient(t, server)

	// 1. Open the WatchServers stream
	serverStream, err := client.WatchServers(ctx, &pbserverdiscovery.WatchServersRequest{Wan: false})
	require.NoError(t, err)
	rspCh := handleReadyServersStream(t, serverStream)

	// Expect to get an Unauthenticated error immediately.
	err = mustGetError(t, rspCh)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
}

func handleReadyServersStream(t *testing.T, stream pbserverdiscovery.ServerDiscoveryService_WatchServersClient) <-chan serversOrError {
	t.Helper()

	rspCh := make(chan serversOrError)
	go func() {
		for {
			rsp, err := stream.Recv()
			if errors.Is(err, io.EOF) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return
			}
			rspCh <- serversOrError{
				rsp: rsp,
				err: err,
			}
		}
	}()
	return rspCh
}

func mustGetServers(t *testing.T, ch <-chan serversOrError) *pbserverdiscovery.WatchServersResponse {
	t.Helper()

	select {
	case rsp := <-ch:
		require.NoError(t, rsp.err)
		return rsp.rsp
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for WatchServersResponse")
		return nil
	}
}

func mustGetError(t *testing.T, ch <-chan serversOrError) error {
	t.Helper()

	select {
	case rsp := <-ch:
		require.Error(t, rsp.err)
		return rsp.err
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for WatchServersResponse")
		return nil
	}
}

type serversOrError struct {
	rsp *pbserverdiscovery.WatchServersResponse
	err error
}
