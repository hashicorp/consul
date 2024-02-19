// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	controller "github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

var injectedError = errors.New("injected error")

func errQuery(_ cache.ReadOnlyCache, _ ...any) (cache.ResourceIterator, error) {
	return nil, injectedError
}

func TestController_API(t *testing.T) {
	t.Parallel()

	idx := indexers.DecodedSingleIndexer("genre", index.SingleValueFromArgs(func(value string) ([]byte, error) {
		var b index.Builder
		b.String(value)
		return b.Bytes(), nil
	}), func(res *resource.DecodedResource[*pbdemov2.Artist]) (bool, []byte, error) {
		var b index.Builder
		b.String(res.Data.Genre.String())
		return true, b.Bytes(), nil
	})

	rec := newTestReconciler()
	init := newTestInitializer(1)
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	concertsChan := make(chan controller.Event)
	defer close(concertsChan)
	concertSource := &controller.Source{Source: concertsChan}
	concertMapper := func(ctx context.Context, rt controller.Runtime, event controller.Event) ([]controller.Request, error) {
		artistID := event.Obj.(*Concert).artistID
		var requests []controller.Request
		requests = append(requests, controller.Request{ID: artistID})
		return requests, nil
	}

	ctrl := controller.
		NewController("artist", pbdemov2.ArtistType, idx).
		WithWatch(pbdemov2.AlbumType, dependency.MapOwner, indexers.OwnerIndex("owner")).
		WithQuery("some-query", errQuery).
		WithCustomWatch(concertSource, concertMapper).
		WithBackoff(10*time.Millisecond, 100*time.Millisecond).
		WithReconciler(rec).
		WithInitializer(init)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	// Wait for initialization to complete
	init.wait(t)

	t.Run("managed resource type", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		rt, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		// ensure that the cache index is being properly managed
		dec := resourcetest.MustDecode[*pbdemov2.Artist](t, res)
		resources, err := rt.Cache.List(pbdemov2.ArtistType, "genre", dec.Data.Genre.String())
		require.NoError(t, err)
		prototest.AssertElementsMatch(t, []*pbresource.Resource{rsp.Resource}, resources)

		// ensure that the query was successfully registered - as we should not do equality
		// checks on functions we are using a constant error return query to ensure it was
		// registered properly.
		iter, err := rt.Cache.Query("some-query", "irrelevant")
		require.ErrorIs(t, err, injectedError)
		require.Nil(t, iter)
	})

	t.Run("watched resource type", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 500*time.Millisecond)

		album, err := demo.GenerateV2Album(rsp.Resource.Id)
		require.NoError(t, err)

		albumRsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)

		_, req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		album2, err := demo.GenerateV2Album(rsp.Resource.Id)
		require.NoError(t, err)

		albumRsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: album2})
		require.NoError(t, err)

		rt, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		// ensure that the watched type cache is being updated
		resources, err := rt.Cache.List(pbdemov2.AlbumType, "owner", rsp.Resource.Id)
		require.NoError(t, err)
		prototest.AssertElementsMatch(t, []*pbresource.Resource{albumRsp1.Resource, albumRsp2.Resource}, resources)
	})

	t.Run("custom watched resource type", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 500*time.Millisecond)

		concertsChan <- controller.Event{Obj: &Concert{name: "test-concert", artistID: rsp.Resource.Id}}

		_, watchedReq := rec.wait(t)
		prototest.AssertDeepEqual(t, req.ID, watchedReq.ID)

		otherArtist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		concertsChan <- controller.Event{Obj: &Concert{name: "test-concert", artistID: otherArtist.Id}}

		_, watchedReq = rec.wait(t)
		prototest.AssertDeepEqual(t, otherArtist.Id, watchedReq.ID)
	})

	t.Run("error retries", func(t *testing.T) {
		rec.failNext(errors.New("KABOOM"))

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		// Reconciler should be called with the same request again.
		_, req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("panic retries", func(t *testing.T) {
		rec.panicNext("KABOOM")

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		// Reconciler should be called with the same request again.
		_, req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("defer", func(t *testing.T) {
		rec.failNext(controller.RequeueAfter(1 * time.Second))

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 750*time.Millisecond)

		_, req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})
}

func TestController_API_InitializeRetry(t *testing.T) {
	t.Parallel()

	// Configure initializer to error initially in order to test retries
	expectedInitAttempts := 2
	init := newTestInitializer(expectedInitAttempts)

	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)
	rec := newTestReconciler()

	ctrl := controller.
		NewController("artist", pbdemov2.ArtistType).
		WithBackoff(10*time.Millisecond, 100*time.Millisecond).
		WithReconciler(rec).
		WithInitializer(init)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	// Wait for initialization attempts to complete
	for i := 0; i < expectedInitAttempts; i++ {
		init.wait(t)
	}

	// Create a resource and expect it to reconcile now that initialization is complete
	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	rec.wait(t)
}

func waitForAtomicBoolValue(t testutil.TestingTB, actual *atomic.Bool, expected bool) {
	t.Helper()
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, expected, actual.Load())
	})
}

