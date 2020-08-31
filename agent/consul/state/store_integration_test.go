package state

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestStore_IntegrationWithEventPublisher_ACLTokenUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testACLTokensStateStore(t)

	// Setup token and wait for good state
	token := createTokenAndWaitForACLEventPublish(t, s)

	// Register the subscription.
	subscription := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := stream.NewEventPublisher(ctx, newTestSnapshotHandlers(s), 0)
	s.db.publisher = publisher
	sub, err := publisher.Subscribe(subscription)
	require.NoError(err)
	defer sub.Unsubscribe()

	eventCh := testRunSub(sub)

	// Stream should get EndOfSnapshot
	e := assertEvent(t, eventCh)
	require.True(e.IsEndOfSnapshot())

	// Update an unrelated token.
	token2 := &structs.ACLToken{
		AccessorID: "a7bbf480-8440-4f55-acfc-6fdca25cb13e",
		SecretID:   "72e81982-7a0f-491f-a60e-c9c802ac1402",
	}
	token2.SetHash(false)
	require.NoError(s.ACLTokenSet(3, token2.Clone(), false))

	// Ensure there's no reset event.
	assertNoEvent(t, eventCh)

	// Now update the token used in the subscriber.
	token3 := &structs.ACLToken{
		AccessorID:  "3af117a9-2233-4cf4-8ff8-3c749c9906b4",
		SecretID:    "4268ce0d-d7ae-4718-8613-42eba9036020",
		Description: "something else",
	}
	token3.SetHash(false)
	require.NoError(s.ACLTokenSet(4, token3.Clone(), false))

	// Ensure the reset event was sent.
	err = assertErr(t, eventCh)
	require.Equal(stream.ErrSubscriptionClosed, err)

	// Register another subscription.
	subscription2 := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	sub2, err := publisher.Subscribe(subscription2)
	require.NoError(err)
	defer sub2.Unsubscribe()

	eventCh2 := testRunSub(sub2)

	// Expect initial EoS
	e = assertEvent(t, eventCh2)
	require.True(e.IsEndOfSnapshot())

	// Delete the unrelated token.
	require.NoError(s.ACLTokenDeleteByAccessor(5, token2.AccessorID, nil))

	// Ensure there's no reset event.
	assertNoEvent(t, eventCh2)

	// Delete the token used by the subscriber.
	require.NoError(s.ACLTokenDeleteByAccessor(6, token.AccessorID, nil))

	// Ensure the reset event was sent.
	err = assertErr(t, eventCh2)
	require.Equal(stream.ErrSubscriptionClosed, err)
}

