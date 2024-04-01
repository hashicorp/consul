// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/controller/queue"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/protoutil"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Runtime contains the dependencies required by reconcilers.
type Runtime struct {
	Client pbresource.ResourceServiceClient
	Logger hclog.Logger
	Cache  cache.ReadOnlyCache
}

// controllerRunner contains the actual implementation of running a controller
// including creating watches, calling the reconciler, handling retries, etc.
type controllerRunner struct {
	ctrl *Controller
	// watchClient will be used by the controller infrastructure internally to
	// perform watches and maintain the cache. On servers, this client will use
	// the in-memory gRPC clients which DO NOT cause cloning of data returned by
	// the resource service. This is desirable so that our cache doesn't incur
	// the overhead of duplicating all resources that are watched. Generally
	// dependency mappers and reconcilers should not be given this client so
	// that they can be free to modify the data they are returned.
	watchClient pbresource.ResourceServiceClient
	// runtimeClient will be used by dependency mappers and reconcilers to
	// access the resource service. On servers, this client will use the in-memory
	// gRPC client wrapped with the cloning client to force cloning of protobuf
	// messages as they pass through the client. This is desirable so that
	// controllers and their mappers can be free to modify the data returned
	// to them without having to think about the fact that the data should
	// be immutable as it is shared with the controllers cache as well as the
	// resource service itself.
	runtimeClient pbresource.ResourceServiceClient
	logger        hclog.Logger
	cache         cache.Cache
}

func newControllerRunner(c *Controller, client pbresource.ResourceServiceClient, defaultLogger hclog.Logger) *controllerRunner {
	return &controllerRunner{
		ctrl:          c,
		watchClient:   client,
		runtimeClient: pbresource.NewCloningResourceServiceClient(client),
		logger:        c.buildLogger(defaultLogger),
		// Do not build the cache here. If we build/set it when the controller runs
		// then if a controller is restarted it will invalidate the previous cache automatically.
	}
}

func (cr *controllerRunner) run(ctx context.Context) error {
	cr.logger.Debug("controller running")
	defer cr.logger.Debug("controller stopping")

	// Initialize the controller if required
	if cr.ctrl.initializer != nil {
		cr.logger.Debug("controller initializing")
		err := cr.ctrl.initializer.Initialize(ctx, cr.runtime(cr.logger))
		if err != nil {
			return err
		}
		cr.logger.Debug("controller initialized")
	}

	cr.cache = cr.ctrl.buildCache()
	defer func() {
		// once no longer running we should nil out the cache
		// so that we don't hold pointers to resources which may
		// become out of date in the future.
		cr.cache = nil
	}()

	if cr.ctrl.startCb != nil {
		cr.ctrl.startCb(ctx, cr.runtime(cr.logger))
	}

	if cr.ctrl.stopCb != nil {
		defer cr.ctrl.stopCb(ctx, cr.runtime(cr.logger))
	}

	// Before we launch the reconciler or the dependency mappers, ensure the
	// cache is properly primed to avoid errant reconciles.
	//
	// Without doing this the cache is unsafe for general use without causing
	// reconcile regressions in certain cases.
	{
		cr.logger.Debug("priming caches")
		primeGroup, primeGroupCtx := errgroup.WithContext(ctx)
		// Managed Type Events
		primeGroup.Go(func() error {
			return cr.primeCache(primeGroupCtx, cr.ctrl.managedTypeWatch.watchedType)
		})
		for _, w := range cr.ctrl.watches {
			watcher := w
			// Watched Type Events
			primeGroup.Go(func() error {
				return cr.primeCache(primeGroupCtx, watcher.watchedType)
			})
		}

		if err := primeGroup.Wait(); err != nil {
			return err
		}
		cr.logger.Debug("priming caches complete")
	}

	group, groupCtx := errgroup.WithContext(ctx)
	recQueue := runQueue[Request](groupCtx, cr.ctrl)

	// Managed Type Events → Managed Type Reconciliation Queue
	group.Go(func() error {
		return cr.watch(groupCtx, cr.ctrl.managedTypeWatch.watchedType, func(res *pbresource.Resource) {
			recQueue.Add(Request{ID: res.Id})
		})
	})

	for _, w := range cr.ctrl.watches {
		mapQueue := runQueue[mapperRequest](groupCtx, cr.ctrl)
		watcher := w

		// Watched Type Events → Watched Type Mapper Queue
		group.Go(func() error {
			return cr.watch(groupCtx, watcher.watchedType, func(res *pbresource.Resource) {
				mapQueue.Add(mapperRequest{res: res})
			})
		})

		// Watched Type Mapper Queue → Watched Type Mapper → Managed Type Reconciliation Queue
		group.Go(func() error {
			return cr.runMapper(groupCtx, watcher, mapQueue, recQueue, func(ctx context.Context, runtime Runtime, itemType queue.ItemType) ([]Request, error) {
				return watcher.mapper(ctx, runtime, itemType.(mapperRequest).res)
			})
		})
	}

	for _, cw := range cr.ctrl.customWatches {
		customMapQueue := runQueue[Event](groupCtx, cr.ctrl)
		watcher := cw
		// Custom Events → Custom Mapper Queue
		group.Go(func() error {
			return watcher.source.Watch(groupCtx, func(e Event) {
				customMapQueue.Add(e)
			})
		})

		// Custom Mapper Queue → Custom Dependency Mapper → Managed Type Reconciliation Queue
		group.Go(func() error {
			return cr.runCustomMapper(groupCtx, watcher, customMapQueue, recQueue, func(ctx context.Context, runtime Runtime, itemType queue.ItemType) ([]Request, error) {
				return watcher.mapper(ctx, runtime, itemType.(Event))
			})
		})
	}

	// Managed Type Reconciliation Queue → Reconciler
	group.Go(func() error {
		return cr.runReconciler(groupCtx, recQueue)
	})

	return group.Wait()
}