func TestController_WithForceReconcileEvery_UpsertSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	// Given a controller
	// When the controller reconciles a resource due to an upsert and succeeds
	// Then the controller manager should scheduled a forced reconcile after forceReconcileEvery
	rec := newTestReconciler()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	// Create sizeable gap between reconcile #1 and forced reconcile #2 to ensure the delay occurs
	forceReconcileEvery := 5 * time.Second
	ctrl := controller.
		NewController("artist", pbdemov2.ArtistType).
		WithLogger(testutil.Logger(t)).
		WithForceReconcileEvery(forceReconcileEvery).
		WithReconciler(rec)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Verify reconcile #1 happens immediately
	_, req := rec.waitFor(t, time.Second)
	prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

	// Verify no reconciles occur between reconcile #1 and forced reconcile #2.
	// Remove a second for max jitter (20% of 5s) and one more second to be safe.
	rec.expectNoRequest(t, forceReconcileEvery-time.Second-time.Second)

	// Verify forced reconcile #2 occurred (forceReconcileEvery - 1s - 1s + 3s > forceReconcileEvery)
	_, req = rec.waitFor(t, time.Second*3)
	prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
}

func TestController_WithForceReconcileEvery_SkipOnReconcileError(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	// Given a controller configured with a forceReconcileEvery duration
	// When the controller reconciles a resource due to an upsert and returns an error
	// Then the controller manager should not schedule a forced reconcile and allow
	//      the existing error handling to schedule a rate-limited retry
	rec := newTestReconciler()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	// Large enough gap to test for a period of no-reconciles
	forceReconcileEvery := 5 * time.Second
	ctrl := controller.
		NewController("artist", pbdemov2.ArtistType).
		WithLogger(testutil.Logger(t)).
		WithForceReconcileEvery(forceReconcileEvery).
		WithReconciler(rec)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	// Setup reconcile #1 to fail
	rec.failNext(errors.New("reconcile #1 error"))
	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Observe failed reconcile #1
	_, req := rec.wait(t)
	prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

	// Observe successful (rate-limited retry) reconcile #2. By not failNext'ing it,
	// we're expecting it now'ish.
	_, req = rec.wait(t)
	prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

	// Observe no forced reconcile for gap after last successful reconcile
	// -1s for 20% jitter reduction
	// -1s for just to be safe
	rec.expectNoRequest(t, forceReconcileEvery-time.Second-time.Second)

	// Finally observe forced reconcile #3 up to 1 sec past (5-1-1+3) accumulated forceReconcileEvery delay
	_, req = rec.waitFor(t, 3*time.Second)
	prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
}

func TestController_WithForceReconcileEvery_SkipOnDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()
	// Given a controller configured with a forceReconcileEvery duration
	// When the controller reconciles a resource due to a delete and succeeds
	// Then the controller manager should not schedule a forced reconcile
	rec := newTestReconciler()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	// Large enough gap to test for a period of no-reconciles
	forceReconcileEvery := 5 * time.Second
	ctrl := controller.
		NewController("artist", pbdemov2.ArtistType).
		WithLogger(testutil.Logger(t)).
		WithForceReconcileEvery(forceReconcileEvery).
		WithReconciler(rec)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Account for reconcile #1 due to initial write
	_, req := rec.wait(t)
	prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

	// Perform a delete
	_, err = client.Delete(testContext(t), &pbresource.DeleteRequest{Id: rsp.Resource.Id})
	require.NoError(t, err)

	// Account for the reconcile #2 due to the delete
	_, req = rec.wait(t)

	// Account for the deferred forced reconcile #3 from the original write event since deferred
	// reconciles don't seem to be de-duped against non-deferred reconciles.
	_, req = rec.waitFor(t, forceReconcileEvery)

	// Verify no further reconciles occur
	rec.expectNoRequest(t, forceReconcileEvery)
}

