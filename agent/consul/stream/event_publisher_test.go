// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
)

type intTopic int

func (i intTopic) String() string {
	return fmt.Sprintf("%d", i)
}

var testTopic Topic = intTopic(999)

func TestEventPublisher_SubscribeWithIndex0(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(0)
	registerTestSnapshotHandlers(t, publisher)
	go publisher.Run(ctx)

	sub, err := publisher.Subscribe(req)
	require.NoError(t, err)
	defer sub.Unsubscribe()
	eventCh := runSubscription(ctx, sub)

	next := getNextEvent(t, eventCh)
	require.Equal(t, testSnapshotEvent, next)

	next = getNextEvent(t, eventCh)
	require.True(t, next.IsEndOfSnapshot())

	assertNoResult(t, eventCh)

	events := []Event{{
		Topic:   testTopic,
		Payload: simplePayload{key: "sub-key", value: "the-published-event-payload"},
	}}
	publisher.Publish(events)

	// Subscriber should see the published event
	next = getNextEvent(t, eventCh)
	expected := Event{
		Topic:   testTopic,
		Payload: simplePayload{key: "sub-key", value: "the-published-event-payload"},
	}
	require.Equal(t, expected, next)

	// Subscriber should not see events for other keys
	publisher.Publish([]Event{{
		Topic:   testTopic,
		Payload: simplePayload{key: "other-key", value: "this-should-not-reach-the-subscriber"},
	}})
	assertNoResult(t, eventCh)
}

var testSnapshotEvent = Event{
	Topic:   testTopic,
	Payload: simplePayload{key: "sub-key", value: "snapshot-event-payload"},
	Index:   1,
}

type simplePayload struct {
	key        string
	value      string
	noReadPerm bool
}

func (p simplePayload) HasReadPermission(acl.Authorizer) bool {
	return !p.noReadPerm
}

func (p simplePayload) Subject() Subject { return StringSubject(p.key) }

func (p simplePayload) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	panic("simplePayload does not implement ToSubscriptionEvent")
}

func registerTestSnapshotHandlers(t *testing.T, publisher *EventPublisher) {
	t.Helper()

	testTopicHandler := func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		if req.Topic != testTopic {
			return 0, fmt.Errorf("unexpected topic: %v", req.Topic)
		}
		buf.Append([]Event{testSnapshotEvent})
		return 1, nil
	}

	require.NoError(t, publisher.RegisterHandler(testTopic, testTopicHandler, false))
}

func runSubscription(ctx context.Context, sub *Subscription) <-chan eventOrErr {
	eventCh := make(chan eventOrErr, 1)
	go func() {
		for {
			es, err := sub.Next(ctx)
			eventCh <- eventOrErr{
				Event: es,
				Err:   err,
			}
			if err != nil {
				return
			}
		}
	}()
	return eventCh
}

type eventOrErr struct {
	Event Event
	Err   error
}

func getNextEvent(t *testing.T, eventCh <-chan eventOrErr) Event {
	t.Helper()
	select {
	case next := <-eventCh:
		require.NoError(t, next.Err)
		return next.Event
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout waiting for event from subscription")
		return Event{}
	}
}

func assertNoResult(t *testing.T, eventCh <-chan eventOrErr) {
	t.Helper()
	select {
	case next := <-eventCh:
		require.NoError(t, next.Err)
		t.Fatalf("received unexpected event: %#v", next.Event.Payload)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestEventPublisher_ShutdownClosesSubscriptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fn := func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		return 0, nil
	}

	publisher := NewEventPublisher(time.Second)
	registerTestSnapshotHandlers(t, publisher)
	publisher.RegisterHandler(intTopic(22), fn, false)
	publisher.RegisterHandler(intTopic(33), fn, false)
	go publisher.Run(ctx)

	sub1, err := publisher.Subscribe(&SubscribeRequest{Topic: intTopic(22), Subject: SubjectNone})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	sub2, err := publisher.Subscribe(&SubscribeRequest{Topic: intTopic(33), Subject: SubjectNone})
	require.NoError(t, err)
	defer sub2.Unsubscribe()

	cancel() // Shutdown

	err = consumeSub(context.Background(), sub1)
	require.Equal(t, err, ErrShuttingDown)

	_, err = sub2.Next(context.Background())
	require.Equal(t, err, ErrShuttingDown)
}

func consumeSub(ctx context.Context, sub *Subscription) error {
	for {
		event, err := sub.Next(ctx)
		switch {
		case err != nil:
			return err
		case event.IsEndOfSnapshot():
			continue
		}
	}
}

