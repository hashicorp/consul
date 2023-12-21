// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-hclog"
)

// TestController is most useful when writing unit tests for a controller where
// individual Reconcile calls need to be made instead of having a Manager
// execute the controller in response to watch events.
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
