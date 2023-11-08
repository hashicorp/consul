// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"testing"
	time "time"

	"github.com/stretchr/testify/require"
)

func noopUnSub() {}

func TestSubscription(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	eb := newEventBuffer()

	index := uint64(100)

	startHead := eb.Head()

	// Start with an event in the buffer
	publishTestEvent(index, eb, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("test"),
	}
	sub := newSubscription(req, startHead, noopUnSub)

	// First call to sub.Next should return our published event immediately
	start := time.Now()
	got, err := sub.Next(ctx)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed < 200*time.Millisecond,
		"Event should have been delivered immediately, took %s", elapsed)
	require.Equal(t, index, got.Index)

	// Schedule an event publish in a while
	index++
	start = time.Now()
	time.AfterFunc(200*time.Millisecond, func() {
		publishTestEvent(index, eb, "test")
	})

	// Next call should block until event is delivered
	got, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Event should have been delivered after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Event should have been delivered after short time, took %s", elapsed)
	require.Equal(t, index, got.Index)

	index++
	publishTestEvent(index, eb, "test")

	start = time.Now()
	got, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed < 200*time.Millisecond,
		"Event should have been delivered immediately, took %s", elapsed)
	require.Equal(t, index, got.Index)
	require.Equal(t, "test", got.Payload.(simplePayload).key)

	// Cancelling the subscription context should unblock Next
	start = time.Now()
	time.AfterFunc(200*time.Millisecond, func() {
		cancel()
	})

	_, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.Error(t, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Event should have been delivered after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Event should have been delivered after short time, took %s", elapsed)
}

func TestSubscription_Close(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	eb := newEventBuffer()
	index := uint64(100)
	startHead := eb.Head()

	// Start with an event in the buffer
	publishTestEvent(index, eb, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("test"),
	}
	sub := newSubscription(req, startHead, noopUnSub)

	// First call to sub.Next should return our published event immediately
	start := time.Now()
	got, err := sub.Next(ctx)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed < 200*time.Millisecond,
		"Event should have been delivered immediately, took %s", elapsed)
	require.Equal(t, index, got.Index)

	// Schedule a Close simulating the server deciding this subscroption
	// needs to reset (e.g. on ACL perm change).
	start = time.Now()
	time.AfterFunc(200*time.Millisecond, func() {
		sub.forceClose()
	})

	_, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.Error(t, err)
	require.Equal(t, ErrSubForceClosed, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Reload should have happened after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Reload should have been delivered after short time, took %s", elapsed)
}

func publishTestEvent(index uint64, b *eventBuffer, key string) {
	e := Event{
		Index:   index,
		Topic:   testTopic,
		Payload: simplePayload{key: key},
	}
	b.Append([]Event{e})
}

func TestNewEventsFromBatch(t *testing.T) {
	t.Run("single item", func(t *testing.T) {
		first := Event{
			Topic:   testTopic,
			Index:   1234,
			Payload: simplePayload{key: "key"},
		}
		e := newEventFromBatch(SubscribeRequest{}, []Event{first})
		require.Equal(t, first, e)
	})
	t.Run("many items", func(t *testing.T) {
		events := []Event{
			newSimpleEvent("foo", 9999),
			newSimpleEvent("foo", 9999),
			newSimpleEvent("zee", 9999),
		}
		req := SubscribeRequest{Topic: testTopic}
		e := newEventFromBatch(req, events)
		expected := Event{
			Topic:   testTopic,
			Index:   9999,
			Payload: newPayloadEvents(events...),
		}
		require.Equal(t, expected, e)
	})
}
