// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package subscribe

import (
	"context"
	"errors"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	grpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestServer_Subscribe_SubjectIsRequired(t *testing.T) {
	backend := newTestBackend(t)

	addr := runTestServer(t, NewServer(backend, hclog.New(nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	//nolint:staticcheck
	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))

	client := pbsubscribe.NewStateChangeSubscriptionClient(conn)

	stream, err := client.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
		Topic: pbsubscribe.Topic_ServiceHealth,
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "either WildcardSubject or NamedSubject.Key is required")
}

func TestServer_Subscribe_IntegrationWithBackend(t *testing.T) {
	backend := newTestBackend(t)
	addr := runTestServer(t, NewServer(backend, hclog.New(nil)))
	ids := newCounter()

	var req *structs.RegisterRequest
	testutil.RunStep(t, "register two instances of the redis service", func(t *testing.T) {
		req = &structs.RegisterRequest{
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

		req = &structs.RegisterRequest{
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
	})

	testutil.RunStep(t, "register a service by a different name", func(t *testing.T) {
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
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	//nolint:staticcheck
	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))

	chEvents := make(chan eventOrError, 0)
	var snapshotEvents []*pbsubscribe.Event

	testutil.RunStep(t, "setup a client and subscribe to a topic", func(t *testing.T) {
		streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key:       "redis",
					Namespace: pbcommon.DefaultEnterpriseMeta.Namespace,
				},
			},
		})
		require.NoError(t, err)

		go recvEvents(chEvents, streamHandle)
		for i := 0; i < 3; i++ {
			snapshotEvents = append(snapshotEvents, getEvent(t, chEvents))
		}
	})

	testutil.RunStep(t, "receive the initial snapshot of events", func(t *testing.T) {
		expected := []*pbsubscribe.Event{
			{
				Index: ids.For("reg3"),
				Payload: &pbsubscribe.Event_ServiceHealth{
					ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
						Op: pbsubscribe.CatalogOp_Register,
						CheckServiceNode: &pbservice.CheckServiceNode{
							Node: &pbservice.Node{
								Node:       "node1",
								Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
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
								Proxy: &pbservice.ConnectProxyConfig{
									MeshGateway:      &pbservice.MeshGatewayConfig{},
									Expose:           &pbservice.ExposeConfig{},
									TransparentProxy: &pbservice.TransparentProxyConfig{},
									AccessLogs:       &pbservice.AccessLogsConfig{},
								},
								Connect:        &pbservice.ServiceConnect{},
								RaftIndex:      raftIndex(ids, "reg2", "reg2"),
								EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
							},
						},
					},
				},
			},
			{
				Index: ids.For("reg3"),
				Payload: &pbsubscribe.Event_ServiceHealth{
					ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
						Op: pbsubscribe.CatalogOp_Register,
						CheckServiceNode: &pbservice.CheckServiceNode{
							Node: &pbservice.Node{
								Node:       "node2",
								Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
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
								Proxy: &pbservice.ConnectProxyConfig{
									MeshGateway:      &pbservice.MeshGatewayConfig{},
									Expose:           &pbservice.ExposeConfig{},
									TransparentProxy: &pbservice.TransparentProxyConfig{},
									AccessLogs:       &pbservice.AccessLogsConfig{},
								},
								Connect:        &pbservice.ServiceConnect{},
								RaftIndex:      raftIndex(ids, "reg3", "reg3"),
								EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
							},
						},
					},
				},
			},
			{
				Index:   ids.For("reg3"),
				Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
			},
		}
		prototest.AssertDeepEqual(t, expected, snapshotEvents)
	})

	testutil.RunStep(t, "update the registration by adding a check", func(t *testing.T) {
		req.Check = &structs.HealthCheck{
			Node:        "node2",
			CheckID:     "check1",
			ServiceID:   "redis1",
			ServiceName: "redis",
			Name:        "check 1",
		}
		require.NoError(t, backend.store.EnsureRegistration(ids.Next("update"), req))

		event := getEvent(t, chEvents)
		expectedEvent := &pbsubscribe.Event{
			Index: ids.Last(),
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbservice.CheckServiceNode{
						Node: &pbservice.Node{
							Node:       "node2",
							Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
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
							Proxy: &pbservice.ConnectProxyConfig{
								MeshGateway:      &pbservice.MeshGatewayConfig{},
								Expose:           &pbservice.ExposeConfig{},
								TransparentProxy: &pbservice.TransparentProxyConfig{},
								AccessLogs:       &pbservice.AccessLogsConfig{},
							},
							Connect:        &pbservice.ServiceConnect{},
							RaftIndex:      raftIndex(ids, "reg3", "reg3"),
							EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
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
								EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
								Definition: &pbservice.HealthCheckDefinition{
									Interval:                       &durationpb.Duration{},
									Timeout:                        &durationpb.Duration{},
									DeregisterCriticalServiceAfter: &durationpb.Duration{},
									TTL:                            &durationpb.Duration{},
								},
								LastCheckStartTime: &timestamppb.Timestamp{},
							},
						},
					},
				},
			},
		}
		prototest.AssertDeepEqual(t, expectedEvent, event)
	})
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
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting on event from server")
	}
	return nil
}

