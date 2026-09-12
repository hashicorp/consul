// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// fastTestWaiter returns a retry.Waiter that never leaves its MinWait (zero
// delay) regime within the small number of retries these tests exercise, so
// they run quickly and deterministically.
func fastTestWaiter() *retry.Waiter {
	return &retry.Waiter{MinFailures: 10, MaxWait: 10 * time.Millisecond}
}

func TestLocalMaterializer(t *testing.T) {
	const (
		index = 123
		topic = pbsubscribe.Topic_ServiceResolver
		key   = "web"
		token = "some-acl-token"
	)

	var (
		snapshotEvent = stream.Event{
			Topic: topic,
			Index: index,
			Payload: state.EventPayloadConfigEntry{
				Value: &structs.ServiceResolverConfigEntry{
					Name: key,
					Meta: map[string]string{"snapshot": "true"},
				},
			},
		}

		publishedEvent1 = stream.Event{
			Topic: topic,
			Index: index + 1,
			Payload: state.EventPayloadConfigEntry{
				Value: &structs.ServiceResolverConfigEntry{
					Name: key,
					Meta: map[string]string{"published": "true"},
				},
			},
		}

		publishedEvent2 = stream.Event{
			Topic: topic,
			Index: index + 2,
			Payload: state.EventPayloadConfigEntry{
				Value: &structs.ServiceResolverConfigEntry{
					Name: key,
					Meta: map[string]string{"published": "true"},
				},
			},
		}
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := stream.NewEventPublisher(10 * time.Second)
	publisher.RegisterHandler(topic, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		buf.Append([]stream.Event{snapshotEvent})
		return index, nil
	}, false)
	go publisher.Run(ctx)

	// This allows us to swap the authorizer out at runtime.
	authz := &struct{ acl.Authorizer }{acl.AllowAll()}

	aclResolver := NewMockACLResolver(t)
	aclResolver.On("ResolveTokenAndDefaultMeta", token, mock.Anything, mock.Anything).
		Return(resolver.Result{Authorizer: authz}, nil)

	view := newTestView()

	m := NewLocalMaterializer(LocalMaterializerDeps{
		Backend:     publisher,
		ACLResolver: aclResolver,
		Deps: Deps{
			View: view,
			Request: func(index uint64) *pbsubscribe.SubscribeRequest {
				return &pbsubscribe.SubscribeRequest{
					Topic: topic,
					Index: index,
					Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
						NamedSubject: &pbsubscribe.NamedSubject{
							Key: key,
						},
					},
					Token: token,
				}
			},
		},
	})
	go m.Run(ctx)

	// Check that the view received the snapshot events.
	events := view.getEvents(t)
	require.Len(t, events, 1)
	require.Equal(t, snapshotEvent.Payload.ToSubscriptionEvent(index), events[0])

	publisher.Publish([]stream.Event{publishedEvent1})

	// Check that the view received the published events.
	events = view.getEvents(t)
	require.Len(t, events, 1)
	require.Equal(t, publishedEvent1.Payload.ToSubscriptionEvent(index+1), events[0])

	// Replace the authorizer and check that we don't receive newly published events.
	authz.Authorizer = acl.DenyAll()
	publisher.Publish([]stream.Event{publishedEvent2})
	view.expectNoEvents(t)
}

