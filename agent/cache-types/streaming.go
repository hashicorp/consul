package cachetype

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"google.golang.org/grpc"
)

const StreamTimeout = 10 * time.Minute

type StreamingClient interface {
	Subscribe(ctx context.Context, in *stream.SubscribeRequest, opts ...grpc.CallOption) (stream.Consul_SubscribeClient, error)
}

type EventHandler interface {
	HandleEvent(event *stream.Event)
	State(idx uint64) interface{}
	Reset()
}

// Subscriber runs a streaming subscription for a single topic/key combination.
type Subscriber struct {
	client  StreamingClient
	request stream.SubscribeRequest
	handler EventHandler

	lastResult interface{}
	resultCh   chan watchResult
	ctx        context.Context
	cancelFunc func()

	lock sync.RWMutex
}

type watchResult struct {
	result     interface{}
	forceReset bool
}

func NewSubscriber(client StreamingClient, req stream.SubscribeRequest, handler EventHandler) *Subscriber {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Subscriber{
		client:     client,
		request:    req,
		handler:    handler,
		resultCh:   make(chan watchResult, 1),
		ctx:        ctx,
		cancelFunc: cancel,
	}

	return s
}

// Close stops the subscriber by cancelling the context.
func (s *Subscriber) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cancelFunc()
	s.client = nil
	s.handler = nil
	return nil
}

// run starts a Subscribe call for the subscriber's key, receives the initial
// snapshot of state and then sends the state to the watcher as soon as its
// watched index requirement is met. Run may only be called once per subscriber.
func (s *Subscriber) run() {
	defer s.cancelFunc()

START:
	// Start a new Subscribe call.
	streamHandle, err := s.client.Subscribe(s.ctx, &s.request)

	// If something went wrong setting up the stream, return the error.
	if err != nil {
		s.resultCh <- watchResult{result: err}
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
			case s.resultCh <- watchResult{result: err}:
			default:
			}
			return
		}

		// Check to see if this is a special "reload stream" message before processing
		// the event.
		if event.GetReloadStream() {
			fmt.Printf("[WARN] got ACL reset event\n")
			s.handler.Reset()

			// Cancel the context to end the streaming call and create a new one.
			s.lock.Lock()
			s.cancelFunc()
			s.ctx, s.cancelFunc = context.WithCancel(context.Background())
			s.lock.Unlock()

			goto START
		}

		// Update our version of the state based on the event/op.
		var finishedSnapshot bool
		switch event.Topic {
		case s.request.Topic:
			if !event.GetEndOfSnapshot() {
				s.handler.HandleEvent(event)
			} else {
				fmt.Printf("SNAPSHOT FINISHED\n")
				snapshotDone = true
				finishedSnapshot = true
			}
		default:
			// should never happen
			panic(fmt.Sprintf("received invalid event topic %s", event.Topic.String()))
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
		case s.resultCh <- watchResult{result: result, forceReset: finishedSnapshot}:
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
		if err, ok := r.result.(error); ok {
			// Reset the state/subscriber if there was an error.
			result.State = nil
			return result, err
		}

		// Wait for another update if the result wasn't higher than the requested index.
		// If this update came from a snapshot finishing, we return no matter what, as the
		// state/index could have changed as a result of ACL updates or other filtering.
		index := getIndex(r.result)
		if index <= opts.MinIndex && !r.forceReset {
			fmt.Println("WAITING FOR HIGHER INDEX")
			goto WAIT
		}
		fmt.Println("RETURNING UPDATE")
		result.Value = r.result
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
