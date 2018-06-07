package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test basic creation
func TestIntentionApply_new(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	assert.NotEmpty(reply)

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal(resp.Index, actual.ModifyIndex)
		assert.WithinDuration(now, actual.CreatedAt, 5*time.Second)
		assert.WithinDuration(now, actual.UpdatedAt, 5*time.Second)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		ixn.Intention.UpdatePrecedence()
		assert.Equal(ixn.Intention, actual)
	}
}

// Test the source type defaults
func TestIntentionApply_defaultSourceType(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	assert.NotEmpty(reply)

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal(structs.IntentionSourceConsul, actual.SourceType)
	}
}

// Shouldn't be able to create with an ID set
func TestIntentionApply_createWithID(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	assert.NotNil(err)
	assert.Contains(err, "ID must be empty")
}

// Test basic updating
func TestIntentionApply_updateGood(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	assert.NotEmpty(reply)

	// Read CreatedAt
	var createdAt time.Time
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		createdAt = actual.CreatedAt
	}

	// Sleep a bit so that the updated at will definitely be different, not much
	time.Sleep(1 * time.Millisecond)

	// Update
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.SourceName = "*"
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal(createdAt, actual.CreatedAt)
		assert.WithinDuration(time.Now(), actual.UpdatedAt, 5*time.Second)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		ixn.Intention.UpdatePrecedence()
		assert.Equal(ixn.Intention, actual)
	}
}

// Shouldn't be able to update a non-existent intention
func TestIntentionApply_updateNonExist(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	assert.NotNil(err)
	assert.Contains(err, "Cannot modify non-existent intention")
}

// Test basic deleting
func TestIntentionApply_deleteGood(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        "test",
			SourceName:      "test",
			DestinationNS:   "test",
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
		},
	}
	var reply string

	// Create
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	assert.NotEmpty(reply)

	// Delete
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		assert.NotNil(err)
		assert.Contains(err, ErrIntentionNotFound.Error())
	}
}

// Test apply with a deny ACL
func TestIntentionApply_aclDeny(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
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
	assert.True(acl.IsErrPermissionDenied(err))

	// Now add the token and try again.
	ixn.WriteRequest.Token = token
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  ixn.Intention.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal(resp.Index, actual.ModifyIndex)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		ixn.Intention.UpdatePrecedence()
		assert.Equal(ixn.Intention, actual)
	}
}

// Test apply with delete and a default deny ACL
func TestIntentionApply_aclDelete(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do a delete with no token; this should get rejected.
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	ixn.WriteRequest.Token = ""
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	assert.True(acl.IsErrPermissionDenied(err))

	// Try again with the original token. This should go through.
	ixn.WriteRequest.Token = token
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Verify it is gone
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), ErrIntentionNotFound.Error())
	}
}

// Test apply with update and a default deny ACL
func TestIntentionApply_aclUpdate(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do an update without a token; this should get rejected.
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.WriteRequest.Token = ""
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	assert.True(acl.IsErrPermissionDenied(err))

	// Try again with the original token; this should go through.
	ixn.WriteRequest.Token = token
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
}

// Test apply with a management token
func TestIntentionApply_aclManagement(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	ixn.Intention.ID = reply

	// Update
	ixn.Op = structs.IntentionOpUpdate
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Delete
	ixn.Op = structs.IntentionOpDelete
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
}

// Test update changing the name where an ACL won't allow it
func TestIntentionApply_aclUpdateChange(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do an update without a token; this should get rejected.
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.DestinationName = "foo"
	ixn.WriteRequest.Token = token
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	assert.True(acl.IsErrPermissionDenied(err))
}

// Test reading with ACLs
func TestIntentionGet_acl(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with service write permissions. This will grant
	// intentions read.
	var token string
	{
		var rules = `
service "foo" {
	policy = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	ixn.Intention.ID = reply

	// Read without token should be error
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		assert.True(acl.IsErrPermissionDenied(err))
		assert.Len(resp.Intentions, 0)
	}

	// Read with token should work
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  ixn.Intention.ID,
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
	}
}

func TestIntentionList(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Test with no intentions inserted yet
	{
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		assert.NotNil(resp.Intentions)
		assert.Len(resp.Intentions, 0)
	}
}

// Test listing with ACLs
func TestIntentionList_acl(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with service write permissions. This will grant
	// intentions read.
	var token string
	{
		var rules = `
service "foo" {
	policy = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Create a few records
	for _, name := range []string{"foobar", "bar", "baz"} {
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		ixn.Intention.DestinationName = name
		ixn.WriteRequest.Token = "root"

		// Create
		var reply string
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	}

	// Test with no token
	{
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		assert.Len(resp.Intentions, 0)
	}

	// Test with management token
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		assert.Len(resp.Intentions, 3)
	}

	// Test with user token
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedIntentions
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		assert.Len(resp.Intentions, 1)
	}
}

