// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errGuardClosed = errors.New("guard closed")

type guardTestTopic string

func (g guardTestTopic) String() string { return string(g) }

func guardedPublisher(t *testing.T, handlerCalls *atomic.Int64) (*EventPublisher, Topic, Topic) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pub := NewEventPublisher(time.Minute)
	handler := func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		handlerCalls.Add(1)
		buf.Append([]Event{{Topic: req.Topic, Index: 1, Payload: simplePayload{key: "k", value: "v"}}})
		return 1, nil
	}
	guarded, unguarded := guardTestTopic("guarded"), guardTestTopic("unguarded")
	require.NoError(t, pub.RegisterHandler(guarded, handler, false))
	require.NoError(t, pub.RegisterUnguardedHandler(unguarded, handler, false))
	go pub.Run(ctx)
	return pub, guarded, unguarded
}

func TestEventPublisher_SnapshotGuard(t *testing.T) {
	var calls atomic.Int64
	pub, guarded, unguarded := guardedPublisher(t, &calls)

	var open atomic.Bool
	var held atomic.Int64
	pub.SetSnapshotGuard(func() (func(), error) {
		if !open.Load() {
			return nil, errGuardClosed
		}
		held.Add(1)
		return func() { held.Add(-1) }, nil
	})

	subscribe := func(topic Topic) (*Subscription, error) {
		return pub.Subscribe(&SubscribeRequest{Topic: topic, Subject: StringSubject("k")})
	}
	firstEvent := func(sub *Subscription) (Event, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return sub.Next(ctx)
	}

	// A closed guard refuses a guarded topic with the guard's error, never
	// calls the handler, and the failure is not cached for the next subscriber.
	sub, err := subscribe(guarded)
	require.NoError(t, err)
	_, err = firstEvent(sub)
	require.ErrorIs(t, err, errGuardClosed)
	sub.Unsubscribe()
	require.Zero(t, calls.Load())

	// An unguarded topic is not affected.
	sub, err = subscribe(unguarded)
	require.NoError(t, err)
	ev, err := firstEvent(sub)
	require.NoError(t, err)
	require.Equal(t, uint64(1), ev.Index)
	sub.Unsubscribe()
	require.Equal(t, int64(1), calls.Load())

	// Once the guard opens, the same guarded topic gets a snapshot, and the
	// guard is released when the handler is done.
	open.Store(true)
	sub, err = subscribe(guarded)
	require.NoError(t, err)
	ev, err = firstEvent(sub)
	require.NoError(t, err)
	require.Equal(t, uint64(1), ev.Index)
	sub.Unsubscribe()
	require.Equal(t, int64(2), calls.Load())
	require.Zero(t, held.Load(), "guard must be released after the handler")
}

func TestEventPublisher_SnapshotGuardAppliesToHandlersRegisteredLater(t *testing.T) {
	var calls atomic.Int64
	pub, _, _ := guardedPublisher(t, &calls)
	pub.SetSnapshotGuard(func() (func(), error) { return nil, errGuardClosed })

	later := guardTestTopic("later")
	require.NoError(t, pub.RegisterHandler(later, func(SubscribeRequest, SnapshotAppender) (uint64, error) {
		calls.Add(1)
		return 1, nil
	}, false))

	sub, err := pub.Subscribe(&SubscribeRequest{Topic: later, Subject: StringSubject("k")})
	require.NoError(t, err)
	defer sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = sub.Next(ctx)
	require.ErrorIs(t, err, errGuardClosed)
	require.Zero(t, calls.Load())
}

func TestEventPublisher_NoGuardByDefault(t *testing.T) {
	var calls atomic.Int64
	pub, guarded, _ := guardedPublisher(t, &calls)

	sub, err := pub.Subscribe(&SubscribeRequest{Topic: guarded, Subject: StringSubject("k")})
	require.NoError(t, err)
	defer sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = sub.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), calls.Load())
}