type testBackend struct {
	publisher   *stream.EventPublisher
	store       *state.Store
	authorizer  func(token string, entMeta *acl.EnterpriseMeta) acl.Authorizer
	forwardConn *gogrpc.ClientConn
}

func (b testBackend) ResolveTokenAndDefaultMeta(
	token string,
	entMeta *acl.EnterpriseMeta,
	_ *acl.AuthorizerContext,
) (acl.Authorizer, error) {
	return b.authorizer(token, entMeta), nil
}

func (b testBackend) Forward(_ structs.RPCInfo, fn func(*gogrpc.ClientConn) error) (handled bool, err error) {
	if b.forwardConn != nil {
		return true, fn(b.forwardConn)
	}
	return false, nil
}

func (b testBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return b.publisher.Subscribe(req)
}

func newTestBackend(t *testing.T) *testBackend {
	t.Helper()
	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	publisher := stream.NewEventPublisher(10 * time.Second)

	store := state.NewStateStoreWithEventPublisher(gc, publisher)

	// normally the handlers are registered on the FSM as state stores may come
	// and go during snapshot restores. For the purposes of this test backend though we
	// just register them directly to
	require.NoError(t, publisher.RegisterHandler(state.EventTopicCARoots, store.CARootsSnapshot, false))
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealth, store.ServiceHealthSnapshot, false))
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealthConnect, store.ServiceHealthSnapshot, false))

	ctx, cancel := context.WithCancel(context.Background())
	go publisher.Run(ctx)
	t.Cleanup(cancel)

	allowAll := func(string, *acl.EnterpriseMeta) acl.Authorizer {
		return acl.AllowAll()
	}
	return &testBackend{publisher: publisher, store: store, authorizer: allowAll}
}

var _ Backend = (*testBackend)(nil)

