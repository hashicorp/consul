// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package submatview_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/consul/agent/rpcclient"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-internal/services/subscribe"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

func TestStore_IntegrationWithBackend(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	var maxIndex uint64 = 200
	count := &counter{latest: 3}
	producers := map[string]*eventProducer{
		state.EventSubjectService{Key: "srv1"}.String(): newEventProducer(pbsubscribe.Topic_ServiceHealth, "srv1", count, maxIndex),
		state.EventSubjectService{Key: "srv2"}.String(): newEventProducer(pbsubscribe.Topic_ServiceHealth, "srv2", count, maxIndex),
		state.EventSubjectService{Key: "srv3"}.String(): newEventProducer(pbsubscribe.Topic_ServiceHealth, "srv3", count, maxIndex),
	}

	sh := snapshotHandler{producers: producers}
	pub := stream.NewEventPublisher(10 * time.Millisecond)
	pub.RegisterHandler(pbsubscribe.Topic_ServiceHealth, sh.Snapshot, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pub.Run(ctx)

	store := submatview.NewStore(hclog.New(nil))
	go store.Run(ctx)

	addr := runServer(t, pub)

	consumers := []*consumer{
		newConsumer(t, addr, store, "srv1"),
		newConsumer(t, addr, store, "srv1"),
		newConsumer(t, addr, store, "srv1"),
		newConsumer(t, addr, store, "srv2"),
		newConsumer(t, addr, store, "srv2"),
		newConsumer(t, addr, store, "srv2"),
	}

	group, gctx := errgroup.WithContext(ctx)
	for i := range producers {
		producer := producers[i]
		group.Go(func() error {
			producer.Produce(gctx, pub)
			return nil
		})
	}

	for i := range consumers {
		consumer := consumers[i]
		group.Go(func() error {
			return consumer.Consume(gctx, maxIndex)
		})
	}

	_ = group.Wait()

	for i, consumer := range consumers {
		t.Run(fmt.Sprintf("consumer %d", i), func(t *testing.T) {
			require.True(t, len(consumer.states) > 2, "expected more than %d events", len(consumer.states))

			expected := producers[state.EventSubjectService{Key: consumer.srvName}.String()].nodesByIndex
			for idx, nodes := range consumer.states {
				assertDeepEqual(t, idx, expected[idx], nodes)
			}
		})
	}
}

func assertDeepEqual(t *testing.T, idx uint64, x, y interface{}) {
	t.Helper()
	if diff := cmp.Diff(x, y, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("assertion failed: values at index %d are not equal\n--- expected\n+++ actual\n%v", idx, diff)
	}
}

func stateFromUpdates(u cache.UpdateEvent) []string {
	var result []string
	for _, node := range u.Result.(*structs.IndexedCheckServiceNodes).Nodes {
		result = append(result, node.Node.Node)
	}

	sort.Strings(result)
	return result
}

func runServer(t *testing.T, pub *stream.EventPublisher) net.Addr {
	subSrv := &subscribe.Server{
		Backend: backend{pub: pub},
		Logger:  hclog.New(nil),
	}
	srv := grpc.NewServer()
	pbsubscribe.RegisterStateChangeSubscriptionServer(srv, subSrv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var g errgroup.Group
	g.Go(func() error {
		return srv.Serve(lis)
	})
	t.Cleanup(func() {
		srv.Stop()
		if err := g.Wait(); err != nil {
			t.Log(err.Error())
		}
	})

	return lis.Addr()
}

type backend struct {
	pub *stream.EventPublisher
}

func (b backend) ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (acl.Authorizer, error) {
	return acl.AllowAll(), nil
}

func (b backend) Forward(structs.RPCInfo, func(*grpc.ClientConn) error) (handled bool, err error) {
	return false, nil
}

func (b backend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return b.pub.Subscribe(req)
}

var _ subscribe.Backend = (*backend)(nil)

type eventProducer struct {
	rand         *rand.Rand
	counter      *counter
	topic        stream.Topic
	srvName      string
	nodesByIndex map[uint64][]string
	nodesLock    sync.Mutex
	maxIndex     uint64
}

func newEventProducer(
	topic stream.Topic,
	srvName string,
	counter *counter,
	maxIndex uint64,
) *eventProducer {
	return &eventProducer{
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		counter:      counter,
		nodesByIndex: map[uint64][]string{},
		topic:        topic,
		srvName:      srvName,
		maxIndex:     maxIndex,
	}
}

var minEventDelay = 5 * time.Millisecond

func (e *eventProducer) Produce(ctx context.Context, pub *stream.EventPublisher) {
	var nodes []string
	var nextID int

	for ctx.Err() == nil {
		var event stream.Event

		action := e.rand.Intn(3)
		if len(nodes) == 0 {
			action = 1
		}

		idx := e.counter.Next()
		switch action {

		case 0: // Deregister
			nodeIdx := e.rand.Intn(len(nodes))
			node := nodes[nodeIdx]
			nodes = append(nodes[:nodeIdx], nodes[nodeIdx+1:]...)

			event = stream.Event{
				Topic: e.topic,
				Index: idx,
				Payload: state.EventPayloadCheckServiceNode{
					Op: pbsubscribe.CatalogOp_Deregister,
					Value: &structs.CheckServiceNode{
						Node: &structs.Node{Node: node},
						Service: &structs.NodeService{
							ID:      e.srvName,
							Service: e.srvName,
						},
					},
				},
			}

		case 1: // Register new
			node := nodeName(nextID)
			nodes = append(nodes, node)
			nextID++

			event = stream.Event{
				Topic: e.topic,
				Index: idx,
				Payload: state.EventPayloadCheckServiceNode{
					Op: pbsubscribe.CatalogOp_Register,
					Value: &structs.CheckServiceNode{
						Node: &structs.Node{Node: node},
						Service: &structs.NodeService{
							ID:      e.srvName,
							Service: e.srvName,
						},
					},
				},
			}

		case 2: // Register update
			node := nodes[e.rand.Intn(len(nodes))]
			event = stream.Event{
				Topic: e.topic,
				Index: idx,
				Payload: state.EventPayloadCheckServiceNode{
					Op: pbsubscribe.CatalogOp_Register,
					Value: &structs.CheckServiceNode{
						Node: &structs.Node{Node: node},
						Service: &structs.NodeService{
							ID:      e.srvName,
							Service: e.srvName,
						},
					},
				},
			}
		}

		e.nodesLock.Lock()
		pub.Publish([]stream.Event{event})
		e.nodesByIndex[idx] = copyNodeList(nodes)
		e.nodesLock.Unlock()

		if idx > e.maxIndex {
			return
		}

		delay := time.Duration(rand.Intn(25)) * time.Millisecond
		time.Sleep(minEventDelay + delay)
	}
}

func nodeName(i int) string {
	return fmt.Sprintf("node-%d", i)
}

func copyNodeList(nodes []string) []string {
	result := make([]string, len(nodes))
	copy(result, nodes)
	sort.Strings(result)
	return result
}

type counter struct {
	latest uint64
}

func (c *counter) Next() uint64 {
	return atomic.AddUint64(&c.latest, 1)
}

type consumer struct {
	healthClient *health.Client
	states       map[uint64][]string
	srvName      string
}

func newConsumer(t *testing.T, addr net.Addr, store *submatview.Store, srv string) *consumer {
	//nolint:staticcheck
	conn, err := grpc.Dial(addr.String(), grpc.WithInsecure())
	require.NoError(t, err)

	c := &health.Client{
		Client: rpcclient.Client{
			UseStreamingBackend: true,
			ViewStore:           store,
			MaterializerDeps: rpcclient.MaterializerDeps{
				Conn:   conn,
				Logger: hclog.New(nil),
			},
		},
	}

	return &consumer{
		healthClient: c,
		states:       make(map[uint64][]string),
		srvName:      srv,
	}
}

func (c *consumer) Consume(ctx context.Context, maxIndex uint64) error {
	req := structs.ServiceSpecificRequest{ServiceName: c.srvName}
	updateCh := make(chan cache.UpdateEvent, 10)

	group, cctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return c.healthClient.Notify(cctx, req, "", func(ctx context.Context, event cache.UpdateEvent) {
			select {
			case updateCh <- event:
			case <-ctx.Done():
			}
		})
	})
	group.Go(func() error {
		var idx uint64
		for {
			if idx >= maxIndex {
				return nil
			}
			select {
			case u := <-updateCh:
				idx = u.Meta.Index
				c.states[u.Meta.Index] = stateFromUpdates(u)
			case <-cctx.Done():
				return nil
			}
		}
	})
	return group.Wait()
}

type snapshotHandler struct {
	producers map[string]*eventProducer
}

func (s *snapshotHandler) Snapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (index uint64, err error) {
	producer := s.producers[req.Subject.String()]

	producer.nodesLock.Lock()
	defer producer.nodesLock.Unlock()
	idx := atomic.LoadUint64(&producer.counter.latest)

	// look backwards for an index that was used by the producer
	nodes, ok := producer.nodesByIndex[idx]
	for !ok && idx > 0 {
		idx--
		nodes, ok = producer.nodesByIndex[idx]
	}

	for _, node := range nodes {
		event := stream.Event{
			Topic: producer.topic,
			Index: idx,
			Payload: state.EventPayloadCheckServiceNode{
				Op: pbsubscribe.CatalogOp_Register,
				Value: &structs.CheckServiceNode{
					Node: &structs.Node{Node: node},
					Service: &structs.NodeService{
						ID:      producer.srvName,
						Service: producer.srvName,
					},
				},
			},
		}
		buf.Append([]stream.Event{event})
	}
	return idx, nil
}
