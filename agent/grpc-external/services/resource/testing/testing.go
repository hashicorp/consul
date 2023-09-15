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

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	internal "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

func randomACLIdentity(t *testing.T) structs.ACLIdentity {
	id, err := uuid.GenerateUUID()
	require.NoError(t, err)

	return &structs.ACLToken{AccessorID: id}
}

func AuthorizerFrom(t *testing.T, policyStrs ...string) resolver.Result {
	policies := []*acl.Policy{}
	for _, policyStr := range policyStrs {
		policy, err := acl.NewPolicyFromSource(policyStr, nil, nil)
		require.NoError(t, err)
		policies = append(policies, policy)
	}

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), policies, nil)
	require.NoError(t, err)

	return resolver.Result{
		Authorizer:  authz,
		ACLIdentity: randomACLIdentity(t),
	}
}

// RunResourceService runs a Resource Service for the duration of the test and
// returns a client to interact with it. ACLs will be disabled and only the
// default partition and namespace are available.
func RunResourceService(t *testing.T, registerFns ...func(resource.Registry)) pbresource.ResourceServiceClient {
	// Provide a resolver which will default partition and namespace when not provided. This is similar to user
	// initiated requests.
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

	return RunResourceServiceWithACL(t, mockACLResolver, registerFns...)
}

func RunResourceServiceWithACL(t *testing.T, aclResolver svc.ACLResolver, registerFns ...func(resource.Registry)) pbresource.ResourceServiceClient {
	t.Helper()

	backend, err := inmem.NewBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go backend.Run(ctx)

	registry := resource.NewRegistry()
	for _, fn := range registerFns {
		fn(registry)
	}

	server := grpc.NewServer()

	mockTenancyBridge := &svc.MockTenancyBridge{}
	mockTenancyBridge.On("PartitionExists", resource.DefaultPartitionName).Return(true, nil)
	mockTenancyBridge.On("NamespaceExists", resource.DefaultPartitionName, resource.DefaultNamespaceName).Return(true, nil)
	mockTenancyBridge.On("IsPartitionMarkedForDeletion", resource.DefaultPartitionName).Return(false, nil)
	mockTenancyBridge.On("IsNamespaceMarkedForDeletion", resource.DefaultPartitionName, resource.DefaultNamespaceName).Return(false, nil)

	svc.NewServer(svc.Config{
		Backend:         backend,
		Registry:        registry,
		Logger:          testutil.Logger(t),
		ACLResolver:     aclResolver,
		V1TenancyBridge: mockTenancyBridge,
	}).Register(server)

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

	return pbresource.NewResourceServiceClient(conn)
}
