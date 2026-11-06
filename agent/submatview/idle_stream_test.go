// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// scriptedStreamClient plays one script per Subscribe call. After a script is
// exhausted the stream stays open and silent, like a server that keeps the
// stream up (and answers keepalives) without publishing anything.
type scriptedStreamClient struct {
	lock    sync.Mutex
	scripts [][]*pbsubscribe.Event
	indexes []uint64
	// hangFirstSubscribe makes the first Subscribe call block until its context
	// is cancelled, like a connection that never completes the call.
	hangFirstSubscribe bool
}

func (c *scriptedStreamClient) Subscribe(
	ctx context.Context,
	req *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	c.lock.Lock()
	n := len(c.indexes)
	c.indexes = append(c.indexes, req.Index)
	hang := c.hangFirstSubscribe && n == 0
	var script []*pbsubscribe.Event
	if n < len(c.scripts) {
		script = c.scripts[n]
	}
	c.lock.Unlock()

	if hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &scriptedStream{ctx: ctx, events: script}, nil
}

func (c *scriptedStreamClient) subscribeIndexes() []uint64 {
	c.lock.Lock()
	defer c.lock.Unlock()
	return append([]uint64(nil), c.indexes...)
}

type scriptedStream struct {
	grpc.ClientStream
	ctx    context.Context
	events []*pbsubscribe.Event
}

func (s *scriptedStream) Recv() (*pbsubscribe.Event, error) {
	if len(s.events) > 0 {
		e := s.events[0]
		s.events = s.events[1:]
		return e, nil
	}
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

func newScriptedMaterializer(client StreamClient, idleTimeout time.Duration) *RPCMaterializer {
	deps := Deps{
		View:   &fakeView{srvs: make(map[string]*pbservice.CheckServiceNode)},
		Logger: hclog.NewNullLogger(),
		Request: func(index uint64) *pbsubscribe.SubscribeRequest {
			return &pbsubscribe.SubscribeRequest{
				Topic: pbsubscribe.Topic_ServiceHealth,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "key",
						Namespace: pbcommon.DefaultEnterpriseMeta.Namespace,
					},
				},
				Token:      "abcd",
				Datacenter: "dc1",
				Index:      index,
			}
		},
	}
	deps.IdleTimeout = idleTimeout
	return NewRPCMaterializer(client, deps)
}

func silentThenFreshSnapshotScripts() [][]*pbsubscribe.Event {
	return [][]*pbsubscribe.Event{
		{newEventServiceHealthRegister(3, 1, "srv1"), newEndOfSnapshotEvent(5)},
		{
			newNewSnapshotToFollowEvent(),
			newEventServiceHealthRegister(6, 1, "srv1"),
			newEventServiceHealthRegister(7, 2, "srv1"),
			newEndOfSnapshotEvent(7),
		},
	}
}

// With the check disabled (the default) a stream that stays open but stops
// delivering events is never detected: the materializer keeps serving the stale
// view and never resubscribes.
func TestRPCMaterializer_SilentStreamIsNotRecoveredWhenDisabled(t *testing.T) {
	client := &scriptedStreamClient{scripts: silentThenFreshSnapshotScripts()}
	mat := newScriptedMaterializer(client, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mat.Run(ctx)

	first, err := mat.Query(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(5), first.Index)
	require.Len(t, first.Value.(fakeResult).srvs, 1)

	qctx, qcancel := context.WithTimeout(ctx, 600*time.Millisecond)
	defer qcancel()
	stale, err := mat.Query(qctx, 5)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, uint64(5), stale.Index, "the blocking query keeps returning the frozen index")
	require.Equal(t, []uint64{0}, client.subscribeIndexes(), "no resubscribe ever happens")
}

