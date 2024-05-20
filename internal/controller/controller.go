// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// DependencyMapper is called when a dependency watched via WithWatch is changed
// to determine which of the controller's managed resources need to be reconciled.
type DependencyMapper func(
	ctx context.Context,
	rt Runtime,
	res *pbresource.Resource,
) ([]Request, error)

// Controller runs a reconciliation loop to respond to changes in resources and
// their dependencies. It is heavily inspired by Kubernetes' controller pattern:
// https://kubernetes.io/docs/concepts/architecture/controller/
//
// Use the builder methods in this package (starting with NewController) to construct
// a controller, and then pass it to a Manager to be executed.
type Controller struct {
	name             string
	reconciler       Reconciler
	initializer      Initializer
	managedTypeWatch *watch
	watches          map[string]*watch
	queries          map[string]cache.Query
	customWatches    []customWatch
	placement        Placement
	baseBackoff      time.Duration
	maxBackoff       time.Duration
	logger           hclog.Logger
	startCb          RuntimeCallback
	stopCb           RuntimeCallback
	// forceReconcileEvery is the time to wait after a successful reconciliation
	// before forcing a reconciliation. The net result is a reconciliation of
	// the managed type on a regular interval. This ensures that the state of the
	// world is continually reconciled, hence correct in the face of missed events
	// or other issues.
	forceReconcileEvery time.Duration
}

type RuntimeCallback func(context.Context, Runtime)

// NewController creates a controller that is setup to watched the managed type.
// Extra cache indexes may be provided as well and these indexes will be automatically managed.
// Typically, further calls to other builder methods will be needed to fully configure
// the controller such as using WithReconcile to define the code that will be called
// when the managed resource needs reconciliation.
func NewController(name string, managedType *pbresource.Type, indexes ...*index.Index) *Controller {
	w := &watch{
		watchedType: managedType,
		indexes:     make(map[string]*index.Index),
	}

	for _, idx := range indexes {
		w.addIndex(idx)
	}

	return &Controller{
		name:                name,
		managedTypeWatch:    w,
		watches:             make(map[string]*watch),
		queries:             make(map[string]cache.Query),
		forceReconcileEvery: 8 * time.Hour,
	}
}

// WithNotifyStart registers a callback to be run when the controller is being started.
// This happens prior to watches being started and with a fresh cache.
func (ctl *Controller) WithNotifyStart(start RuntimeCallback) *Controller {
	ctl.startCb = start
	return ctl
}

// WithNotifyStop registers a callback to be run when the controller has been stopped.
// This happens after all the watches and mapper/reconcile queues have been stopped. The
// cache will contain everything that was present when we started stopping watches.
func (ctl *Controller) WithNotifyStop(stop RuntimeCallback) *Controller {
	ctl.stopCb = stop
	return ctl
}

// WithReconciler changes the controller's reconciler.
func (ctl *Controller) WithReconciler(reconciler Reconciler) *Controller {
	if reconciler == nil {
		panic("reconciler must not be nil")
	}

	ctl.reconciler = reconciler
	return ctl
}

// WithWatch enables watching of the specified resource type and mapping it to the managed type
// via the provided DependencyMapper. Extra cache indexes to calculate on the watched type
// may also be provided.
func (ctl *Controller) WithWatch(watchedType *pbresource.Type, mapper DependencyMapper, indexes ...*index.Index) *Controller {
	key := resource.ToGVK(watchedType)

	_, alreadyWatched := ctl.watches[key]
	if alreadyWatched {
		panic(fmt.Sprintf("resource type %q already has a configured watch", key))
	}

	w := newWatch(watchedType, mapper)

	for _, idx := range indexes {
		w.addIndex(idx)
	}

	ctl.watches[key] = w

	return ctl
}

// WithQuery will add a named query to the controllers cache for usage during reconcile or in dependency mappers
func (ctl *Controller) WithQuery(queryName string, fn cache.Query) *Controller {
	_, duplicate := ctl.queries[queryName]
	if duplicate {
		panic(fmt.Sprintf("a predefined cache query with name %q already exists", queryName))
	}

	ctl.queries[queryName] = fn
	return ctl
}

// WithCustomWatch adds a new custom watch. Custom watches do not affect the controller cache.
func (ctl *Controller) WithCustomWatch(source *Source, mapper CustomDependencyMapper) *Controller {
	if source == nil {
		panic("source must not be nil")
	}

	if mapper == nil {
		panic("mapper must not be nil")
	}

	ctl.customWatches = append(ctl.customWatches, customWatch{source, mapper})
	return ctl
}

// WithLogger changes the controller's logger.
func (ctl *Controller) WithLogger(logger hclog.Logger) *Controller {
	if logger == nil {
		panic("logger must not be nil")
	}

	ctl.logger = logger
	return ctl
}

// WithBackoff changes the base and maximum backoff values for the controller's
// retry rate limiter.
func (ctl *Controller) WithBackoff(base, max time.Duration) *Controller {
	ctl.baseBackoff = base
	ctl.maxBackoff = max
	return ctl
}

// WithPlacement changes where and how many replicas of the controller will run.
// In the majority of cases, the default placement (one leader elected instance
// per cluster) is the most appropriate and you shouldn't need to override it.
func (ctl *Controller) WithPlacement(placement Placement) *Controller {
	ctl.placement = placement
	return ctl
}

// WithForceReconcileEvery controls how often a resource gets periodically reconciled
// to ensure that the state of the world is correct (8 hours is the default).
// This exists for tests only and should not be customized by controller authors!
func (ctl *Controller) WithForceReconcileEvery(duration time.Duration) *Controller {
	ctl.logger.Warn("WithForceReconcileEvery is for testing only and should not be set by controllers")
	ctl.forceReconcileEvery = duration
	return ctl
}

