package subscribe

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/types"
)

func TestServer_Subscribe_IntegrationWithBackend(t *testing.T) {
	backend, err := newTestBackend()
	require.NoError(t, err)
	srv := &Server{Backend: backend, Logger: hclog.New(nil)}
	addr := newTestServer(t, srv)
	ids := newCounter()

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
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("other"), req))
	}
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
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg2"), req))
	}
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
	require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg3"), req))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))

	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
	streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   "redis",
	})
	require.NoError(t, err)

	chEvents := make(chan eventOrError, 0)
	go recvEvents(chEvents, streamHandle)

	var snapshotEvents []*pbsubscribe.Event
	for i := 0; i < 3; i++ {
		snapshotEvents = append(snapshotEvents, getEvent(t, chEvents))
	}

	expected := []*pbsubscribe.Event{
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Index: ids.Last(),
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbservice.CheckServiceNode{
						Node: &pbservice.Node{
							Node:       "node1",
							Datacenter: "dc1",
							Address:    "3.4.5.6",
							RaftIndex:  raftIndex(ids, "reg2", "reg2"),
						},
						Service: &pbservice.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "3.4.5.6",
							Port:    8080,
							Weights: &pbservice.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbservice.ConnectProxyConfig{
								MeshGateway: pbservice.MeshGatewayConfig{},
								Expose:      pbservice.ExposeConfig{},
							},
							RaftIndex:      raftIndex(ids, "reg2", "reg2"),
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
						},
					},
				},
			},
		},
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Index: ids.Last(),
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbservice.CheckServiceNode{
						Node: &pbservice.Node{
							Node:       "node2",
							Datacenter: "dc1",
							Address:    "1.2.3.4",
							RaftIndex:  raftIndex(ids, "reg3", "reg3"),
						},
						Service: &pbservice.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "1.1.1.1",
							Port:    8080,
							Weights: &pbservice.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbservice.ConnectProxyConfig{
								MeshGateway: pbservice.MeshGatewayConfig{},
								Expose:      pbservice.ExposeConfig{},
							},
							RaftIndex:      raftIndex(ids, "reg3", "reg3"),
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
						},
					},
				},
			},
		},
		{
			Topic:   pbsubscribe.Topic_ServiceHealth,
			Key:     "redis",
			Index:   ids.Last(),
			Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
		},
	}
	assertDeepEqual(t, expected, snapshotEvents)

	// Update the registration by adding a check.
	req.Check = &structs.HealthCheck{
		Node:        "node2",
		CheckID:     types.CheckID("check1"),
		ServiceID:   "redis1",
		ServiceName: "redis",
		Name:        "check 1",
	}
	require.NoError(t, backend.store.EnsureRegistration(ids.Next("update"), req))

	event := getEvent(t, chEvents)
	expectedEvent := &pbsubscribe.Event{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   "redis",
		Index: ids.Last(),
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Register,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Node: &pbservice.Node{
						Node:       "node2",
						Datacenter: "dc1",
						Address:    "1.2.3.4",
						RaftIndex:  raftIndex(ids, "reg3", "reg3"),
					},
					Service: &pbservice.NodeService{
						ID:      "redis1",
						Service: "redis",
						Address: "1.1.1.1",
						Port:    8080,
						Weights: &pbservice.Weights{Passing: 1, Warning: 1},
						// Sad empty state
						Proxy: pbservice.ConnectProxyConfig{
							MeshGateway: pbservice.MeshGatewayConfig{},
							Expose:      pbservice.ExposeConfig{},
						},
						RaftIndex:      raftIndex(ids, "reg3", "reg3"),
						EnterpriseMeta: pbcommon.EnterpriseMeta{},
					},
					Checks: []*pbservice.HealthCheck{
						{
							CheckID:        "check1",
							Name:           "check 1",
							Node:           "node2",
							Status:         "critical",
							ServiceID:      "redis1",
							ServiceName:    "redis",
							RaftIndex:      raftIndex(ids, "update", "update"),
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
						},
					},
				},
			},
		},
	}
	assertDeepEqual(t, expectedEvent, event)
}

type eventOrError struct {
	event *pbsubscribe.Event
	err   error
}

// recvEvents from handle and sends them to the provided channel.
func recvEvents(ch chan eventOrError, handle pbsubscribe.StateChangeSubscription_SubscribeClient) {
	defer close(ch)
	for {
		event, err := handle.Recv()
		switch {
		case errors.Is(err, io.EOF):
			return
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return
		case err != nil:
			ch <- eventOrError{err: err}
			return
		default:
			ch <- eventOrError{event: event}
		}
	}
}

