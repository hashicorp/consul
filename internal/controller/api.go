package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ForType begins building a Controller for the given resource type.
func ForType(managedType *pbresource.Type) Controller {
	return Controller{managedType: managedType}
}

// WithReconciler adds the given reconciler to the controller being built.
func (c Controller) WithReconciler(reconciler Reconciler) Controller {
	c.reconciler = reconciler
	return c
}

// WithWatch adds a watch on the given type/dependency to the controller being
// built. mapper will be called to determine which resources must be reconciled
// as a result of a watched resource changing.
func (c Controller) WithWatch(watchedType *pbresource.Type, mapper DependencyMapper) Controller {
	c.watches = append(c.watches, watch{watchedType, mapper})
	return c
}

// WithLogger adds the given logger to the controller being built.
func (c Controller) WithLogger(logger hclog.Logger) Controller {
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

// Controller runs a reconciliation loop to respond to changes in resources and
// their dependencies. It is heavily inspired by Kubernetes' controller pattern:
// https://kubernetes.io/docs/concepts/architecture/controller/
//
// Use the builder methods in this package (starting with ForType) to construct
// a controller, and then pass it to a Manager to be executed.
type Controller struct {
	managedType *pbresource.Type
	reconciler  Reconciler
	logger      hclog.Logger
	watches     []watch
	baseBackoff time.Duration
	maxBackoff  time.Duration
}

type watch struct {
	watchedType *pbresource.Type
	mapper      DependencyMapper
}

// Request represents a request to reconcile the resource with the given ID.
type Request struct {
	// ID of the resource that needs to be reconciled.
	ID *pbresource.ID
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

// DependencyMapper is called when a dependency watched via WithWatch is changed
// to determine which of the controller's managed resources need to be reconciled.
type DependencyMapper func(
	ctx context.Context,
	rt Runtime,
	res *pbresource.Resource,
) ([]Request, error)

// MapOwner implements a DependencyMapper that returns the updated resource's owner.
func MapOwner(_ context.Context, _ Runtime, res *pbresource.Resource) ([]Request, error) {
	var reqs []Request
	if res.Owner != nil {
		reqs = append(reqs, Request{ID: res.Owner})
	}
	return reqs, nil
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
