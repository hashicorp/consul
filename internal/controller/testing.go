// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TestController is most useful when writing unit tests for a controller where
// individual Reconcile calls need to be made instead of having a Manager
// execute the controller in response to watch events.
//
// TODO(controller-testing) Ideally this would live within the controllertest
// package. However it makes light use of unexported fields on the Controller
// and therefore cannot live in another package without more refactorings
// to have the Controller include a config struct of sorts defined in an
// internal package with exported fields. For now this seems fine.
type TestController struct {
	c      *Controller
	cache  cache.Cache
	client pbresource.ResourceServiceClient
	logger hclog.Logger
}

// NewTestController will create a new TestController from the provided Controller
// and ResourceServiceClient. The test controller will build the controllers
// cache with the configured indexes and will maintain the cached state in response
// to Write, WriteStatus and Delete calls made on the wrapped ResourceServiceClient.
// Call the Runtime() method to get at the wrapped client.
func NewTestController(ctl *Controller, client pbresource.ResourceServiceClient) *TestController {
	ctlCache := ctl.buildCache()
	return &TestController{
		c:      ctl,
		cache:  ctlCache,
		client: cache.NewCachedClient(ctlCache, client),
		logger: ctl.buildLogger(hclog.NewNullLogger()),
	}
}

func (tc *TestController) WithLogger(logger hclog.Logger) *TestController {
	tc.logger = tc.c.buildLogger(logger)
	return tc
}

// Reconcile invokes the controllers configured reconciler with the cache enabled Runtime
func (tc *TestController) Reconcile(ctx context.Context, req Request) error {
	return tc.c.reconciler.Reconcile(ctx, tc.Runtime(), req)
}

// Reconciler returns the controllers configured reconciler
func (tc *TestController) Reconciler() Reconciler {
	return tc.c.reconciler
}

// Runtime returns the Runtime that should be used for calls to reconcile or to perform
// operations that would also affect the managed cache.
func (tc *TestController) Runtime() Runtime {
	return Runtime{
		Client: tc.client,
		Logger: tc.logger,
		Cache:  tc.cache,
	}
}

// DryRunMapper will trigger the appropriate DependencyMapper for an update of
// the provided type and return the requested reconciles.
//
// Useful for testing just the DependencyMapper+Cache interactions for chains
// that are more complicated than just a full controller interaction test would
// be able to easily verify.
func (tc *TestController) DryRunMapper(ctx context.Context, res *pbresource.Resource) ([]Request, error) {
	return tc.c.dryRunMapper(ctx, tc.Runtime(), res)
}
