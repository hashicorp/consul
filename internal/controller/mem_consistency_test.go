// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

var (
	fakeType = &pbresource.Type{
		Group:        "testing",
		GroupVersion: "v1",
		Kind:         "Fake",
	}

	fakeV2Type = &pbresource.Type{
		Group:        "testing",
		GroupVersion: "v2",
		Kind:         "Fake",
	}
)

type memCheckResult struct {
	clientGet      *pbresource.Resource
	clientGetError error
	cacheGet       *pbresource.Resource
	cacheGetError  error
}

type memCheckReconciler struct {
	mu          sync.Mutex
	closed      bool
	reconcileCh chan memCheckResult
	mapCh       chan memCheckResult
}

func newMemCheckReconciler(t testutil.TestingTB) *memCheckReconciler {
	t.Helper()

	r := &memCheckReconciler{
		reconcileCh: make(chan memCheckResult, 10),
		mapCh:       make(chan memCheckResult, 10),
	}

	t.Cleanup(r.Shutdown)
	return r
}

func (r *memCheckReconciler) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	close(r.reconcileCh)
	close(r.mapCh)
}

func (r *memCheckReconciler) requireNotClosed(t testutil.TestingTB) {
	t.Helper()
	if r.closed {
		require.FailNow(t, "the memCheckReconciler has been closed")
	}
}

func (r *memCheckReconciler) checkReconcileResult(t testutil.TestingTB, ctx context.Context, res *pbresource.Resource) {
	t.Helper()
	r.requireEqualNotSameMemCheckResult(t, ctx, r.reconcileCh, res)
}

func (r *memCheckReconciler) checkMapResult(t testutil.TestingTB, ctx context.Context, res *pbresource.Resource) {
	t.Helper()
	r.requireEqualNotSameMemCheckResult(t, ctx, r.mapCh, res)
}

func (r *memCheckReconciler) requireEqualNotSameMemCheckResult(t testutil.TestingTB, ctx context.Context, ch <-chan memCheckResult, res *pbresource.Resource) {
	t.Helper()

	select {
	case result := <-ch:
		require.NoError(t, result.clientGetError)
		require.NoError(t, result.cacheGetError)
		// Equal but NotSame means the values are all the same but
		// the pointers are different. Note that this probably doesn't
		// check that the values within the resource haven't been shallow
		// copied but that probably should be checked elsewhere
		prototest.AssertDeepEqual(t, res, result.clientGet)
		require.NotSame(t, res, result.clientGet)
		prototest.AssertDeepEqual(t, res, result.cacheGet)
		require.NotSame(t, res, result.cacheGet)
	case <-ctx.Done():
		require.Fail(t, "didn't receive mem check result before context cancellation", ctx.Err())
	}
}

func (r *memCheckReconciler) Reconcile(ctx context.Context, rt Runtime, req Request) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.getAndSend(ctx, rt, req.ID, r.reconcileCh)
	}
	return nil
}

func (r *memCheckReconciler) MapToNothing(
	ctx context.Context,
	rt Runtime,
	res *pbresource.Resource,
) ([]Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.getAndSend(ctx, rt, res.Id, r.mapCh)
	}
	return nil, nil
}

func (*memCheckReconciler) getAndSend(ctx context.Context, rt Runtime, id *pbresource.ID, ch chan<- memCheckResult) {
	var res memCheckResult
	response, err := rt.Client.Read(ctx, &pbresource.ReadRequest{
		Id: id,
	})
	res.clientGetError = err
	if response != nil {
		res.clientGet = response.Resource
	}

	res.cacheGet, res.cacheGetError = rt.Cache.Get(id.Type, "id", id)

	ch <- res
}

