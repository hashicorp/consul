package stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type intTopic int

func (i intTopic) String() string {
	return fmt.Sprintf("%d", i)
}

var testTopic Topic = intTopic(999)

func TestEventPublisher_SubscribeWithIndex0(t *testing.T) {
	req := &SubscribeRequest{
		Topic: testTopic,
		Key:   "sub-key",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(newTestSnapshotHandlers(), 0)
	go publisher.Run(ctx)

	sub, err := publisher.Subscribe(req)
	require.NoError(t, err)
	eventCh := runSubscription(ctx, sub)

	next := getNextEvent(t, eventCh)
	require.Equal(t, testSnapshotEvent, next)

	next = getNextEvent(t, eventCh)
	require.True(t, next.IsEndOfSnapshot())

	assertNoResult(t, eventCh)

	events := []Event{{
		Topic:   testTopic,
		Key:     "sub-key",
		Payload: "the-published-event-payload",
	}}
	publisher.Publish(events)

	// Subscriber should see the published event
	next = getNextEvent(t, eventCh)
	expected := Event{Payload: "the-published-event-payload", Key: "sub-key", Topic: testTopic}
	require.Equal(t, expected, next)
}

var testSnapshotEvent = Event{
	Topic:   testTopic,
	Payload: "snapshot-event-payload",
	Key:     "sub-key",
	Index:   1,
}

func newTestSnapshotHandlers() SnapshotHandlers {
	return SnapshotHandlers{
		testTopic: func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
			if req.Topic != testTopic {
				return 0, fmt.Errorf("unexpected topic: %v", req.Topic)
			}
			buf.Append([]Event{testSnapshotEvent})
			return 1, nil
		},
	}
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

	handlers := newTestSnapshotHandlers()
	fn := func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		return 0, nil
	}
	handlers[intTopic(22)] = fn
	handlers[intTopic(33)] = fn

	publisher := NewEventPublisher(handlers, time.Second)
	go publisher.Run(ctx)

	sub1, err := publisher.Subscribe(&SubscribeRequest{Topic: intTopic(22)})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	sub2, err := publisher.Subscribe(&SubscribeRequest{Topic: intTopic(33)})
	require.NoError(t, err)
	defer sub2.Unsubscribe()

	cancel() // Shutdown

	err = consumeSub(context.Background(), sub1)
	require.Equal(t, err, ErrSubscriptionClosed)

	_, err = sub2.Next(context.Background())
	require.Equal(t, err, ErrSubscriptionClosed)
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
		Topic: testTopic,
		Key:   "sub-key",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(newTestSnapshotHandlers(), time.Second)
	go publisher.Run(ctx)
	_, err := publisher.Subscribe(req)
	require.NoError(t, err)

	publisher.snapshotHandlers[testTopic] = func(_ SubscribeRequest, _ SnapshotAppender) (uint64, error) {
		return 0, fmt.Errorf("error should not be seen, cache should have been used")
	}

	sub, err := publisher.Subscribe(req)
	require.NoError(t, err)

	eventCh := runSubscription(ctx, sub)
	next := getNextEvent(t, eventCh)
	require.Equal(t, testSnapshotEvent, next)

	next = getNextEvent(t, eventCh)
	require.True(t, next.IsEndOfSnapshot())

	// Now subscriber should block waiting for updates
	assertNoResult(t, eventCh)

	expected := Event{
		Topic:   testTopic,
		Key:     "sub-key",
		Payload: "the-published-event-payload",
		Index:   3,
	}
	publisher.Publish([]Event{expected})

	// Subscriber should see the published event
	next = getNextEvent(t, eventCh)
	require.Equal(t, expected, next)
}

func TestEventPublisher_SubscribeWithIndexNotZero_CanResume(t *testing.T) {
	req := &SubscribeRequest{
		Topic: testTopic,
		Key:   "sub-key",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(newTestSnapshotHandlers(), time.Second)
	go publisher.Run(ctx)
	// Include the same event in the topicBuffer
	publisher.publishEvent([]Event{testSnapshotEvent})

	runStep(t, "start a subscription and unsub", func(t *testing.T) {
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

	runStep(t, "resume the subscription", func(t *testing.T) {
		newReq := *req
		newReq.Index = 1
		sub, err := publisher.Subscribe(&newReq)
		require.NoError(t, err)

		eventCh := runSubscription(ctx, sub)
		assertNoResult(t, eventCh)

		expected := Event{
			Topic:   testTopic,
			Key:     "sub-key",
			Index:   3,
			Payload: "event-3",
		}
		publisher.publishEvent([]Event{expected})

		next := getNextEvent(t, eventCh)
		require.Equal(t, expected, next)
	})
}

func TestEventPublisher_SubscribeWithIndexNotZero_NewSnapshot(t *testing.T) {
	req := &SubscribeRequest{
		Topic: testTopic,
		Key:   "sub-key",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(newTestSnapshotHandlers(), 0)
	go publisher.Run(ctx)
	// Include the same event in the topicBuffer
	publisher.publishEvent([]Event{testSnapshotEvent})

	runStep(t, "start a subscription and unsub", func(t *testing.T) {
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
		Key:     "sub-key",
		Index:   3,
		Payload: "event-3",
	}

	runStep(t, "publish an event while unsubed", func(t *testing.T) {
		publisher.publishEvent([]Event{nextEvent})
	})

	runStep(t, "resume the subscription", func(t *testing.T) {
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
		Topic: testTopic,
		Key:   "sub-key",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(newTestSnapshotHandlers(), time.Second)
	go publisher.Run(ctx)
	// Include the same event in the topicBuffer
	publisher.publishEvent([]Event{testSnapshotEvent})

	runStep(t, "start a subscription and unsub", func(t *testing.T) {
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
		Key:     "sub-key",
		Index:   3,
		Payload: "event-3",
	}

	runStep(t, "publish an event while unsubed", func(t *testing.T) {
		publisher.publishEvent([]Event{nextEvent})
	})

	publisher.snapshotHandlers[testTopic] = func(_ SubscribeRequest, _ SnapshotAppender) (uint64, error) {
		return 0, fmt.Errorf("error should not be seen, cache should have been used")
	}

	runStep(t, "resume the subscription", func(t *testing.T) {
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

		next = getNextEvent(t, eventCh)
		require.Equal(t, nextEvent, next)
	})
}

func runStep(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}