func runQueue[T queue.ItemType](ctx context.Context, ctrl *Controller) queue.WorkQueue[T] {
	base, max := ctrl.backoff()
	return queue.RunWorkQueue[T](ctx, base, max)
}

func (cr *controllerRunner) primeCache(ctx context.Context, typ *pbresource.Type) error {
	wl, err := cr.watchClient.WatchList(ctx, &pbresource.WatchListRequest{
		Type: typ,
		Tenancy: &pbresource.Tenancy{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
		},
	})
	if err != nil {
		cr.handleInvalidControllerWatch(err)
		cr.logger.Error("failed to create cache priming watch", "error", err)
		return err
	}

	for {
		event, err := wl.Recv()
		if err != nil {
			cr.handleInvalidControllerWatch(err)
			cr.logger.Warn("error received from cache priming watch", "error", err)
			return err
		}

		switch {
		case event.GetUpsert() != nil:
			cr.cache.Insert(event.GetUpsert().Resource)
		case event.GetDelete() != nil:
			cr.cache.Delete(event.GetDelete().Resource)
		case event.GetEndOfSnapshot() != nil:
			// This concludes the initial snapshot. The cache is primed.
			return nil
		default:
			cr.logger.Warn("skipping unexpected event type", "type", hclog.Fmt("%T", event.GetEvent()))
			continue
		}
	}
}