func watchListEvents(t testutil.TestingTB, events ...*pbresource.WatchEvent) pbresource.ResourceService_WatchListClient {
	t.Helper()
	ctx := testutil.TestContext(t)

	watchListClient := mockpbresource.NewResourceService_WatchListClient(t)

	// Return the events in the specified order as soon as they are requested
	for _, event := range events {
		evt := event
		watchListClient.EXPECT().
			Recv().
			RunAndReturn(func() (*pbresource.WatchEvent, error) {
				return evt, nil
			}).
			Once()
	}

	watchListClient.EXPECT().
		Recv().
		RunAndReturn(func() (*pbresource.WatchEvent, error) {
			return &pbresource.WatchEvent{
				Event: &pbresource.WatchEvent_EndOfSnapshot_{
					EndOfSnapshot: &pbresource.WatchEvent_EndOfSnapshot{},
				},
			}, nil
		}).
		Once()

	// Now that all specified events have been exhausted we loop until the test finishes
	// and the context bound to the tests lifecycle has been cancelled. This prevents getting
	// any weird errors from the controller manager/runner.
	watchListClient.EXPECT().
		Recv().
		RunAndReturn(func() (*pbresource.WatchEvent, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}).
		Maybe()

	return watchListClient
}

// TestControllerRuntimeMemoryCloning mainly is testing that the runtimes
// provided to reconcilers and dependency mappers will return data from
// the resource service client and the cache that have been cloned so that
// the controller should be free to modify the data as needed.
func TestControllerRuntimeMemoryCloning(t *testing.T) {
	ctx := testutil.TestContext(t)

	// create some resources to use during the test
	res1 := resourcetest.Resource(fakeType, "foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	res2 := resourcetest.Resource(fakeV2Type, "bar").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	// create the reconciler that will read the desired resource
	// from both the resource service client and the cache client.
	reconciler := newMemCheckReconciler(t)

	// Create the v1 watch list client to be returned when the controller runner
	// calls WatchList on the v1 testing type.
	v1WatchListClientCreate := func() pbresource.ResourceService_WatchListClient {
		return watchListEvents(t, &pbresource.WatchEvent{
			Event: &pbresource.WatchEvent_Upsert_{
				Upsert: &pbresource.WatchEvent_Upsert{
					Resource: res1,
				},
			},
		})
	}

	// Create the v2 watch list client to be returned when the controller runner
	// calls WatchList on the v2 testing type.
	v2WatchListClientCreate := func() pbresource.ResourceService_WatchListClient {
		return watchListEvents(t, nil, &pbresource.WatchEvent{
			Event: &pbresource.WatchEvent_Upsert_{
				Upsert: &pbresource.WatchEvent_Upsert{
					Resource: res2,
				},
			},
		})
	}

	// Create the mock resource service client
	mres := mockpbresource.NewResourceServiceClient(t)
	// Setup the expectation for the controller runner to issue a WatchList
	// request for the managed type (fake v2 type)
	mres.EXPECT().
		WatchList(mock.Anything, &pbresource.WatchListRequest{
			Type: fakeV2Type,
			Tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				Namespace: storage.Wildcard,
			},
		}).
		RunAndReturn(func(_ context.Context, _ *pbresource.WatchListRequest, _ ...grpc.CallOption) (pbresource.ResourceService_WatchListClient, error) {
			return v2WatchListClientCreate(), nil
		}).
		Twice() // once for cache prime, once for the rest

	// Setup the expectation for the controller runner to issue a WatchList
	// request for the secondary Watch type (fake v1 type)
	mres.EXPECT().
		WatchList(mock.Anything, &pbresource.WatchListRequest{
			Type: fakeType,
			Tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				Namespace: storage.Wildcard,
			},
		}).
		RunAndReturn(func(_ context.Context, _ *pbresource.WatchListRequest, _ ...grpc.CallOption) (pbresource.ResourceService_WatchListClient, error) {
			return v1WatchListClientCreate(), nil
		}).
		Twice() // once for cache prime, once for the rest

	// The cloning resource clients will forward actual calls onto the main resource service client.
	// Here we are configuring the service mock to return either of the resources depending on the
	// id present in the request.
	mres.EXPECT().
		Read(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, req *pbresource.ReadRequest, opts ...grpc.CallOption) (*pbresource.ReadResponse, error) {
			res := res2
			if resource.EqualID(res1.Id, req.Id) {
				res = res1
			}
			return &pbresource.ReadResponse{Resource: res}, nil
		}).
		Times(0)

	// create the test controller
	ctl := NewController("test", fakeV2Type).
		WithWatch(fakeType, reconciler.MapToNothing).
		WithReconciler(reconciler)

	// create the controller manager and register our test controller
	manager := NewManager(mres, testutil.Logger(t))
	manager.Register(ctl)

	// run the controller manager
	manager.SetRaftLeader(true)
	go manager.Run(ctx)

	// All future assertions should easily be able to run within 5s although they
	// should typically run a couple orders of magnitude faster.
	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	t.Cleanup(cancel)

	// validate that the v2 resource type event was seen and that the
	// cache and the resource service client return cloned resources
	reconciler.checkReconcileResult(t, timeLimitedCtx, res2)

	// Validate that the dependency mapper's resource and cache clients return
	// cloned resources.
	reconciler.checkMapResult(t, timeLimitedCtx, res1)
}