func TestStore_IntegrationWithEventPublisher_ACLPolicyUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testACLTokensStateStore(t)

	// Create token and wait for good state
	token := createTokenAndWaitForACLEventPublish(t, s)

	// Register the subscription.
	subscription := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := stream.NewEventPublisher(ctx, newTestSnapshotHandlers(s), 0)
	s.db.publisher = publisher
	sub, err := publisher.Subscribe(subscription)
	require.NoError(err)
	defer sub.Unsubscribe()

	eventCh := testRunSub(sub)

	// Ignore the end of snapshot event
	e := assertEvent(t, eventCh)
	require.True(e.IsEndOfSnapshot(), "event should be a EoS got %v", e)

	// Update an unrelated policy.
	policy2 := structs.ACLPolicy{
		ID:          testPolicyID_C,
		Name:        "foo-read",
		Rules:       `node "foo" { policy = "read" }`,
		Syntax:      acl.SyntaxCurrent,
		Datacenters: []string{"dc1"},
	}
	policy2.SetHash(false)
	require.NoError(s.ACLPolicySet(3, &policy2))

	// Ensure there's no reset event.
	assertNoEvent(t, eventCh)

	// Now update the policy used in the subscriber.
	policy3 := structs.ACLPolicy{
		ID:          testPolicyID_A,
		Name:        "node-read",
		Rules:       `node_prefix "" { policy = "write" }`,
		Syntax:      acl.SyntaxCurrent,
		Datacenters: []string{"dc1"},
	}
	policy3.SetHash(false)
	require.NoError(s.ACLPolicySet(4, &policy3))

	// Ensure the reset event was sent.
	assertReset(t, eventCh, true)

	// Register another subscription.
	subscription2 := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	sub, err = publisher.Subscribe(subscription2)
	require.NoError(err)

	eventCh = testRunSub(sub)

	// Ignore the end of snapshot event
	e = assertEvent(t, eventCh)
	require.True(e.IsEndOfSnapshot(), "event should be a EoS got %v", e)

	// Delete the unrelated policy.
	require.NoError(s.ACLPolicyDeleteByID(5, testPolicyID_C, nil))

	// Ensure there's no reload event.
	assertNoEvent(t, eventCh)

	// Delete the policy used by the subscriber.
	require.NoError(s.ACLPolicyDeleteByID(6, testPolicyID_A, nil))

	// Ensure the reload event was sent.
	err = assertErr(t, eventCh)
	require.Equal(stream.ErrSubscriptionClosed, err)

	// Register another subscription.
	subscription3 := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	sub, err = publisher.Subscribe(subscription3)
	require.NoError(err)
	defer sub.Unsubscribe()

	eventCh = testRunSub(sub)

	// Ignore the end of snapshot event
	e = assertEvent(t, eventCh)
	require.True(e.IsEndOfSnapshot(), "event should be a EoS got %v", e)

	// Now update the policy used in role B, but not directly in the token.
	policy4 := structs.ACLPolicy{
		ID:          testPolicyID_B,
		Name:        "node-read",
		Rules:       `node_prefix "foo" { policy = "read" }`,
		Syntax:      acl.SyntaxCurrent,
		Datacenters: []string{"dc1"},
	}
	policy4.SetHash(false)
	require.NoError(s.ACLPolicySet(7, &policy4))

	// Ensure the reset event was sent.
	assertReset(t, eventCh, true)
}

func TestStore_IntegrationWithEventPublisher_ACLRoleUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testACLTokensStateStore(t)

	// Create token and wait for good state
	token := createTokenAndWaitForACLEventPublish(t, s)

	// Register the subscription.
	subscription := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := stream.NewEventPublisher(ctx, newTestSnapshotHandlers(s), 0)
	s.db.publisher = publisher
	sub, err := publisher.Subscribe(subscription)
	require.NoError(err)

	eventCh := testRunSub(sub)

	// Stream should get EndOfSnapshot
	e := assertEvent(t, eventCh)
	require.True(e.IsEndOfSnapshot())

	// Update an unrelated role (the token has role testRoleID_B).
	role := structs.ACLRole{
		ID:          testRoleID_A,
		Name:        "unrelated-role",
		Description: "test",
	}
	role.SetHash(false)
	require.NoError(s.ACLRoleSet(3, &role))

	// Ensure there's no reload event.
	assertNoEvent(t, eventCh)

	// Now update the role used by the token in the subscriber.
	role2 := structs.ACLRole{
		ID:          testRoleID_B,
		Name:        "my-new-role",
		Description: "changed",
	}
	role2.SetHash(false)
	require.NoError(s.ACLRoleSet(4, &role2))

	// Ensure the reload event was sent.
	assertReset(t, eventCh, false)

	// Register another subscription.
	subscription2 := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	sub, err = publisher.Subscribe(subscription2)
	require.NoError(err)

	eventCh = testRunSub(sub)

	// Ignore the end of snapshot event
	e = assertEvent(t, eventCh)
	require.True(e.IsEndOfSnapshot(), "event should be a EoS got %v", e)

	// Delete the unrelated policy.
	require.NoError(s.ACLRoleDeleteByID(5, testRoleID_A, nil))

	// Ensure there's no reload event.
	assertNoEvent(t, eventCh)

	// Delete the policy used by the subscriber.
	require.NoError(s.ACLRoleDeleteByID(6, testRoleID_B, nil))

	// Ensure the reload event was sent.
	assertReset(t, eventCh, false)
}

type nextResult struct {
	Events []stream.Event
	Err    error
}