func TestEventPublisher_SubscribeWithIndex0_FromCache(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(time.Second)
	registerTestSnapshotHandlers(t, publisher)
	go publisher.Run(ctx)

	sub, err := publisher.Subscribe(req)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	publisher.snapshotHandlers[testTopic] = func(_ SubscribeRequest, _ SnapshotAppender) (uint64, error) {
		return 0, fmt.Errorf("error should not be seen, cache should have been used")
	}

	sub, err = publisher.Subscribe(req)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	eventCh := runSubscription(ctx, sub)
	next := getNextEvent(t, eventCh)
	require.Equal(t, testSnapshotEvent, next)

	next = getNextEvent(t, eventCh)
	require.True(t, next.IsEndOfSnapshot())

	// Now subscriber should block waiting for updates
	assertNoResult(t, eventCh)

	expected := Event{
		Topic:   testTopic,
		Payload: simplePayload{key: "sub-key", value: "the-published-event-payload"},
		Index:   3,
	}
	publisher.Publish([]Event{expected})

	// Subscriber should see the published event
	next = getNextEvent(t, eventCh)
	require.Equal(t, expected, next)
}

func TestEventPublisher_SubscribeWithIndexNotZero_CanResume(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(time.Second)
	registerTestSnapshotHandlers(t, publisher)
	go publisher.Run(ctx)

	simulateExistingSubscriber(t, publisher, req)

	// Publish the testSnapshotEvent, to ensure that it is skipped over when
	// splicing the topic buffer onto the snapshot.
	publisher.publishEvent([]Event{testSnapshotEvent})

	testutil.RunStep(t, "start a subscription and unsub", func(t *testing.T) {
		sub, err := publisher.Subscribe(req)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		eventCh := runSubscription(ctx, sub)

		next := getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot())
		require.Equal(t, uint64(1), next.Index)
	})

	testutil.RunStep(t, "resume the subscription", func(t *testing.T) {
		newReq := *req
		newReq.Index = 1
		sub, err := publisher.Subscribe(&newReq)
		require.NoError(t, err)

		eventCh := runSubscription(ctx, sub)
		assertNoResult(t, eventCh)

		expected := Event{
			Topic:   testTopic,
			Index:   3,
			Payload: simplePayload{key: "sub-key", value: "event-3"},
		}
		publisher.publishEvent([]Event{expected})

		next := getNextEvent(t, eventCh)
		require.Equal(t, expected, next)
	})
}

func TestEventPublisher_SubscribeWithIndexNotZero_NewSnapshot(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(0)
	registerTestSnapshotHandlers(t, publisher)
	go publisher.Run(ctx)
	// Include the same event in the topicBuffer
	publisher.publishEvent([]Event{testSnapshotEvent})

	testutil.RunStep(t, "start a subscription and unsub", func(t *testing.T) {
		sub, err := publisher.Subscribe(req)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		eventCh := runSubscription(ctx, sub)

		next := getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot())
		require.Equal(t, uint64(1), next.Index)
	})

	nextEvent := Event{
		Topic:   testTopic,
		Index:   3,
		Payload: simplePayload{key: "sub-key", value: "event-3"},
	}

	testutil.RunStep(t, "publish an event while unsubed", func(t *testing.T) {
		publisher.publishEvent([]Event{nextEvent})
	})

	testutil.RunStep(t, "resume the subscription", func(t *testing.T) {
		newReq := *req
		newReq.Index = 1
		sub, err := publisher.Subscribe(&newReq)
		require.NoError(t, err)

		eventCh := runSubscription(ctx, sub)
		next := getNextEvent(t, eventCh)
		require.True(t, next.IsNewSnapshotToFollow(), next)

		next = getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot())
	})
}

func TestEventPublisher_SubscribeWithIndexNotZero_NewSnapshotFromCache(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(time.Second)
	registerTestSnapshotHandlers(t, publisher)
	go publisher.Run(ctx)

	simulateExistingSubscriber(t, publisher, req)

	// Publish the testSnapshotEvent, to ensure that it is skipped over when
	// splicing the topic buffer onto the snapshot.
	publisher.publishEvent([]Event{testSnapshotEvent})

	testutil.RunStep(t, "start a subscription and unsub", func(t *testing.T) {
		sub, err := publisher.Subscribe(req)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		eventCh := runSubscription(ctx, sub)

		next := getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot())
		require.Equal(t, uint64(1), next.Index)
	})

	nextEvent := Event{
		Topic:   testTopic,
		Index:   3,
		Payload: simplePayload{key: "sub-key", value: "event-3"},
	}

	testutil.RunStep(t, "publish an event while unsubed", func(t *testing.T) {
		publisher.publishEvent([]Event{nextEvent})
	})

	publisher.snapshotHandlers[testTopic] = func(_ SubscribeRequest, _ SnapshotAppender) (uint64, error) {
		return 0, fmt.Errorf("error should not be seen, cache should have been used")
	}

	testutil.RunStep(t, "resume the subscription", func(t *testing.T) {
		newReq := *req
		newReq.Index = 1
		sub, err := publisher.Subscribe(&newReq)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		eventCh := runSubscription(ctx, sub)
		next := getNextEvent(t, eventCh)
		require.True(t, next.IsNewSnapshotToFollow(), next)

		next = getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot())

		next = getNextEvent(t, eventCh)
		require.Equal(t, nextEvent, next)
	})
}