func getEvent(t *testing.T, ch chan eventOrError) *pbsubscribe.Event {
	t.Helper()
	select {
	case item := <-ch:
		require.NoError(t, item.err)
		return item.event
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting on event from server")
	}
	return nil
}

func assertDeepEqual(t *testing.T, x, y interface{}) {
	t.Helper()
	if diff := cmp.Diff(x, y); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}

type testBackend struct {
	store       *state.Store
	authorizer  func(token string) acl.Authorizer
	forwardConn *gogrpc.ClientConn
}

func (b testBackend) ResolveToken(token string) (acl.Authorizer, error) {
	return b.authorizer(token), nil
}

func (b testBackend) Forward(_ string, fn func(*gogrpc.ClientConn) error) (handled bool, err error) {
	if b.forwardConn != nil {
		return true, fn(b.forwardConn)
	}
	return false, nil
}

func (b testBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return b.store.EventPublisher().Subscribe(req)
}

func newTestBackend() (*testBackend, error) {
	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStateStore(gc)
	if err != nil {
		return nil, err
	}
	allowAll := func(_ string) acl.Authorizer {
		return acl.AllowAll()
	}
	return &testBackend{store: store, authorizer: allowAll}, nil
}

var _ Backend = (*testBackend)(nil)

func newTestServer(t *testing.T, server *Server) net.Addr {
	addr := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	var grpcServer *gogrpc.Server
	handler := grpc.NewHandler(addr, func(srv *gogrpc.Server) {
		grpcServer = srv
		pbsubscribe.RegisterStateChangeSubscriptionServer(srv, server)
	})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(logError(t, lis.Close))

	go grpcServer.Serve(lis)
	g := new(errgroup.Group)
	g.Go(func() error {
		return grpcServer.Serve(lis)
	})
	t.Cleanup(func() {
		if err := handler.Shutdown(); err != nil {
			t.Logf("grpc server shutdown: %v", err)
		}
		if err := g.Wait(); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	})
	return lis.Addr()
}

type counter struct {
	value  uint64
	labels map[string]uint64
}

func (c *counter) Next(label string) uint64 {
	c.value++
	c.labels[label] = c.value
	return c.value
}

func (c *counter) For(label string) uint64 {
	return c.labels[label]
}

func (c *counter) Last() uint64 {
	return c.value
}

func newCounter() *counter {
	return &counter{labels: make(map[string]uint64)}
}

func raftIndex(ids *counter, created, modified string) pbcommon.RaftIndex {
	return pbcommon.RaftIndex{
		CreateIndex: ids.For(created),
		ModifyIndex: ids.For(modified),
	}
}

