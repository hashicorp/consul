package consul

import (
	"fmt"
	"io/ioutil"
	"net/rpc"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACLEndpoint_Bootstrap(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "0.8.0" // Too low for auto init of bootstrap.
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Expect an error initially since ACL bootstrap is not initialized.
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.ACL
	// We can only do some high
	// level checks on the ACL since we don't have control over the UUID or
	// Raft indexes at this level.
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.ID) != len("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx") ||
		!strings.HasPrefix(out.Name, "Bootstrap Token") ||
		out.Type != structs.ACLTokenTypeManagement ||
		out.CreateIndex == 0 || out.ModifyIndex == 0 {
		t.Fatalf("bad: %#v", out)
	}

	// Finally, make sure that another attempt is rejected.
	err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", &arg, &out)
	if err.Error() != structs.ACLBootstrapNotAllowedErr.Error() {
		t.Fatalf("err: %v", err)
	}
}

func TestACLEndpoint_BootstrapTokens(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Expect an error initially since ACL bootstrap is not initialized.
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.ACLToken
	// We can only do some high
	// level checks on the ACL since we don't have control over the UUID or
	// Raft indexes at this level.
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.BootstrapTokens", &arg, &out))
	require.Equal(t, 36, len(out.AccessorID))
	require.True(t, strings.HasPrefix(out.Description, "Bootstrap Token"))
	require.Equal(t, out.Type, structs.ACLTokenTypeManagement)
	require.True(t, out.CreateIndex > 0)
	require.Equal(t, out.CreateIndex, out.ModifyIndex)

	// Finally, make sure that another attempt is rejected.
	err := msgpackrpc.CallWithCodec(codec, "ACL.BootstrapTokens", &arg, &out)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), structs.ACLBootstrapNotAllowedErr.Error()))

	_, resetIdx, err := s1.fsm.State().CanBootstrapACLToken()

	resetPath := filepath.Join(dir1, "acl-bootstrap-reset")
	require.NoError(t, ioutil.WriteFile(resetPath, []byte(fmt.Sprintf("%d", resetIdx)), 0600))

	oldID := out.AccessorID
	// Finally, make sure that another attempt is rejected.
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.BootstrapTokens", &arg, &out))
	require.Equal(t, 36, len(out.AccessorID))
	require.NotEqual(t, oldID, out.AccessorID)
	require.True(t, strings.HasPrefix(out.Description, "Bootstrap Token"))
	require.Equal(t, out.Type, structs.ACLTokenTypeManagement)
	require.True(t, out.CreateIndex > 0)
	require.Equal(t, out.CreateIndex, out.ModifyIndex)
}