func testRunSub(sub *stream.Subscription) <-chan nextResult {
	eventCh := make(chan nextResult, 1)
	go func() {
		for {
			es, err := sub.Next(context.TODO())
			eventCh <- nextResult{
				Events: es,
				Err:    err,
			}
			if err != nil {
				return
			}
		}
	}()
	return eventCh
}

func assertNoEvent(t *testing.T, eventCh <-chan nextResult) {
	t.Helper()
	select {
	case next := <-eventCh:
		require.NoError(t, next.Err)
		require.Len(t, next.Events, 1)
		t.Fatalf("got unwanted event: %#v", next.Events[0].Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

func assertEvent(t *testing.T, eventCh <-chan nextResult) *stream.Event {
	t.Helper()
	select {
	case next := <-eventCh:
		require.NoError(t, next.Err)
		require.Len(t, next.Events, 1)
		return &next.Events[0]
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("no event after 100ms")
	}
	return nil
}

func assertErr(t *testing.T, eventCh <-chan nextResult) error {
	t.Helper()
	select {
	case next := <-eventCh:
		require.Error(t, next.Err)
		return next.Err
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("no err after 100ms")
	}
	return nil
}

// assertReset checks that a ResetStream event is send to the subscription
// within 100ms. If allowEOS is true it will ignore any intermediate events that
// come before the reset provided they are EndOfSnapshot events because in many
// cases it's non-deterministic whether the snapshot will complete before the
// acl reset is handled.
func assertReset(t *testing.T, eventCh <-chan nextResult, allowEOS bool) {
	t.Helper()
	for {
		select {
		case next := <-eventCh:
			if allowEOS {
				if next.Err == nil && len(next.Events) == 1 && next.Events[0].IsEndOfSnapshot() {
					continue
				}
			}
			require.Error(t, next.Err)
			require.Equal(t, stream.ErrSubscriptionClosed, next.Err)
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("no err after 100ms")
		}
	}
}

var topicService stream.Topic = topic("test-topic-service")

func newTestSnapshotHandlers(s *Store) stream.SnapshotHandlers {
	return stream.SnapshotHandlers{
		topicService: func(req *stream.SubscribeRequest, snap stream.SnapshotAppender) (uint64, error) {
			idx, nodes, err := s.ServiceNodes(nil, req.Key, nil)
			if err != nil {
				return idx, err
			}

			for _, node := range nodes {
				event := stream.Event{
					Topic:   req.Topic,
					Key:     req.Key,
					Index:   node.ModifyIndex,
					Payload: node,
				}
				snap.Append([]stream.Event{event})
			}
			return idx, nil
		},
	}
}

func createTokenAndWaitForACLEventPublish(t *testing.T, s *Store) *structs.ACLToken {
	token := &structs.ACLToken{
		AccessorID:  "3af117a9-2233-4cf4-8ff8-3c749c9906b4",
		SecretID:    "4268ce0d-d7ae-4718-8613-42eba9036020",
		Description: "something",
		Policies: []structs.ACLTokenPolicyLink{
			{ID: testPolicyID_A},
		},
		Roles: []structs.ACLTokenRoleLink{
			{ID: testRoleID_B},
		},
	}
	token.SetHash(false)

	// If we subscribe immediately after we create a token we race with the
	// publisher that is publishing the ACL token event for the token we just
	// created. That means that the subscription we create right after will often
	// be immediately reset. The most reliable way to avoid that without just
	// sleeping for some arbitrary time is to pre-subscribe using the token before
	// it actually exists (which works because the publisher doesn't check tokens
	// it assumes something lower down did that) and then wait for it to be reset
	// so we know the initial token write event has been sent out before
	// continuing...
	req := &stream.SubscribeRequest{
		Topic: topicService,
		Key:   "nope",
		Token: token.SecretID,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := stream.NewEventPublisher(ctx, newTestSnapshotHandlers(s), 0)
	s.db.publisher = publisher
	sub, err := publisher.Subscribe(req)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	eventCh := testRunSub(sub)

	// Create the ACL token to be used in the subscription.
	require.NoError(t, s.ACLTokenSet(2, token.Clone(), false))

	// Wait for the pre-subscription to be reset
	assertReset(t, eventCh, true)

	return token
}