func (cr *controllerRunner) watch(ctx context.Context, typ *pbresource.Type, add func(*pbresource.Resource)) error {
	wl, err := cr.watchClient.WatchList(ctx, &pbresource.WatchListRequest{
		Type: typ,
		Tenancy: &pbresource.Tenancy{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
		},
	})
	if err != nil {
		cr.handleInvalidControllerWatch(err)
		cr.logger.Error("failed to create watch", "error", err)
		return err
	}

	for {
		event, err := wl.Recv()
		if err != nil {
			cr.handleInvalidControllerWatch(err)
			cr.logger.Warn("error received from watch", "error", err)
			return err
		}

		// Keep the cache up to date. There main reason to do this here is
		// to ensure that any mapper/reconciliation queue deduping won't
		// hide events from being observed and updating the cache state.
		// Therefore we should do this before any queueing.
		var resource *pbresource.Resource
		switch {
		case event.GetUpsert() != nil:
			resource = event.GetUpsert().GetResource()
			cr.cache.Insert(resource)
		case event.GetDelete() != nil:
			resource = event.GetDelete().GetResource()
			cr.cache.Delete(resource)
		case event.GetEndOfSnapshot() != nil:
			continue // ignore
		default:
			cr.logger.Warn("skipping unexpected event type", "type", hclog.Fmt("%T", event.GetEvent()))
			continue
		}

		// Before adding the resource into the queue we must clone it.
		// While we want the cache to not have duplicate copies of all the
		// data, we do want downstream consumers like dependency mappers and
		// controllers to be able to freely modify the data they are given.
		// Therefore we clone the resource here to prevent any accidental
		// mutation of data held by the cache (and presumably by the resource
		// service assuming that the regular client we were given is the inmem
		// variant)
		add(protoutil.Clone(resource))
	}
}

func (cr *controllerRunner) runMapper(
	ctx context.Context,
	w *watch,
	from queue.WorkQueue[mapperRequest],
	to queue.WorkQueue[Request],
	mapper func(ctx context.Context, runtime Runtime, itemType queue.ItemType) ([]Request, error),
) error {
	logger := cr.logger.With("watched_resource_type", resource.ToGVK(w.watchedType))

	for {
		item, shutdown := from.Get()
		if shutdown {
			return nil
		}

		if err := cr.doMap(ctx, mapper, to, item, logger); err != nil {
			from.AddRateLimited(item)
			from.Done(item)
			continue
		}

		from.Forget(item)
		from.Done(item)
	}
}

func (cr *controllerRunner) runCustomMapper(
	ctx context.Context,
	cw customWatch,
	from queue.WorkQueue[Event],
	to queue.WorkQueue[Request],
	mapper func(ctx context.Context, runtime Runtime, itemType queue.ItemType) ([]Request, error),
) error {
	logger := cr.logger.With("watched_event", cw.source)

	for {
		item, shutdown := from.Get()
		if shutdown {
			return nil
		}

		if err := cr.doMap(ctx, mapper, to, item, logger); err != nil {
			from.AddRateLimited(item)
			from.Done(item)
			continue
		}

		from.Forget(item)
		from.Done(item)
	}
}

func (cr *controllerRunner) doMap(ctx context.Context, mapper func(ctx context.Context, runtime Runtime, itemType queue.ItemType) ([]Request, error), to queue.WorkQueue[Request], item queue.ItemType, logger hclog.Logger) error {
	var reqs []Request
	if err := cr.handlePanic(func() error {
		var err error
		reqs, err = mapper(ctx, cr.runtime(logger.With("map-request-key", item.Key())), item)
		return err
	}); err != nil {
		return err
	}

	for _, r := range reqs {
		if !resource.EqualType(r.ID.Type, cr.ctrl.managedTypeWatch.watchedType) {
			logger.Error("dependency mapper returned request for a resource of the wrong type",
				"type_expected", resource.ToGVK(cr.ctrl.managedTypeWatch.watchedType),
				"type_got", resource.ToGVK(r.ID.Type),
			)
			continue
		}
		to.Add(r)
	}
	return nil
}