func TestACLEndpoint_Apply(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := out

	// Verify
	state := s1.fsm.State()
	_, s, err := state.ACLTokenGetBySecret(nil, out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Fatalf("should not be nil")
	}
	if s.SecretID != out {
		t.Fatalf("bad: %v", s)
	}
	if s.Description != "User token" {
		t.Fatalf("bad: %v", s)
	}

	// Do a delete
	arg.Op = structs.ACLDelete
	arg.ACL.ID = out
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	_, s, err = state.ACLTokenGetBySecret(nil, id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s != nil {
		t.Fatalf("bad: %v", s)
	}
}

func TestACLEndpoint_Update_PurgeCache(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := out

	// Resolve
	acl1, err := s1.ResolveToken(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl1 == nil {
		t.Fatalf("should not be nil")
	}
	if !acl1.KeyRead("foo") {
		t.Fatalf("should be allowed")
	}

	// Do an update
	arg.ACL.ID = out
	arg.ACL.Rules = `{"key": {"": {"policy": "deny"}}}`
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve again
	acl2, err := s1.ResolveToken(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl2 == nil {
		t.Fatalf("should not be nil")
	}
	if acl2 == acl1 {
		t.Fatalf("should not be cached")
	}
	if acl2.KeyRead("foo") {
		t.Fatalf("should not be allowed")
	}

	// Do a delete
	arg.Op = structs.ACLDelete
	arg.ACL.Rules = ""
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve again
	acl3, err := s1.ResolveToken(id)
	if !acl.IsErrNotFound(err) {
		t.Fatalf("err: %v", err)
	}
	if acl3 != nil {
		t.Fatalf("should be nil")
	}
}

func TestACLEndpoint_Apply_CustomID(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			ID:   "foobarbaz", // Specify custom ID, does not exist
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != "foobarbaz" {
		t.Fatalf("bad token ID: %s", out)
	}

	// Verify
	state := s1.fsm.State()
	_, s, err := state.ACLTokenGetBySecret(nil, out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Fatalf("should not be nil")
	}
	if s.SecretID != out {
		t.Fatalf("bad: %v", s)
	}
	if s.Description != "User token" {
		t.Fatalf("bad: %v", s)
	}
}

func TestACLEndpoint_Apply_Denied(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
	}
	var out string
	err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACLEndpoint_Apply_DeleteAnon(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLDelete,
		ACL: structs.ACL{
			ID:   anonymousToken,
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out)
	if err == nil || !strings.Contains(err.Error(), "delete anonymous") {
		t.Fatalf("err: %v", err)
	}
}

func TestACLEndpoint_Apply_RootChange(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			ID:   "manage",
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out)
	if err == nil || !strings.Contains(err.Error(), "root ACL") {
		t.Fatalf("err: %v", err)
	}
}

func TestACLEndpoint_Get(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.ACLSpecificRequest{
		Datacenter: "dc1",
		ACL:        out,
	}
	var acls structs.IndexedACLs
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Get", &getR, &acls); err != nil {
		t.Fatalf("err: %v", err)
	}

	if acls.Index == 0 {
		t.Fatalf("Bad: %v", acls)
	}
	if len(acls.ACLs) != 1 {
		t.Fatalf("Bad: %v", acls)
	}
	s := acls.ACLs[0]
	if s.ID != out {
		t.Fatalf("bad: %v", s)
	}
}

func TestACLEndpoint_GetPolicy(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.ACLPolicyResolveLegacyRequest{
		Datacenter: "dc1",
		ACL:        out,
	}

	var acls structs.ACLPolicyResolveLegacyResponse
	retry.Run(t, func(r *retry.R) {

		if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", &getR, &acls); err != nil {
			t.Fatalf("err: %v", err)
		}

		if acls.Policy == nil {
			t.Fatalf("Bad: %v", acls)
		}
		if acls.TTL != 30*time.Second {
			t.Fatalf("bad: %v", acls)
		}
	})

	// Do a conditional lookup with etag
	getR.ETag = acls.ETag
	var out2 structs.ACLPolicyResolveLegacyResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", &getR, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if out2.Policy != nil {
		t.Fatalf("Bad: %v", out2)
	}
	if out2.TTL != 30*time.Second {
		t.Fatalf("bad: %v", out2)
	}
}

func TestACLEndpoint_List(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	ids := []string{}
	for i := 0; i < 5; i++ {
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name: "User token",
				Type: structs.ACLTokenTypeClient,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out string
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		ids = append(ids, out)
	}

	getR := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "root"},
	}
	var acls structs.IndexedACLs
	if err := msgpackrpc.CallWithCodec(codec, "ACL.List", &getR, &acls); err != nil {
		t.Fatalf("err: %v", err)
	}

	if acls.Index == 0 {
		t.Fatalf("Bad: %v", acls)
	}

	// 5  + master
	if len(acls.ACLs) != 6 {
		t.Fatalf("Bad: %v", acls.ACLs)
	}
	for i := 0; i < len(acls.ACLs); i++ {
		s := acls.ACLs[i]
		if s.ID == anonymousToken || s.ID == "root" {
			continue
		}
		if !lib.StrContains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Name != "User token" {
			t.Fatalf("bad: %v", s)
		}
	}
}

func TestACLEndpoint_List_Denied(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	getR := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var acls structs.IndexedACLs
	err := msgpackrpc.CallWithCodec(codec, "ACL.List", &getR, &acls)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACLEndpoint_ReplicationStatus(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc2"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
	})
	s1.tokens.UpdateACLReplicationToken("secret")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	getR := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	retry.Run(t, func(r *retry.R) {
		var status structs.ACLReplicationStatus
		err := msgpackrpc.CallWithCodec(codec, "ACL.ReplicationStatus", &getR, &status)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if !status.Enabled || !status.Running || status.SourceDatacenter != "dc2" {
			r.Fatalf("bad: %#v", status)
		}
	})
}

