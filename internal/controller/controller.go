// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/sync/errgroup"

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
		mapQueue := runQueue[mapperRequest](groupCtx, c.ctrl)

		// Watched Type Events → Mapper Queue
		group.Go(func() error {
			return c.watch(groupCtx, watch.watchedType, func(res *pbresource.Resource) {
				mapQueue.Add(mapperRequest{res: res})
			})
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
	base, max := ctrl.backoff()
	return queue.RunWorkQueue[T](ctx, base, max)
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
	from queue.WorkQueue[mapperRequest],
	to queue.WorkQueue[Request],
) error {
	logger := c.logger.With("watched_resource_type", resource.ToGVK(w.watchedType))

	for {
		item, shutdown := from.Get()
		if shutdown {
			return nil
		}

		var reqs []Request
		err := c.handlePanic(func() error {
			var err error
			reqs, err = w.mapper(ctx, c.runtime(), item.res)
			return err
		})
		if err != nil {
			from.AddRateLimited(item)
			from.Done(item)
			continue
		}

		for _, r := range reqs {
			if !resource.EqualType(r.ID.Type, c.ctrl.managedType) {
				logger.Error("dependency mapper returned request for a resource of the wrong type",
					"type_expected", resource.ToGVK(c.ctrl.managedType),
					"type_got", resource.ToGVK(r.ID.Type),
				)
				continue
			}
			to.Add(r)
		}

		from.Forget(item)
		from.Done(item)
	}
}

func (c *controllerRunner) runReconciler(ctx context.Context, queue queue.WorkQueue[Request]) error {
	for {
		req, shutdown := queue.Get()
		if shutdown {
			return nil
		}

		c.logger.Trace("handling request", "request", req)
		err := c.handlePanic(func() error {
			return c.ctrl.reconciler.Reconcile(ctx, c.runtime(), req)
		})
		if err == nil {
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

func (c *controllerRunner) handlePanic(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := hclog.Stacktrace()
			c.logger.Error("controller panic",
				"panic", r,
				"stack", stack,
			)
			err = fmt.Errorf("panic [recovered]: %v", r)
			return
		}
	}()

	return fn()
}

func (c *controllerRunner) runtime() Runtime {
	return Runtime{
		Client: c.client,
		Logger: c.logger,
	}
}

type mapperRequest struct{ res *pbresource.Resource }

// Key satisfies the queue.ItemType interface. It returns a string which will be
// used to de-duplicate requests in the queue.
func (i mapperRequest) Key() string {
	return fmt.Sprintf(
		"type=%q,part=%q,peer=%q,ns=%q,name=%q,uid=%q",
		resource.ToGVK(i.res.Id.Type),
		i.res.Id.Tenancy.Partition,
		i.res.Id.Tenancy.PeerName,
		i.res.Id.Tenancy.Namespace,
		i.res.Id.Name,
		i.res.Id.Uid,
	)
}