func runTestServer(t *testing.T, server *Server) net.Addr {
	addr := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	var grpcServer *gogrpc.Server
	handler := grpc.NewHandler(
		hclog.New(nil),
		addr,
		func(srv *gogrpc.Server) {
			grpcServer = srv
			pbsubscribe.RegisterStateChangeSubscriptionServer(srv, server)
		},
		nil,
		rate.NullRequestLimitsHandler(),
	)

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

func raftIndex(ids *counter, created, modified string) *pbcommon.RaftIndex {
	return &pbcommon.RaftIndex{
		CreateIndex: ids.For(created),
		ModifyIndex: ids.For(modified),
	}
}

func TestServer_Subscribe_IntegrationWithBackend_ForwardToDC(t *testing.T) {
	backendLocal := newTestBackend(t)
	addrLocal := runTestServer(t, NewServer(backendLocal, hclog.New(nil)))

	backendRemoteDC := newTestBackend(t)
	srvRemoteDC := NewServer(backendRemoteDC, hclog.New(nil))
	addrRemoteDC := runTestServer(t, srvRemoteDC)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	//nolint:staticcheck
	connRemoteDC, err := gogrpc.DialContext(ctx, addrRemoteDC.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, connRemoteDC.Close))
	backendLocal.forwardConn = connRemoteDC

	ids := newCounter()

	var req *structs.RegisterRequest
	testutil.RunStep(t, "register three services", func(t *testing.T) {
		req = &structs.RegisterRequest{
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
		req = &structs.RegisterRequest{
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
		req = &structs.RegisterRequest{
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
	})

	//nolint:staticcheck
	connLocal, err := gogrpc.DialContext(ctx, addrLocal.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, connLocal.Close))

	chEvents := make(chan eventOrError, 0)
	var snapshotEvents []*pbsubscribe.Event

	testutil.RunStep(t, "setup a client and subscribe to a topic", func(t *testing.T) {
		streamClient := pbsubscribe.NewStateChangeSubscriptionClient(connLocal)
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key:       "redis",
					Namespace: pbcommon.DefaultEnterpriseMeta.Namespace,
				},
			},
			Datacenter: "dc2",
		})
		require.NoError(t, err)
		go recvEvents(chEvents, streamHandle)

		for i := 0; i < 3; i++ {
			snapshotEvents = append(snapshotEvents, getEvent(t, chEvents))
		}
	})

	testutil.RunStep(t, "receive the initial snapshot of events", func(t *testing.T) {
		expected := []*pbsubscribe.Event{
			{
				Index: ids.Last(),
				Payload: &pbsubscribe.Event_ServiceHealth{
					ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
						Op: pbsubscribe.CatalogOp_Register,
						CheckServiceNode: &pbservice.CheckServiceNode{
							Node: &pbservice.Node{
								Node:       "node1",
								Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
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
								Proxy: &pbservice.ConnectProxyConfig{
									MeshGateway:      &pbservice.MeshGatewayConfig{},
									Expose:           &pbservice.ExposeConfig{},
									TransparentProxy: &pbservice.TransparentProxyConfig{},
									AccessLogs:       &pbservice.AccessLogsConfig{},
								},
								Connect:        &pbservice.ServiceConnect{},
								EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
								RaftIndex:      raftIndex(ids, "reg2", "reg2"),
							},
						},
					},
				},
			},
			{
				Index: ids.Last(),
				Payload: &pbsubscribe.Event_ServiceHealth{
					ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
						Op: pbsubscribe.CatalogOp_Register,
						CheckServiceNode: &pbservice.CheckServiceNode{
							Node: &pbservice.Node{
								Node:       "node2",
								Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
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
								Proxy: &pbservice.ConnectProxyConfig{
									MeshGateway:      &pbservice.MeshGatewayConfig{},
									Expose:           &pbservice.ExposeConfig{},
									TransparentProxy: &pbservice.TransparentProxyConfig{},
									AccessLogs:       &pbservice.AccessLogsConfig{},
								},
								Connect:        &pbservice.ServiceConnect{},
								EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
								RaftIndex:      raftIndex(ids, "reg3", "reg3"),
							},
						},
					},
				},
			},
			{
				Index:   ids.Last(),
				Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
			},
		}
		prototest.AssertDeepEqual(t, expected, snapshotEvents)
	})

	testutil.RunStep(t, "update the registration by adding a check", func(t *testing.T) {
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
			Index: ids.Last(),
			Payload: &pbsubscribe.Event_ServiceHealth{
				ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
					Op: pbsubscribe.CatalogOp_Register,
					CheckServiceNode: &pbservice.CheckServiceNode{
						Node: &pbservice.Node{
							Node:       "node2",
							Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
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
							Proxy: &pbservice.ConnectProxyConfig{
								MeshGateway:      &pbservice.MeshGatewayConfig{},
								Expose:           &pbservice.ExposeConfig{},
								TransparentProxy: &pbservice.TransparentProxyConfig{},
								AccessLogs:       &pbservice.AccessLogsConfig{},
							},
							Connect:        &pbservice.ServiceConnect{},
							EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
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
								EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
								Definition: &pbservice.HealthCheckDefinition{
									Interval:                       &durationpb.Duration{},
									Timeout:                        &durationpb.Duration{},
									DeregisterCriticalServiceAfter: &durationpb.Duration{},
									TTL:                            &durationpb.Duration{},
								},
							},
						},
					},
				},
			},
		}
		prototest.AssertDeepEqual(t, expectedEvent, event)
	})
}