func TestACLEndpoint_TokenRead(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	token, err := upsertTestToken(codec, "root", "dc1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl := ACL{srv: s1}

	// exists and matches what we created
	{
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      token.AccessorID,
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenResponse{}

		err := acl.TokenRead(&req, &resp)
		assert.NoError(err)

		if !reflect.DeepEqual(resp.Token, token) {
			t.Fatalf("tokens are not equal: %v != %v", resp.Token, token)
		}
	}

	// nil when token does not exist
	{
		fakeID, err := uuid.GenerateUUID()
		assert.NoError(err)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      fakeID,
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenResponse{}

		err = acl.TokenRead(&req, &resp)
		assert.Nil(resp.Token)
		assert.NoError(err)
	}

	// validates ID format
	{
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      "definitely-really-certainly-not-a-uuid",
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenResponse{}

		err := acl.TokenRead(&req, &resp)
		assert.Nil(resp.Token)
		assert.EqualError(err, "failed acl token lookup: failed acl token lookup: index error: UUID must be 36 characters")
	}
}

func TestACLEndpoint_TokenClone(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	t1, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}

	req := structs.ACLTokenSetRequest{
		Datacenter:   "dc1",
		ACLToken:     structs.ACLToken{AccessorID: t1.AccessorID},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	t2 := structs.ACLToken{}

	err = acl.TokenClone(&req, &t2)
	assert.NoError(err)

	assert.Equal(t1.Description, t2.Description)
	assert.Equal(t1.Policies, t2.Policies)
	assert.Equal(t1.Rules, t2.Rules)
	assert.Equal(t1.Local, t2.Local)
	assert.NotEqual(t1.AccessorID, t2.AccessorID)
	assert.NotEqual(t1.SecretID, t2.SecretID)
}

func TestACLEndpoint_TokenSet(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	acl := ACL{srv: s1}
	var tokenID string

	// Create it
	{
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		assert.NoError(err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		assert.NoError(err)
		token := tokenResp.Token

		assert.NotNil(token.AccessorID)
		assert.Equal(token.Description, "foobar")
		assert.Equal(token.AccessorID, resp.AccessorID)

		tokenID = token.AccessorID
	}
	// Update it
	{
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "new-description",
				AccessorID:  tokenID,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		assert.NoError(err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		assert.NoError(err)
		token := tokenResp.Token

		assert.NotNil(token.AccessorID)
		assert.Equal(token.Description, "new-description")
		assert.Equal(token.AccessorID, resp.AccessorID)
	}
}
func TestACLEndpoint_TokenSet_anon(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	policy, err := upsertTestPolicy(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}

	// Assign the policies to a token
	tokenUpsertReq := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			AccessorID: structs.ACLTokenAnonymousID,
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: policy.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	token := structs.ACLToken{}
	err = acl.TokenSet(&tokenUpsertReq, &token)
	assert.NoError(err)
	assert.NotEmpty(token.SecretID)

	tokenResp, err := retrieveTestToken(codec, "root", "dc1", structs.ACLTokenAnonymousID)
	assert.Equal(len(tokenResp.Token.Policies), 1)
	assert.Equal(tokenResp.Token.Policies[0].ID, policy.ID)
}

func TestACLEndpoint_TokenDelete(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	existingToken, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}

	// deletes a token
	{
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      existingToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		assert.NoError(err)

		// Make sure the token is gone
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", existingToken.AccessorID)
		assert.Nil(tokenResp.Token)
		assert.NoError(err)
	}

	// errors when token doesn't exist
	{
		fakeID, err := uuid.GenerateUUID()
		assert.NoError(err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      fakeID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		assert.NoError(err)

		// token should be nil
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", existingToken.AccessorID)
		assert.Nil(tokenResp.Token)
		assert.NoError(err)
	}
}
func TestACLEndpoint_TokenDelete_anon(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	acl := ACL{srv: s1}

	req := structs.ACLTokenDeleteRequest{
		Datacenter:   "dc1",
		TokenID:      structs.ACLTokenAnonymousID,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	var resp string

	err := acl.TokenDelete(&req, &resp)
	assert.EqualError(err, "Delete operation not permitted on the anonymous token")

	// Make sure the token is still there
	tokenResp, err := retrieveTestToken(codec, "root", "dc1", structs.ACLTokenAnonymousID)
	assert.NotNil(tokenResp.Token)
}

func TestACLEndpoint_TokenList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	t1, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	t2, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}

	req := structs.ACLTokenListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLTokenListResponse{}

	err = acl.TokenList(&req, &resp)
	assert.NoError(err)

	tokens := []string{t1.AccessorID, t2.AccessorID}
	var retrievedTokens []string

	for _, v := range resp.Tokens {
		retrievedTokens = append(retrievedTokens, v.AccessorID)
	}
	assert.Subset(retrievedTokens, tokens)
}

func TestACLEndpoint_TokenBatchRead(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	t1, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	t2, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}
	tokens := []string{t1.AccessorID, t2.AccessorID}

	req := structs.ACLTokenBatchGetRequest{
		Datacenter:   "dc1",
		AccessorIDs:  tokens,
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLTokenBatchResponse{}

	err = acl.TokenBatchRead(&req, &resp)
	assert.NoError(err)

	var retrievedTokens []string

	for _, v := range resp.Tokens {
		retrievedTokens = append(retrievedTokens, v.AccessorID)
	}
	assert.EqualValues(retrievedTokens, tokens)
}

func TestACLEndpoint_PolicyRead(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	policy, err := upsertTestPolicy(codec, "root", "dc1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl := ACL{srv: s1}

	req := structs.ACLPolicyGetRequest{
		Datacenter:   "dc1",
		PolicyID:     policy.ID,
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLPolicyResponse{}

	err = acl.PolicyRead(&req, &resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(resp.Policy, policy) {
		t.Fatalf("tokens are not equal: %v != %v", resp.Policy, policy)
	}
}

func TestACLEndpoint_PolicyBatchRead(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	t1, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	t2, err := upsertTestToken(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}
	tokens := []string{t1.AccessorID, t2.AccessorID}

	req := structs.ACLTokenBatchGetRequest{
		Datacenter:   "dc1",
		AccessorIDs:  tokens,
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLTokenBatchResponse{}

	err = acl.TokenBatchRead(&req, &resp)
	assert.NoError(err)

	var retrievedTokens []string

	for _, v := range resp.Tokens {
		retrievedTokens = append(retrievedTokens, v.AccessorID)
	}
	assert.EqualValues(retrievedTokens, tokens)
}

func TestACLEndpoint_PolicySet(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	acl := ACL{srv: s1}
	var policyID string

	// Create it
	{
		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				Description: "foobar",
				Name:        "baz",
				Rules:       "service \"\" { policy = \"read\" }",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		assert.NoError(err)
		assert.NotNil(resp.ID)

		// Get the policy directly to validate that it exists
		policyResp, err := retrieveTestPolicy(codec, "root", "dc1", resp.ID)
		assert.NoError(err)
		policy := policyResp.Policy

		assert.NotNil(policy.ID)
		assert.Equal(policy.Description, "foobar")
		assert.Equal(policy.Name, "baz")
		assert.Equal(policy.Rules, "service \"\" { policy = \"read\" }")

		policyID = policy.ID
	}

	// Update it
	{
		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				ID:          policyID,
				Description: "bat",
				Name:        "bar",
				Rules:       "service \"\" { policy = \"write\" }",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		assert.NoError(err)
		assert.NotNil(resp.ID)

		// Get the policy directly to validate that it exists
		policyResp, err := retrieveTestPolicy(codec, "root", "dc1", resp.ID)
		assert.NoError(err)
		policy := policyResp.Policy

		assert.NotNil(policy.ID)
		assert.Equal(policy.Description, "bat")
		assert.Equal(policy.Name, "bar")
		assert.Equal(policy.Rules, "service \"\" { policy = \"write\" }")
	}
}

func TestACLEndpoint_PolicySet_globalManagement(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	acl := ACL{srv: s1}

	// Can't change the rules
	{

		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				ID:    structs.ACLPolicyGlobalManagementID,
				Name:  "foobar", // This is required to get past validation
				Rules: "service \"\" { policy = \"write\" }",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		assert.EqualError(err, "Changing the Rules for the builtin global-management policy is not permitted")
	}

	// Can rename it
	{
		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				ID:    structs.ACLPolicyGlobalManagementID,
				Name:  "foobar",
				Rules: structs.ACLPolicyGlobalManagement,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		assert.NoError(err)

		// Get the policy again
		policyResp, err := retrieveTestPolicy(codec, "root", "dc1", structs.ACLPolicyGlobalManagementID)
		assert.NoError(err)
		policy := policyResp.Policy

		assert.Equal(policy.ID, structs.ACLPolicyGlobalManagementID)
		assert.Equal(policy.Name, "foobar")

	}
}

func TestACLEndpoint_PolicyDelete(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	existingPolicy, err := upsertTestPolicy(codec, "root", "dc1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl := ACL{srv: s1}

	req := structs.ACLPolicyDeleteRequest{
		Datacenter:   "dc1",
		PolicyID:     existingPolicy.ID,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	var resp string

	err = acl.PolicyDelete(&req, &resp)
	assert.NoError(err)

	// Make sure the policy is gone
	tokenResp, err := retrieveTestPolicy(codec, "root", "dc1", existingPolicy.ID)
	assert.Nil(tokenResp.Policy)
}

func TestACLEndpoint_PolicyDelete_globalManagement(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	acl := ACL{srv: s1}

	req := structs.ACLPolicyDeleteRequest{
		Datacenter:   "dc1",
		PolicyID:     structs.ACLPolicyGlobalManagementID,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var resp string

	err := acl.PolicyDelete(&req, &resp)

	assert.EqualError(err, "Delete operation not permitted on the builtin global-management policy")
}

func TestACLEndpoint_PolicyList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	p1, err := upsertTestPolicy(codec, "root", "dc1")
	assert.NoError(err)

	p2, err := upsertTestPolicy(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}

	req := structs.ACLPolicyListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLPolicyListResponse{}

	err = acl.PolicyList(&req, &resp)
	assert.NoError(err)

	policies := []string{p1.ID, p2.ID}
	var retrievedPolicies []string

	for _, v := range resp.Policies {
		retrievedPolicies = append(retrievedPolicies, v.ID)
	}
	assert.Subset(retrievedPolicies, policies)
}

func TestACLEndpoint_PolicyResolve(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	p1, err := upsertTestPolicy(codec, "root", "dc1")
	assert.NoError(err)

	p2, err := upsertTestPolicy(codec, "root", "dc1")
	assert.NoError(err)

	acl := ACL{srv: s1}

	policies := []string{p1.ID, p2.ID}

	// Assign the policies to a token
	tokenUpsertReq := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: p1.ID,
				},
				structs.ACLTokenPolicyLink{
					ID: p2.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	token := structs.ACLToken{}
	err = acl.TokenSet(&tokenUpsertReq, &token)
	assert.NoError(err)
	assert.NotEmpty(token.SecretID)

	resp := structs.ACLPolicyBatchResponse{}
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter:   "dc1",
		PolicyIDs:    []string{p1.ID, p2.ID},
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	err = acl.PolicyResolve(&req, &resp)
	assert.NoError(err)

	var retrievedPolicies []string

	for _, v := range resp.Policies {
		retrievedPolicies = append(retrievedPolicies, v.ID)
	}
	assert.EqualValues(retrievedPolicies, policies)
}

// upsertTestToken creates a token for testing purposes
func upsertTestToken(codec rpc.ClientCodec, masterToken string, datacenter string) (*structs.ACLToken, error) {
	arg := structs.ACLTokenSetRequest{
		Datacenter: datacenter,
		ACLToken: structs.ACLToken{
			Description: "User token",
			Local:       false,
			Policies:    nil,
		},
		WriteRequest: structs.WriteRequest{Token: masterToken},
	}

	var out structs.ACLToken

	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &arg, &out)

	if err != nil {
		return nil, err
	}

	if out.AccessorID == "" {
		return nil, fmt.Errorf("AccessorID is nil: %v", out)
	}

	return &out, nil
}

// retrieveTestToken returns a policy for testing purposes
func retrieveTestToken(codec rpc.ClientCodec, masterToken string, datacenter string, id string) (*structs.ACLTokenResponse, error) {
	arg := structs.ACLTokenGetRequest{
		Datacenter:   datacenter,
		TokenID:      id,
		TokenIDType:  structs.ACLTokenAccessor,
		QueryOptions: structs.QueryOptions{Token: masterToken},
	}

	var out structs.ACLTokenResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

// upsertTestPolicy creates a policy for testing purposes
func upsertTestPolicy(codec rpc.ClientCodec, masterToken string, datacenter string) (*structs.ACLPolicy, error) {
	// Make sure test policies can't collide
	policyUnq, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	arg := structs.ACLPolicySetRequest{
		Datacenter: datacenter,
		Policy: structs.ACLPolicy{
			Name: fmt.Sprintf("test-policy-%s", policyUnq),
		},
		WriteRequest: structs.WriteRequest{Token: masterToken},
	}

	var out structs.ACLPolicy

	err = msgpackrpc.CallWithCodec(codec, "ACL.PolicySet", &arg, &out)

	if err != nil {
		return nil, err
	}

	if out.ID == "" {
		return nil, fmt.Errorf("ID is nil: %v", out)
	}

	return &out, nil
}

// retrieveTestPolicy returns a policy for testing purposes
func retrieveTestPolicy(codec rpc.ClientCodec, masterToken string, datacenter string, id string) (*structs.ACLPolicyResponse, error) {
	arg := structs.ACLPolicyGetRequest{
		Datacenter:   datacenter,
		PolicyID:     id,
		QueryOptions: structs.QueryOptions{Token: masterToken},
	}

	var out structs.ACLPolicyResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.PolicyRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}
