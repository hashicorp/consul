package consul

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	gogrpc "google.golang.org/grpc"

	grpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/testrpc"
)

func TestSubscribeBackend_IntegrationWithServer_TLSEnabled(t *testing.T) {
	t.Parallel()

	// TODO(rb): add tests for the wanfed/alpn variations

	_, conf1 := testServerConfig(t)
	conf1.TLSConfig.InternalRPC.VerifyIncoming = true
	conf1.TLSConfig.InternalRPC.VerifyOutgoing = true
	conf1.RPCConfig.EnableStreaming = true
	configureTLS(conf1)
	server, err := newServer(t, conf1)
	require.NoError(t, err)
	defer server.Shutdown()

	client, builder := newClientWithGRPCResolver(t, configureTLS, clientConfigVerifyOutgoing)

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
		require.NoError(t, server.RPC("Catalog.Register", &req, &out))
	}

	// Start a Subscribe call to our streaming endpoint from the client.
	{
		pool := grpc.NewClientConnPool(grpc.ClientConnPoolConfig{
			Servers:               builder,
			TLSWrapper:            grpc.TLSWrapper(client.tlsConfigurator.OutgoingRPCWrapper()),
			UseTLSForDC:           client.tlsConfigurator.UseTLS,
			DialingFromServer:     true,
			DialingFromDatacenter: "dc1",
		})
		conn, err := pool.ClientConn("dc1")
		require.NoError(t, err)

		streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req := &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key: "redis",
				},
			},
		}
		streamHandle, err := streamClient.Subscribe(ctx, req)
		require.NoError(t, err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go receiveSubscribeEvents(t, eventCh, streamHandle)

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
		require.Len(t, snapshotEvents, 2)
	}

	// Start a Subscribe call to our streaming endpoint from the server's loopback client.
	{
		pool := grpc.NewClientConnPool(grpc.ClientConnPoolConfig{
			Servers:               builder,
			TLSWrapper:            grpc.TLSWrapper(client.tlsConfigurator.OutgoingRPCWrapper()),
			UseTLSForDC:           client.tlsConfigurator.UseTLS,
			DialingFromServer:     true,
			DialingFromDatacenter: "dc1",
		})
		conn, err := pool.ClientConn("dc1")
		require.NoError(t, err)

		retryFailedConn(t, conn)

		streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req := &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key: "redis",
				},
			},
		}
		streamHandle, err := streamClient.Subscribe(ctx, req)
		require.NoError(t, err)

		// Start a goroutine to read updates off the pbsubscribe.
		eventCh := make(chan *pbsubscribe.Event, 0)
		go receiveSubscribeEvents(t, eventCh, streamHandle)

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
		require.Len(t, snapshotEvents, 2)
	}
}

// receiveSubscribeEvents and send them to the channel.
func receiveSubscribeEvents(t *testing.T, ch chan *pbsubscribe.Event, handle pbsubscribe.StateChangeSubscription_SubscribeClient) {
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

func TestSubscribeBackend_IntegrationWithServer_TLSReload(t *testing.T) {
	t.Parallel()

	// Set up a server with initially bad certificates.
	_, conf1 := testServerConfig(t)
	conf1.TLSConfig.InternalRPC.VerifyIncoming = true
	conf1.TLSConfig.InternalRPC.VerifyOutgoing = true
	conf1.TLSConfig.InternalRPC.CAFile = "../../test/ca/root.cer"
	conf1.TLSConfig.InternalRPC.CertFile = "../../test/key/ssl-cert-snakeoil.pem"
	conf1.TLSConfig.InternalRPC.KeyFile = "../../test/key/ssl-cert-snakeoil.key"
	conf1.RPCConfig.EnableStreaming = true

	server, err := newServer(t, conf1)
	require.NoError(t, err)
	defer server.Shutdown()

	// Set up a client with valid certs and verify_outgoing = true
	client, builder := newClientWithGRPCResolver(t, configureTLS, clientConfigVerifyOutgoing)

	testrpc.WaitForLeader(t, server.RPC, "dc1")

	// Subscribe calls should fail initially
	joinLAN(t, client, server)

	pool := grpc.NewClientConnPool(grpc.ClientConnPoolConfig{
		Servers:               builder,
		TLSWrapper:            grpc.TLSWrapper(client.tlsConfigurator.OutgoingRPCWrapper()),
		UseTLSForDC:           client.tlsConfigurator.UseTLS,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})
	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)

	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req := &pbsubscribe.SubscribeRequest{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
			NamedSubject: &pbsubscribe.NamedSubject{
				Key: "redis",
			},
		},
	}
	_, err = streamClient.Subscribe(ctx, req)
	require.Error(t, err)

	// Reload the server with valid certs
	newConf := server.config.TLSConfig
	newConf.InternalRPC.CertFile = "../../test/key/ourdomain.cer"
	newConf.InternalRPC.KeyFile = "../../test/key/ourdomain.key"
	server.tlsConfigurator.Update(newConf)

	// Try the subscribe call again
	retryFailedConn(t, conn)

	streamClient = pbsubscribe.NewStateChangeSubscriptionClient(conn)
	_, err = streamClient.Subscribe(ctx, req)
	require.NoError(t, err)
}

