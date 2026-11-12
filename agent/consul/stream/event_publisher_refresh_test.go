// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventPublisher_RefreshAllTopicsWith(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	publisher := NewEventPublisher(time.Minute)
	registerTestSnapshotHandlers(t, publisher)
	go publisher.Run(ctx)

	req := &SubscribeRequest{Topic: testTopic, Subject: StringSubject("sub-key")}
	existing, err := publisher.Subscribe(req)
	require.NoError(t, err)
	defer existing.Unsubscribe()

	subscribed := make(chan struct{})
	fnRan := make(chan struct{})
	publisher.RefreshAllTopicsWith(func() {
		close(fnRan)
		go func() {
			sub, err := publisher.Subscribe(req)
			if err == nil {
				sub.Unsubscribe()
			}
			close(subscribed)
		}()

		// A Subscribe must wait for fn to finish: it cannot build a snapshot
		// from state that is being swapped.
		select {
		case <-subscribed:
			t.Error("Subscribe completed while the refresh function was running")
		case <-time.After(100 * time.Millisecond):
		}
	})

	select {
	case <-fnRan:
	default:
		t.Fatal("refresh function was not called")
	}

	select {
	case <-subscribed:
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not complete after the refresh")
	}

	// Existing subscriptions are force-closed so clients resubscribe.
	for {
		_, err := existing.Next(ctx)
		if err != nil {
			require.ErrorIs(t, err, ErrSubForceClosed)
			break
		}
	}
}
