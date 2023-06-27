// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peerstream

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// testInitialIndex is the first index that will be used in simulated updates.
//
// This is set to something arbitrarily high so that we can ignore the initial
// snapshot which may or may not be empty depending on timing.
const testInitialIndex uint64 = 9000

// TestExportedServiceSubscription tests the exported services view and the backing submatview.LocalMaterializer.
func TestExportedServiceSubscription(t *testing.T) {
	s := &stateMap{
		states: make(map[string]*serviceState),
	}

	sh := snapshotHandler{stateMap: s}
	pub := stream.NewEventPublisher(10 * time.Millisecond)
	pub.RegisterHandler(pbsubscribe.Topic_ServiceHealth, sh.Snapshot, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pub.Run(ctx)

	apiSN := structs.NewServiceName("api", nil)
	webSN := structs.NewServiceName("web", nil)

	newRegisterHealthEvent := func(id, service string) stream.Event {
		return stream.Event{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Payload: state.EventPayloadCheckServiceNode{
				Op: pbsubscribe.CatalogOp_Register,
				Value: &structs.CheckServiceNode{
					Node: &structs.Node{},
					Service: &structs.NodeService{
						ID:      id,
						Service: service,
					},
				},
			},
		}
	}

	// List of updates to the state store:
	// - api: {register api-1, register api-2, register api-3}
	// - web: {register web-1, deregister web-1, register web-2}1
	events := []map[string]stream.Event{
		{
			apiSN.String(): newRegisterHealthEvent("api-1", "api"),
			webSN.String(): newRegisterHealthEvent("web-1", "web"),
		},
		{
			apiSN.String(): newRegisterHealthEvent("api-2", "api"),
			webSN.String(): newRegisterHealthEvent("web-1", "web"),
		},
		{
			apiSN.String(): newRegisterHealthEvent("api-3", "api"),
			webSN.String(): newRegisterHealthEvent("web-2", "web"),
		},
	}

	// store represents Consul's memdb state store.
	// A stream of event updates
	store := store{stateMap: s, pub: pub}

	// This errgroup is used to issue simulate async updates to the state store,
	// and also consume that fixed number of updates.
	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		store.simulateUpdates(gctx, events)
		return nil
	})

	// viewStore is the store shared by the two service consumer's materializers.
	// It is intentionally not run in the errgroup because it will block until the context is canceled.
	viewStore := submatview.NewStore(hclog.New(nil))
	go viewStore.Run(ctx)

	// Each consumer represents a subscriber to exported service updates, and will consume
	// stream events for the service name it is interested in.
	consumers := make(map[string]*consumer)
	for _, svc := range []structs.ServiceName{apiSN, webSN} {
		c := &consumer{
			viewStore:   viewStore,
			publisher:   pub,
			seenByIndex: make(map[uint64][]string),
		}
		service := svc
		group.Go(func() error {
			return c.consume(gctx, service.Name, len(events))
		})
		consumers[service.String()] = c
	}

	// Wait until all the events have been simulated and consumed.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = group.Wait()
	}()

	select {
	case <-done:
		// finished
	case <-time.After(500 * time.Millisecond):
		// timed out, the Wait context will be cancelled by
		t.Fatalf("timed out waiting for producers and consumers")
	}

	for svc, c := range consumers {
		require.NotEmpty(t, c.seenByIndex)

		// Note that store.states[svc].idsByIndex does not assert against a slice of expectations because
		// the index that the different events will arrive in the simulation is not deterministic.
		require.Equal(t, store.states[svc].idsByIndex, c.seenByIndex)
	}
}

// stateMap is a map keyed by service to the state of the store at different indexes
type stateMap struct {
	mu     sync.Mutex
	states map[string]*serviceState
}

type store struct {
	*stateMap

	pub *stream.EventPublisher
}

