package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
)

// Test basic creation
func TestIntentionApply_new(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string

	// Record now to check created at time
	now := time.Now()

	// Create
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(reply)

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(resp.Index, actual.ModifyIndex)
		require.WithinDuration(now, actual.CreatedAt, 5*time.Second)
		require.WithinDuration(now, actual.UpdatedAt, 5*time.Second)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		actual.Hash = ixn.Intention.Hash
		ixn.Intention.UpdatePrecedence()
		require.Equal(ixn.Intention, actual)
	}
}

// Test the source type defaults
func TestIntentionApply_defaultSourceType(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
		},
	}
	var reply string

	// Create
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(reply)

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(structs.IntentionSourceConsul, actual.SourceType)
	}
}

// Shouldn't be able to create with an ID set
func TestIntentionApply_createWithID(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			ID:         generateUUID(),
			SourceName: "test",
		},
	}
	var reply string

	// Create
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.NotNil(err)
	require.Contains(err, "ID must be empty")
}

// Test basic updating
func TestIntentionApply_updateGood(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string

	// Create
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(reply)

	// Read CreatedAt
	var createdAt time.Time
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		createdAt = actual.CreatedAt
	}

	// Sleep a bit so that the updated at will definitely be different, not much
	time.Sleep(1 * time.Millisecond)

	// Update
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.SourceName = "*"
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(createdAt, actual.CreatedAt)
		require.WithinDuration(time.Now(), actual.UpdatedAt, 5*time.Second)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		actual.Hash = ixn.Intention.Hash
		ixn.Intention.UpdatePrecedence()
		require.Equal(ixn.Intention, actual)
	}
}

// Shouldn't be able to update a non-existent intention
func TestIntentionApply_updateNonExist(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpUpdate,
		Intention: &structs.Intention{
			ID:         generateUUID(),
			SourceName: "test",
		},
	}
	var reply string

	// Create
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.NotNil(err)
	require.Contains(err, "Cannot modify non-existent intention")
}

// Test basic deleting
func TestIntentionApply_deleteGood(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceName:      "test",
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
		},
	}
	var reply string

	// Create
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(reply)

	// Delete
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.NotNil(err)
		require.Contains(err, ErrIntentionNotFound.Error())
	}
}

