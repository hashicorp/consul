package cachetype

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/consul/stream"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// Recommended name for registration.
	StreamingHealthServicesName = "streaming-health-services"

	StreamTimeout = 10 * time.Minute
)

// StreamingHealthServices supports fetching discovering service instances via the
// catalog using the streaming gRPC endpoint.
type StreamingHealthServices struct {
	Client stream.ConsulClient
}

func (c *StreamingHealthServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a ServiceSpecificRequest.
	reqReal, ok := req.(*structs.ServiceSpecificRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Start a new subscription if one isn't already running.
	var sub *subscriber
	if opts.LastResult == nil || opts.LastResult.State == nil {
		sub = &subscriber{
			client:   c.Client,
			service:  reqReal.ServiceName,
			resultCh: make(chan interface{}, 0),
		}
		go sub.run()
	} else {
		sub = opts.LastResult.State.(*subscriber)
	}

	result.State = sub

	// If the requested index is lower than what we've already seen, return immediately.
	sub.lock.RLock()
	if sub.lastResult.Index > opts.MinIndex {
		result.Value = &sub.lastResult
		result.Index = sub.lastResult.Index
		sub.lock.RUnlock()
		return result, nil
	}
	sub.lock.RUnlock()

	// Wait for an update based on the index we're blocking on.
	timeout := time.After(StreamTimeout)
WAIT:
	select {
	case r := <-sub.resultCh:
		switch reply := r.(type) {
		case *structs.IndexedCheckServiceNodes:
			if reply.Index <= opts.MinIndex {
				goto WAIT
			}
			result.Value = reply
			result.Index = reply.Index
		case error:
			result.Value = reply
		}
	case <-timeout:
		sub.lock.RLock()
		result.Value = &sub.lastResult
		result.Index = sub.lastResult.Index
		sub.lock.RUnlock()
	}

	return result, nil
}

func (c *StreamingHealthServices) SupportsBlocking() bool {
	return true
}

// subscriber runs a subscription for a single topic/key combination.
type subscriber struct {
	client     stream.ConsulClient
	service    string
	lastResult structs.IndexedCheckServiceNodes
	lastUpdate time.Time
	resultCh   chan interface{}

	cancelFunc func()

	lock sync.RWMutex
}

// Close stops the subscriber by cancelling the context.
func (s *subscriber) Close() error {
	s.cancelFunc()
	return nil
}

// run starts a Subscribe call for the subscriber's key, receives the initial
// snapshot of state and then sends the state to the watcher as soon as its
// watched index requirement is met.
func (s *subscriber) run() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	defer s.cancelFunc()

	// Start a new Subscribe call.
	streamHandle, err := s.client.Subscribe(ctx, &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   s.service,
	})

	// If something went wrong setting up the stream, return the error.
	if err != nil {
		// Do a blocking send since there should always be a Fetch goroutine watching
		// on the startup of a subscriber.
		s.resultCh <- err
		return
	}

	// Run the main loop to receive events.
	var snapshotDone bool
	index := uint64(1)
	state := make(map[string]structs.CheckServiceNode)
	for {
		event, err := streamHandle.Recv()
		if err != nil {
			// Do a non-blocking send of the error, in case there's no Fetch watching.
			select {
			case s.resultCh <- err:
			default:
			}
			return
		}

		// Update our version of the state based on the event/op.
		switch event.Topic {
		case stream.Topic_ServiceHealth:
			serviceHealth := event.GetServiceHealth()
			node := serviceHealth.ServiceNode
			id := fmt.Sprintf("%s/%s", node.Node.Node, node.Service.ID)
			if event.Index > index {
				index = event.Index
			}

			switch serviceHealth.Op {
			case stream.CatalogOp_Register:
				checkServiceNode := stream.FromCheckServiceNode(serviceHealth.ServiceNode)
				state[id] = checkServiceNode
			case stream.CatalogOp_Deregister:
				delete(state, id)
			}
		case stream.Topic_EndOfSnapshot:
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
		s.lock.Lock()
		s.lastResult = result
		s.lock.Unlock()

		// Send the new view of the state to the watcher.
		select {
		case s.resultCh <- &result:
			s.lastUpdate = time.Now()
		default:
		}
	}
}