// buildCache will construct a controller Cache given the watches/indexes that have
// been added to the controller. This is mainly to be used by the TestController and
// Manager when setting up how things
func (ctl *Controller) buildCache() cache.Cache {
	c := cache.New()

	addWatchToCache(c, ctl.managedTypeWatch)
	for _, w := range ctl.watches {
		addWatchToCache(c, w)
	}

	for name, query := range ctl.queries {
		if err := c.AddQuery(name, query); err != nil {
			panic(err)
		}
	}

	return c
}

// dryRunMapper will trigger the appropriate DependencyMapper for an update of
// the provided type and return the requested reconciles.
//
// This is mainly to be used by the TestController.
func (ctl *Controller) dryRunMapper(
	ctx context.Context,
	rt Runtime,
	res *pbresource.Resource,
) ([]Request, error) {
	if resource.EqualType(ctl.managedTypeWatch.watchedType, res.Id.Type) {
		return nil, nil // no-op
	}

	for _, w := range ctl.watches {
		if resource.EqualType(w.watchedType, res.Id.Type) {
			return w.mapper(ctx, rt, res)
		}
	}
	return nil, fmt.Errorf("no mapper for type: %s", resource.TypeToString(res.Id.Type))
}

// String returns a textual description of the controller, useful for debugging.
func (ctl *Controller) String() string {
	watchedTypes := make([]string, 0, len(ctl.watches))
	for watchedType := range ctl.watches {
		watchedTypes = append(watchedTypes, watchedType)
	}
	base, max := ctl.backoff()
	return fmt.Sprintf(
		"<Controller managed_type=%s, watched_types=[%s], backoff=<base=%s, max=%s>, placement=%s>",
		resource.ToGVK(ctl.managedTypeWatch.watchedType),
		strings.Join(watchedTypes, ", "),
		base, max,
		ctl.placement,
	)
}

func (ctl *Controller) backoff() (time.Duration, time.Duration) {
	base := ctl.baseBackoff
	if base == 0 {
		base = 5 * time.Millisecond
	}
	max := ctl.maxBackoff
	if max == 0 {
		max = 1000 * time.Second
	}
	return base, max
}

func (ctl *Controller) buildLogger(defaultLogger hclog.Logger) hclog.Logger {
	logger := defaultLogger
	if ctl.logger != nil {
		logger = ctl.logger
	}

	return logger.With("controller", ctl.name, "managed_type", resource.ToGVK(ctl.managedTypeWatch.watchedType))
}

func addWatchToCache(c cache.Cache, w *watch) {
	c.AddType(w.watchedType)
	for _, index := range w.indexes {
		if err := c.AddIndex(w.watchedType, index); err != nil {
			panic(err)
		}
	}
}

// Placement determines where and how many replicas of the controller will run.
type Placement int

const (
	// PlacementSingleton ensures there is a single, leader-elected, instance of
	// the controller running in the cluster at any time. It's the default and is
	// suitable for most use-cases.
	PlacementSingleton Placement = iota

	// PlacementEachServer ensures there is a replica of the controller running on
	// each server in the cluster. It is useful for cases where the controller is
	// responsible for applying some configuration resource to the server whenever
	// it changes (e.g. rate-limit configuration). Generally, controllers in this
	// placement mode should not modify resources.
	PlacementEachServer
)

// String satisfies the fmt.Stringer interface.
func (p Placement) String() string {
	switch p {
	case PlacementSingleton:
		return "singleton"
	case PlacementEachServer:
		return "each-server"
	}
	panic(fmt.Sprintf("unknown placement %d", p))
}

// Reconciler implements the business logic of a controller.
type Reconciler interface {
	// Reconcile the resource identified by req.ID.
	Reconcile(ctx context.Context, rt Runtime, req Request) error
}

// RequeueAfterError is an error that allows a Reconciler to override the
// exponential backoff behavior of the Controller, rather than applying
// the backoff algorithm, returning a RequeueAfterError will cause the
// Controller to reschedule the Request at a given time in the future.
type RequeueAfterError time.Duration

// Error implements the error interface.
func (r RequeueAfterError) Error() string {
	return fmt.Sprintf("requeue at %s", time.Duration(r))
}

// RequeueAfter constructs a RequeueAfterError with the given duration
// setting.
func RequeueAfter(after time.Duration) error {
	return RequeueAfterError(after)
}

// RequeueNow constructs a RequeueAfterError that reschedules the Request
// immediately.
func RequeueNow() error {
	return RequeueAfterError(0)
}

// Request represents a request to reconcile the resource with the given ID.
type Request struct {
	// ID of the resource that needs to be reconciled.
	ID *pbresource.ID
}

// Key satisfies the queue.ItemType interface. It returns a string which will be
// used to de-duplicate requests in the queue.
func (r Request) Key() string {
	return fmt.Sprintf(
		"part=%q,ns=%q,name=%q,uid=%q",
		r.ID.Tenancy.Partition,
		r.ID.Tenancy.Namespace,
		r.ID.Name,
		r.ID.Uid,
	)
}

// Initializer implements the business logic that is executed when the
// controller is first started.
type Initializer interface {
	Initialize(ctx context.Context, rt Runtime) error
}

// WithInitializer changes the controller's initializer.
func (c *Controller) WithInitializer(initializer Initializer) *Controller {
	c.initializer = initializer
	return c
}