// Test apply with a deny ACL
func TestIntentionApply_aclDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with write permissions
	var token string
	{
		var rules = `
service "foo" {
	policy = "deny"
	intentions = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"

	// Create without a token should error since default deny
	var reply string
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(acl.IsErrPermissionDenied(err))

	// Now add the token and try again.
	ixn.WriteRequest.Token = token
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  ixn.Intention.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedIntentions
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(resp.Index, actual.ModifyIndex)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		actual.Hash = ixn.Intention.Hash
		ixn.Intention.UpdatePrecedence()
		require.Equal(ixn.Intention, actual)
	}
}

func TestIntention_WildcardACLEnforcement(t *testing.T) {
	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	// create some test policies.

	writeToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service_prefix "" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	readToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service_prefix "" { policy = "deny" intentions = "read" }`)
	require.NoError(t, err)
	exactToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service "*" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	wildcardPrefixToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service_prefix "*" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	fooToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service "foo" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	denyToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service_prefix "" { policy = "deny" intentions = "deny" }`)
	require.NoError(t, err)

	doIntentionCreate := func(t *testing.T, token string, deny bool) string {
		t.Helper()
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention: &structs.Intention{
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "*",
				Action:          structs.IntentionActionAllow,
				SourceType:      structs.IntentionSourceConsul,
			},
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
			return ""
		} else {
			require.NoError(t, err)
			require.NotEmpty(t, reply)
			return reply
		}
	}

	t.Run("deny-write-for-read-token", func(t *testing.T) {
		// This tests ensures that tokens with only read access to all intentions
		// cannot create a wildcard intention
		doIntentionCreate(t, readToken.SecretID, true)
	})

	t.Run("deny-write-for-exact-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service "*" {
		//    intentions = "write"
		// }
		// will not actually allow creating an intention with a wildcard service name
		doIntentionCreate(t, exactToken.SecretID, true)
	})

	t.Run("deny-write-for-prefix-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service_prefix "*" {
		//    intentions = "write"
		// }
		// will not actually allow creating an intention with a wildcard service name
		doIntentionCreate(t, wildcardPrefixToken.SecretID, true)
	})

	var intentionID string
	allowWriteOk := t.Run("allow-write", func(t *testing.T) {
		// tests that a token with all the required privileges can create
		// intentions with a wildcard destination
		intentionID = doIntentionCreate(t, writeToken.SecretID, false)
	})

	requireAllowWrite := func(t *testing.T) {
		t.Helper()
		if !allowWriteOk {
			t.Skip("Skipping because the allow-write subtest failed")
		}
	}

	doIntentionRead := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  intentionID,
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
		} else {
			require.NoError(t, err)
			require.Len(t, resp.Intentions, 1)
			require.Equal(t, "*", resp.Intentions[0].DestinationName)
		}
	}

	t.Run("allow-read-for-write-token", func(t *testing.T) {
		doIntentionRead(t, writeToken.SecretID, false)
	})

	t.Run("allow-read-for-read-token", func(t *testing.T) {
		doIntentionRead(t, readToken.SecretID, false)
	})

	t.Run("allow-read-for-exact-wildcard-token", func(t *testing.T) {
		// this is allowed because, the effect of the policy is to grant
		// intention:write on the service named "*". When reading the
		// intention we will validate that the token has read permissions
		// for any intention that would match the wildcard.
		doIntentionRead(t, exactToken.SecretID, false)
	})

	t.Run("allow-read-for-prefix-wildcard-token", func(t *testing.T) {
		// this is allowed for the same reasons as for the
		// exact-wildcard-token case
		doIntentionRead(t, wildcardPrefixToken.SecretID, false)
	})

	t.Run("deny-read-for-deny-token", func(t *testing.T) {
		doIntentionRead(t, denyToken.SecretID, true)
	})

	doIntentionList := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp)
		// even with permission denied this should return success but with an empty list
		require.NoError(t, err)
		if deny {
			require.Empty(t, resp.Intentions)
		} else {
			require.Len(t, resp.Intentions, 1)
			require.Equal(t, "*", resp.Intentions[0].DestinationName)
		}
	}

	t.Run("allow-list-for-write-token", func(t *testing.T) {
		doIntentionList(t, writeToken.SecretID, false)
	})

	t.Run("allow-list-for-read-token", func(t *testing.T) {
		doIntentionList(t, readToken.SecretID, false)
	})

	t.Run("allow-list-for-exact-wildcard-token", func(t *testing.T) {
		doIntentionList(t, exactToken.SecretID, false)
	})

	t.Run("allow-list-for-prefix-wildcard-token", func(t *testing.T) {
		doIntentionList(t, wildcardPrefixToken.SecretID, false)
	})

	t.Run("deny-list-for-deny-token", func(t *testing.T) {
		doIntentionList(t, denyToken.SecretID, true)
	})

	doIntentionMatch := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "*",
					},
				},
			},
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentionMatches
		err := msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp)
		if deny {
			require.Error(t, err)
			require.Empty(t, resp.Matches)
		} else {
			require.NoError(t, err)
			require.Len(t, resp.Matches, 1)
			require.Len(t, resp.Matches[0], 1)
			require.Equal(t, "*", resp.Matches[0][0].DestinationName)
		}
	}

	t.Run("allow-match-for-write-token", func(t *testing.T) {
		doIntentionMatch(t, writeToken.SecretID, false)
	})

	t.Run("allow-match-for-read-token", func(t *testing.T) {
		doIntentionMatch(t, readToken.SecretID, false)
	})

	t.Run("allow-match-for-exact-wildcard-token", func(t *testing.T) {
		doIntentionMatch(t, exactToken.SecretID, false)
	})

	t.Run("allow-match-for-prefix-wildcard-token", func(t *testing.T) {
		doIntentionMatch(t, wildcardPrefixToken.SecretID, false)
	})

	t.Run("deny-match-for-deny-token", func(t *testing.T) {
		doIntentionMatch(t, denyToken.SecretID, true)
	})

	doIntentionUpdate := func(t *testing.T, token string, dest string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpUpdate,
			Intention: &structs.Intention{
				ID:              intentionID,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: dest,
				Action:          structs.IntentionActionAllow,
				SourceType:      structs.IntentionSourceConsul,
			},
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("deny-update-for-foo-token", func(t *testing.T) {
		doIntentionUpdate(t, fooToken.SecretID, "foo", true)
	})

	t.Run("allow-update-for-prefix-token", func(t *testing.T) {
		// this tests that regardless of going from a wildcard intention
		// to a non-wildcard or the opposite direction that the permissions
		// are checked correctly. This also happens to leave the intention
		// in a state ready for verifying similar things with deletion
		doIntentionUpdate(t, writeToken.SecretID, "foo", false)
		doIntentionUpdate(t, writeToken.SecretID, "*", false)
	})

	doIntentionDelete := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpDelete,
			Intention: &structs.Intention{
				ID: intentionID,
			},
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("deny-delete-for-read-token", func(t *testing.T) {
		doIntentionDelete(t, readToken.SecretID, true)
	})

	t.Run("deny-delete-for-exact-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service "*" {
		//    intentions = "write"
		// }
		// will not actually allow deleting an intention with a wildcard service name
		doIntentionDelete(t, exactToken.SecretID, true)
	})

	t.Run("deny-delete-for-prefix-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service_prefix "*" {
		//    intentions = "write"
		// }
		// will not actually allow creating an intention with a wildcard service name
		doIntentionDelete(t, wildcardPrefixToken.SecretID, true)
	})

	t.Run("allow-delete", func(t *testing.T) {
		// tests that a token with all the required privileges can delete
		// intentions with a wildcard destination
		doIntentionDelete(t, writeToken.SecretID, false)
	})
}

// Test apply with delete and a default deny ACL
func TestIntentionApply_aclDelete(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with write permissions
	var token string
	{
		var rules = `