func TestController_Placement(t *testing.T) {
	t.Parallel()

	t.Run("singleton", func(t *testing.T) {
		var running atomic.Bool
		running.Store(false)

		rec := newTestReconciler()
		client := svctest.NewResourceServiceBuilder().
			WithRegisterFns(demo.RegisterTypes).
			WithCloningDisabled().
			Run(t)

		ctrl := controller.
			NewController("artist", pbdemov2.ArtistType).
			WithWatch(pbdemov2.AlbumType, dependency.MapOwner).
			WithPlacement(controller.PlacementSingleton).
			WithNotifyStart(func(context.Context, controller.Runtime) {
				running.Store(true)
			}).
			WithNotifyStop(func(context.Context, controller.Runtime) {
				running.Store(false)
			}).
			WithReconciler(rec)

		mgr := controller.NewManager(client, testutil.Logger(t))
		mgr.Register(ctrl)
		go mgr.Run(testContext(t))

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		// Reconciler should not be called until we're the Raft leader.
		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)
		rec.expectNoRequest(t, 500*time.Millisecond)

		// Become the leader and check the reconciler is called.
		mgr.SetRaftLeader(true)
		waitForAtomicBoolValue(t, &running, true)
		_, _ = rec.wait(t)

		// Should not be called after losing leadership.
		mgr.SetRaftLeader(false)
		waitForAtomicBoolValue(t, &running, false)
		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)
		rec.expectNoRequest(t, 500*time.Millisecond)
	})

	t.Run("each server", func(t *testing.T) {
		var running atomic.Bool
		running.Store(false)

		rec := newTestReconciler()
		client := svctest.NewResourceServiceBuilder().
			WithRegisterFns(demo.RegisterTypes).
			Run(t)

		ctrl := controller.
			NewController("artist", pbdemov2.ArtistType).
			WithWatch(pbdemov2.AlbumType, dependency.MapOwner).
			WithPlacement(controller.PlacementEachServer).
			WithNotifyStart(func(context.Context, controller.Runtime) {
				running.Store(true)
			}).
			WithReconciler(rec)

		mgr := controller.NewManager(client, testutil.Logger(t))
		mgr.Register(ctrl)
		go mgr.Run(testContext(t))
		waitForAtomicBoolValue(t, &running, true)

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		// Reconciler should be called even though we're not the Raft leader.
		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)
		_, _ = rec.wait(t)
	})
}

func TestController_String(t *testing.T) {
	ctrl := controller.
		NewController("artist", pbdemov2.ArtistType).
		WithWatch(pbdemov2.AlbumType, dependency.MapOwner).
		WithBackoff(5*time.Second, 1*time.Hour).
		WithPlacement(controller.PlacementEachServer)

	require.Equal(t,
		`<Controller managed_type=demo.v2.Artist, watched_types=[demo.v2.Album], backoff=<base=5s, max=1h0m0s>, placement=each-server>`,
		ctrl.String(),
	)
}

func TestController_NoReconciler(t *testing.T) {
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(demo.RegisterTypes).
		Run(t)

	mgr := controller.NewManager(client, testutil.Logger(t))

	ctrl := controller.NewController("artist", pbdemov2.ArtistType)
	require.PanicsWithValue(t,
		fmt.Sprintf("cannot register controller without a reconciler %s", ctrl.String()),
		func() { mgr.Register(ctrl) })
}

