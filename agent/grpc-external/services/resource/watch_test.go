package resource

import (
	context "context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage/inmem"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestWatchList_TypeNotFound(t *testing.T) {
	server := NewServer(Config{registry: resource.NewRegistry()})
	client := testClient(t, server)

	stream, err := client.WatchList(context.Background(), &pbresource.WatchListRequest{
		Type:       typev1,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	err = mustGetError(t, rspCh)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type mesh/v1/service not registered")
}

func TestWatchList_GroupVersionMatches(t *testing.T) {
	ctx := testContext(t)
	registry := resource.NewRegistry()
	registry.Register(resource.Registration{Type: typev1})
	backend, err := inmem.NewBackend()
	require.NoError(t, err)
	go backend.Run(ctx)
	server := NewServer(Config{registry: registry, backend: backend})
	client := testClient(t, server)

	// create a watch
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       typev1,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	// insert and verify upsert event received
	r1, err := backend.WriteCAS(ctx, resourcev1)
	require.NoError(t, err)
	rsp := mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchListResponse_OPERATION_UPSERT, rsp.Operation)
	prototest.AssertDeepEqual(t, r1, rsp.Resource)

	// update and verify upsert event received
	r2 := clone(r1)
	r2, err = backend.WriteCAS(ctx, r2)
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchListResponse_OPERATION_UPSERT, rsp.Operation)
	prototest.AssertDeepEqual(t, r2, rsp.Resource)

	// delete and verify delete event received
	err = backend.DeleteCAS(ctx, r2.Id, r2.Version)
	require.NoError(t, err)
	rsp = mustGetResource(t, rspCh)
	require.Equal(t, pbresource.WatchListResponse_OPERATION_DELETE, rsp.Operation)
}

func TestWatchList_GroupVersionMismatch(t *testing.T) {
	// Given a watch on typev2 that only differs from typev1 by GroupVersion
	// When a resource of typev1 is created/updated/deleted
	// Then no watch events should be emitted
	registry := resource.NewRegistry()
	registry.Register(resource.Registration{Type: typev1})
	registry.Register(resource.Registration{Type: typev2})
	backend, err := inmem.NewBackend()
	require.NoError(t, err)
	ctx := testContext(t)
	go backend.Run(ctx)
	server := NewServer(Config{registry: registry, backend: backend})
	client := testClient(t, server)

	// create a watch for typev2
	stream, err := client.WatchList(ctx, &pbresource.WatchListRequest{
		Type:       typev2,
		Tenancy:    tenancy,
		NamePrefix: "",
	})
	require.NoError(t, err)
	rspCh := handleResourceStream(t, stream)

	// insert
	r1, err := backend.WriteCAS(ctx, resourcev1)
	require.NoError(t, err)

	// update
	r2 := clone(r1)
	r2, err = backend.WriteCAS(ctx, r2)
	require.NoError(t, err)

	// delete
	err = backend.DeleteCAS(ctx, r2.Id, r2.Version)
	require.NoError(t, err)

	// verify no events received
	mustGetNoResource(t, rspCh)
}

func mustGetNoResource(t *testing.T, ch <-chan resourceOrError) {
	t.Helper()

	select {
	case rsp := <-ch:
		require.NoError(t, rsp.err)
		require.Nil(t, rsp.rsp, "expected nil response with no error")
	case <-time.After(1 * time.Second):
		// Dan: Any way to do this without the annoying delay?
		return
	}
}

func mustGetResource(t *testing.T, ch <-chan resourceOrError) *pbresource.WatchListResponse {
	t.Helper()

	select {
	case rsp := <-ch:
		require.NoError(t, rsp.err)
		return rsp.rsp
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for WatchListResponse")
		return nil
	}
}

func mustGetError(t *testing.T, ch <-chan resourceOrError) error {
	t.Helper()

	select {
	case rsp := <-ch:
		require.Error(t, rsp.err)
		return rsp.err
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for WatchListResponse")
		return nil
	}
}

func handleResourceStream(t *testing.T, stream pbresource.ResourceService_WatchListClient) <-chan resourceOrError {
	t.Helper()

	rspCh := make(chan resourceOrError)
	go func() {
		for {
			rsp, err := stream.Recv()
			if errors.Is(err, io.EOF) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return
			}
			rspCh <- resourceOrError{
				rsp: rsp,
				err: err,
			}
		}
	}()
	return rspCh
}

var (
	tenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}
	typev1 = &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v1",
		Kind:         "service",
	}
	typev2 = &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v2",
		Kind:         "service",
	}
	resourcev1 = &pbresource.Resource{
		Id: &pbresource.ID{
			Uid:     "someUid",
			Name:    "someName",
			Type:    typev1,
			Tenancy: tenancy,
		},
		Version: "",
	}
)

type resourceOrError struct {
	rsp *pbresource.WatchListResponse
	err error
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
