// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testing

import (
	"context"

	"github.com/fullstorydev/grpchan/inprocgrpc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

// NewResourceServiceBuilder is the preferred way to configure and run
// an isolated in-process instance of the resource service for unit
// testing. The final call to `Run()` returns a client you can use for
// making requests.
func NewResourceServiceBuilder() *Builder {
	b := &Builder{
		registry: resource.NewRegistry(),
		// Always make sure the builtin tenancy exists.
		tenancies: []*pbresource.Tenancy{resource.DefaultNamespacedTenancy()},
		cloning:   true,
	}
	return b
}

// Registry provides access to the constructed registry post-Run() when
// needed by other test dependencies.
func (b *Builder) Registry() resource.Registry {
	return b.registry
}

// ServiceImpl provides access to the actual server side implementation of the resource service. This should never be
// used/accessed without good reason. The current justifying use case is to monkeypatch the ACL resolver post-creation
// to allow unfettered writes which some ACL related tests require to put test data in place.
func (b *Builder) ServiceImpl() *svc.Server {
	return b.serviceImpl
}

func (b *Builder) WithRegisterFns(registerFns ...func(resource.Registry)) *Builder {
	for _, registerFn := range registerFns {
		b.registerFns = append(b.registerFns, registerFn)
	}
	return b
}

func (b *Builder) WithACLResolver(aclResolver svc.ACLResolver) *Builder {
	b.aclResolver = aclResolver
	return b
}

// WithTenancies adds additional partitions and namespaces if default/default
// is not sufficient.
func (b *Builder) WithTenancies(tenancies ...*pbresource.Tenancy) *Builder {
	for _, tenancy := range tenancies {
		b.tenancies = append(b.tenancies, tenancy)
	}
	return b
}

// WithCloningDisabled disables resource service client functionality that will
// clone protobuf message types as they pass through. By default
// cloning is enabled.
//
// For in-process gRPC interactions we prefer to use an in-memory gRPC client. This
// allows our controller infrastructure to avoid any unnecessary protobuf serialization
// and deserialization and for controller caching to not duplicate memory that the
// resource service is already holding on to. However, clients (including controllers)
// often want to be able to perform read-modify-write ops and for the sake of not
// forcing all call sites to be aware of the shared memory and to not touch it we
// enable cloning in the clients that we give to those bits of code.
func (b *Builder) WithCloningDisabled() *Builder {
	b.cloning = false
	return b
}

// Run starts the resource service and returns a client.
func (b *Builder) Run(t testutil.TestingTB) pbresource.ResourceServiceClient {
	// backend cannot be customized
	backend, err := inmem.NewBackend()
	require.NoError(t, err)

	// start the backend and add teardown hook
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go backend.Run(ctx)

	for _, registerFn := range b.registerFns {
		registerFn(b.registry)
	}

	// use mock tenancy bridge. default/default has already been added out of the box
	mockTenancyBridge := &svc.MockTenancyBridge{}

	for _, tenancy := range b.tenancies {
		mockTenancyBridge.On("PartitionExists", tenancy.Partition).Return(true, nil)
		mockTenancyBridge.On("NamespaceExists", tenancy.Partition, tenancy.Namespace).Return(true, nil)
		mockTenancyBridge.On("IsPartitionMarkedForDeletion", tenancy.Partition).Return(false, nil)
		mockTenancyBridge.On("IsNamespaceMarkedForDeletion", tenancy.Partition, tenancy.Namespace).Return(false, nil)
	}

	tenancyBridge := mockTenancyBridge

	if b.aclResolver == nil {
		// When not provided (regardless of V1 tenancy or V2 tenancy), configure an ACL resolver
		// that has ACLs disabled and fills in "default" for the partition and namespace when
		// not provided. This is similar to user initiated requests.
		//
		// Controllers under test should be providing full tenancy since they will run with the DANGER_NO_AUTH.
		mockACLResolver := &svc.MockACLResolver{}
		mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
			Return(testutils.ACLsDisabled(t), nil).
			Run(func(args mock.Arguments) {
				// Caller expecting passed in tokenEntMeta and authorizerContext to be filled in.
				tokenEntMeta := args.Get(1).(*acl.EnterpriseMeta)
				if tokenEntMeta != nil {
					FillEntMeta(tokenEntMeta)
				}

				authzContext := args.Get(2).(*acl.AuthorizerContext)
				if authzContext != nil {
					FillAuthorizerContext(authzContext)
				}
			})
		b.aclResolver = mockACLResolver
	}

	// ent only
	b.ensureLicenseManager()

	config := b.newConfig(testutil.Logger(t), backend, tenancyBridge)

	b.serviceImpl = svc.NewServer(*config)
	ch := &inprocgrpc.Channel{}
	pbresource.RegisterResourceServiceServer(ch, b.serviceImpl)
	client := pbresource.NewResourceServiceClient(ch)

	if b.cloning {
		// enable protobuf cloning wrapper
		client = pbresource.NewCloningResourceServiceClient(client)
	}

	return client
}
