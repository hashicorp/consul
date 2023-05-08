package connectca

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	resolver "github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/connect"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
	"github.com/hashicorp/consul/sdk/testutil"
)

const testACLToken = "acl-token"

func TestWatchRoots_ConnectDisabled(t *testing.T) {
	server := NewServer(Config{ConnectEnabled: false})

	// Begin the stream.
	client := testClient(t, server)
	stream, err := client.WatchRoots(context.Background(), &pbconnectca.WatchRootsRequest{})
	require.NoError(t, err)
	rspCh := handleRootsStream(t, stream)

	err = mustGetError(t, rspCh)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
	require.Contains(t, status.Convert(err).Message(), "Connect")
}

func TestWatchRoots_Success(t *testing.T) {
	fsm, publisher := setupFSMAndPublisher(t)

	// Set the initial roots and CA configuration.
	rootA := connect.TestCA(t, nil)
	_, err := fsm.GetStore().CARootSetCAS(1, 0, structs.CARoots{rootA})
	require.NoError(t, err)

	err = fsm.GetStore().CASetConfig(0, &structs.CAConfiguration{ClusterID: "cluster-id"})
	require.NoError(t, err)

	// Mock the ACL Resolver to return an authorizer with `service:write`.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Publisher:      publisher,
		GetStore:       func() StateStore { return fsm.GetStore() },
		Logger:         testutil.Logger(t),
		ACLResolver:    aclResolver,
		ConnectEnabled: true,
	})

	// Begin the stream.
	client := testClient(t, server)
	stream, err := client.WatchRoots(ctx, &pbconnectca.WatchRootsRequest{})
	require.NoError(t, err)
	rspCh := handleRootsStream(t, stream)

	// Expect an initial message containing current roots (provided by the snapshot).
	roots := mustGetRoots(t, rspCh)
	require.Equal(t, "cluster-id.consul", roots.TrustDomain)
	require.Equal(t, rootA.ID, roots.ActiveRootId)
	require.Len(t, roots.Roots, 1)
	require.Equal(t, rootA.ID, roots.Roots[0].Id)

	// Rotate the roots.
	rootB := connect.TestCA(t, nil)
	_, err = fsm.GetStore().CARootSetCAS(2, 1, structs.CARoots{rootB})
	require.NoError(t, err)

	// Expect another event containing the new roots.
	roots = mustGetRoots(t, rspCh)
	require.Equal(t, "cluster-id.consul", roots.TrustDomain)
	require.Equal(t, rootB.ID, roots.ActiveRootId)
	require.Len(t, roots.Roots, 1)
	require.Equal(t, rootB.ID, roots.Roots[0].Id)
}

func TestWatchRoots_InvalidACLToken(t *testing.T) {
	fsm, publisher := setupFSMAndPublisher(t)

	// Set the initial CA configuration.
	err := fsm.GetStore().CASetConfig(0, &structs.CAConfiguration{ClusterID: "cluster-id"})
	require.NoError(t, err)

	// Mock the ACL resolver to return ErrNotFound.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Publisher:      publisher,
		GetStore:       func() StateStore { return fsm.GetStore() },
		Logger:         testutil.Logger(t),
		ACLResolver:    aclResolver,
		ConnectEnabled: true,
	})

	// Start the stream.
	client := testClient(t, server)
	stream, err := client.WatchRoots(ctx, &pbconnectca.WatchRootsRequest{})
	require.NoError(t, err)
	rspCh := handleRootsStream(t, stream)

	// Expect to get an Unauthenticated error immediately.
	err = mustGetError(t, rspCh)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
}