// Test basic matching. We don't need to exhaustively test inputs since this
// is tested in the agent/consul/state package.
func TestIntentionMatch_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create some records
	{
		insert := [][]string{
			{"foo", "*", "foo", "*"},
			{"foo", "*", "foo", "bar"},
			{"foo", "*", "foo", "baz"}, // shouldn't match
			{"foo", "*", "bar", "bar"}, // shouldn't match
			{"foo", "*", "bar", "*"},   // shouldn't match
			{"foo", "*", "*", "*"},
			{"bar", "*", "foo", "bar"}, // duplicate destination different source
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
			assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Match
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: "foo",
					Name:      "bar",
				},
			},
		},
	}
	var resp structs.IndexedIntentionMatches
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp))
	assert.Len(resp.Matches, 1)

	expected := [][]string{
		{"bar", "*", "foo", "bar"},
		{"foo", "*", "foo", "bar"},
		{"foo", "*", "foo", "*"},
		{"foo", "*", "*", "*"},
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
	assert.Equal(expected, actual)
}

// Test matching with ACLs
func TestIntentionMatch_acl(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with service write permissions. This will grant
	// intentions read.
	var token string
	{
		var rules = `
service "bar" {
	policy = "write"
}`

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Create some records
	{
		insert := [][]string{
			{"foo", "*"},
			{"foo", "bar"},
			{"foo", "baz"}, // shouldn't match
			{"bar", "bar"}, // shouldn't match
			{"bar", "*"},   // shouldn't match
			{"*", "*"},
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			ixn.Intention.DestinationNS = v[0]
			ixn.Intention.DestinationName = v[1]
			ixn.WriteRequest.Token = "root"

			// Create
			var reply string
			assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
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
						Namespace: "foo",
						Name:      "bar",
					},
				},
			},
		}
		var resp structs.IndexedIntentionMatches
		err := msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp)
		assert.True(acl.IsErrPermissionDenied(err))
		assert.Len(resp.Matches, 0)
	}

	// Test with proper token
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "foo",
						Name:      "bar",
					},
				},
			},
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedIntentionMatches
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp))
		assert.Len(resp.Matches, 1)

		expected := [][]string{{"foo", "bar"}, {"foo", "*"}, {"*", "*"}}
		var actual [][]string
		for _, ixn := range resp.Matches[0] {
			actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
		}

		assert.Equal(expected, actual)
	}
}

// Test the Check method defaults to allow with no ACL set.
func TestIntentionCheck_defaultNoACL(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Test
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "foo",
			SourceName:      "bar",
			DestinationNS:   "foo",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	var resp structs.IntentionQueryCheckResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(resp.Allowed)
}

// Test the Check method defaults to deny with whitelist ACLs.
func TestIntentionCheck_defaultACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "foo",
			SourceName:      "bar",
			DestinationNS:   "foo",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = "root"
	var resp structs.IntentionQueryCheckResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.False(resp.Allowed)
}

// Test the Check method defaults to deny with blacklist ACLs.
func TestIntentionCheck_defaultACLAllow(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "allow"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "foo",
			SourceName:      "bar",
			DestinationNS:   "foo",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = "root"
	var resp structs.IntentionQueryCheckResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(resp.Allowed)
}

// Test the Check method requires service:read permission.
func TestIntentionCheck_aclDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "foo",
			SourceName:      "qux",
			DestinationNS:   "foo",
			DestinationName: "baz",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = token
	var resp structs.IntentionQueryCheckResponse
	err := msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp)
	require.True(acl.IsErrPermissionDenied(err))
}

// Test the Check method returns allow/deny properly.
func TestIntentionCheck_match(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token))
	}

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "*", "foo", "*"},
			{"foo", "*", "foo", "bar"},
			{"bar", "*", "foo", "bar"}, // duplicate destination different source
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
			ixn.WriteRequest.Token = "root"

			// Create
			var reply string
			require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "foo",
			SourceName:      "qux",
			DestinationNS:   "foo",
			DestinationName: "bar",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = token
	var resp structs.IntentionQueryCheckResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(resp.Allowed)

	// Test no match for sanity
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Check: &structs.IntentionQueryCheck{
				SourceNS:        "baz",
				SourceName:      "qux",
				DestinationNS:   "foo",
				DestinationName: "bar",
				SourceType:      structs.IntentionSourceConsul,
			},
		}
		req.Token = token
		var resp structs.IntentionQueryCheckResponse
		require.Nil(msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
		require.False(resp.Allowed)
	}
}