func TestController_Watch(t *testing.T) {
	t.Parallel()

	t.Run("partitioned scoped resources", func(t *testing.T) {
		rec := newTestReconciler()

		client := svctest.NewResourceServiceBuilder().
			WithRegisterFns(demo.RegisterTypes).
			Run(t)

		ctrl := controller.
			NewController("labels", pbdemov1.RecordLabelType).
			WithReconciler(rec)

		mgr := controller.NewManager(client, testutil.Logger(t))
		mgr.SetRaftLeader(true)
		mgr.Register(ctrl)

		ctx := testContext(t)
		go mgr.Run(ctx)

		res, err := demo.GenerateV1RecordLabel("test")
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("cluster scoped resources", func(t *testing.T) {
		rec := newTestReconciler()

		client := svctest.NewResourceServiceBuilder().
			WithRegisterFns(demo.RegisterTypes).
			Run(t)

		ctrl := controller.
			NewController("executives", pbdemov1.ExecutiveType).
			WithReconciler(rec)

		mgr := controller.NewManager(client, testutil.Logger(t))
		mgr.SetRaftLeader(true)
		mgr.Register(ctrl)

		go mgr.Run(testContext(t))

		exec, err := demo.GenerateV1Executive("test", "CEO")
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: exec})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("namespace scoped resources", func(t *testing.T) {
		rec := newTestReconciler()

		client := svctest.NewResourceServiceBuilder().
			WithRegisterFns(demo.RegisterTypes).
			Run(t)

		ctrl := controller.
			NewController("artists", pbdemov2.ArtistType).
			WithReconciler(rec)

		mgr := controller.NewManager(client, testutil.Logger(t))
		mgr.SetRaftLeader(true)
		mgr.Register(ctrl)

		go mgr.Run(testContext(t))

		artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: artist})
		require.NoError(t, err)

		_, req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})
}

func newTestReconciler() *testReconciler {
	return &testReconciler{
		calls:  make(chan requestArgs),
		errors: make(chan error, 1),
		panics: make(chan any, 1),
	}
}

type requestArgs struct {
	req controller.Request
	rt  controller.Runtime
}

type testReconciler struct {
	calls  chan requestArgs
	errors chan error
	panics chan any
}

func (r *testReconciler) Reconcile(_ context.Context, rt controller.Runtime, req controller.Request) error {
	r.calls <- requestArgs{req: req, rt: rt}

	select {
	case err := <-r.errors:
		return err
	case p := <-r.panics:
		panic(p)
	default:
		return nil
	}
}

func (r *testReconciler) failNext(err error) {
	r.errors <- err
}

func (r *testReconciler) panicNext(p any) {
	r.panics <- p
}

func (r *testReconciler) expectNoRequest(t *testing.T, duration time.Duration) {
	t.Helper()

	started := time.Now()
	select {
	case args := <-r.calls:
		t.Fatalf("expected no request for %s, but got: %s after %s", duration, args.req.ID, time.Since(started))
	case <-time.After(duration):
	}
}

func (r *testReconciler) wait(t *testing.T) (controller.Runtime, controller.Request) {
	t.Helper()
	return r.waitFor(t, 500*time.Millisecond)
}

func (r *testReconciler) waitFor(t *testing.T, duration time.Duration) (controller.Runtime, controller.Request) {
	t.Helper()

	var args requestArgs
	select {
	case args = <-r.calls:
	case <-time.After(duration):
		t.Fatalf("Reconcile was not called after %v", duration)
	}
	return args.rt, args.req
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return ctx
}

type Concert struct {
	name     string
	artistID *pbresource.ID
}

func (c Concert) Key() string {
	return c.name
}

func newTestInitializer(errorCount int) *testInitializer {
	return &testInitializer{
		calls:            make(chan error, 1),
		expectedAttempts: errorCount,
	}
}

type testInitializer struct {
	expectedAttempts int // number of times the initializer should run to test retries
	attempts         int // running count of times initialize is called
	calls            chan error
}

func (i *testInitializer) Initialize(_ context.Context, _ controller.Runtime) error {
	i.attempts++
	if i.attempts < i.expectedAttempts {
		// Return an error to cause a retry
		err := errors.New("initialization error")
		i.calls <- err
		return err
	} else {
		i.calls <- nil
		return nil
	}
}

func (i *testInitializer) wait(t *testing.T) {
	t.Helper()
	select {
	case err := <-i.calls:
		if err == nil {
			// Initialize did not error, no more calls should be expected
			close(i.calls)
		}
		return
	case <-time.After(1000 * time.Millisecond):
		t.Fatal("Initialize was not called after 1000ms")
	}
}
