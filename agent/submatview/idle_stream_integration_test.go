// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package submatview_test

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-internal/services/subscribe"
	"github.com/hashicorp/consul/agent/rpcclient"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// silentServer wraps the real subscribe server and, once armed for a token,
// swallows every event of the streams that token already has open. The stream
// stays up and no error is returned, like a server that stops publishing for a
// topic without telling the client. Streams opened afterwards are unaffected.
type silentServer struct {
	*subscribe.Server
	armed sync.Map // token -> *atomic.Int64 generation of streams to swallow
	opens sync.Map // token -> *atomic.Int64 number of streams opened
}

func (s *silentServer) counter(m *sync.Map, token string) *atomic.Int64 {
	v, _ := m.LoadOrStore(token, &atomic.Int64{})
	return v.(*atomic.Int64)
}

// silence swallows the events of all streams the token has open right now.
func (s *silentServer) silence(token string) {
	s.counter(&s.armed, token).Store(s.counter(&s.opens, token).Load())
}

func (s *silentServer) streamsOpened(token string) int64 {
	return s.counter(&s.opens, token).Load()
}

func (s *silentServer) Subscribe(req *pbsubscribe.SubscribeRequest, srv pbsubscribe.StateChangeSubscription_SubscribeServer) error {
	gen := s.counter(&s.opens, req.Token).Add(1)
	return s.Server.Subscribe(req, &swallowStream{
		StateChangeSubscription_SubscribeServer: srv,
		armed:                                   s.counter(&s.armed, req.Token),
		gen:                                     gen,
	})
}

type swallowStream struct {
	pbsubscribe.StateChangeSubscription_SubscribeServer
	armed *atomic.Int64
	gen   int64
}

func (s *swallowStream) Send(e *pbsubscribe.Event) error {
	if s.gen <= s.armed.Load() {
		return nil
	}
	return s.StateChangeSubscription_SubscribeServer.Send(e)
}

type nodeSet struct {
	lock  sync.Mutex
	index uint64
	nodes []string
}

// register commits the node and then publishes the event, outside the lock, as
// the FSM does after a commit. Snapshot handlers run under the publisher's
// lock, so holding this lock across Publish could deadlock against them.
func (n *nodeSet) register(pub *stream.EventPublisher, name string) {
	n.lock.Lock()
	n.index++
	idx := n.index
	n.nodes = append(n.nodes, name)
	n.lock.Unlock()
	pub.Publish([]stream.Event{n.event(name, idx)})
}

func (n *nodeSet) event(name string, idx uint64) stream.Event {
	return stream.Event{
		Topic: pbsubscribe.Topic_ServiceHealth,
		Index: idx,
		Payload: state.EventPayloadCheckServiceNode{
			Op: pbsubscribe.CatalogOp_Register,
			Value: &structs.CheckServiceNode{
				Node:    &structs.Node{Node: name},
				Service: &structs.NodeService{ID: "srv1", Service: "srv1"},
			},
		},
	}
}

func (n *nodeSet) snapshot(_ stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for _, name := range n.nodes {
		buf.Append([]stream.Event{n.event(name, n.index)})
	}
	return n.index, nil
}

type tokenWatcher struct {
	errors atomic.Int64
	latest atomic.Uint64
	nodes  atomic.Value // []string
}

func (w *tokenWatcher) nodeNames() []string {
	v, _ := w.nodes.Load().([]string)
	return v
}

