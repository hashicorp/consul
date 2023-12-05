// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/acl"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	internal "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/internal/tenancy"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type builder struct {
	registry     resource.Registry
	registerFns  []func(resource.Registry)
	useV2Tenancy bool
	tenancies    []*pbresource.Tenancy
	aclResolver  svc.ACLResolver
	serviceImpl  *svc.Server
}

// NewResourceServiceBuilder is the preferred way to configure and run
// an isolated in-process instance of the resource service for unit
// testing. The final call to `Run()` returns a client you can use for
// making requests.
func NewResourceServiceBuilder() *builder {
	b := &builder{
		useV2Tenancy: false,
		registry:     resource.NewRegistry(),
		// Regardless of whether using mock of v2tenancy, always make sure
		// the builtin tenancy exists.
		tenancies: []*pbresource.Tenancy{resource.DefaultNamespacedTenancy()},
	}
	return b
}

// WithV2Tenancy configures which tenancy bridge is used.
//
// true  => real v2 default partition and namespace via v2 tenancy bridge
// false => mock default partition and namespace since v1 tenancy bridge can't be used (not spinning up an entire server here)
func (b *builder) WithV2Tenancy(useV2Tenancy bool) *builder {
	b.useV2Tenancy = useV2Tenancy
	return b
}

// Registry provides access to the constructed registry post-Run() when
// needed by other test dependencies.
func (b *builder) Registry() resource.Registry {
	return b.registry
}

// ServiceImpl provides access to the actual server side implemenation of the resource service. This should never be used
// used/accessed without good reason. The current justifying use case is to monkeypatch the ACL resolver post-creation
// to allow unfettered writes which some ACL related tests require to put test data in place.
func (b *builder) ServiceImpl() *svc.Server {
	return b.serviceImpl
}

func (b *builder) WithRegisterFns(registerFns ...func(resource.Registry)) *builder {
	for _, registerFn := range registerFns {
		b.registerFns = append(b.registerFns, registerFn)
	}
	return b
}

func (b *builder) WithACLResolver(aclResolver svc.ACLResolver) *builder {
	b.aclResolver = aclResolver
	return b
}

// WithTenancies adds additional partitions and namespaces if default/default
// is not sufficient.
func (b *builder) WithTenancies(tenancies ...*pbresource.Tenancy) *builder {
	for _, tenancy := range tenancies {
		b.tenancies = append(b.tenancies, tenancy)
	}
	return b
}

// Run starts the resource service and returns a client.
func (b *builder) Run(t *testing.T) pbresource.ResourceServiceClient {
	// backend cannot be customized
	backend, err := inmem.NewBackend()
	require.NoError(t, err)

	// start the backend and add teardown hook
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go backend.Run(ctx)

	// Automatically add tenancy types if v2 tenancy enabled
	if b.useV2Tenancy {
		b.registerFns = append(b.registerFns, tenancy.RegisterTypes)
	}

	for _, registerFn := range b.registerFns {
		registerFn(b.registry)
	}

	var tenancyBridge resource.TenancyBridge
	if !b.useV2Tenancy {
		// use mock tenancy bridge. default/default has already been added out of the box
		mockTenancyBridge := &svc.MockTenancyBridge{}

		for _, tenancy := range b.tenancies {
			mockTenancyBridge.On("PartitionExists", tenancy.Partition).Return(true, nil)
			mockTenancyBridge.On("NamespaceExists", tenancy.Partition, tenancy.Namespace).Return(true, nil)
			mockTenancyBridge.On("IsPartitionMarkedForDeletion", tenancy.Partition).Return(false, nil)
			mockTenancyBridge.On("IsNamespaceMarkedForDeletion", tenancy.Partition, tenancy.Namespace).Return(false, nil)
		}

		tenancyBridge = mockTenancyBridge
	} else {
		// use v2 tenancy bridge. population comes later after client injected.
		tenancyBridge = tenancy.NewV2TenancyBridge()
	}

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

	config := svc.Config{
		Logger:        testutil.Logger(t),
		Registry:      b.registry,
		Backend:       backend,
		ACLResolver:   b.aclResolver,
		TenancyBridge: tenancyBridge,
		UseV2Tenancy:  b.useV2Tenancy,
	}

	server := grpc.NewServer()

	b.serviceImpl = svc.NewServer(config)
	b.serviceImpl.Register(server)

	pipe := internal.NewPipeListener()
	go server.Serve(pipe)
	t.Cleanup(server.Stop)

	conn, err := grpc.Dial("",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(pipe.DialContext),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := pbresource.NewResourceServiceClient(conn)

	// HACK ALERT: The client needs to be injected into the V2TenancyBridge
	// after it has been created due the the circular dependency. This will
	// go away when the tenancy bridge is removed and V1 is no more, however
	// long that takes.
	switch config.TenancyBridge.(type) {
	case *tenancy.V2TenancyBridge:
		config.TenancyBridge.(*tenancy.V2TenancyBridge).WithClient(client)
		// Default partition namespace can finally be created
		require.NoError(t, initTenancy(ctx, backend))

		for _, tenancy := range b.tenancies {
			if tenancy.Partition == resource.DefaultPartitionName && tenancy.Namespace == resource.DefaultNamespaceName {
				continue
			}
			t.Fatalf("TODO: implement creation of passed in v2 tenancy: %v", tenancy)
		}
	}
	return client
}
