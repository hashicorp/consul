package cachetype

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/types"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// Recommended name for registration.
	StreamingHealthServicesName = "streaming-health-services"

	StreamTimeout = 10 * time.Minute
)

var ErrSnapshotTimeout = errors.New("timed out getting service health results")

// StreamingHealthServices supports fetching discovering service instances via the
// catalog using the streaming gRPC endpoint.
type StreamingHealthServices struct {
	client stream.ConsulClient

	subscriptions map[string]*subscriber

	lock sync.RWMutex
}

func NewStreamingHealthServices(client stream.ConsulClient) *StreamingHealthServices {
	return &StreamingHealthServices{
		client:        client,
		subscriptions: make(map[string]*subscriber),
	}
}

type streamingWatcher struct {
	index    uint64
	updateCh chan interface{}
}

func (c *StreamingHealthServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a ServiceSpecificRequest.
	reqReal, ok := req.(*structs.ServiceSpecificRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	service := reqReal.ServiceName
	watcher := &streamingWatcher{
		index:    opts.MinIndex,
		updateCh: make(chan interface{}, 1),
	}

	// Start a subscriber for this key if one isn't already running.
	c.lock.Lock()
	key := req.CacheInfo().Key
	subscription, ok := c.subscriptions[key]
	if !ok {
		subscription = &subscriber{
			client:  c.client,
			service: service,
			key:     key,
			watcher: watcher,
			cleanupFunc: func() {
				c.lock.Lock()
				delete(c.subscriptions, key)
				c.lock.Unlock()
			},
		}
		c.subscriptions[key] = subscription
		go subscription.run()
	} else {
		// Update the watcher if there is an existing subscription.
		subscription.watcher = watcher
	}
	c.lock.Unlock()

	// Set up a watcher if the requested index hasn't been reached yet,
	// or if there hasn't been an initial state returned.
	subscription.lock.Lock()
	idx := subscription.lastResult.Index
	blocking := opts.MinIndex >= idx || idx == 0
	if blocking {
		subscription.watcher = watcher
	}
	subscription.lock.Unlock()

	// If we don't need to block on a future index, and there's already a
	// stored result, just return that.
	if !blocking {
		subscription.lock.RLock()
		result.Value = &subscription.lastResult
		result.Index = subscription.lastResult.Index
		subscription.lock.RUnlock()

		return result, nil
	}

	// Wait for an update based on the index we're blocking on.
	select {
	case r := <-watcher.updateCh:
		switch reply := r.(type) {
		case *structs.IndexedCheckServiceNodes:
			result.Value = reply
			result.Index = reply.Index
		case error:
			result.Value = reply
		}
	case <-time.After(StreamTimeout):
		c.lock.RLock()
		result.Value = &subscription.lastResult
		result.Index = subscription.lastResult.Index
		c.lock.RUnlock()
	}

	return result, nil
}

func (c *StreamingHealthServices) SupportsBlocking() bool {
	return true
}

// subscriber runs a subscription for a single topic/key combination.
type subscriber struct {
	client      stream.ConsulClient
	service     string
	key         string
	watcher     *streamingWatcher
	lastResult  structs.IndexedCheckServiceNodes
	lastUpdate  time.Time
	cleanupFunc func()

	lock sync.RWMutex
}

// run starts a Subscribe call for the subscriber's key, receives the initial
// snapshot of state and then sends the state to the watcher as soon as its
// watched index requirement is met.
func (s *subscriber) run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer s.cleanupFunc()

	// Start a new Subscribe call.
	streamHandle, err := s.client.Subscribe(ctx, &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   s.service,
	})

	// If something went wrong setting up the stream, return the error.
	sendErr := func(err error) {
		s.lock.Lock()
		if s.watcher != nil {
			s.watcher.updateCh <- err
		}
		s.watcher = nil
		s.lock.Unlock()
	}
	if err != nil {
		sendErr(err)
		return
	}

	// Set up a separate goroutine to shut down if the value has been
	// un-fetched for long enough. Ideally there would be a way to extend
	// the deadline of the context instead, but that isn't possible currently.
	s.lastUpdate = time.Now()
	go func() {
		for {
			s.lock.RLock()
			deadline := s.lastUpdate.Add(StreamTimeout)
			s.lock.RUnlock()
			waitDuration := deadline.Sub(time.Now())
			select {
			case <-ctx.Done():
				// Context finished already, nothing to do.
				return
			case <-time.After(waitDuration):
				s.lock.RLock()
				expired := time.Now().After(s.lastUpdate.Add(StreamTimeout))
				s.lock.RUnlock()
				if expired {
					cancel()
					return
				}
			}
		}
	}()

	// Run the main loop to receive events.
	var snapshotDone bool
	index := uint64(1)
	state := make(map[string]structs.CheckServiceNode)
	for {
		event, err := streamHandle.Recv()
		if err != nil {
			sendErr(err)
			return
		}

		// If this isn't the special "end of snapshot" message, update our
		// version of the state based on the event/op.
		if !event.EndOfSnapshot {
			id := fmt.Sprintf("%s/%s", event.ServiceHealth.Node, event.ServiceHealth.Service)
			if event.Index > index {
				index = event.Index
			}

			switch event.ServiceHealth.Op {
			case stream.CatalogOp_Register:
				checkServiceNode := convertEventToCheckServiceNode(event)
				state[id] = checkServiceNode
			case stream.CatalogOp_Deregister:
				delete(state, id)
			}
		} else {
			snapshotDone = true
		}

		// Don't go any further if the snapshot is still being sent.
		if !snapshotDone {
			continue
		}

		// Construct the most recent view of the state.
		var result structs.IndexedCheckServiceNodes
		for _, node := range state {
			result.Nodes = append(result.Nodes, node)
		}
		result.Index = index

		// Send the new view of the state to the watcher.
		s.lock.Lock()
		s.lastResult = result
		if s.watcher != nil && index > s.watcher.index {
			s.watcher.updateCh <- &result
			s.lastUpdate = time.Now()
		}
		s.lock.Unlock()
	}
}

// convertEventToCheckServiceNode converts the protobuf Event into a
// structs.CheckServiceNode to be returned through the API.
func convertEventToCheckServiceNode(event *stream.Event) structs.CheckServiceNode {
	n := event.ServiceHealth
	node := structs.CheckServiceNode{
		Node: &structs.Node{
			Node:    n.Node,
			ID:      types.NodeID(n.Id),
			Address: n.Address,
		},
	}
	if n.Service != "" {
		node.Service = &structs.NodeService{
			Service: n.Service,
			Port:    int(n.Port),
		}
	}
	for _, check := range n.Checks {
		node.Checks = append(node.Checks, &structs.HealthCheck{
			Name:        check.Name,
			Status:      check.Status,
			CheckID:     types.CheckID(check.CheckID),
			ServiceID:   check.ServiceID,
			ServiceName: check.ServiceName,
		})
	}

	return node
}
