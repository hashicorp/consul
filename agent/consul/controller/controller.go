package controller

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"golang.org/x/sync/errgroup"
)

// much of this is a re-implementation of
// https://github.com/kubernetes-sigs/controller-runtime/blob/release-0.13/pkg/internal/controller/controller.go

// Transformer is a function that takes one type of config entry that has changed
// and transforms that into a set of reconciliation requests to enqueue.
type Transformer func(entry structs.ConfigEntry) []Request

// Controller subscribes to a set of watched resources from the
// state store and delegates processing them to a given Reconciler.
// If a Reconciler errors while processing a Request, then the
// Controller handles rescheduling the Request to be re-processed.
type Controller interface {
	// Run begins the Controller's main processing loop. When the given
	// context is canceled, the Controller stops processing any remaining work.
	// The Run function should only ever be called once.
	Run(ctx context.Context) error
	// Subscribe tells the controller to subscribe to updates for config entries based
	// on the given request. Optional transformation functions can also be passed in
	// to Subscribe, allowing a controller to map a config entry to a different type of
	// request under the hood (i.e. watching a dependency and triggering a Reconcile on
	// the dependent resource). This should only ever be called prior to calling Run.
	Subscribe(request *stream.SubscribeRequest, transformers ...Transformer) Controller
	// WithBackoff changes the base and maximum backoff values for the Controller's
	// Request retry rate limiter. This should only ever be called prior to
	// running Run.
	WithBackoff(base, max time.Duration) Controller
	// WithWorkers sets the number of worker goroutines used to process the queue
	// this defaults to 1 goroutine.
	WithWorkers(i int) Controller
	// WithQueueFactory allows a Controller to replace its underlying work queue
	// implementation. This is most useful for testing. This should only ever be called
	// prior to running Run.
	WithQueueFactory(fn func(ctx context.Context, baseBackoff time.Duration, maxBackoff time.Duration) WorkQueue) Controller
}

var _ Controller = &controller{}

type subscription struct {
	request      *stream.SubscribeRequest
	transformers []Transformer
}

// controller implements the Controller interface
type controller struct {
	// reconciler is the Reconciler that processes all subscribed
	// Requests
	reconciler Reconciler

	// makeQueue is the factory used for creating the work queue, generally
	// this shouldn't be touched, but can be updated for testing purposes
	makeQueue func(ctx context.Context, baseBackoff time.Duration, maxBackoff time.Duration) WorkQueue
	// workers is the number of workers to use to process data
	workers int
	// work is the internal work queue that pending Requests are added to
	work WorkQueue
	// baseBackoff is the starting backoff time for the work queue's rate limiter
	baseBackoff time.Duration
	// maxBackoff is the maximum backoff time for the work queue's rate limiter
	maxBackoff time.Duration

	// subscriptions is a list of subscription requests for retrieving configuration entries
	subscriptions []subscription
	// publisher is the event publisher that should be subscribed to for any updates
	publisher state.EventPublisher

	// running ensures that we are only calling Run a single time
	running int32
}

// New returns a new Controller associated with the given state store and reconciler.
func New(publisher state.EventPublisher, reconciler Reconciler) Controller {
	return &controller{
		reconciler:  reconciler,
		publisher:   publisher,
		workers:     1,
		baseBackoff: 5 * time.Millisecond,
		maxBackoff:  1000 * time.Second,
		makeQueue:   RunWorkQueue,
	}
}

// Subscribe tells the controller to subscribe to updates for config entries of the
// given kind and with the associated enterprise metadata. This should only ever be
// called prior to running Start.
func (c *controller) Subscribe(request *stream.SubscribeRequest, transformers ...Transformer) Controller {
	c.ensureNotRunning()

	c.subscriptions = append(c.subscriptions, subscription{
		request:      request,
		transformers: transformers,
	})
	return c
}

// WithBackoff changes the base and maximum backoff values for the Controller's
// Request retry rate limiter. This should only ever be called prior to
// running Start.
func (c *controller) WithBackoff(base, max time.Duration) Controller {
	c.ensureNotRunning()

	c.baseBackoff = base
	c.maxBackoff = max
	return c
}

