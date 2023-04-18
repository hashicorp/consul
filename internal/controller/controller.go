package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/consul/agent/consul/controller/queue"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// controllerRunner contains the actual implementation of running a controller
// including creating watches, calling the reconciler, handling retries, etc.
type controllerRunner struct {
	ctrl   Controller
	client pbresource.ResourceServiceClient
	logger hclog.Logger
}

func (c *controllerRunner) run(ctx context.Context) error {
	c.logger.Debug("controller running")
	defer c.logger.Debug("controller stopping")

	group, groupCtx := errgroup.WithContext(ctx)

	queue := c.runQueue(groupCtx)
	group.Go(func() error { return c.watchManagedType(groupCtx, queue) })
	group.Go(func() error { return c.runReconciler(groupCtx, queue) })

	return group.Wait()
}

type workQueue queue.WorkQueue[Request]

func (c *controllerRunner) runQueue(ctx context.Context) workQueue {
	baseBackoff := c.ctrl.baseBackoff
	if baseBackoff == 0 {
		baseBackoff = 5 * time.Millisecond
	}

	maxBackoff := c.ctrl.maxBackoff
	if maxBackoff == 0 {
		maxBackoff = 1000 * time.Second
	}

	return queue.RunWorkQueue[Request](ctx, baseBackoff, maxBackoff)
}

func (c *controllerRunner) watchManagedType(ctx context.Context, queue workQueue) error {
	watch, err := c.client.WatchList(ctx, &pbresource.WatchListRequest{
		Type: c.ctrl.managedType,
		Tenancy: &pbresource.Tenancy{
			Partition: storage.Wildcard,
			PeerName:  storage.Wildcard,
			Namespace: storage.Wildcard,
		},
	})
	if err != nil {
		c.logger.Error("failed to create watch on managed resource type", "error", err)
		return err
	}

	for {
		event, err := watch.Recv()
		if err != nil {
			c.logger.Warn("error received from watch", "error", err)
			return err
		}
		queue.Add(Request{ID: event.Resource.Id})
	}
}

func (c *controllerRunner) runReconciler(ctx context.Context, queue workQueue) error {
	for {
		req, shutdown := queue.Get()
		if shutdown {
			return nil
		}

		c.logger.Trace("handling request", "request", req)
		if err := c.reconcile(ctx, req); err == nil {
			queue.Forget(req)
		} else {
			var requeueAfter RequeueAfterError
			if errors.As(err, &requeueAfter) {
				queue.Forget(req)
				queue.AddAfter(req, time.Duration(requeueAfter))
			} else {
				queue.AddRateLimited(req)
			}
		}
		queue.Done(req)
	}
}

func (c *controllerRunner) reconcile(ctx context.Context, req Request) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := hclog.Stacktrace()
			c.logger.Error("panic from controller reconciler",
				"panic", r,
				"stack", stack,
			)
			err = fmt.Errorf("panic [recovered]: %v", r)
			return
		}
	}()

	return c.ctrl.reconciler.Reconcile(ctx, Runtime{
		Client: c.client,
		Logger: c.logger,
	}, req)
}
