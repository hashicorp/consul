package submatview

import (
	"context"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

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