service "foo" {
	policy = "deny"
	intentions = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = token

	// Create
	var reply string
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do a delete with no token; this should get rejected.
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	ixn.WriteRequest.Token = ""
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(acl.IsErrPermissionDenied(err))

	// Try again with the original token. This should go through.
	ixn.WriteRequest.Token = token
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Verify it is gone
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.NotNil(err)
		require.Contains(err.Error(), ErrIntentionNotFound.Error())
	}
}

// Test apply with update and a default deny ACL
func TestIntentionApply_aclUpdate(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with write permissions
	var token string
	{
		var rules = `
service "foo" {
	policy = "deny"
	intentions = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = token

	// Create
	var reply string
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do an update without a token; this should get rejected.
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.WriteRequest.Token = ""
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(acl.IsErrPermissionDenied(err))

	// Try again with the original token; this should go through.
	ixn.WriteRequest.Token = token
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
}

// Test apply with a management token
func TestIntentionApply_aclManagement(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = "root"

	// Create
	var reply string
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	ixn.Intention.ID = reply

	// Update
	ixn.Op = structs.IntentionOpUpdate
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Delete
	ixn.Op = structs.IntentionOpDelete
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
}

// Test update changing the name where an ACL won't allow it
func TestIntentionApply_aclUpdateChange(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with write permissions
	var token string
	{
		var rules = `
service "foo" {
	policy = "deny"
	intentions = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "bar"
	ixn.WriteRequest.Token = "root"

	// Create
	var reply string
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do an update without a token; this should get rejected.
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.DestinationName = "foo"
	ixn.WriteRequest.Token = token
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(acl.IsErrPermissionDenied(err))
}

// Test reading with ACLs
func TestIntentionGet_acl(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with service write permissions. This will grant
	// intentions read on either end of an intention.
	token, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	service "foobar" {
		policy = "write"
	}`)
	require.NoError(t, err)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = "root"

	// Create
	var reply string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	ixn.Intention.ID = reply

	t.Run("Read by ID without token should be error", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Len(t, resp.Intentions, 0)
	})

	t.Run("Read by ID with token should work", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  ixn.Intention.ID,
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}

		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
	})

	t.Run("Read by Exact without token should be error", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Exact: &structs.IntentionQueryExact{
				SourceNS:        structs.IntentionDefaultNamespace,
				SourceName:      "api",
				DestinationNS:   structs.IntentionDefaultNamespace,
				DestinationName: "foobar",
			},
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Len(t, resp.Intentions, 0)
	})

	t.Run("Read by Exact with token should work", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Exact: &structs.IntentionQueryExact{
				SourceNS:        structs.IntentionDefaultNamespace,
				SourceName:      "api",
				DestinationNS:   structs.IntentionDefaultNamespace,
				DestinationName: "foobar",
			},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}

		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
	})
}

func TestIntentionList(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	waitForLeaderEstablishment(t, s1)

	// Test with no intentions inserted yet
	{
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.NotNil(resp.Intentions)
		require.Len(resp.Intentions, 0)
	}
}

// Test listing with ACLs
func TestIntentionList_acl(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, testServerACLConfig(nil))
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)
	waitForNewACLs(t, s1)

	token, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service_prefix "foo" { policy = "write" }`)
	require.NoError(t, err)

	// Create a few records
	for _, name := range []string{"foobar", "bar", "baz"} {
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		ixn.Intention.SourceNS = "default"
		ixn.Intention.DestinationNS = "default"
		ixn.Intention.DestinationName = name
		ixn.WriteRequest.Token = TestDefaultMasterToken

		// Create
		var reply string
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	}

	// Test with no token
	t.Run("no-token", func(t *testing.T) {
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 0)
	})

	// Test with management token
	t.Run("master-token", func(t *testing.T) {
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: TestDefaultMasterToken},
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 3)
	})

	// Test with user token
	t.Run("user-token", func(t *testing.T) {
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 1)
	})

	t.Run("filtered", func(t *testing.T) {
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
			QueryOptions: structs.QueryOptions{
				Token:  TestDefaultMasterToken,
				Filter: "DestinationName == foobar",
			},
		}

		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 1)
	})
}

