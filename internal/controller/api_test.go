// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestController_API(t *testing.T) {
	t.Parallel()

	rec := newTestReconciler()
	client := svctest.RunResourceService(t, demo.RegisterTypes)

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
		ForType(demo.TypeV2Artist).
		WithWatch(demo.TypeV2Album, controller.MapOwner).
		WithCustomWatch(concertSource, concertMapper).
		WithBackoff(10*time.Millisecond, 100*time.Millisecond).
		WithReconciler(rec)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	t.Run("managed resource type", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("watched resource type", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 500*time.Millisecond)

		album, err := demo.GenerateV2Album(rsp.Resource.Id)
		require.NoError(t, err)

		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: album})
		require.NoError(t, err)

		req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("custom watched resource type", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 500*time.Millisecond)

		concertsChan <- controller.Event{Obj: &Concert{name: "test-concert", artistID: rsp.Resource.Id}}

		watchedReq := rec.wait(t)
		prototest.AssertDeepEqual(t, req.ID, watchedReq.ID)

		otherArtist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		concertsChan <- controller.Event{Obj: &Concert{name: "test-concert", artistID: otherArtist.Id}}

		watchedReq = rec.wait(t)
		prototest.AssertDeepEqual(t, otherArtist.Id, watchedReq.ID)
	})

	t.Run("error retries", func(t *testing.T) {
		rec.failNext(errors.New("KABOOM"))

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		// Reconciler should be called with the same request again.
		req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("panic retries", func(t *testing.T) {
		rec.panicNext("KABOOM")

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		// Reconciler should be called with the same request again.
		req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})

	t.Run("defer", func(t *testing.T) {
		rec.failNext(controller.RequeueAfter(1 * time.Second))

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 750*time.Millisecond)

		req = rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)
	})
}

func TestController_Placement(t *testing.T) {
	t.Parallel()

	t.Run("singleton", func(t *testing.T) {
		rec := newTestReconciler()
		client := svctest.RunResourceService(t, demo.RegisterTypes)

		ctrl := controller.
			ForType(demo.TypeV2Artist).
			WithWatch(demo.TypeV2Album, controller.MapOwner).
			WithPlacement(controller.PlacementSingleton).
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
		_ = rec.wait(t)

		// Should not be called after losing leadership.
		mgr.SetRaftLeader(false)
		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)
		rec.expectNoRequest(t, 500*time.Millisecond)
	})

	t.Run("each server", func(t *testing.T) {
		rec := newTestReconciler()
		client := svctest.RunResourceService(t, demo.RegisterTypes)

		ctrl := controller.
			ForType(demo.TypeV2Artist).
			WithWatch(demo.TypeV2Album, controller.MapOwner).
			WithPlacement(controller.PlacementEachServer).
			WithReconciler(rec)

		mgr := controller.NewManager(client, testutil.Logger(t))
		mgr.Register(ctrl)
		go mgr.Run(testContext(t))

		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		// Reconciler should be called even though we're not the Raft leader.
		_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)
		_ = rec.wait(t)
	})
}

func TestController_String(t *testing.T) {
	ctrl := controller.
		ForType(demo.TypeV2Artist).
		WithWatch(demo.TypeV2Album, controller.MapOwner).
		WithBackoff(5*time.Second, 1*time.Hour).
		WithPlacement(controller.PlacementEachServer)

	require.Equal(t,
		`<Controller managed_type="demo.v2.Artist", watched_types=["demo.v2.Album"], backoff=<base="5s", max="1h0m0s">, placement="each-server">`,
		ctrl.String(),
	)
}

func TestController_NoReconciler(t *testing.T) {
	client := svctest.RunResourceService(t, demo.RegisterTypes)
	mgr := controller.NewManager(client, testutil.Logger(t))

	ctrl := controller.ForType(demo.TypeV2Artist)
	require.PanicsWithValue(t,
		`cannot register controller without a reconciler <Controller managed_type="demo.v2.Artist", watched_types=[], backoff=<base="5ms", max="16m40s">, placement="singleton">`,
		func() { mgr.Register(ctrl) })
}

func newTestReconciler() *testReconciler {
	return &testReconciler{
		calls:  make(chan controller.Request),
		errors: make(chan error, 1),
		panics: make(chan any, 1),
	}
}

type testReconciler struct {
	calls  chan controller.Request
	errors chan error
	panics chan any
}

func (r *testReconciler) Reconcile(_ context.Context, _ controller.Runtime, req controller.Request) error {
	r.calls <- req

	select {
	case err := <-r.errors:
		return err
	case p := <-r.panics:
		panic(p)
	default:
		return nil
	}
}

func (r *testReconciler) failNext(err error) { r.errors <- err }
func (r *testReconciler) panicNext(p any)    { r.panics <- p }

func (r *testReconciler) expectNoRequest(t *testing.T, duration time.Duration) {
	t.Helper()

	started := time.Now()
	select {
	case req := <-r.calls:
		t.Fatalf("expected no request for %s, but got: %s after %s", duration, req.ID, time.Since(started))
	case <-time.After(duration):
	}
}

func (r *testReconciler) wait(t *testing.T) controller.Request {
	t.Helper()

	var req controller.Request
	select {
	case req = <-r.calls:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Reconcile was not called after 500ms")
	}
	return req
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