func TestServer_Subscribe_IntegrationWithBackend_ForwardToDC(t *testing.T) {
	backendLocal, err := newTestBackend()
	require.NoError(t, err)
	addrLocal := newTestServer(t, &Server{Backend: backendLocal, Logger: hclog.New(nil)})

	backendRemoteDC, err := newTestBackend()
	require.NoError(t, err)
	srvRemoteDC := &Server{Backend: backendRemoteDC, Logger: hclog.New(nil)}
	addrRemoteDC := newTestServer(t, srvRemoteDC)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	connRemoteDC, err := gogrpc.DialContext(ctx, addrRemoteDC.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, connRemoteDC.Close))
	backendLocal.forwardConn = connRemoteDC

	ids := newCounter()
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
		require.NoError(t, backendRemoteDC.store.EnsureRegistration(ids.Next("reg1"), req))
	}
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
		require.NoError(t, backendRemoteDC.store.EnsureRegistration(ids.Next("reg2"), req))
	}

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
	require.NoError(t, backendRemoteDC.store.EnsureRegistration(ids.Next("reg3"), req))

	connLocal, err := gogrpc.DialContext(ctx, addrLocal.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, connLocal.Close))

	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(connLocal)
	streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
		Topic:      pbsubscribe.Topic_ServiceHealth,
		Key:        "redis",
		Datacenter: "dc2",
	})
	require.NoError(t, err)

	chEvents := make(chan eventOrError, 0)
	go recvEvents(chEvents, streamHandle)

	var snapshotEvents []*pbsubscribe.Event
	for i := 0; i < 3; i++ {
		snapshotEvents = append(snapshotEvents, getEvent(t, chEvents))
	}

	expected := []*pbsubscribe.Event{
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Index: ids.Last(),
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbservice.CheckServiceNode{
						Node: &pbservice.Node{
							Node:       "node1",
							Datacenter: "dc2",
							Address:    "3.4.5.6",
							RaftIndex:  raftIndex(ids, "reg2", "reg2"),
						},
						Service: &pbservice.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "3.4.5.6",
							Port:    8080,
							Weights: &pbservice.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbservice.ConnectProxyConfig{
								MeshGateway: pbservice.MeshGatewayConfig{},
								Expose:      pbservice.ExposeConfig{},
							},
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
							RaftIndex:      raftIndex(ids, "reg2", "reg2"),
						},
					},
				},
			},
		},
		{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "redis",
			Index: ids.Last(),
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbservice.CheckServiceNode{
						Node: &pbservice.Node{
							Node:       "node2",
							Datacenter: "dc2",
							Address:    "1.2.3.4",
							RaftIndex:  raftIndex(ids, "reg3", "reg3"),
						},
						Service: &pbservice.NodeService{
							ID:      "redis1",
							Service: "redis",
							Address: "1.1.1.1",
							Port:    8080,
							Weights: &pbservice.Weights{Passing: 1, Warning: 1},
							// Sad empty state
							Proxy: pbservice.ConnectProxyConfig{
								MeshGateway: pbservice.MeshGatewayConfig{},
								Expose:      pbservice.ExposeConfig{},
							},
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
							RaftIndex:      raftIndex(ids, "reg3", "reg3"),
						},
					},
				},
			},
		},
		{
			Topic:   pbsubscribe.Topic_ServiceHealth,
			Key:     "redis",
			Index:   ids.Last(),
			Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
		},
	}
	assertDeepEqual(t, expected, snapshotEvents)

	// Update the registration by adding a check.
	req.Check = &structs.HealthCheck{
		Node:        "node2",
		CheckID:     types.CheckID("check1"),
		ServiceID:   "redis1",
		ServiceName: "redis",
		Name:        "check 1",
	}
	require.NoError(t, backendRemoteDC.store.EnsureRegistration(ids.Next("update"), req))

	event := getEvent(t, chEvents)
	expectedEvent := &pbsubscribe.Event{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   "redis",
		Index: ids.Last(),
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Register,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Node: &pbservice.Node{
						Node:       "node2",
						Datacenter: "dc2",
						Address:    "1.2.3.4",
						RaftIndex:  raftIndex(ids, "reg3", "reg3"),
					},
					Service: &pbservice.NodeService{
						ID:        "redis1",
						Service:   "redis",
						Address:   "1.1.1.1",
						Port:      8080,
						RaftIndex: raftIndex(ids, "reg3", "reg3"),
						Weights:   &pbservice.Weights{Passing: 1, Warning: 1},
						// Sad empty state
						Proxy: pbservice.ConnectProxyConfig{
							MeshGateway: pbservice.MeshGatewayConfig{},
							Expose:      pbservice.ExposeConfig{},
						},
						EnterpriseMeta: pbcommon.EnterpriseMeta{},
					},
					Checks: []*pbservice.HealthCheck{
						{
							CheckID:        "check1",
							Name:           "check 1",
							Node:           "node2",
							Status:         "critical",
							ServiceID:      "redis1",
							ServiceName:    "redis",
							RaftIndex:      raftIndex(ids, "update", "update"),
							EnterpriseMeta: pbcommon.EnterpriseMeta{},
						},
					},
				},
			},
		},
	}
	assertDeepEqual(t, expectedEvent, event)
}

// TODO: test case for converting stream.Events to pbsubscribe.Events, including framing events

