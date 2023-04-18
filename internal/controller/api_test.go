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

func TestControllerAPI(t *testing.T) {
	rec := newTestReconciler()
	client := svctest.RunResourceService(t, demo.RegisterTypes)

	ctrl := controller.
		ForType(demo.TypeV2Artist).
		WithBackoff(10*time.Millisecond, 100*time.Millisecond).
		WithReconciler(rec)

	mgr := controller.NewManager(client, testutil.Logger(t))
	mgr.Register(ctrl)
	mgr.SetRaftLeader(true)
	go mgr.Run(testContext(t))

	t.Run("basic", func(t *testing.T) {
		res, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		require.NoError(t, err)

		req := rec.wait(t)
		prototest.AssertDeepEqual(t, rsp.Resource.Id, req.ID)

		rec.expectNoRequest(t, 1*time.Second)
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