// A stream that goes silent is detected and re-established from the current
// index, and the view catches up without any caller seeing an error.
func TestRPCMaterializer_IdleStreamIsReestablished(t *testing.T) {
	client := &scriptedStreamClient{scripts: silentThenFreshSnapshotScripts()}
	mat := newScriptedMaterializer(client, 150*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mat.Run(ctx)

	first, err := mat.Query(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(5), first.Index)

	qctx, qcancel := context.WithTimeout(ctx, 5*time.Second)
	defer qcancel()
	fresh, err := mat.Query(qctx, 5)
	require.NoError(t, err, "the idle resubscribe must not surface as an error to callers")
	require.Equal(t, uint64(7), fresh.Index)
	require.Len(t, fresh.Value.(fakeResult).srvs, 2)

	require.Equal(t, []uint64{0, 5}, client.subscribeIndexes()[:2],
		"the second subscription resumes from the index the view had reached")
}

// A stream that never delivers anything, not even a snapshot, is also retried.
func TestRPCMaterializer_IdleBeforeSnapshotIsRetried(t *testing.T) {
	client := &scriptedStreamClient{scripts: [][]*pbsubscribe.Event{
		nil,
		{newEventServiceHealthRegister(3, 1, "srv1"), newEndOfSnapshotEvent(5)},
	}}
	mat := newScriptedMaterializer(client, 150*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mat.Run(ctx)

	qctx, qcancel := context.WithTimeout(ctx, 5*time.Second)
	defer qcancel()
	result, err := mat.Query(qctx, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(5), result.Index)
	require.GreaterOrEqual(t, len(client.subscribeIndexes()), 2)
}

// A topic that is merely quiet is not disturbed as long as the stream keeps
// delivering messages within the timeout.
func TestRPCMaterializer_ActiveStreamIsNotInterrupted(t *testing.T) {
	client := NewTestStreamingClient(pbcommon.DefaultEnterpriseMeta.Namespace)
	client.QueueEvents(newEndOfSnapshotEvent(2))
	mat := newScriptedMaterializer(client, 250*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mat.Run(ctx)

	_, err := mat.Query(ctx, 0)
	require.NoError(t, err)

	for i := 1; i <= 12; i++ {
		time.Sleep(60 * time.Millisecond)
		client.QueueEvents(newEventServiceHealthRegister(uint64(10+i), i, "srv1"))
	}

	client.lock.RLock()
	subscriptions := len(client.subClients)
	client.lock.RUnlock()
	require.Equal(t, 1, subscriptions, "a stream that keeps delivering must never be resubscribed")
}

// Each idle resubscribe is counted, so a stalling stream is visible in telemetry.
func TestRPCMaterializer_IdleResubscribeEmitsMetric(t *testing.T) {
	sink := metrics.NewInmemSink(10*time.Millisecond, time.Minute)
	cfg := metrics.DefaultConfig("consul")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	_, err := metrics.NewGlobal(cfg, sink)
	require.NoError(t, err)

	client := &scriptedStreamClient{scripts: silentThenFreshSnapshotScripts()}
	mat := newScriptedMaterializer(client, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mat.Run(ctx)

	qctx, qcancel := context.WithTimeout(ctx, 5*time.Second)
	defer qcancel()
	_, err = mat.Query(qctx, 5)
	require.NoError(t, err)

	var count int
	for _, interval := range sink.Data() {
		interval.RLock()
		if c, ok := interval.Counters["consul.streaming.idle_resubscribe"]; ok {
			count += c.Count
		}
		interval.RUnlock()
	}
	require.GreaterOrEqual(t, count, 1, "the idle resubscribe counter must be emitted")
}

// A Subscribe call that never completes is also detected and retried.
func TestRPCMaterializer_IdleDuringSubscribeIsRetried(t *testing.T) {
	client := &scriptedStreamClient{
		hangFirstSubscribe: true,
		scripts: [][]*pbsubscribe.Event{
			nil,
			{newEventServiceHealthRegister(3, 1, "srv1"), newEndOfSnapshotEvent(5)},
		},
	}
	mat := newScriptedMaterializer(client, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mat.Run(ctx)

	qctx, qcancel := context.WithTimeout(ctx, 5*time.Second)
	defer qcancel()
	result, err := mat.Query(qctx, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(5), result.Index)
	require.GreaterOrEqual(t, len(client.subscribeIndexes()), 2)
}

func TestIdleStreamErr_Message(t *testing.T) {
	err := error(idleStreamErr{idle: time.Minute})
	require.EqualError(t, err, "no events received on the subscription for 1m0s")

	var idle idleStreamErr
	require.ErrorAs(t, fmt.Errorf("wrapped: %w", err), &idle)
	require.Equal(t, time.Minute, idle.idle)
}

// Jitter must never push a valid, very large timeout past the largest
// time.Duration: a wrapped negative timeout would fire immediately.
func TestIdleWatchdog_JitterCannotOverflow(t *testing.T) {
	for _, timeout := range []time.Duration{
		time.Minute,
		2500000 * time.Hour,
		time.Duration(math.MaxInt64) - time.Second,
		time.Duration(math.MaxInt64),
	} {
		for i := 0; i < 1000; i++ {
			jitter := idleJitter(timeout)
			require.GreaterOrEqual(t, int64(jitter), int64(0), "timeout %s", timeout)
			require.GreaterOrEqual(t, int64(timeout+jitter), int64(timeout), "timeout %s overflowed", timeout)
		}
	}

	fired := make(chan struct{}, 1)
	w := newIdleWatchdog(time.Duration(math.MaxInt64), func() { fired <- struct{}{} })
	defer w.stop()
	require.Greater(t, int64(w.timeout), int64(0))
	select {
	case <-fired:
		t.Fatal("a very large timeout must not fire immediately")
	case <-time.After(100 * time.Millisecond):
	}
	require.False(t, w.fired())
}
