package cachetype

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"google.golang.org/grpc"
)

type StreamingClient interface {
	Subscribe(ctx context.Context, in *stream.SubscribeRequest, opts ...grpc.CallOption) (stream.Consul_SubscribeClient, error)
}

type EventHandler interface {
	HandleEvent(event *stream.Event)
	State(idx uint64) interface{}
}

// Subscriber runs a streaming subscription for a single topic/key combination.
type Subscriber struct {
	client  StreamingClient
	request stream.SubscribeRequest
	handler EventHandler

	lastResult interface{}
	lastUpdate time.Time
	resultCh   chan interface{}
	cancelFunc func()

	lock sync.RWMutex
}

func NewSubscriber(client StreamingClient, req stream.SubscribeRequest, handler EventHandler) *Subscriber {
	s := &Subscriber{
		client:   client,
		request:  req,
		handler:  handler,
		resultCh: make(chan interface{}, 0),
	}

	return s
}

// Close stops the subscriber by cancelling the context.
func (s *Subscriber) Close() error {
	s.cancelFunc()
	return nil
}

// run starts a Subscribe call for the subscriber's key, receives the initial
// snapshot of state and then sends the state to the watcher as soon as its
// watched index requirement is met.
func (s *Subscriber) run() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	defer s.cancelFunc()

	// Start a new Subscribe call.
	streamHandle, err := s.client.Subscribe(ctx, &s.request)

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
		case s.request.Topic:
			s.handler.HandleEvent(event)
		case stream.Topic_EndOfSnapshot:
			snapshotDone = true
		}

		if event.Index > index {
			index = event.Index
		}

		// Don't go any further if the snapshot is still being sent.
		if !snapshotDone {
			continue
		}

		// Construct the most recent view of the state.
		result := s.handler.State(index)
		s.lock.Lock()
		s.lastResult = result
		s.lock.Unlock()

		// Send the new view of the state to the watcher.
		select {
		case s.resultCh <- result:
			s.lastUpdate = time.Now()
		default:
		}
	}
}

type IndexFunc func(v interface{}) uint64

// watchSubscriber returns a result based on the given FetchOptions and SubscribeRequest,
// creating a new subscriber if necessary as well as blocking based on the given index.
func watchSubscriber(client StreamingClient, opts cache.FetchOptions, req stream.SubscribeRequest,
	handler EventHandler, getIndex IndexFunc) (cache.FetchResult, error) {
	var result cache.FetchResult

	// Start a new subscription if one isn't already running.
	var sub *Subscriber
	if opts.LastResult == nil || opts.LastResult.State == nil {
		sub = NewSubscriber(client, req, handler)
		go sub.run()
	} else {
		sub = opts.LastResult.State.(*Subscriber)
	}

	result.State = sub

	// If the requested index is lower than what we've already seen, return immediately.
	sub.lock.RLock()
	value := sub.lastResult
	sub.lock.RUnlock()
	index := getIndex(value)
	if index > opts.MinIndex {
		result.Value = value
		result.Index = index
		return result, nil
	}

	// Wait for an update based on the index we're blocking on.
	timeout := time.After(StreamTimeout)
WAIT:
	select {
	case r := <-sub.resultCh:
		// If an error was returned, exit immediately.
		if err, ok := r.(error); ok {
			return result, err
		}

		// Wait for another update if the result wasn't higher than the requested index.
		index := getIndex(r)
		if index <= opts.MinIndex {
			goto WAIT
		}
		result.Value = r
		result.Index = index
	case <-timeout:
		sub.lock.RLock()
		value := sub.lastResult
		sub.lock.RUnlock()
		index := getIndex(value)
		result.Value = value
		result.Index = index
	}

	return result, nil
}