// WithWorkers sets the number of worker goroutines used to process the queue
// this defaults to 1 goroutine.
func (c *controller) WithWorkers(i int) Controller {
	c.ensureNotRunning()

	if i <= 0 {
		i = 1
	}
	c.workers = i
	return c
}

// WithQueueFactory changes the initialization method for the Controller's work
// queue, this is predominantly just used for testing. This should only ever be called
// prior to running Start.
func (c *controller) WithQueueFactory(fn func(ctx context.Context, baseBackoff time.Duration, maxBackoff time.Duration) WorkQueue) Controller {
	c.ensureNotRunning()

	c.makeQueue = fn
	return c
}

// ensureNotRunning makes sure we aren't trying to reconfigure an already
// running controller, it panics if Run has already been invoked
func (c *controller) ensureNotRunning() {
	if atomic.LoadInt32(&c.running) == 1 {
		panic("cannot configure controller once Run is called")
	}
}

// Run begins the Controller's main processing loop. When the given
// context is canceled, the Controller stops processing any remaining work.
// The Run function should only ever be called once, calling it multiple
// times will result in a panic.
func (c *controller) Run(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		panic("Run cannot be called more than once")
	}

	group, groupCtx := errgroup.WithContext(ctx)

	// set up our queue
	c.work = c.makeQueue(groupCtx, c.baseBackoff, c.maxBackoff)

	for _, sub := range c.subscriptions {
		// store a reference for the closure
		sub := sub
		group.Go(func() error {
			var index uint64

			subscription, err := c.publisher.Subscribe(sub.request)
			if err != nil {
				return err
			}
			defer subscription.Unsubscribe()

			for {
				event, err := subscription.Next(ctx)
				switch {
				case errors.Is(err, context.Canceled):
					return nil
				case err != nil:
					return err
				}

				if event.IsFramingEvent() {
					continue
				}

				if event.Index <= index {
					continue
				}

				index = event.Index

				if err := c.processEvent(sub, event); err != nil {
					return err
				}
			}
		})
	}

	for i := 0; i < c.workers; i++ {
		group.Go(func() error {
			for {
				request, shutdown := c.work.Get()
				if shutdown {
					// Stop working
					return nil
				}
				c.reconcileHandler(groupCtx, request)
				// Done is called here because it is required to be called
				// when we've finished processing each request
				c.work.Done(request)
			}
		})
	}

	<-groupCtx.Done()
	return nil
}

func (c *controller) processEvent(sub subscription, event stream.Event) error {
	switch payload := event.Payload.(type) {
	case state.EventPayloadConfigEntry:
		c.enqueueEntry(payload.Value, sub.transformers...)
		return nil
	case *stream.PayloadEvents:
		for _, event := range payload.Items {
			if err := c.processEvent(sub, event); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unhandled event type: %T", payload)
	}
}

// enqueueEntry adds all of the given entry into the work queue. If given
// one or more transformation functions, it will enqueue all of the resulting
// reconciliation requests returned from each Transformer.
func (c *controller) enqueueEntry(entry structs.ConfigEntry, transformers ...Transformer) {
	if len(transformers) == 0 {
		c.work.Add(Request{
			Kind: entry.GetKind(),
			Name: entry.GetName(),
			Meta: entry.GetEnterpriseMeta(),
		})
	} else {
		for _, fn := range transformers {
			for _, request := range fn(entry) {
				c.work.Add(request)
			}
		}
	}
}

// reconcile wraps the reconciler in a panic handler
func (c *controller) reconcile(ctx context.Context, req Request) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic [recovered]: %v", r)
			return
		}
	}()
	return c.reconciler.Reconcile(ctx, req)
}

// reconcileHandler invokes the reconciler and looks at its return value
// to determine whether the request should be rescheduled
func (c *controller) reconcileHandler(ctx context.Context, req Request) {
	if err := c.reconcile(ctx, req); err != nil {
		// handle the case where we're specifically told to requeue later
		var requeueAfter RequeueAfterError
		if errors.As(err, &requeueAfter) {
			c.work.Forget(req)
			c.work.AddAfter(req, time.Duration(requeueAfter))
			return
		}

		// fallback to rate limit ourselves
		c.work.AddRateLimited(req)
		return
	}

	// if no error then Forget this request so it is not retried
	c.work.Forget(req)
}