// TestRunnerSharedMemoryCache is mainly testing to ensure that resources
// within the cache are shared with the resource service and have not been
// cloned.
func TestControllerRunnerSharedMemoryCache(t *testing.T) {
	ctx := testutil.TestContext(t)

	// create resource to use during the test
	res := resourcetest.Resource(fakeV2Type, "bar").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	// create the reconciler that will read the desired resource
	// from both the resource service client and the cache client.
	reconciler := newMemCheckReconciler(t)

	// Create the v2 watch list client to be returned when the controller runner
	// calls WatchList on the v2 testing type.
	v2WatchListClientCreate := func() pbresource.ResourceService_WatchListClient {
		return watchListEvents(t, nil, &pbresource.WatchEvent{
			Event: &pbresource.WatchEvent_Upsert_{
				Upsert: &pbresource.WatchEvent_Upsert{
					Resource: res,
				},
			},
		})
	}

	// Create the mock resource service client
	mres := mockpbresource.NewResourceServiceClient(t)
	// Setup the expectation for the controller runner to issue a WatchList
	// request for the managed type (fake v2 type)
	mres.EXPECT().
		WatchList(mock.Anything, &pbresource.WatchListRequest{
			Type: fakeV2Type,
			Tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				Namespace: storage.Wildcard,
			},
		}).
		RunAndReturn(func(_ context.Context, _ *pbresource.WatchListRequest, _ ...grpc.CallOption) (pbresource.ResourceService_WatchListClient, error) {
			return v2WatchListClientCreate(), nil
		}).
		Twice() // once for cache prime, once for the rest

	// The cloning resource clients will forward actual calls onto the main resource service client.
	// Here we are configuring the service mock to return our singular resource always.
	mres.EXPECT().
		Read(mock.Anything, mock.Anything).
		Return(&pbresource.ReadResponse{Resource: res}, nil).
		Times(0)

	// create the test controller
	ctl := NewController("test", fakeV2Type).
		WithReconciler(reconciler)

	runner := newControllerRunner(ctl, mres, testutil.Logger(t))
	go runner.run(ctx)

	// Wait for reconcile to be called before we check the values in
	// the cache. This will also validate that the resource service client
	// and cache client given to the reconciler cloned the resource but
	// that is tested more thoroughly in another test and isn't of primary
	// concern here.
	reconciler.checkReconcileResult(t, ctx, res)

	// Now validate that the cache hold the same resource pointer as the original data
	actual, err := runner.cache.Get(fakeV2Type, "id", res.Id)
	require.NoError(t, err)
	require.Same(t, res, actual)
}