func TestServer_Subscribe_IntegrationWithBackend_FilterEventsByACLToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	if testing.Short() {
		t.Skip("too slow for -short run")
	}

	backend := newTestBackend(t)
	addr := runTestServer(t, NewServer(backend, hclog.New(nil)))
	token := "this-token-is-good"

	testutil.RunStep(t, "create an ACL policy", func(t *testing.T) {
		rules := `
service "foo" {
	policy = "write"
}
node "node1" {
	policy = "write"
}
`
		cfg := &acl.Config{WildcardName: structs.WildcardSpecifier}
		authorizer, err := acl.NewAuthorizerFromRules(rules, cfg, nil)
		require.NoError(t, err)
		authorizer = acl.NewChainedAuthorizer([]acl.Authorizer{authorizer, acl.DenyAll()})
		require.Equal(t, acl.Deny, authorizer.NodeRead("denied", nil))

		// TODO: is there any easy way to do this with the acl package?
		backend.authorizer = func(tok string, _ *acl.EnterpriseMeta) acl.Authorizer {
			if tok == token {
				return authorizer
			}
			return acl.DenyAll()
		}
	})

	ids := newCounter()
	var req *structs.RegisterRequest

	testutil.RunStep(t, "register services", func(t *testing.T) {
		req = &structs.RegisterRequest{
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
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	//nolint:staticcheck
	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))
	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)

	chEvents := make(chan eventOrError, 0)

	testutil.RunStep(t, "setup a client, subscribe to a topic, and receive a snapshot", func(t *testing.T) {
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key:       "foo",
					Namespace: pbcommon.DefaultEnterpriseMeta.Namespace,
				},
			},
			Token: token,
		})
		require.NoError(t, err)

		go recvEvents(chEvents, streamHandle)

		event := getEvent(t, chEvents)
		require.Equal(t, "foo", event.GetServiceHealth().CheckServiceNode.Service.Service)
		require.Equal(t, "node1", event.GetServiceHealth().CheckServiceNode.Node.Node)

		require.True(t, getEvent(t, chEvents).GetEndOfSnapshot())
	})

	testutil.RunStep(t, "update the service to receive an event", func(t *testing.T) {
		req = &structs.RegisterRequest{
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

		event := getEvent(t, chEvents)
		service := event.GetServiceHealth().CheckServiceNode.Service
		require.Equal(t, "foo", service.Service)
		require.Equal(t, int32(1234), service.Port)
	})

	testutil.RunStep(t, "updates to the service on the denied node, should not send an event", func(t *testing.T) {
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

		assertNoEvents(t, chEvents)
	})

	testutil.RunStep(t, "subscribe to a topic where events are not visible", func(t *testing.T) {
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key: "bar",
				},
			},
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
		assertNoEvents(t, chEvents)
	})
}

