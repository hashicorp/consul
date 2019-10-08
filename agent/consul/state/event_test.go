package state

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestPublisher_ACLTokenUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testACLTokensStateStore(t)

	// Create the ACL token to be used in the subscription.
	token := &structs.ACLToken{
		AccessorID:  "3af117a9-2233-4cf4-8ff8-3c749c9906b4",
		SecretID:    "4268ce0d-d7ae-4718-8613-42eba9036020",
		Description: "something",
	}
	require.NoError(s.ACLTokenSet(2, token.Clone(), false))

	// Register the subscription.
	subscription := &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   "nope",
		Token: token.SecretID,
	}
	eventCh, err := s.publisher.Subscribe(subscription)
	require.NoError(err)

	// Ignore the initial acl update event if we see it.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
	}

	// Update an unrelated token.
	token2 := &structs.ACLToken{
		AccessorID: "a7bbf480-8440-4f55-acfc-6fdca25cb13e",
		SecretID:   "72e81982-7a0f-491f-a60e-c9c802ac1402",
	}
	require.NoError(s.ACLTokenSet(3, token2.Clone(), false))

	// Ensure there's no reload event.
	select {
	case e := <-eventCh:
		t.Fatalf("got unwanted event: %#v", e.GetPayload())
	case <-time.After(100 * time.Millisecond):
	}

	// Now update the token used in the subscriber.
	token3 := &structs.ACLToken{
		AccessorID:  "3af117a9-2233-4cf4-8ff8-3c749c9906b4",
		SecretID:    "4268ce0d-d7ae-4718-8613-42eba9036020",
		Description: "something else",
	}
	require.NoError(s.ACLTokenSet(4, token3.Clone(), false))

	// Ensure the reload event was sent.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not get event")
	}

	// Register another subscription.
	subscription2 := &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   "nope",
		Token: token.SecretID,
	}
	eventCh, err = s.publisher.Subscribe(subscription2)
	require.NoError(err)

	// Delete the unrelated token.
	require.NoError(s.ACLTokenDeleteByAccessor(5, token2.AccessorID))

	// Ensure there's no reload event.
	select {
	case e := <-eventCh:
		t.Fatalf("got unwanted event: %v", e)
	case <-time.After(100 * time.Millisecond):
	}

	// Delete the token used by the subscriber.
	require.NoError(s.ACLTokenDeleteByAccessor(6, token.AccessorID))

	// Ensure the reload event was sent.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not get event")
	}
}

func TestPublisher_ACLPolicyUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testACLTokensStateStore(t)

	// Create the ACL token to be used in the subscription.
	token := &structs.ACLToken{
		AccessorID:  "3af117a9-2233-4cf4-8ff8-3c749c9906b4",
		SecretID:    "4268ce0d-d7ae-4718-8613-42eba9036020",
		Description: "something",
		Policies: []structs.ACLTokenPolicyLink{
			structs.ACLTokenPolicyLink{
				ID: testPolicyID_A,
			},
		},
	}
	require.NoError(s.ACLTokenSet(2, token.Clone(), false))

	// Register the subscription.
	subscription := &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   "nope",
		Token: token.SecretID,
	}
	eventCh, err := s.publisher.Subscribe(subscription)
	require.NoError(err)

	// Ignore the initial acl update event if we see it.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
	}

	// Update an unrelated policy.
	policy2 := structs.ACLPolicy{
		ID:          testPolicyID_B,
		Name:        "foo-read",
		Rules:       `node "foo" { policy = "read" }`,
		Syntax:      acl.SyntaxCurrent,
		Datacenters: []string{"dc1"},
	}
	require.NoError(s.ACLPolicySet(3, &policy2))

	// Ensure there's no reload event.
	select {
	case e := <-eventCh:
		t.Fatalf("got unwanted event: %#v", e.GetPayload())
	case <-time.After(100 * time.Millisecond):
	}

	// Now update the policy used in the subscriber.
	policy3 := structs.ACLPolicy{
		ID:          testPolicyID_A,
		Name:        "node-read",
		Rules:       `node_prefix "" { policy = "write" }`,
		Syntax:      acl.SyntaxCurrent,
		Datacenters: []string{"dc1"},
	}
	require.NoError(s.ACLPolicySet(4, &policy3))

	// Ensure the reload event was sent.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not get event")
	}

	// Register another subscription.
	subscription2 := &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   "nope",
		Token: token.SecretID,
	}
	eventCh, err = s.publisher.Subscribe(subscription2)
	require.NoError(err)

	// Delete the unrelated policy.
	require.NoError(s.ACLPolicyDeleteByID(5, testPolicyID_B))

	// Ensure there's no reload event.
	select {
	case e := <-eventCh:
		t.Fatalf("got unwanted event: %v", e)
	case <-time.After(100 * time.Millisecond):
	}

	// Delete the policy used by the subscriber.
	require.NoError(s.ACLPolicyDeleteByID(6, testPolicyID_A))

	// Ensure the reload event was sent.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not get event")
	}
}

func TestPublisher_ACLRoleUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testACLTokensStateStore(t)

	// Create the ACL token to be used in the subscription.
	token := &structs.ACLToken{
		AccessorID:  "3af117a9-2233-4cf4-8ff8-3c749c9906b4",
		SecretID:    "4268ce0d-d7ae-4718-8613-42eba9036020",
		Description: "something",
		Roles: []structs.ACLTokenRoleLink{
			structs.ACLTokenRoleLink{
				ID: testRoleID_A,
			},
		},
	}
	require.NoError(s.ACLTokenSet(2, token.Clone(), false))

	// Register the subscription.
	subscription := &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   "nope",
		Token: token.SecretID,
	}
	eventCh, err := s.publisher.Subscribe(subscription)
	require.NoError(err)

	// Ignore the initial acl update event if we see it.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
	}

	// Update an unrelated role.
	role := structs.ACLRole{
		ID:          testRoleID_B,
		Name:        "unrelated-role",
		Description: "test",
	}
	require.NoError(s.ACLRoleSet(3, &role))

	// Ensure there's no reload event.
	select {
	case e := <-eventCh:
		t.Fatalf("got unwanted event: %#v", e.GetPayload())
	case <-time.After(100 * time.Millisecond):
	}

	// Now update the role used by the token in the subscriber.
	role2 := structs.ACLRole{
		ID:          testRoleID_A,
		Name:        "my-new-role",
		Description: "changed",
	}
	require.NoError(s.ACLRoleSet(4, &role2))

	// Ensure the reload event was sent.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not get event")
	}

	// Register another subscription.
	subscription2 := &stream.SubscribeRequest{
		Topic: stream.Topic_ServiceHealth,
		Key:   "nope",
		Token: token.SecretID,
	}
	eventCh, err = s.publisher.Subscribe(subscription2)
	require.NoError(err)

	// Delete the unrelated policy.
	require.NoError(s.ACLRoleDeleteByID(5, testRoleID_B))

	// Ensure there's no reload event.
	select {
	case e := <-eventCh:
		t.Fatalf("got unwanted event: %v", e)
	case <-time.After(100 * time.Millisecond):
	}

	// Delete the policy used by the subscriber.
	require.NoError(s.ACLRoleDeleteByID(6, testRoleID_A))

	// Ensure the reload event was sent.
	select {
	case e := <-eventCh:
		require.True(e.GetReloadStream())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not get event")
	}
}
