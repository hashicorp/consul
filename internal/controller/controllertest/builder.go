// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllertest

import (
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type Builder struct {
	serviceBuilder        *svctest.Builder
	controllerRegisterFns []func(*controller.Manager)
}

// NewControllerTestBuilder starts to build out out the necessary controller testing
// runtime for lightweight controller integration testing. This will run a single
// in-memory resource service instance and the controller manager. Usage of this
// builder is an easy way to ensure that all the right resource gRPC connections/clients
// get set up appropriately in a manner identical to how they would be on a full
// running server.
func NewControllerTestBuilder() *Builder {
	return &Builder{
		// disable cloning because we will enable it after passing the non-cloning variant
		// to the controller manager.
		serviceBuilder: svctest.NewResourceServiceBuilder().WithCloningDisabled(),
	}
}

// Registry retrieves the resource registry from the internal im-mem resource service.
func (b *Builder) Registry() resource.Registry {
	return b.serviceBuilder.Registry()
}

// WithResourceRegisterFns allows configuring functions to be run to register resource
// types with the internal in-mem resource service for the duration of the test.
func (b *Builder) WithResourceRegisterFns(registerFns ...func(resource.Registry)) *Builder {
	b.serviceBuilder = b.serviceBuilder.WithRegisterFns(registerFns...)
	return b
}

// WithControllerRegisterFns allows configuring a set of controllers that should be registered
// with the controller manager and executed during Run.
func (b *Builder) WithControllerRegisterFns(registerFns ...func(*controller.Manager)) *Builder {
	for _, registerFn := range registerFns {
		b.controllerRegisterFns = append(b.controllerRegisterFns, registerFn)
	}
	return b
}

// WithACLResolver is used to provide an ACLResolver implementation to the internal resource service.
func (b *Builder) WithACLResolver(aclResolver svc.ACLResolver) *Builder {
	b.serviceBuilder = b.serviceBuilder.WithACLResolver(aclResolver)
	return b
}

// WithTenancies adds additional tenancies if default/default is not sufficient.
func (b *Builder) WithTenancies(tenancies ...*pbresource.Tenancy) *Builder {
	b.serviceBuilder = b.serviceBuilder.WithTenancies(tenancies...)
	return b
}

// Run executes both the internal resource service and the controller manager.
// The controller manager gets told it is the Raft leader so all controllers
// will get executed. The resource service, controller manager and all controllers
// will be stopped when the test finishes by registering a cleanup method on
// the test.
func (b *Builder) Run(t testutil.TestingTB) pbresource.ResourceServiceClient {
	t.Helper()

	ctx := testutil.TestContext(t)

	client := b.serviceBuilder.Run(t)

	mgr := controller.NewManager(client, testutil.Logger(t))
	for _, register := range b.controllerRegisterFns {
		register(mgr)
	}

	mgr.SetRaftLeader(true)
	go mgr.Run(ctx)

	// auto-clone messages going through the client so that test
	// code is free to modify objects in place without cloning themselves.
	return pbresource.NewCloningResourceServiceClient(client)
}