func TestWatchRoots_ACLTokenInvalidated(t *testing.T) {
	fsm, publisher := setupFSMAndPublisher(t)

	// Set the initial roots and CA configuration.
	rootA := connect.TestCA(t, nil)
	_, err := fsm.GetStore().CARootSetCAS(1, 0, structs.CARoots{rootA})
	require.NoError(t, err)

	err = fsm.GetStore().CASetConfig(2, &structs.CAConfiguration{ClusterID: "cluster-id"})
	require.NoError(t, err)

	// Mock the ACL Resolver to return an authorizer with `service:write` the
	// first two times it is called (initial connect and first re-auth).
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil).Twice()

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Publisher:      publisher,
		GetStore:       func() StateStore { return fsm.GetStore() },
		Logger:         testutil.Logger(t),
		ACLResolver:    aclResolver,
		ConnectEnabled: true,
	})

	// Start the stream.
	client := testClient(t, server)
	stream, err := client.WatchRoots(ctx, &pbconnectca.WatchRootsRequest{})
	require.NoError(t, err)
	rspCh := handleRootsStream(t, stream)

	// Consume the initial response.
	mustGetRoots(t, rspCh)

	// Update the ACL token to cause the subscription to be force-closed.
	accessorID, err := uuid.GenerateUUID()
	require.NoError(t, err)
	err = fsm.GetStore().ACLTokenSet(1, &structs.ACLToken{
		AccessorID: accessorID,
		SecretID:   testACLToken,
	})
	require.NoError(t, err)

	// Update the roots.
	rootB := connect.TestCA(t, nil)
	_, err = fsm.GetStore().CARootSetCAS(3, 1, structs.CARoots{rootB})
	require.NoError(t, err)

	// Expect the stream to remain open and to receive the new roots.
	mustGetRoots(t, rspCh)

	// Simulate deleting the ACL token.
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)

	// Update the ACL token to cause the subscription to be force-closed.
	err = fsm.GetStore().ACLTokenSet(1, &structs.ACLToken{
		AccessorID: accessorID,
		SecretID:   testACLToken,
	})
	require.NoError(t, err)

	// Expect the stream to be terminated.
	err = mustGetError(t, rspCh)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
}

func TestWatchRoots_StateStoreAbandoned(t *testing.T) {
	fsm, publisher := setupFSMAndPublisher(t)

	// Set the initial roots and CA configuration.
	rootA := connect.TestCA(t, nil)
	_, err := fsm.GetStore().CARootSetCAS(1, 0, structs.CARoots{rootA})
	require.NoError(t, err)

	err = fsm.GetStore().CASetConfig(0, &structs.CAConfiguration{ClusterID: "cluster-a"})
	require.NoError(t, err)

	// Mock the ACL Resolver to return an authorizer with `service:write`.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Publisher:      publisher,
		GetStore:       func() StateStore { return fsm.GetStore() },
		Logger:         testutil.Logger(t),
		ACLResolver:    aclResolver,
		ConnectEnabled: true,
	})

	// Begin the stream.
	client := testClient(t, server)
	stream, err := client.WatchRoots(ctx, &pbconnectca.WatchRootsRequest{})
	require.NoError(t, err)
	rspCh := handleRootsStream(t, stream)

	// Consume the initial roots.
	mustGetRoots(t, rspCh)

	// Simulate a snapshot restore.
	storeB := testutils.TestStateStore(t, publisher)

	rootB := connect.TestCA(t, nil)
	_, err = storeB.CARootSetCAS(1, 0, structs.CARoots{rootB})
	require.NoError(t, err)

	err = storeB.CASetConfig(0, &structs.CAConfiguration{ClusterID: "cluster-b"})
	require.NoError(t, err)

	fsm.ReplaceStore(storeB)

	// Expect to get the new store's roots.
	newRoots := mustGetRoots(t, rspCh)
	require.Equal(t, "cluster-b.consul", newRoots.TrustDomain)
	require.Len(t, newRoots.Roots, 1)
	require.Equal(t, rootB.ID, newRoots.ActiveRootId)
}

func mustGetRoots(t *testing.T, ch <-chan rootsOrError) *pbconnectca.WatchRootsResponse {
	t.Helper()

	select {
	case rsp := <-ch:
		require.NoError(t, rsp.err)
		return rsp.rsp
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for WatchRootsResponse")
		return nil
	}
}

func mustGetError(t *testing.T, ch <-chan rootsOrError) error {
	t.Helper()

	select {
	case rsp := <-ch:
		require.Error(t, rsp.err)
		return rsp.err
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for WatchRootsResponse")
		return nil
	}
}

func handleRootsStream(t *testing.T, stream pbconnectca.ConnectCAService_WatchRootsClient) <-chan rootsOrError {
	t.Helper()

	rspCh := make(chan rootsOrError)
	go func() {
		for {
			rsp, err := stream.Recv()
			if errors.Is(err, io.EOF) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return
			}
			rspCh <- rootsOrError{
				rsp: rsp,
				err: err,
			}
		}
	}()
	return rspCh
}

type rootsOrError struct {
	rsp *pbconnectca.WatchRootsResponse
	err error
}
