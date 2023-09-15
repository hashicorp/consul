// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/controller/queue"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ForType begins building a Controller for the given resource type.
func ForType(managedType *pbresource.Type) Controller {
	return Controller{managedType: managedType}
}

// WithReconciler changes the controller's reconciler.
func (c Controller) WithReconciler(reconciler Reconciler) Controller {
	if reconciler == nil {
		panic("reconciler must not be nil")
	}

	c.reconciler = reconciler
	return c
}

// WithWatch adds a watch on the given type/dependency to the controller. mapper
// will be called to determine which resources must be reconciled as a result of
// a watched resource changing.
func (c Controller) WithWatch(watchedType *pbresource.Type, mapper DependencyMapper) Controller {
	if watchedType == nil {
		panic("watchedType must not be nil")
	}

	if mapper == nil {
		panic("mapper must not be nil")
	}

	c.watches = append(c.watches, watch{watchedType, mapper})
	return c
}

// WithCustomWatch adds a custom watch on the given dependency to the controller. Custom mapper
// will be called to map events produced by source to the controller's watched type.
func (c Controller) WithCustomWatch(source *Source, mapper CustomDependencyMapper) Controller {
	if source == nil {
		panic("source must not be nil")
	}

	if mapper == nil {
		panic("mapper must not be nil")
	}

	c.customWatches = append(c.customWatches, customWatch{source, mapper})
	return c
}

// WithLogger changes the controller's logger.
func (c Controller) WithLogger(logger hclog.Logger) Controller {
	if logger == nil {
		panic("logger must not be nil")
	}

	c.logger = logger
	return c
}

// WithBackoff changes the base and maximum backoff values for the controller's
// retry rate limiter.
func (c Controller) WithBackoff(base, max time.Duration) Controller {
	c.baseBackoff = base
	c.maxBackoff = max
	return c
}

// WithPlacement changes where and how many replicas of the controller will run.
// In the majority of cases, the default placement (one leader elected instance
// per cluster) is the most appropriate and you shouldn't need to override it.
func (c Controller) WithPlacement(placement Placement) Controller {
	c.placement = placement
	return c
}

// String returns a textual description of the controller, useful for debugging.
func (c Controller) String() string {
	watchedTypes := make([]string, len(c.watches))
	for idx, w := range c.watches {
		watchedTypes[idx] = fmt.Sprintf("%q", resource.ToGVK(w.watchedType))
	}
	base, max := c.backoff()
	return fmt.Sprintf(
		"<Controller managed_type=%q, watched_types=[%s], backoff=<base=%q, max=%q>, placement=%q>",
		resource.ToGVK(c.managedType),
		strings.Join(watchedTypes, ", "),
		base, max,
		c.placement,
	)
}

func (c Controller) backoff() (time.Duration, time.Duration) {
	base := c.baseBackoff
	if base == 0 {
		base = 5 * time.Millisecond
	}
	max := c.maxBackoff
	if max == 0 {
		max = 1000 * time.Second
	}
	return base, max
}

// Controller runs a reconciliation loop to respond to changes in resources and
// their dependencies. It is heavily inspired by Kubernetes' controller pattern:
// https://kubernetes.io/docs/concepts/architecture/controller/
//
// Use the builder methods in this package (starting with ForType) to construct
// a controller, and then pass it to a Manager to be executed.
type Controller struct {
	managedType   *pbresource.Type
	reconciler    Reconciler
	logger        hclog.Logger
	watches       []watch
	customWatches []customWatch
	baseBackoff   time.Duration
	maxBackoff    time.Duration
	placement     Placement
}

type watch struct {
	watchedType *pbresource.Type
	mapper      DependencyMapper
}

// Watch is responsible for watching for custom events from source and adding them to
// the event queue.
func (s *Source) Watch(ctx context.Context, add func(e Event)) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-s.Source:
			if !ok {
				return nil
			}
			add(evt)
		}
	}
}

// Source is used as a generic source of events. This can be used when events aren't coming from resources
// stored by the resource API.
type Source struct {
	Source <-chan Event
}

// Event captures an event in the system which the API can choose to respond to.
type Event struct {
	Obj queue.ItemType
}

// Key returns a string that will be used to de-duplicate items in the queue.
func (e Event) Key() string {
	return e.Obj.Key()
}

// customWatch represent a Watch on a custom Event source and a Mapper to map said
// Events into Requests that the controller can respond to.
type customWatch struct {
	source *Source
	mapper CustomDependencyMapper
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
		"part=%q,peer=%q,ns=%q,name=%q,uid=%q",
		r.ID.Tenancy.Partition,
		r.ID.Tenancy.PeerName,
		r.ID.Tenancy.Namespace,
		r.ID.Name,
		r.ID.Uid,
	)
}

// Runtime contains the dependencies required by reconcilers.
type Runtime struct {
	Client pbresource.ResourceServiceClient
	Logger hclog.Logger
}

// Reconciler implements the business logic of a controller.
type Reconciler interface {
	// Reconcile the resource identified by req.ID.
	Reconcile(ctx context.Context, rt Runtime, req Request) error
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
