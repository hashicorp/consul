package serverdiscovery

import (
	"context"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/autopilotevents"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/proto-public/pbserverdiscovery"
)

type mockSnapshotHandler struct {
	mock.Mock
}

func newMockSnapshotHandler(t *testing.T) *mockSnapshotHandler {
	handler := &mockSnapshotHandler{}
	t.Cleanup(func() {
		handler.AssertExpectations(t)
	})
	return handler
}

func (m *mockSnapshotHandler) handle(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	ret := m.Called(req, buf)
	return ret.Get(0).(uint64), ret.Error(1)
}

func (m *mockSnapshotHandler) expect(token string, requestIndex uint64, eventIndex uint64, payload autopilotevents.EventPayloadReadyServers) {
	m.On("handle", stream.SubscribeRequest{
		Topic:   autopilotevents.EventTopicReadyServers,
		Subject: stream.SubjectNone,
		Token:   token,
		Index:   requestIndex,
	}, mock.Anything).Once().Run(func(args mock.Arguments) {
		buf := args.Get(1).(stream.SnapshotAppender)
		buf.Append([]stream.Event{
			{
				Topic:   autopilotevents.EventTopicReadyServers,
				Index:   eventIndex,
				Payload: payload,
			},
		})
	}).Return(eventIndex, nil)
}

func newMockACLResolver(t *testing.T) *MockACLResolver {
	t.Helper()
	m := &MockACLResolver{}
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

func setupPublisher(t *testing.T) (*mockSnapshotHandler, state.EventPublisher) {
	t.Helper()

	handler := newMockSnapshotHandler(t)

	publisher := stream.NewEventPublisher(10 * time.Second)
	publisher.RegisterHandler(autopilotevents.EventTopicReadyServers, handler.handle, false)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go publisher.Run(ctx)

	return handler, publisher

}

func testClient(t *testing.T, server *Server) pbserverdiscovery.ServerDiscoveryServiceClient {
	t.Helper()

	addr := testutils.RunTestServer(t, server)

	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbserverdiscovery.NewServerDiscoveryServiceClient(conn)
}