func TestServer_Subscribe_IntegrationWithBackend_FilterEventsByACLToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for -short run")
	}

	backend, err := newTestBackend()
	require.NoError(t, err)
	srv := &Server{Backend: backend, Logger: hclog.New(nil)}
	addr := newTestServer(t, srv)

	// Create a policy for the test token.
	rules := `
service "foo" {
	policy = "write"
}
node "node1" {
	policy = "write"
}
`
	authorizer, err := acl.NewAuthorizerFromRules(
		"1", 0, rules, acl.SyntaxCurrent,
		&acl.Config{WildcardName: structs.WildcardSpecifier},
		nil)
	require.NoError(t, err)
	authorizer = acl.NewChainedAuthorizer([]acl.Authorizer{authorizer, acl.DenyAll()})
	require.Equal(t, acl.Deny, authorizer.NodeRead("denied", nil))

	// TODO: is there any easy way to do this with the acl package?
	token := "this-token-is-good"
	backend.authorizer = func(tok string) acl.Authorizer {
		if tok == token {
			return authorizer
		}
		return acl.DenyAll()
	}

	ids := newCounter()
	{
		req := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "node1",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "foo",
				Service: "foo",
			},
			Check: &structs.HealthCheck{
				CheckID:   "service:foo",
				Name:      "service:foo",
				Node:      "node1",
				ServiceID: "foo",
				Status:    api.HealthPassing,
			},
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg1"), req))

		// Register a service which should be denied
		req = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "node1",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "bar",
				Service: "bar",
			},
			Check: &structs.HealthCheck{
				CheckID:   "service:bar",
				Name:      "service:bar",
				Node:      "node1",
				ServiceID: "bar",
			},
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg2"), req))

		req = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "denied",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "foo",
				Service: "foo",
			},
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg3"), req))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))
	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)

	// Start a Subscribe call to our streaming endpoint for the service we have access to.
	{
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "foo",
			Token: token,
		})
		require.NoError(t, err)

		chEvents := make(chan eventOrError, 0)
		go recvEvents(chEvents, streamHandle)

		event := getEvent(t, chEvents)
		require.Equal(t, "foo", event.GetServiceHealth().CheckServiceNode.Service.Service)
		require.Equal(t, "node1", event.GetServiceHealth().CheckServiceNode.Node.Node)

		require.True(t, getEvent(t, chEvents).GetEndOfSnapshot())

		// Update the service with a new port to trigger a new event.
		req := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "node1",
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
				Node:      "node1",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg4"), req))

		event = getEvent(t, chEvents)
		service := event.GetServiceHealth().CheckServiceNode.Service
		require.Equal(t, "foo", service.Service)
		require.Equal(t, int32(1234), service.Port)

		// Now update the service on the denied node and make sure we don't see an event.
		req = &structs.RegisterRequest{
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
				Node:      "denied",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg5"), req))

		select {
		case event := <-chEvents:
			t.Fatalf("should not have received event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Start another subscribe call for bar, which the token shouldn't have access to.
	{
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Key:   "bar",
			Token: token,
		})
		require.NoError(t, err)

		chEvents := make(chan eventOrError, 0)
		go recvEvents(chEvents, streamHandle)

		require.True(t, getEvent(t, chEvents).GetEndOfSnapshot())

		// Update the service and make sure we don't get a new event.
		req := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "node1",
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
				Node:      "node1",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("reg6"), req))

		select {
		case event := <-chEvents:
			t.Fatalf("should not have received event: %v", event)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func TestServer_Subscribe_IntegrationWithBackend_ACLUpdate(t *testing.T) {
	backend, err := newTestBackend()
	require.NoError(t, err)
	srv := &Server{Backend: backend, Logger: hclog.New(nil)}
	addr := newTestServer(t, srv)

	rules := `
service "foo" {
	policy = "write"
}
node "node1" {
	policy = "write"
}
`
	authorizer, err := acl.NewAuthorizerFromRules(
		"1", 0, rules, acl.SyntaxCurrent,
		&acl.Config{WildcardName: structs.WildcardSpecifier},
		nil)
	require.NoError(t, err)
	authorizer = acl.NewChainedAuthorizer([]acl.Authorizer{authorizer, acl.DenyAll()})
	require.Equal(t, acl.Deny, authorizer.NodeRead("denied", nil))

	// TODO: is there any easy way to do this with the acl package?
	token := "this-token-is-good"
	backend.authorizer = func(tok string) acl.Authorizer {
		if tok == token {
			return authorizer
		}
		return acl.DenyAll()
	}

	ids := newCounter()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))
	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)

	streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Key:   "foo",
		Token: token,
	})
	require.NoError(t, err)

	chEvents := make(chan eventOrError, 0)
	go recvEvents(chEvents, streamHandle)

	require.True(t, getEvent(t, chEvents).GetEndOfSnapshot())

	tokenID, err := uuid.GenerateUUID()
	require.NoError(t, err)

	aclToken := &structs.ACLToken{
		AccessorID: tokenID,
		SecretID:   token,
		Rules:      "",
	}
	require.NoError(t, backend.store.ACLTokenSet(ids.Next("update"), aclToken, false))

	select {
	case item := <-chEvents:
		require.Error(t, item.err, "got event: %v", item.event)
		s, _ := status.FromError(item.err)
		require.Equal(t, codes.Aborted, s.Code())
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for aborted error")
	}
}

func logError(t *testing.T, f func() error) func() {
	return func() {
		if err := f(); err != nil {
			t.Logf(err.Error())
		}
	}
}
