// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
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

// Deprecated: use NewResourceServiceBuilder instead
func testServer(t *testing.T) *svc.Server {
	t.Helper()

	backend, err := inmem.NewBackend()
	require.NoError(t, err)
	go backend.Run(testContext(t))

	// Mock the ACL Resolver to "allow all" for testing.
	mockACLResolver := &svc.MockACLResolver{}
	mockACLResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil).
		Run(func(args mock.Arguments) {
			// Caller expecting passed in tokenEntMeta and authorizerContext to be filled in.
			tokenEntMeta := args.Get(1).(*acl.EnterpriseMeta)
			if tokenEntMeta != nil {
				fillEntMeta(tokenEntMeta)
			}

			authzContext := args.Get(2).(*acl.AuthorizerContext)
			if authzContext != nil {
				fillAuthorizerContext(authzContext)
			}
		})

	// Mock the tenancy bridge since we can't use the real thing.
	mockTenancyBridge := &svc.MockTenancyBridge{}
	mockTenancyBridge.On("PartitionExists", resource.DefaultPartitionName).Return(true, nil)
	mockTenancyBridge.On("NamespaceExists", resource.DefaultPartitionName, resource.DefaultNamespaceName).Return(true, nil)
	mockTenancyBridge.On("PartitionExists", mock.Anything).Return(false, nil)
	mockTenancyBridge.On("NamespaceExists", mock.Anything, mock.Anything).Return(false, nil)
	mockTenancyBridge.On("IsPartitionMarkedForDeletion", resource.DefaultPartitionName).Return(false, nil)
	mockTenancyBridge.On("IsNamespaceMarkedForDeletion", resource.DefaultPartitionName, resource.DefaultNamespaceName).Return(false, nil)

	return svc.NewServer(svc.Config{
		Logger:        testutil.Logger(t),
		Registry:      resource.NewRegistry(),
		Backend:       backend,
		ACLResolver:   mockACLResolver,
		TenancyBridge: mockTenancyBridge,
	})
}

// Deprecated: use NewResourceServiceBuilder instead
func testClient(t *testing.T, server *svc.Server) pbresource.ResourceServiceClient {
	t.Helper()

	addr := testutils.RunTestServer(t, server)

	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbresource.NewResourceServiceClient(conn)
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func modifyArtist(t *testing.T, res *pbresource.Resource) *pbresource.Resource {
	t.Helper()

	var artist pbdemov2.Artist
	require.NoError(t, res.Data.UnmarshalTo(&artist))
	artist.Name = fmt.Sprintf("The artist formerly known as %s", artist.Name)

	data, err := anypb.New(&artist)
	require.NoError(t, err)

	res = clone(res)
	res.Data = data
	return res
}

// wildcardTenancyCases returns permutations of tenancy and type scope used as input
// to endpoints that accept wildcards for tenancy.
func wildcardTenancyCases() map[string]struct {
	typ     *pbresource.Type
	tenancy *pbresource.Tenancy
} {
	return map[string]struct {
		typ     *pbresource.Type
		tenancy *pbresource.Tenancy
	}{
		"namespaced type with empty partition": {
			typ: demo.TypeV2Artist,
			tenancy: &pbresource.Tenancy{
				Partition: "",
				Namespace: resource.DefaultNamespaceName,
			},
		},
		"namespaced type with empty namespace": {
			typ: demo.TypeV2Artist,
			tenancy: &pbresource.Tenancy{
				Partition: resource.DefaultPartitionName,
				Namespace: "",
			},
		},
		"namespaced type with empty partition and namespace": {
			typ: demo.TypeV2Artist,
			tenancy: &pbresource.Tenancy{
				Partition: "",
				Namespace: "",
			},
		},
		"namespaced type with wildcard partition and empty namespace": {
			typ: demo.TypeV2Artist,
			tenancy: &pbresource.Tenancy{
				Partition: "*",
				Namespace: "",
			},
		},
		"namespaced type with empty partition and wildcard namespace": {
			typ: demo.TypeV2Artist,
			tenancy: &pbresource.Tenancy{
				Partition: "",
				Namespace: "*",
			},
		},
		"partitioned type with empty partition": {
			typ: demo.TypeV1RecordLabel,
			tenancy: &pbresource.Tenancy{
				Partition: "",
				Namespace: "",
			},
		},
		"partitioned type with wildcard partition": {
			typ: demo.TypeV1RecordLabel,
			tenancy: &pbresource.Tenancy{
				Partition: "*",
			},
		},
		"partitioned type with wildcard partition and namespace": {
			typ: demo.TypeV1RecordLabel,
			tenancy: &pbresource.Tenancy{
				Partition: "*",
				Namespace: "*",
			},
		},
		"cluster type with empty partition and namespace": {
			typ: demo.TypeV1Executive,
			tenancy: &pbresource.Tenancy{
				Partition: "",
				Namespace: "",
			},
		},

		"cluster type with wildcard partition and namespace": {
			typ: demo.TypeV1Executive,
			tenancy: &pbresource.Tenancy{
				Partition: "*",
				Namespace: "*",
			},
		},
	}
}

// tenancyCases returns permutations of valid tenancy structs in a resource id to use as inputs.
// - the id is for a recordLabel when the resource is partition scoped
// - the id is for an artist when the resource is namespace scoped
func tenancyCases() map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID {
	tenancyCases := map[string]func(artistId, recordlabelId *pbresource.ID) *pbresource.ID{
		"namespaced resource provides nonempty partition and namespace": func(artistId, recordLabelId *pbresource.ID) *pbresource.ID {
			return artistId
		},
		"namespaced resource inherits tokens partition when empty": func(artistId, _ *pbresource.ID) *pbresource.ID {
			id := clone(artistId)
			id.Tenancy.Partition = ""
			return id
		},
		"namespaced resource inherits tokens namespace when empty": func(artistId, _ *pbresource.ID) *pbresource.ID {
			id := clone(artistId)
			id.Tenancy.Namespace = ""
			return id
		},
		"namespaced resource inherits tokens partition and namespace when empty": func(artistId, _ *pbresource.ID) *pbresource.ID {
			id := clone(artistId)
			id.Tenancy.Partition = ""
			id.Tenancy.Namespace = ""
			return id
		},
		"namespaced resource inherits tokens partition and namespace when tenancy nil": func(artistId, _ *pbresource.ID) *pbresource.ID {
			id := clone(artistId)
			id.Tenancy = nil
			return id
		},
		"partitioned resource provides nonempty partition": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
			return recordLabelId
		},
		"partitioned resource inherits tokens partition when empty": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
			id := clone(recordLabelId)
			id.Tenancy.Partition = ""
			return id
		},
		"partitioned resource inherits tokens partition when tenancy nil": func(_, recordLabelId *pbresource.ID) *pbresource.ID {
			id := clone(recordLabelId)
			id.Tenancy = nil
			return id
		},
	}
	return tenancyCases
}

type blockOnceBackend struct {
	storage.Backend

	done            uint32
	readCompletedCh chan struct{}
	blockCh         chan struct{}
}

func (b *blockOnceBackend) Read(ctx context.Context, consistency storage.ReadConsistency, id *pbresource.ID) (*pbresource.Resource, error) {
	res, err := b.Backend.Read(ctx, consistency, id)

	// Block for exactly one call to Read. All subsequent calls (including those
	// concurrent to the blocked call) will return immediately.
	if atomic.CompareAndSwapUint32(&b.done, 0, 1) {
		close(b.readCompletedCh)
		<-b.blockCh
	}

	return res, err
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