func TestEventPublisher_SubscribeWithIndexNotZero_NewSnapshot_WithCache(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
		Index:   1,
	}

	nextEvent := Event{
		Topic:   testTopic,
		Index:   3,
		Payload: simplePayload{key: "sub-key", value: "event-3"},
	}

	testTopicHandler := func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		if req.Topic != testTopic {
			return 0, fmt.Errorf("unexpected topic: %v", req.Topic)
		}
		buf.Append([]Event{testSnapshotEvent})
		buf.Append([]Event{nextEvent})
		return 3, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(time.Second)
	publisher.RegisterHandler(testTopic, testTopicHandler, false)
	go publisher.Run(ctx)

	simulateExistingSubscriber(t, publisher, req)

	// Publish the events, to ensure they are is skipped over when splicing the
	// topic buffer onto the snapshot.
	publisher.publishEvent([]Event{testSnapshotEvent})
	publisher.publishEvent([]Event{nextEvent})

	testutil.RunStep(t, "start a subscription and unsub", func(t *testing.T) {
		sub, err := publisher.Subscribe(req)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		eventCh := runSubscription(ctx, sub)
		next := getNextEvent(t, eventCh)
		require.True(t, next.IsNewSnapshotToFollow(), next)

		next = getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.Equal(t, nextEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot(), next)
		require.Equal(t, uint64(3), next.Index)
	})

	publisher.snapshotHandlers[testTopic] = func(_ SubscribeRequest, _ SnapshotAppender) (uint64, error) {
		return 0, fmt.Errorf("error should not be seen, cache should have been used")
	}

	testutil.RunStep(t, "resume the subscription", func(t *testing.T) {
		newReq := *req
		newReq.Index = 0
		sub, err := publisher.Subscribe(&newReq)
		require.NoError(t, err)

		eventCh := runSubscription(ctx, sub)
		next := getNextEvent(t, eventCh)
		require.Equal(t, testSnapshotEvent, next)

		next = getNextEvent(t, eventCh)
		require.Equal(t, nextEvent, next)

		next = getNextEvent(t, eventCh)
		require.True(t, next.IsEndOfSnapshot())
	})
}

func TestEventPublisher_Unsubscribe_ClosesSubscription(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	publisher := NewEventPublisher(time.Second)
	registerTestSnapshotHandlers(t, publisher)

	sub, err := publisher.Subscribe(req)
	require.NoError(t, err)

	_, err = sub.Next(ctx)
	require.NoError(t, err)

	sub.Unsubscribe()
	_, err = sub.Next(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription was closed by unsubscribe")
}

func TestEventPublisher_Unsubscribe_FreesResourcesWhenThereAreNoSubscribers(t *testing.T) {
	req := &SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	}

	publisher := NewEventPublisher(time.Second)
	registerTestSnapshotHandlers(t, publisher)

	sub1, err := publisher.Subscribe(req)
	require.NoError(t, err)

	// Expect a topic buffer and snapshot to have been created.
	publisher.lock.Lock()
	require.NotNil(t, publisher.topicBuffers[req.topicSubject()])
	require.NotNil(t, publisher.snapCache[req.topicSubject()])
	publisher.lock.Unlock()

	// Create another subscription and close the old one, to ensure the buffer and
	// snapshot stick around as long as there's at least one subscriber.
	sub2, err := publisher.Subscribe(req)
	require.NoError(t, err)

	sub1.Unsubscribe()

	publisher.lock.Lock()
	require.NotNil(t, publisher.topicBuffers[req.topicSubject()])
	require.NotNil(t, publisher.snapCache[req.topicSubject()])
	publisher.lock.Unlock()

	// Close the other subscription and expect the buffer and snapshot to have
	// been cleaned up.
	sub2.Unsubscribe()

	publisher.lock.Lock()
	require.Nil(t, publisher.topicBuffers[req.topicSubject()])
	require.Nil(t, publisher.snapCache[req.topicSubject()])
	publisher.lock.Unlock()
}