func TestServer_Subscribe_IntegrationWithBackend_ACLUpdate(t *testing.T) {
	backend := newTestBackend(t)
	addr := runTestServer(t, NewServer(backend, hclog.New(nil)))
	token := "this-token-is-good"

	testutil.RunStep(t, "create an ACL policy", func(t *testing.T) {
		rules := `
service "foo" {
	policy = "write"
}
node "node1" {
	policy = "write"
}
`
		authorizer, err := acl.NewAuthorizerFromRules(rules, &acl.Config{WildcardName: structs.WildcardSpecifier}, nil)
		require.NoError(t, err)
		authorizer = acl.NewChainedAuthorizer([]acl.Authorizer{authorizer, acl.DenyAll()})
		require.Equal(t, acl.Deny, authorizer.NodeRead("denied", nil))

		// TODO: is there any easy way to do this with the acl package?
		backend.authorizer = func(tok string, _ *acl.EnterpriseMeta) acl.Authorizer {
			if tok == token {
				return authorizer
			}
			return acl.DenyAll()
		}
	})

	ids := newCounter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	//nolint:staticcheck
	conn, err := gogrpc.DialContext(ctx, addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(logError(t, conn.Close))

	chEvents := make(chan eventOrError, 0)

	testutil.RunStep(t, "setup a client and subscribe to a topic", func(t *testing.T) {
		streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
		streamHandle, err := streamClient.Subscribe(ctx, &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key: "foo",
				},
			},
			Token: token,
		})
		require.NoError(t, err)

		go recvEvents(chEvents, streamHandle)
		require.True(t, getEvent(t, chEvents).GetEndOfSnapshot())
	})

	testutil.RunStep(t, "updates to the token should close the stream", func(t *testing.T) {
		tokenID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		aclToken := &structs.ACLToken{
			AccessorID: tokenID,
			SecretID:   token,
		}
		require.NoError(t, backend.store.ACLTokenSet(ids.Next("update"), aclToken))

		select {
		case item := <-chEvents:
			require.Error(t, item.err, "got event instead of an error: %v", item.event)
			s, _ := status.FromError(item.err)
			require.Equal(t, codes.Aborted, s.Code())
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for aborted error")
		}
	})
}

