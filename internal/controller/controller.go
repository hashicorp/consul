package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/consul/controller/queue"
	"github.com/hashicorp/consul/internal/resource"
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
	recQueue := runQueue[Request](groupCtx, c.ctrl)

	// Managed Type Events → Reconciliation Queue
	group.Go(func() error {
		return c.watch(groupCtx, c.ctrl.managedType, func(res *pbresource.Resource) {
			recQueue.Add(Request{ID: res.Id})
		})
	})

	for _, watch := range c.ctrl.watches {
		watch := watch
		mapQueue := runQueue[*pbresource.Resource](groupCtx, c.ctrl)

		// Watched Type Events → Mapper Queue
		group.Go(func() error {
			return c.watch(groupCtx, watch.watchedType, mapQueue.Add)
		})

		// Mapper Queue → Mapper → Reconciliation Queue
		group.Go(func() error {
			return c.runMapper(groupCtx, watch, mapQueue, recQueue)
		})
	}

	// Reconciliation Queue → Reconciler
	group.Go(func() error {
		return c.runReconciler(groupCtx, recQueue)
	})

	return group.Wait()
}

func runQueue[T queue.ItemType](ctx context.Context, ctrl Controller) queue.WorkQueue[T] {
	baseBackoff := ctrl.baseBackoff
	if baseBackoff == 0 {
		baseBackoff = 5 * time.Millisecond
	}

	maxBackoff := ctrl.maxBackoff
	if maxBackoff == 0 {
		maxBackoff = 1000 * time.Second
	}

	return queue.RunWorkQueue[T](ctx, baseBackoff, maxBackoff)
}

func (c *controllerRunner) watch(ctx context.Context, typ *pbresource.Type, add func(*pbresource.Resource)) error {
	watch, err := c.client.WatchList(ctx, &pbresource.WatchListRequest{
		Type: typ,
		Tenancy: &pbresource.Tenancy{
			Partition: storage.Wildcard,
			PeerName:  storage.Wildcard,
			Namespace: storage.Wildcard,
		},
	})
	if err != nil {
		c.logger.Error("failed to create watch", "error", err)
		return err
	}

	for {
		event, err := watch.Recv()
		if err != nil {
			c.logger.Warn("error received from watch", "error", err)
			return err
		}
		add(event.Resource)
	}
}

func (c *controllerRunner) runMapper(
	ctx context.Context,
	w watch,
	from queue.WorkQueue[*pbresource.Resource],
	to queue.WorkQueue[Request],
) error {
	logger := c.logger.With("watched_resource_type", resource.ToGVK(w.watchedType))

	for {
		res, shutdown := from.Get()
		if shutdown {
			return nil
		}

		reqs, err := w.mapper(ctx, c.runtime(), res)
		if err != nil {
			from.AddRateLimited(res)
			from.Done(res)
			continue
		}

		for _, r := range reqs {
			if !proto.Equal(r.ID.Type, c.ctrl.managedType) {
				logger.Error("dependency mapper returned request for a resource of the wrong type",
					"type_expected", resource.ToGVK(c.ctrl.managedType),
					"type_got", resource.ToGVK(r.ID.Type),
				)
				continue
			}
			to.Add(r)
		}

		from.Forget(res)
		from.Done(res)
	}
}

func (c *controllerRunner) runReconciler(ctx context.Context, queue queue.WorkQueue[Request]) error {
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

	return c.ctrl.reconciler.Reconcile(ctx, c.runtime(), req)
}

func (c *controllerRunner) runtime() Runtime {
	return Runtime{
		Client: c.client,
		Logger: c.logger,
	}
}