// simulateExistingSubscriber creates a subscription that remains open throughout
// a test to prevent the topic buffer getting garbage-collected.
//
// It evicts the created snapshot from the cache immediately (simulating an
// existing subscription that has been open long enough the snapshot's TTL has
// been reached) so you can test snapshots getting created afresh.
func simulateExistingSubscriber(t *testing.T, p *EventPublisher, r *SubscribeRequest) {
	t.Helper()

	sub, err := p.Subscribe(r)
	require.NoError(t, err)
	t.Cleanup(sub.Unsubscribe)

	p.lock.Lock()
	delete(p.snapCache, r.topicSubject())
	p.lock.Unlock()
}

func TestEventPublisher_Subscribe_WildcardNotSupported(t *testing.T) {
	publisher := NewEventPublisher(0)

	handler := func(SubscribeRequest, SnapshotAppender) (uint64, error) { return 0, nil }
	require.NoError(t, publisher.RegisterHandler(testTopic, handler, false))

	_, err := publisher.Subscribe(&SubscribeRequest{
		Topic:   testTopic,
		Subject: SubjectWildcard,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not support wildcard subscriptions")
}

func TestEventPublisher_Subscribe_WildcardSupported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := NewEventPublisher(0)
	go publisher.Run(ctx)

	var (
		// These events are in the snapshot.
		a1 = Event{
			Topic:   testTopic,
			Payload: simplePayload{key: "a", value: "1"},
			Index:   1,
		}
		b1 = Event{
			Topic:   testTopic,
			Payload: simplePayload{key: "b", value: "1"},
			Index:   1,
		}

		// These events are published after the subscription begins.
		a2 = Event{
			Topic:   testTopic,
			Payload: simplePayload{key: "a", value: "2"},
			Index:   2,
		}
		b2 = Event{
			Topic:   testTopic,
			Payload: simplePayload{key: "b", value: "2"},
			Index:   2,
		}
	)

	handler := func(_ SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		buf.Append([]Event{a1, b1})
		return 1, nil
	}
	require.NoError(t, publisher.RegisterHandler(testTopic, handler, true))

	sub, err := publisher.Subscribe(&SubscribeRequest{
		Topic:   testTopic,
		Subject: SubjectWildcard,
	})
	require.NoError(t, err)
	t.Cleanup(sub.Unsubscribe)

	eventCh := runSubscription(ctx, sub)

	next := getNextEvent(t, eventCh)
	require.Equal(t, &PayloadEvents{
		Items: []Event{a1, b1},
	}, next.Payload)

	next = getNextEvent(t, eventCh)
	require.True(t, next.IsEndOfSnapshot(), "expected end of snapshot")

	publisher.Publish([]Event{a2, b2})
	next = getNextEvent(t, eventCh)
	require.Equal(t, &PayloadEvents{
		Items: []Event{a2, b2},
	}, next.Payload)
}

func TestEventPublisher_Publish_WildcardNotAllowed(t *testing.T) {
	publisher := NewEventPublisher(0)

	require.Panics(t, func() {
		publisher.Publish([]Event{
			{
				Topic:   testTopic,
				Payload: wildcardPayload{},
			},
		})
	})
}

type wildcardPayload struct{}

func (wildcardPayload) Subject() Subject                              { return SubjectWildcard }
func (wildcardPayload) HasReadPermission(acl.Authorizer) bool         { return true }
func (wildcardPayload) ToSubscriptionEvent(uint64) *pbsubscribe.Event { return &pbsubscribe.Event{} }

func TestEventPublisher_SnapshotIndex0(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := NewEventPublisher(10 * time.Second)
	go publisher.Run(ctx)

	publisher.RegisterHandler(testTopic, func(SubscribeRequest, SnapshotAppender) (uint64, error) {
		return 0, nil
	}, false)

	sub, err := publisher.Subscribe(&SubscribeRequest{
		Topic:   testTopic,
		Subject: StringSubject("sub-key"),
	})
	require.NoError(t, err)
	t.Cleanup(sub.Unsubscribe)

	eventCh := runSubscription(ctx, sub)
	event := getNextEvent(t, eventCh)
	require.True(t, event.IsEndOfSnapshot())

	// Even though the snapshot handler returned 0, the subscriber shouldn't see it.
	require.Equal(t, uint64(1), event.Index)
}