func assertNoEvents(t *testing.T, chEvents chan eventOrError) {
	t.Helper()
	select {
	case event := <-chEvents:
		t.Fatalf("should not have received event: %v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func logError(t *testing.T, f func() error) func() {
	return func() {
		t.Helper()

		if err := f(); err != nil {
			t.Logf(err.Error())
		}
	}
}

func TestNewEventFromSteamEvent(t *testing.T) {
	type testCase struct {
		name     string
		event    stream.Event
		expected *pbsubscribe.Event
	}

	fn := func(t *testing.T, tc testCase) {
		expected := tc.expected
		actual := tc.event.Payload.ToSubscriptionEvent(tc.event.Index)
		prototest.AssertDeepEqual(t, expected, actual, cmpopts.EquateEmpty())
	}

	var testCases = []testCase{
		{
			name:  "end of snapshot",
			event: newEventFromSubscription(t, 0),
			expected: &pbsubscribe.Event{
				Index:   1,
				Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
			},
		},
		{
			name:  "new snapshot to follow",
			event: newEventFromSubscription(t, 22),
			expected: &pbsubscribe.Event{
				Payload: &pbsubscribe.Event_NewSnapshotToFollow{NewSnapshotToFollow: true},
			},
		},
		{
			name: "event batch",
			event: stream.Event{
				Index: 2002,
				Payload: newPayloadEvents(
					stream.Event{
						Index: 2002,
						Payload: state.EventPayloadCheckServiceNode{
							Op: pbsubscribe.CatalogOp_Register,
							Value: &structs.CheckServiceNode{
								Node:    &structs.Node{Node: "node1"},
								Service: &structs.NodeService{Service: "web1"},
							},
						},
					},
					stream.Event{
						Index: 2002,
						Payload: state.EventPayloadCheckServiceNode{
							Op: pbsubscribe.CatalogOp_Deregister,
							Value: &structs.CheckServiceNode{
								Node:    &structs.Node{Node: "node2"},
								Service: &structs.NodeService{Service: "web1"},
							},
						},
					}),
			},
			expected: &pbsubscribe.Event{
				Index: 2002,
				Payload: &pbsubscribe.Event_EventBatch{
					EventBatch: &pbsubscribe.EventBatch{
						Events: []*pbsubscribe.Event{
							{
								Index: 2002,
								Payload: &pbsubscribe.Event_ServiceHealth{
									ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
										Op: pbsubscribe.CatalogOp_Register,
										CheckServiceNode: &pbservice.CheckServiceNode{
											Node: &pbservice.Node{Node: "node1", RaftIndex: &pbcommon.RaftIndex{}},
											Service: &pbservice.NodeService{
												Service: "web1",
												Proxy: &pbservice.ConnectProxyConfig{
													MeshGateway:      &pbservice.MeshGatewayConfig{},
													Expose:           &pbservice.ExposeConfig{},
													TransparentProxy: &pbservice.TransparentProxyConfig{},
													AccessLogs:       &pbservice.AccessLogsConfig{},
												},
												Connect:        &pbservice.ServiceConnect{},
												EnterpriseMeta: &pbcommon.EnterpriseMeta{},
												RaftIndex:      &pbcommon.RaftIndex{},
											},
										},
									},
								},
							},
							{
								Index: 2002,
								Payload: &pbsubscribe.Event_ServiceHealth{
									ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
										Op: pbsubscribe.CatalogOp_Deregister,
										CheckServiceNode: &pbservice.CheckServiceNode{
											Node: &pbservice.Node{Node: "node2", RaftIndex: &pbcommon.RaftIndex{}},
											Service: &pbservice.NodeService{
												Service: "web1",
												Proxy: &pbservice.ConnectProxyConfig{
													MeshGateway:      &pbservice.MeshGatewayConfig{},
													Expose:           &pbservice.ExposeConfig{},
													TransparentProxy: &pbservice.TransparentProxyConfig{},
													AccessLogs:       &pbservice.AccessLogsConfig{},
												},
												Connect:        &pbservice.ServiceConnect{},
												EnterpriseMeta: &pbcommon.EnterpriseMeta{},
												RaftIndex:      &pbcommon.RaftIndex{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "event payload CheckServiceNode",
			event: stream.Event{
				Index: 2002,
				Payload: state.EventPayloadCheckServiceNode{
					Op: pbsubscribe.CatalogOp_Register,
					Value: &structs.CheckServiceNode{
						Node:    &structs.Node{Node: "node1"},
						Service: &structs.NodeService{Service: "web1"},
					},
				},
			},
			expected: &pbsubscribe.Event{
				Index: 2002,
				Payload: &pbsubscribe.Event_ServiceHealth{
					ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
						Op: pbsubscribe.CatalogOp_Register,
						CheckServiceNode: &pbservice.CheckServiceNode{
							Node: &pbservice.Node{Node: "node1", RaftIndex: &pbcommon.RaftIndex{}},
							Service: &pbservice.NodeService{
								Service: "web1",
								Proxy: &pbservice.ConnectProxyConfig{
									MeshGateway:      &pbservice.MeshGatewayConfig{},
									Expose:           &pbservice.ExposeConfig{},
									TransparentProxy: &pbservice.TransparentProxyConfig{},
									AccessLogs:       &pbservice.AccessLogsConfig{},
								},
								Connect:        &pbservice.ServiceConnect{},
								EnterpriseMeta: &pbcommon.EnterpriseMeta{},
								RaftIndex:      &pbcommon.RaftIndex{},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func newPayloadEvents(items ...stream.Event) *stream.PayloadEvents {
	return &stream.PayloadEvents{Items: items}
}

// newEventFromSubscription is used to return framing events. EndOfSnapshot and
// NewSnapshotToFollow are not exported, but we can get them from a subscription.
func newEventFromSubscription(t *testing.T, index uint64) stream.Event {
	t.Helper()

	serviceHealthConnectHandler := func(stream.SubscribeRequest, stream.SnapshotAppender) (index uint64, err error) {
		return 1, nil
	}

	ep := stream.NewEventPublisher(0)
	ep.RegisterHandler(pbsubscribe.Topic_ServiceHealthConnect, serviceHealthConnectHandler, false)
	req := &stream.SubscribeRequest{Topic: pbsubscribe.Topic_ServiceHealthConnect, Subject: stream.SubjectNone, Index: index}

	sub, err := ep.Subscribe(req)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	event, err := sub.Next(ctx)
	require.NoError(t, err)
	return event
}