// Test basic matching. We don't need to exhaustively test inputs since this
// is tested in the agent/consul/state package.
func TestIntentionMatch_good(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create some records
	{
		insert := [][]string{
			{"default", "*", "default", "*"},
			{"default", "*", "default", "bar"},
			{"default", "*", "default", "baz"}, // shouldn't match
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention: &structs.Intention{
					SourceNS:        v[0],
					SourceName:      v[1],
					DestinationNS:   v[2],
					DestinationName: v[3],
					Action:          structs.IntentionActionAllow,
				},
			}

			// Create
			var reply string
			require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Match
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{Name: "bar"},
			},
		},
	}
	var resp structs.IndexedIntentionMatches
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp))
	require.Len(t, resp.Matches, 1)

	expected := [][]string{
		{"default", "*", "default", "bar"},
		{"default", "*", "default", "*"},
	}
	var actual [][]string
	for _, ixn := range resp.Matches[0] {
		actual = append(actual, []string{
			ixn.SourceNS,
			ixn.SourceName,
			ixn.DestinationNS,
			ixn.DestinationName,
		})
	}
	require.Equal(t, expected, actual)
}

// Test matching with ACLs
func TestIntentionMatch_acl(t *testing.T) {
	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	token, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service "bar" { policy = "write" }`)
	require.NoError(t, err)

	// Create some records
	{
		insert := []string{
			"*",
			"bar",
			"baz",
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			ixn.Intention.DestinationName = v
			ixn.WriteRequest.Token = TestDefaultMasterToken

			// Create
			var reply string
			require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Test with no token
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "bar",
					},
				},
			},
		}
		var resp structs.IndexedIntentionMatches
		err := msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Len(t, resp.Matches, 0)
	}

	// Test with proper token
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "bar",
					},
				},
			},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		var resp structs.IndexedIntentionMatches
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp))
		require.Len(t, resp.Matches, 1)

		expected := []string{"bar", "*"}
		var actual []string
		for _, ixn := range resp.Matches[0] {
			actual = append(actual, ixn.DestinationName)
		}

		require.ElementsMatch(t, expected, actual)
	}
}

// Test the Check method defaults to allow with no ACL set.
func TestIntentionCheck_defaultNoACL(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Test
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "bar",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(t, resp.Allowed)
}

// Test the Check method defaults to deny with allowlist ACLs.
func TestIntentionCheck_defaultACLDeny(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "bar",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = "root"
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.False(t, resp.Allowed)
}

// Test the Check method defaults to deny with denylist ACLs.
func TestIntentionCheck_defaultACLAllow(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "allow"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "bar",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = "root"
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(t, resp.Allowed)
}

// Test the Check method requires service:read permission.
func TestIntentionCheck_aclDeny(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with service read permissions. This will grant permission.
	var token string
	{
		var rules = `
service "bar" {
	policy = "read"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "qux",
			DestinationName: "baz",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = token
	var resp structs.IntentionQueryCheckResponse
	err := msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp)
	require.True(t, acl.IsErrPermissionDenied(err))
}

// Test the Check method returns allow/deny properly.
func TestIntentionCheck_match(t *testing.T) {
	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	token, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `service "api" { policy = "read" }`)
	require.NoError(t, err)

	// Create some intentions
	{
		insert := [][]string{
			{"web", "db"},
			{"api", "db"},
			{"web", "api"},
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention: &structs.Intention{
					SourceNS:        "default",
					SourceName:      v[0],
					DestinationNS:   "default",
					DestinationName: v[1],
					Action:          structs.IntentionActionAllow,
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultMasterToken},
			}
			// Create
			var reply string
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "default",
			SourceName:      "web",
			DestinationNS:   "default",
			DestinationName: "api",
			SourceType:      structs.IntentionSourceConsul,
		},
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(t, resp.Allowed)

	// Test no match for sanity
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Check: &structs.IntentionQueryCheck{
				SourceNS:        "default",
				SourceName:      "db",
				DestinationNS:   "default",
				DestinationName: "api",
				SourceType:      structs.IntentionSourceConsul,
			},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		var resp structs.IntentionQueryCheckResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
		require.False(t, resp.Allowed)
	}
}