// maybeScheduleForcedReconcile makes sure that a "reconcile the world" happens periodically for the
// controller's managed type.
func (cr *controllerRunner) maybeScheduleForcedReconcile(queue queue.WorkQueue[Request], req Request) {
	// In order to periodically "reconcile the world", we schedule a deferred reconcile request
	// (aka forced reconcile) minus a sizeable random jitter to avoid a thundering herd.
	//
	// A few notes on how this integrates with existing non-"reconcile the world" requests:
	//
	// 1. Successful reconciles result in a new deferred "reconcile the world" request being scheduled.
	//    The net effect is that the managed type will be continually reconciled regardless of any updates.
	// 2. Failed reconciles are re-queued with a rate limit and get added to the deferred reconcile queue.
	//    Any existing deferred "reconcile the world" request will be replaced by the rate-limited deferred
	//    request.
	// 3. An existing deferred "reconcile the world" request can't be removed on the successful reconcile
	//    of a delete operation. We rely on controller idempotency to eventually process the deferred request
	//    as a no-op.
	_, err := cr.runtimeClient.Read(context.Background(), &pbresource.ReadRequest{Id: req.ID})
	switch {
	case err != nil && status.Code(err) == codes.NotFound:
		// Resource was deleted -> nothing to force reconcile so do nothing
		return
	default:
		// Reconcile of resource upsert was successful or we had an unexpected
		// error. In either case, we should schedule a forced reconcile for completeness.
		scheduleAt := reduceByRandomJitter(cr.ctrl.forceReconcileEvery)
		queue.AddAfter(req, scheduleAt, true)
	}
}

// reduceByRandomJitter returns a duration reduced by a random amount up to 20%.
func reduceByRandomJitter(d time.Duration) time.Duration {
	percent := rand.Float64() * 0.2
	reduction := time.Duration(float64(d) * percent)
	return d - reduction
}

func (cr *controllerRunner) runReconciler(ctx context.Context, queue queue.WorkQueue[Request]) error {
	for {
		req, shutdown := queue.Get()
		if shutdown {
			return nil
		}

		cr.logger.Trace("handling request", "request", req)
		err := cr.handlePanic(func() error {
			return cr.ctrl.reconciler.Reconcile(ctx, cr.runtime(cr.logger.With("resource-id", req.ID.String())), req)
		})
		if err == nil {
			queue.Forget(req)
			cr.maybeScheduleForcedReconcile(queue, req)
		} else {
			cr.logger.Trace("post-processing reconcile failure")
			var requeueAfter RequeueAfterError
			if errors.As(err, &requeueAfter) {
				queue.Forget(req)
				queue.AddAfter(req, time.Duration(requeueAfter), false)
			} else {
				queue.AddRateLimited(req)
			}
		}
		queue.Done(req)
	}
}

func (cr *controllerRunner) handlePanic(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := hclog.Stacktrace()
			cr.logger.Error("controller panic",
				"panic", r,
				"stack", stack,
			)
			err = fmt.Errorf("panic [recovered]: %v", r)
			return
		}
	}()

	return fn()
}

func (cr *controllerRunner) runtime(logger hclog.Logger) Runtime {
	return Runtime{
		// dependency mappers and controllers are always given the cloning client
		// so that they do not have to care about mutating values that they read
		// through the client.
		Client: cr.runtimeClient,
		Logger: logger,
		// ensure that resources queried via the cache get cloned so that the
		// dependency mapper or reconciler is free to modify them.
		Cache: cache.NewCloningReadOnlyCache(cr.cache),
	}
}

func (cr *controllerRunner) handleInvalidControllerWatch(err error) {
	st, ok := status.FromError(err)
	if ok && st.Code() == codes.InvalidArgument {
		panic(fmt.Sprintf("controller %s attempted to initiate an invalid watch: %q. This is a bug within the controller.", cr.ctrl.name, err.Error()))
	}
}

type mapperRequest struct {
	// res is the resource that was watched and is being mapped.
	res *pbresource.Resource
}

// Key satisfies the queue.ItemType interface. It returns a string which will be
// used to de-duplicate requests in the queue.
func (i mapperRequest) Key() string {
	return fmt.Sprintf(
		"type=%q,part=%q,ns=%q,name=%q,uid=%q",
		resource.ToGVK(i.res.Id.Type),
		i.res.Id.Tenancy.Partition,
		i.res.Id.Tenancy.Namespace,
		i.res.Id.Name,
		i.res.Id.Uid,
	)
}