func clientConfigVerifyOutgoing(config *Config) {
	config.TLSConfig.InternalRPC.VerifyOutgoing = true
}

// retryFailedConn forces the ClientConn to reset its backoff timer and retry the connection,
// to simulate the client eventually retrying after the initial failure. This is used both to simulate
// retrying after an expected failure as well as to avoid flakiness when running many tests in parallel.
func retryFailedConn(t *testing.T, conn *gogrpc.ClientConn) {
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

func TestSubscribeBackend_IntegrationWithServer_DeliversAllMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for -short run")
	}
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

	_, server := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.RPCConfig.EnableStreaming = true
	})
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	client, builder := newClientWithGRPCResolver(t)

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
		require.NoError(t, server.RPC("Catalog.Register", &req, &out))
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
			require.NoError(t, server.RPC("Catalog.Register", &req, &out))
			req.Service.Port++
			if req.Service.Port > 100 {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	pool := grpc.NewClientConnPool(grpc.ClientConnPoolConfig{
		Servers:               builder,
		TLSWrapper:            grpc.TLSWrapper(client.tlsConfigurator.OutgoingRPCWrapper()),
		UseTLSForDC:           client.tlsConfigurator.UseTLS,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})
	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)

	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)

	// Now start a whole bunch of streamers in parallel to maximise chance of
	// catching a race.
	n := 5
	var g errgroup.Group
	var updateCount uint64
	for i := 0; i < n; i++ {
		i := i
		g.Go(func() error {
			return verifyMonotonicStreamUpdates(ctx, t, streamClient, i, &updateCount)
		})
	}

	// Wait until all subscribers have verified the first bunch of updates all got
	// delivered.
	require.NoError(t, g.Wait())

	// Sanity check that at least some non-snapshot messages were delivered. We
	// can't know exactly how many because it's timing dependent based on when
	// each subscribers snapshot occurs.
	require.True(t, atomic.LoadUint64(&updateCount) > 0,
		"at least some of the subscribers should have received non-snapshot updates")
}

func newClientWithGRPCResolver(t *testing.T, ops ...func(*Config)) (*Client, *resolver.ServerResolverBuilder) {
	_, config := testClientConfig(t)
	for _, op := range ops {
		op(config)
	}

	builder := resolver.NewServerResolverBuilder(newTestResolverConfig(t,
		"client."+config.Datacenter+"."+string(config.NodeID),
		config.Datacenter,
		"client",
	))

	resolver.Register(builder)
	t.Cleanup(func() {
		resolver.Deregister(builder.Authority())
	})

	deps := newDefaultDeps(t, config)
	deps.Router = router.NewRouter(
		deps.Logger,
		config.Datacenter,
		fmt.Sprintf("%s.%s", config.NodeName, config.Datacenter),
		builder)

	client, err := NewClient(config, deps)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Shutdown()
	})
	return client, builder
}

type testLogger interface {
	Logf(format string, args ...interface{})
}

func verifyMonotonicStreamUpdates(ctx context.Context, logger testLogger, client pbsubscribe.StateChangeSubscriptionClient, i int, updateCount *uint64) error {
	req := &pbsubscribe.SubscribeRequest{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
			NamedSubject: &pbsubscribe.NamedSubject{
				Key: "redis",
			},
		},
	}
	streamHandle, err := client.Subscribe(ctx, req)
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		logger.Logf("subscriber %05d: context cancelled before loop")
		return nil
	case err != nil:
		return err
	}

	snapshotDone := false
	expectPort := int32(0)
	for {
		event, err := streamHandle.Recv()
		switch {
		case err == io.EOF:
			return nil
		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			return nil
		case err != nil:
			return err
		}

		switch {
		case event.GetEndOfSnapshot():
			snapshotDone = true
			logger.Logf("subscriber %05d: snapshot done at index %d, expect next port to be %d", i, event.Index, expectPort)
		case snapshotDone:
			// Verify we get all updates in order
			svc, err := svcOrErr(event)
			if err != nil {
				return err
			}
			switch svc.Port {
			case expectPort:
				atomic.AddUint64(updateCount, 1)
				logger.Logf("subscriber %05d: got event with correct port=%d at index %d", i, expectPort, event.Index)
				expectPort++
			case expectPort - 1:
				logger.Logf("subscriber %05d: got event with repeated prior port=%d at index %d", i, expectPort-1, event.Index)
			default:
				return fmt.Errorf("subscriber %05d: at index %d: expected port %d, got %d",
					i, event.Index, expectPort, svc.Port)
			}
		default:
			// snapshot events
			svc, err := svcOrErr(event)
			if err != nil {
				return err
			}
			if svc.ID == "redis-canary" {
				// Update the expected port we see in the next update to be one more
				// than the port in the snapshot.
				expectPort = svc.Port + 1
				logger.Logf("subscriber %05d: saw canary in snapshot with port %d at index %d", i, svc.Port, event.Index)
			}
		}
		if expectPort > 100 {
			return nil
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