// TestLocalMaterializer_ToleratesTransientACLNotFound verifies that a
// subscription which sees a few consecutive "ACL not found" errors (e.g. from
// a Raft follower momentarily behind on replication) recovers on its own and
// keeps running, instead of being evicted after the very first occurrence.
func TestLocalMaterializer_ToleratesTransientACLNotFound(t *testing.T) {
	const (
		index = 123
		topic = pbsubscribe.Topic_ServiceResolver
		key   = "web"
		token = "some-acl-token"
	)

	snapshotEvent := stream.Event{
		Topic: topic,
		Index: index,
		Payload: state.EventPayloadConfigEntry{
			Value: &structs.ServiceResolverConfigEntry{
				Name: key,
				Meta: map[string]string{"snapshot": "true"},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := stream.NewEventPublisher(10 * time.Second)
	publisher.RegisterHandler(topic, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		buf.Append([]stream.Event{snapshotEvent})
		return index, nil
	}, false)
	go publisher.Run(ctx)

	authz := &struct{ acl.Authorizer }{acl.AllowAll()}

	aclResolver := NewMockACLResolver(t)
	// Fail with a transient "ACL not found" for the first
	// acl.MaxConsecutiveNotFoundTolerance-1 attempts, then succeed.
	for i := 0; i < acl.MaxConsecutiveNotFoundTolerance-1; i++ {
		aclResolver.On("ResolveTokenAndDefaultMeta", token, mock.Anything, mock.Anything).
			Return(resolver.Result{}, acl.ErrNotFound).Once()
	}
	aclResolver.On("ResolveTokenAndDefaultMeta", token, mock.Anything, mock.Anything).
		Return(resolver.Result{Authorizer: authz}, nil)

	view := newTestView()

	m := NewLocalMaterializer(LocalMaterializerDeps{
		Backend:     publisher,
		ACLResolver: aclResolver,
		Deps: Deps{
			View:   view,
			Logger: hclog.New(nil),
			Waiter: fastTestWaiter(),
			Request: func(index uint64) *pbsubscribe.SubscribeRequest {
				return &pbsubscribe.SubscribeRequest{
					Topic: topic,
					Index: index,
					Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
						NamedSubject: &pbsubscribe.NamedSubject{
							Key: key,
						},
					},
					Token: token,
				}
			},
		},
	})

	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()

	// Despite the earlier transient not-found errors, the subscription should
	// recover on its own and deliver the snapshot.
	events := view.getEvents(t)
	require.Len(t, events, 1)
	require.Equal(t, snapshotEvent.Payload.ToSubscriptionEvent(index), events[0])

	select {
	case <-done:
		t.Fatal("materializer should still be running, not evicted, after a below-threshold streak")
	default:
	}
}

// TestLocalMaterializer_TerminatesAfterSustainedACLNotFound verifies that a
// subscription is still correctly evicted once "ACL not found" occurs
// acl.MaxConsecutiveNotFoundTolerance times in a row with no intervening
// success - e.g. a genuinely deleted token - preserving the original behavior
// this mechanism exists for.
func TestLocalMaterializer_TerminatesAfterSustainedACLNotFound(t *testing.T) {
	const (
		topic = pbsubscribe.Topic_ServiceResolver
		key   = "web"
		token = "some-acl-token"
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := stream.NewEventPublisher(10 * time.Second)
	publisher.RegisterHandler(topic, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return 0, nil
	}, false)
	go publisher.Run(ctx)

	aclResolver := NewMockACLResolver(t)
	aclResolver.On("ResolveTokenAndDefaultMeta", token, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)

	view := newTestView()

	m := NewLocalMaterializer(LocalMaterializerDeps{
		Backend:     publisher,
		ACLResolver: aclResolver,
		Deps: Deps{
			View:   view,
			Logger: hclog.New(nil),
			Waiter: fastTestWaiter(),
			Request: func(index uint64) *pbsubscribe.SubscribeRequest {
				return &pbsubscribe.SubscribeRequest{
					Topic: topic,
					Index: index,
					Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
						NamedSubject: &pbsubscribe.NamedSubject{
							Key: key,
						},
					},
					Token: token,
				}
			},
		},
	})

	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("materializer should have been evicted after a sustained ACL not-found streak")
	}
}

func newTestView() *testView {
	return &testView{
		eventsCh: make(chan []*pbsubscribe.Event),
	}
}

type testView struct {
	eventsCh chan []*pbsubscribe.Event
}

func (testView) Reset() {}

func (testView) Result(uint64) any { return nil }

func (v *testView) Update(events []*pbsubscribe.Event) error {
	v.eventsCh <- events
	return nil
}

func (v *testView) getEvents(t *testing.T) []*pbsubscribe.Event {
	t.Helper()

	select {
	case events := <-v.eventsCh:
		return events
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for view to receive events")
		return nil
	}
}

func (v *testView) expectNoEvents(t *testing.T) {
	t.Helper()

	select {
	case <-v.eventsCh:
		t.Fatal("expected no events to be received")
	case <-time.After(100 * time.Millisecond):
	}
}