// watch emulates a blocking-query loop for one ACL token.
func watch(ctx context.Context, c *health.Client, token string, w *tokenWatcher) error {
	var index uint64
	for ctx.Err() == nil {
		req := structs.ServiceSpecificRequest{
			ServiceName: "srv1",
			QueryOptions: structs.QueryOptions{
				UseCache:      true,
				Token:         token,
				MinQueryIndex: index,
				MaxQueryTime:  250 * time.Millisecond,
			},
		}
		result, meta, err := c.ServiceNodes(ctx, req)
		if err != nil {
			// A stream reset can wake blocked queries with an error; callers
			// simply retry, as an HTTP client would.
			if ctx.Err() != nil {
				return nil
			}
			w.errors.Add(1)
			time.Sleep(20 * time.Millisecond)
			continue
		}
		index = meta.Index
		names := make([]string, 0, len(result.Nodes))
		for _, n := range result.Nodes {
			names = append(names, n.Node.Node)
		}
		sort.Strings(names)
		// Publish the node list before its index: a reader that sees an index
		// must also see the nodes that belong to it.
		w.nodes.Store(names)
		w.latest.Store(meta.Index)
	}
	return nil
}

func startSilentableServer(t *testing.T, pub *stream.EventPublisher) (net.Addr, *silentServer) {
	t.Helper()
	srv := &silentServer{Server: &subscribe.Server{Backend: backend{pub: pub}, Logger: hclog.NewNullLogger()}}
	gs := grpc.NewServer()
	pbsubscribe.RegisterStateChangeSubscriptionServer(gs, srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var g errgroup.Group
	g.Go(func() error { return gs.Serve(lis) })
	t.Cleanup(func() {
		gs.Stop()
		_ = g.Wait()
	})
	return lis.Addr(), srv
}

func newHealthClient(t *testing.T, addr net.Addr, store *submatview.Store, idleTimeout time.Duration) *health.Client {
	t.Helper()
	//nolint:staticcheck
	conn, err := grpc.Dial(addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &health.Client{
		Client: rpcclient.Client{
			UseStreamingBackend: true,
			ViewStore:           store,
			QueryOptionDefaults: func(*structs.QueryOptions) {},
			MaterializerDeps: rpcclient.MaterializerDeps{
				Conn:        conn,
				Logger:      hclog.NewNullLogger(),
				IdleTimeout: idleTimeout,
			},
		},
	}
}

// One token's stream goes silent while another token keeps receiving updates:
// without the idle check the affected token's index stays frozen with no error,
// the symptom reported after an availability-zone failure.
func TestStore_SilentStreamStrandsOnlyThatTokenWhenDisabled(t *testing.T) {
	nodes := &nodeSet{}
	pub := stream.NewEventPublisher(10 * time.Millisecond)
	require.NoError(t, pub.RegisterHandler(pbsubscribe.Topic_ServiceHealth, nodes.snapshot, false))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pub.Run(ctx)

	store := submatview.NewStore(hclog.NewNullLogger())
	go store.Run(ctx)

	addr, srv := startSilentableServer(t, pub)
	client := newHealthClient(t, addr, store, 0)

	nodes.register(pub, "node-1")

	var stuck, healthy tokenWatcher
	var g errgroup.Group
	g.Go(func() error { return watch(ctx, client, "stuck-token", &stuck) })
	g.Go(func() error { return watch(ctx, client, "healthy-token", &healthy) })

	require.Eventually(t, func() bool {
		return stuck.latest.Load() == 1 && healthy.latest.Load() == 1
	}, 5*time.Second, 20*time.Millisecond)

	srv.silence("stuck-token")
	nodes.register(pub, "node-2")

	require.Eventually(t, func() bool { return healthy.latest.Load() == 2 }, 5*time.Second, 20*time.Millisecond,
		"a token whose stream still works sees the update")
	require.Equal(t, []string{"node-1", "node-2"}, healthy.nodeNames())

	time.Sleep(1500 * time.Millisecond)
	require.Equal(t, uint64(1), stuck.latest.Load(), "the silenced token never sees the update")
	require.Equal(t, []string{"node-1"}, stuck.nodeNames())
	require.Equal(t, int64(1), srv.streamsOpened("stuck-token"), "and never resubscribes")

	cancel()
	require.NoError(t, g.Wait())
}

// Topic refreshes (a snapshot restore on the server) force every subscriber to
// resubscribe while events keep flowing. Every watcher must still converge on
// the final state; this guards against a view stranded by that race.
func TestStore_ConvergesWhileTopicsAreRefreshed(t *testing.T) {
	nodes := &nodeSet{}
	pub := stream.NewEventPublisher(10 * time.Millisecond)
	require.NoError(t, pub.RegisterHandler(pbsubscribe.Topic_ServiceHealth, nodes.snapshot, false))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pub.Run(ctx)

	store := submatview.NewStore(hclog.NewNullLogger())
	go store.Run(ctx)

	addr, _ := startSilentableServer(t, pub)
	client := newHealthClient(t, addr, store, 0)

	nodes.register(pub, "node-0")
	watchers := map[string]*tokenWatcher{"t1": {}, "t2": {}, "t3": {}}
	var g errgroup.Group
	for token, w := range watchers {
		token, w := token, w
		g.Go(func() error { return watch(ctx, client, token, w) })
	}

	churnCtx, stopChurn := context.WithTimeout(ctx, 2*time.Second)
	defer stopChurn()
	var churn errgroup.Group
	churn.Go(func() error {
		for i := 1; churnCtx.Err() == nil; i++ {
			nodes.register(pub, fmt.Sprintf("node-%d", i))
			time.Sleep(3 * time.Millisecond)
		}
		return nil
	})
	churn.Go(func() error {
		for churnCtx.Err() == nil {
			pub.RefreshAllTopics()
			time.Sleep(40 * time.Millisecond)
		}
		return nil
	})
	require.NoError(t, churn.Wait())

	nodes.register(pub, "node-final")
	nodes.lock.Lock()
	finalIndex, total := nodes.index, len(nodes.nodes)
	nodes.lock.Unlock()

	for token, w := range watchers {
		w := w
		require.Eventually(t, func() bool {
			return w.latest.Load() == finalIndex && len(w.nodeNames()) == total
		}, 10*time.Second, 50*time.Millisecond, "token %s did not converge (index %d, want %d)", token, w.latest.Load(), finalIndex)
	}

	cancel()
	require.NoError(t, g.Wait())
}

// The same silent stream is detected and replaced when the idle check is on:
// the affected token catches up without operator intervention, and the token
// whose stream is healthy is never resubscribed.
func TestStore_IdleCheckRecoversSilentStream(t *testing.T) {
	nodes := &nodeSet{}
	pub := stream.NewEventPublisher(10 * time.Millisecond)
	require.NoError(t, pub.RegisterHandler(pbsubscribe.Topic_ServiceHealth, nodes.snapshot, false))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pub.Run(ctx)

	store := submatview.NewStore(hclog.NewNullLogger())
	go store.Run(ctx)

	addr, srv := startSilentableServer(t, pub)
	client := newHealthClient(t, addr, store, 400*time.Millisecond)

	nodes.register(pub, "node-1")

	var stuck, healthy tokenWatcher
	var g errgroup.Group
	g.Go(func() error { return watch(ctx, client, "stuck-token", &stuck) })
	g.Go(func() error { return watch(ctx, client, "healthy-token", &healthy) })

	require.Eventually(t, func() bool {
		return stuck.latest.Load() == 1 && healthy.latest.Load() == 1
	}, 5*time.Second, 20*time.Millisecond)

	srv.silence("stuck-token")
	nodes.register(pub, "node-2")

	require.Eventually(t, func() bool { return healthy.latest.Load() == 2 }, 5*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return stuck.latest.Load() == 2 }, 10*time.Second, 50*time.Millisecond,
		"the silenced token must recover on its own")
	require.Equal(t, []string{"node-1", "node-2"}, stuck.nodeNames())
	require.GreaterOrEqual(t, srv.streamsOpened("stuck-token"), int64(2), "it resubscribed")

	cancel()
	require.NoError(t, g.Wait())
}