// simulateUpdates will publish events and also store the state at each index for later assertions.
func (s *store) simulateUpdates(ctx context.Context, events []map[string]stream.Event) {
	idx := testInitialIndex

	for _, m := range events {
		if ctx.Err() != nil {
			return
		}

		for svc, event := range m {
			idx++
			event.Index = idx
			s.pub.Publish([]stream.Event{event})

			s.stateMap.mu.Lock()
			svcState, ok := s.states[svc]
			if !ok {
				svcState = &serviceState{
					current:    make(map[string]*structs.CheckServiceNode),
					idsByIndex: make(map[uint64][]string),
				}
				s.states[svc] = svcState
			}
			s.stateMap.mu.Unlock()

			svcState.mu.Lock()
			svcState.idx = idx

			// Updating the svcState.current map allows us to capture snapshots from a stream of add/delete events.
			payload := event.Payload.(state.EventPayloadCheckServiceNode)
			switch payload.Op {
			case pbsubscribe.CatalogOp_Register:
				svcState.current[payload.Value.Service.ID] = payload.Value
			case pbsubscribe.CatalogOp_Deregister:
				delete(svcState.current, payload.Value.Service.ID)
			default:
				panic(fmt.Sprintf("unable to handle op type %v", payload.Op))
			}

			svcState.idsByIndex[idx] = serviceIDsFromMap(svcState.current)
			svcState.mu.Unlock()

			delay := time.Duration(rand.Intn(25)) * time.Millisecond
			time.Sleep(5*time.Millisecond + delay)
		}
	}
}

func serviceIDsFromMap(m map[string]*structs.CheckServiceNode) []string {
	var result []string
	for id := range m {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

type snapshotHandler struct {
	*stateMap
}

type serviceState struct {
	mu  sync.Mutex
	idx uint64

	// The current snapshot of data, given the observed events.
	current map[string]*structs.CheckServiceNode

	// The list of service IDs seen at each index that an update was received for the given service name.
	idsByIndex map[uint64][]string
}

// Snapshot dumps the currently registered service instances.
//
// Snapshot implements stream.SnapshotFunc.
func (s *snapshotHandler) Snapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (index uint64, err error) {
	s.stateMap.mu.Lock()
	svcState, ok := s.states[req.Subject.String()]
	if !ok {
		svcState = &serviceState{
			current:    make(map[string]*structs.CheckServiceNode),
			idsByIndex: make(map[uint64][]string),
		}
		s.states[req.Subject.String()] = svcState
	}
	s.stateMap.mu.Unlock()

	svcState.mu.Lock()
	defer svcState.mu.Unlock()

	for _, node := range svcState.current {
		event := stream.Event{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Index: svcState.idx,
			Payload: state.EventPayloadCheckServiceNode{
				Op:    pbsubscribe.CatalogOp_Register,
				Value: node,
			},
		}
		buf.Append([]stream.Event{event})
	}
	return svcState.idx, nil
}

type consumer struct {
	viewStore   *submatview.Store
	publisher   *stream.EventPublisher
	seenByIndex map[uint64][]string
}

func (c *consumer) consume(ctx context.Context, service string, countExpected int) error {
	group, gctx := errgroup.WithContext(ctx)
	updateCh := make(chan cache.UpdateEvent, 10)

	group.Go(func() error {
		sr := newExportedStandardServiceRequest(
			hclog.New(nil),
			structs.NewServiceName(service, nil),
			c.publisher,
		)
		return c.viewStore.Notify(gctx, sr, "", updateCh)
	})
	group.Go(func() error {
		var n int
		for {
			if n >= countExpected {
				return nil
			}
			select {
			case u := <-updateCh:
				idx := u.Meta.Index

				// This is the initial/empty state. Skip over it and wait for the first
				// real event.
				if idx < testInitialIndex {
					continue
				}

				// Each update contains the current snapshot of registered services.
				c.seenByIndex[idx] = serviceIDsFromUpdates(u)
				n++

			case <-gctx.Done():
				return nil
			}
		}
	})
	return group.Wait()
}

func serviceIDsFromUpdates(u cache.UpdateEvent) []string {
	var result []string
	for _, node := range u.Result.(*pbservice.IndexedCheckServiceNodes).Nodes {
		result = append(result, node.Service.ID)
	}
	sort.Strings(result)
	return result
}
