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
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
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

	token, err := upsertTestToken(codec, "root", "dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	clock := newStoppedClock(time.Now())
	acl := s1.GetEndpoint("ACL").(*ACL)
	acl.clock = clock

	t.Run("exists and matches what we created", func(t *testing.T) {
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      token.AccessorID,
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenResponse{}

		err := acl.TokenRead(&req, &resp)
		require.NoError(t, err)

		if !reflect.DeepEqual(resp.Token, token) {
			t.Fatalf("tokens are not equal: %v != %v", resp.Token, token)
		}
	})

	clock.Reset()

	t.Run("expired tokens are filtered", func(t *testing.T) {
		expTime := clock.Now().Add(15 * time.Minute)

		// insert a token that will expire
		token, err := upsertTestToken(codec, "root", "dc1", func(t *structs.ACLToken) {
			t.ExpirationTime = expTime
		})
		require.NoError(t, err)

		t.Run("readable until expiration", func(t *testing.T) {
			req := structs.ACLTokenGetRequest{
				Datacenter:   "dc1",
				TokenID:      token.AccessorID,
				TokenIDType:  structs.ACLTokenAccessor,
				QueryOptions: structs.QueryOptions{Token: "root"},
			}

			resp := structs.ACLTokenResponse{}

			clock.Set(expTime.Add(-30 * time.Second))
			require.NoError(t, acl.TokenRead(&req, &resp))
			require.Equal(t, token, resp.Token)
		})

		t.Run("not returned when expired", func(t *testing.T) {
			req := structs.ACLTokenGetRequest{
				Datacenter:   "dc1",
				TokenID:      token.AccessorID,
				TokenIDType:  structs.ACLTokenAccessor,
				QueryOptions: structs.QueryOptions{Token: "root"},
			}

			resp := structs.ACLTokenResponse{}

			clock.Set(expTime.Add(1 * time.Second))
			require.NoError(t, acl.TokenRead(&req, &resp))
			require.Nil(t, resp.Token)
		})
	})

	clock.Reset()

	t.Run("nil when token does not exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      fakeID,
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenResponse{}

		err = acl.TokenRead(&req, &resp)
		require.Nil(t, resp.Token)
		require.NoError(t, err)
	})

	clock.Reset()

	t.Run("validates ID format", func(t *testing.T) {
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      "definitely-really-certainly-not-a-uuid",
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenResponse{}

		err := acl.TokenRead(&req, &resp)
		require.Nil(t, resp.Token)
		require.EqualError(t, err, "failed acl token lookup: failed acl token lookup: index error: UUID must be 36 characters")
	})
}

func TestACLEndpoint_TokenClone(t *testing.T) {
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

	t1, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	clock := newStoppedClock(time.Now())
	endpoint := s1.GetEndpoint("ACL").(*ACL)
	endpoint.clock = clock

	t.Run("normal", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter:   "dc1",
			ACLToken:     structs.ACLToken{AccessorID: t1.AccessorID},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		t2 := structs.ACLToken{}

		err = endpoint.TokenClone(&req, &t2)
		require.NoError(t, err)

		require.Equal(t, t1.Description, t2.Description)
		require.Equal(t, t1.Policies, t2.Policies)
		require.Equal(t, t1.Rules, t2.Rules)
		require.Equal(t, t1.Local, t2.Local)
		require.NotEqual(t, t1.AccessorID, t2.AccessorID)
		require.NotEqual(t, t1.SecretID, t2.SecretID)
	})

	clock.Reset()

	t.Run("can't clone expired token", func(t *testing.T) {
		expTime := clock.Now().Add(15 * time.Minute)

		// insert a token that will expire
		t1, err := upsertTestToken(codec, "root", "dc1", func(t *structs.ACLToken) {
			t.ExpirationTime = expTime
		})
		require.NoError(t, err)

		req := structs.ACLTokenSetRequest{
			Datacenter:   "dc1",
			ACLToken:     structs.ACLToken{AccessorID: t1.AccessorID},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		t2 := structs.ACLToken{}

		clock.Set(expTime.Add(1 * time.Second))
		err = endpoint.TokenClone(&req, &t2)
		require.Error(t, err)
		require.Equal(t, acl.ErrNotFound, err)
	})
}

func TestACLEndpoint_TokenSet(t *testing.T) {
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

	clock := newStoppedClock(time.Now())
	acl := s1.GetEndpoint("ACL").(*ACL)
	acl.clock = clock

	var tokenID string

	t.Run("Create it", func(t *testing.T) {
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
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)

		tokenID = token.AccessorID
	})

	clock.Reset()

	t.Run("Update it", func(t *testing.T) {
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
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "new-description")
		require.Equal(t, token.AccessorID, resp.AccessorID)
	})

	clock.Reset()

	t.Run("Create it using Policies linked by id and name", func(t *testing.T) {
		policy1, err := upsertTestPolicy(codec, "root", "dc1")
		require.NoError(t, err)
		policy2, err := upsertTestPolicy(codec, "root", "dc1")
		require.NoError(t, err)

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID: policy1.ID,
					},
					structs.ACLTokenPolicyLink{
						Name: policy2.Name,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err = acl.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Delete both policies to ensure that we skip resolving ID->Name
		// in the returned data.
		require.NoError(t, deleteTestPolicy(codec, "root", "dc1", policy1.ID))
		require.NoError(t, deleteTestPolicy(codec, "root", "dc1", policy2.ID))

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)

		require.Len(t, token.Policies, 2)
		require.Equal(t, policy1.ID, token.Policies[0].ID)
		require.Equal(t, policy2.ID, token.Policies[1].ID)
		// names are returned because they are denormalized on write
		require.Equal(t, policy1.Name, token.Policies[0].Name)
		require.Equal(t, policy2.Name, token.Policies[1].Name)
	})

	for _, test := range []struct {
		name      string
		offset    time.Duration
		errString string
	}{
		{"before create time", -5 * time.Minute, "ExpirationTime cannot be before CreateTime"},
		{"too soon", 30 * time.Second, "ExpirationTime cannot be less than"},
		{"too distant", 25 * time.Hour, "ExpirationTime cannot be more than"},
	} {
		clock.Reset()

		t.Run("Create it with an expiration time that is "+test.name, func(t *testing.T) {
			req := structs.ACLTokenSetRequest{
				Datacenter: "dc1",
				ACLToken: structs.ACLToken{
					Description:    "foobar",
					Policies:       nil,
					Local:          false,
					ExpirationTime: clock.Now().Add(test.offset),
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}

			resp := structs.ACLToken{}

			err := acl.TokenSet(&req, &resp)
			if test.errString != "" {
				requireErrorContains(t, err, test.errString)
			} else {
				require.NotNil(t, err)
			}
		})
	}

	clock.Reset()

	t.Run("Create it with expiration time AND expiration TTL set (error)", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "foobar",
				Policies:       nil,
				Local:          false,
				ExpirationTime: clock.Now().Add(15 * time.Minute),
				ExpirationTTL:  "15m",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		requireErrorContains(t, err, "Expiration TTL and Expiration Time cannot both be set")
	})

	for _, test := range []struct{ name, ttl string }{
		{"invalid", "potatoes"},
		{"zero", "0s"},
		{"negative", "-1m"},
	} {
		clock.Reset()
		t.Run("Create it with an expiration TTL that is "+test.name, func(t *testing.T) {
			req := structs.ACLTokenSetRequest{
				Datacenter: "dc1",
				ACLToken: structs.ACLToken{
					Description:   "foobar",
					Policies:      nil,
					Local:         false,
					ExpirationTTL: test.ttl,
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}

			resp := structs.ACLToken{}

			err := acl.TokenSet(&req, &resp)
			require.NotNil(t, err)
		})
	}

	clock.Reset()

	t.Run("Create it with expiration time using TTLs", func(t *testing.T) {
		expTime := clock.Now().Add(15 * time.Minute)

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:   "foobar",
				Policies:      nil,
				Local:         false,
				ExpirationTTL: "15m",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, expTime, resp.ExpirationTime)

		tokenID = token.AccessorID
	})

	clock.Reset()

	expTime := clock.Now().Add(15 * time.Minute)

	t.Run("Create it with expiration time", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "foobar",
				Policies:       nil,
				Local:          false,
				ExpirationTime: expTime,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, expTime, resp.ExpirationTime)

		tokenID = token.AccessorID
	})

	// leave the clock alone here

	t.Run("Update expiration time is not allowed", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "new-description",
				AccessorID:     tokenID,
				ExpirationTime: clock.Now().Add(3 * time.Hour),
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		requireErrorContains(t, err, "Cannot change expiration time")
	})

	// leave the clock alone here

	t.Run("Update anything except expiration time is ok", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "new-description",
				AccessorID:     tokenID,
				ExpirationTime: expTime,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "new-description")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, expTime, resp.ExpirationTime)
	})

	clock.Reset()

	expTime = clock.Now().Add(2 * time.Minute)

	t.Run("cannot update a token that is past its expiration time", func(t *testing.T) {
		// create a token that will expire
		expiringToken, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
			token.ExpirationTime = expTime
		})
		require.NoError(t, err)

		clock.Set(expTime.Add(1 * time.Hour)) // now 'expiringToken' is expired

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "new-description",
				AccessorID:     expiringToken.AccessorID,
				ExpirationTime: clock.Now().Add(3 * time.Hour),
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		resp := structs.ACLToken{}

		err = acl.TokenSet(&req, &resp)
		requireErrorContains(t, err, "Cannot find token")
	})
}

func TestACLEndpoint_TokenSet_anon(t *testing.T) {
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
	require.NoError(t, err)

	acl := s1.GetEndpoint("ACL").(*ACL)

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
	require.NoError(t, err)
	require.NotEmpty(t, token.SecretID)

	tokenResp, err := retrieveTestToken(codec, "root", "dc1", structs.ACLTokenAnonymousID)
	require.Equal(t, len(tokenResp.Token.Policies), 1)
	require.Equal(t, tokenResp.Token.Policies[0].ID, policy.ID)
}

func TestACLEndpoint_TokenDelete(t *testing.T) {
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

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.Datacenter = "dc2"
		// token replication is required to test deleting non-local tokens in secondary dc
		c.ACLTokenReplication = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	s2.tokens.UpdateACLReplicationToken("root")

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Try to join
	joinWAN(t, s2, s1)

	clock := newStoppedClock(time.Now())

	// Grab the underlying handlers and program their clocks.
	acl := s1.GetEndpoint("ACL").(*ACL)
	acl.clock = clock
	acl2 := s2.GetEndpoint("ACL").(*ACL)
	acl2.clock = clock

	existingToken, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	expTime := clock.Now().Add(2 * time.Minute)

	t.Run("deletes a token that has an expiration time in the future", func(t *testing.T) {
		// create a token that will expire
		testToken, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
			token.ExpirationTime = expTime
		})
		require.NoError(t, err)

		clock.Set(expTime.Add(-30 * time.Second))

		// Make sure the token is listable
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", testToken.AccessorID)
		require.NoError(t, err)
		require.NotNil(t, tokenResp.Token)

		// Now try to delete it (this should work).
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      testToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// Make sure the token is gone
		tokenResp, err = retrieveTestToken(codec, "root", "dc1", testToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, tokenResp.Token)
	})

	clock.Reset()

	expTime = clock.Now().Add(2 * time.Minute)

	t.Run("deletes a token that is past its expiration time", func(t *testing.T) {
		// create a token that will expire
		expiringToken, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
			token.ExpirationTime = expTime
		})
		require.NoError(t, err)

		clock.Set(expTime.Add(1 * time.Hour)) // now 'expiringToken' is expired

		// Make sure the token is not listable (filtered due to expiry)
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", expiringToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, tokenResp.Token)

		// Now try to delete it (this should work).
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      expiringToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// Make sure the token is still gone (this time it's actually gone)
		tokenResp, err = retrieveTestToken(codec, "root", "dc1", expiringToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, tokenResp.Token)
	})

	clock.Reset()

	t.Run("deletes a token", func(t *testing.T) {
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      existingToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// Make sure the token is gone
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", existingToken.AccessorID)
		require.Nil(t, tokenResp.Token)
		require.NoError(t, err)
	})

	clock.Reset()

	t.Run("can't delete itself", func(t *testing.T) {
		readReq := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      "root",
			TokenIDType:  structs.ACLTokenSecret,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		var out structs.ACLTokenResponse

		err := acl.TokenRead(&readReq, &out)

		require.NoError(t, err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      out.Token.AccessorID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string
		err = acl.TokenDelete(&req, &resp)
		require.EqualError(t, err, "Deletion of the request's authorization token is not permitted")
	})

	clock.Reset()

	t.Run("errors when token doesn't exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      fakeID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// token should be nil
		tokenResp, err := retrieveTestToken(codec, "root", "dc1", existingToken.AccessorID)
		require.Nil(t, tokenResp.Token)
		require.NoError(t, err)
	})

	clock.Reset()

	t.Run("don't segfault when attempting to delete non existant token in secondary dc", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc2",
			TokenID:      fakeID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var resp string

		waitForNewACLs(t, s2)

		err = acl2.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// token should be nil
		tokenResp, err := retrieveTestToken(codec2, "root", "dc1", existingToken.AccessorID)
		require.Nil(t, tokenResp.Token)
		require.NoError(t, err)
	})
}

func TestACLEndpoint_TokenDelete_anon(t *testing.T) {
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

	acl := s1.GetEndpoint("ACL").(*ACL)

	req := structs.ACLTokenDeleteRequest{
		Datacenter:   "dc1",
		TokenID:      structs.ACLTokenAnonymousID,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	var resp string

	err := acl.TokenDelete(&req, &resp)
	require.EqualError(t, err, "Delete operation not permitted on the anonymous token")

	// Make sure the token is still there
	tokenResp, err := retrieveTestToken(codec, "root", "dc1", structs.ACLTokenAnonymousID)
	require.NotNil(t, tokenResp.Token)
}

func TestACLEndpoint_TokenList(t *testing.T) {
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

	clock := newStoppedClock(time.Now())
	expTime := clock.Now().Add(2 * time.Minute)

	acl := s1.GetEndpoint("ACL").(*ACL)
	acl.clock = clock

	t1, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	t2, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	t3, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTime = expTime
	})
	require.NoError(t, err)

	masterTokenAccessorID, err := retrieveTestTokenAccessorForSecret(codec, "root", "dc1", "root")
	require.NoError(t, err)

	t.Run("normal", func(t *testing.T) {
		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenListResponse{}

		err = acl.TokenList(&req, &resp)
		require.NoError(t, err)

		tokens := []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
			t1.AccessorID,
			t2.AccessorID,
			t3.AccessorID,
		}

		var retrievedTokens []string
		for _, v := range resp.Tokens {
			retrievedTokens = append(retrievedTokens, v.AccessorID)
		}
		require.ElementsMatch(t, retrievedTokens, tokens)
	})

	clock.Add(1 * time.Hour) // now 't3' is expired

	t.Run("filter expired", func(t *testing.T) {
		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenListResponse{}

		err = acl.TokenList(&req, &resp)
		require.NoError(t, err)

		tokens := []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
			t1.AccessorID,
			t2.AccessorID,
		}

		var retrievedTokens []string
		for _, v := range resp.Tokens {
			retrievedTokens = append(retrievedTokens, v.AccessorID)
		}
		require.ElementsMatch(t, retrievedTokens, tokens)
	})
}

func TestACLEndpoint_TokenBatchRead(t *testing.T) {
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

	clock := newStoppedClock(time.Now())
	expTime := clock.Now().Add(2 * time.Minute)

	acl := s1.GetEndpoint("ACL").(*ACL)
	acl.clock = clock

	t1, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	t2, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	t3, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTime = expTime
	})
	require.NoError(t, err)

	t.Run("normal", func(t *testing.T) {
		tokens := []string{t1.AccessorID, t2.AccessorID, t3.AccessorID}

		req := structs.ACLTokenBatchGetRequest{
			Datacenter:   "dc1",
			AccessorIDs:  tokens,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenBatchResponse{}

		err = acl.TokenBatchRead(&req, &resp)
		require.NoError(t, err)

		var retrievedTokens []string

		for _, v := range resp.Tokens {
			retrievedTokens = append(retrievedTokens, v.AccessorID)
		}
		require.EqualValues(t, retrievedTokens, tokens)
	})

	clock.Add(1 * time.Hour) // now 't3' is expired

	t.Run("returns expired tokens", func(t *testing.T) {
		tokens := []string{t1.AccessorID, t2.AccessorID, t3.AccessorID}

		req := structs.ACLTokenBatchGetRequest{
			Datacenter:   "dc1",
			AccessorIDs:  tokens,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		resp := structs.ACLTokenBatchResponse{}

		err = acl.TokenBatchRead(&req, &resp)
		require.NoError(t, err)

		var retrievedTokens []string

		for _, v := range resp.Tokens {
			retrievedTokens = append(retrievedTokens, v.AccessorID)
		}
		require.EqualValues(t, retrievedTokens, tokens)
	})
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

	acl := s1.GetEndpoint("ACL").(*ACL)

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

	t1, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	t2, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	acl := s1.GetEndpoint("ACL").(*ACL)
	tokens := []string{t1.AccessorID, t2.AccessorID}

	req := structs.ACLTokenBatchGetRequest{
		Datacenter:   "dc1",
		AccessorIDs:  tokens,
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLTokenBatchResponse{}

	err = acl.TokenBatchRead(&req, &resp)
	require.NoError(t, err)

	var retrievedTokens []string

	for _, v := range resp.Tokens {
		retrievedTokens = append(retrievedTokens, v.AccessorID)
	}
	require.EqualValues(t, retrievedTokens, tokens)
}

func TestACLEndpoint_PolicySet(t *testing.T) {
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

	acl := s1.GetEndpoint("ACL").(*ACL)
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
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the policy directly to validate that it exists
		policyResp, err := retrieveTestPolicy(codec, "root", "dc1", resp.ID)
		require.NoError(t, err)
		policy := policyResp.Policy

		require.NotNil(t, policy.ID)
		require.Equal(t, policy.Description, "foobar")
		require.Equal(t, policy.Name, "baz")
		require.Equal(t, policy.Rules, "service \"\" { policy = \"read\" }")

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
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the policy directly to validate that it exists
		policyResp, err := retrieveTestPolicy(codec, "root", "dc1", resp.ID)
		require.NoError(t, err)
		policy := policyResp.Policy

		require.NotNil(t, policy.ID)
		require.Equal(t, policy.Description, "bat")
		require.Equal(t, policy.Name, "bar")
		require.Equal(t, policy.Rules, "service \"\" { policy = \"write\" }")
	}
}

func TestACLEndpoint_PolicySet_globalManagement(t *testing.T) {
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

	acl := s1.GetEndpoint("ACL").(*ACL)

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
		require.EqualError(t, err, "Changing the Rules for the builtin global-management policy is not permitted")
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
		require.NoError(t, err)

		// Get the policy again
		policyResp, err := retrieveTestPolicy(codec, "root", "dc1", structs.ACLPolicyGlobalManagementID)
		require.NoError(t, err)
		policy := policyResp.Policy

		require.Equal(t, policy.ID, structs.ACLPolicyGlobalManagementID)
		require.Equal(t, policy.Name, "foobar")

	}
}

func TestACLEndpoint_PolicyDelete(t *testing.T) {
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

	existingPolicy, err := upsertTestPolicy(codec, "root", "dc1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl := s1.GetEndpoint("ACL").(*ACL)

	req := structs.ACLPolicyDeleteRequest{
		Datacenter:   "dc1",
		PolicyID:     existingPolicy.ID,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	var resp string

	err = acl.PolicyDelete(&req, &resp)
	require.NoError(t, err)

	// Make sure the policy is gone
	tokenResp, err := retrieveTestPolicy(codec, "root", "dc1", existingPolicy.ID)
	require.Nil(t, tokenResp.Policy)
}

func TestACLEndpoint_PolicyDelete_globalManagement(t *testing.T) {
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

	acl := s1.GetEndpoint("ACL").(*ACL)

	req := structs.ACLPolicyDeleteRequest{
		Datacenter:   "dc1",
		PolicyID:     structs.ACLPolicyGlobalManagementID,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var resp string

	err := acl.PolicyDelete(&req, &resp)

	require.EqualError(t, err, "Delete operation not permitted on the builtin global-management policy")
}

func TestACLEndpoint_PolicyList(t *testing.T) {
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

	p1, err := upsertTestPolicy(codec, "root", "dc1")
	require.NoError(t, err)

	p2, err := upsertTestPolicy(codec, "root", "dc1")
	require.NoError(t, err)

	acl := s1.GetEndpoint("ACL").(*ACL)

	req := structs.ACLPolicyListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	resp := structs.ACLPolicyListResponse{}

	err = acl.PolicyList(&req, &resp)
	require.NoError(t, err)

	policies := []string{
		structs.ACLPolicyGlobalManagementID,
		p1.ID,
		p2.ID,
	}
	var retrievedPolicies []string

	for _, v := range resp.Policies {
		retrievedPolicies = append(retrievedPolicies, v.ID)
	}
	require.ElementsMatch(t, retrievedPolicies, policies)
}

func TestACLEndpoint_PolicyResolve(t *testing.T) {
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

	p1, err := upsertTestPolicy(codec, "root", "dc1")
	require.NoError(t, err)

	p2, err := upsertTestPolicy(codec, "root", "dc1")
	require.NoError(t, err)

	acl := s1.GetEndpoint("ACL").(*ACL)

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
	require.NoError(t, err)
	require.NotEmpty(t, token.SecretID)

	resp := structs.ACLPolicyBatchResponse{}
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter:   "dc1",
		PolicyIDs:    []string{p1.ID, p2.ID},
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	err = acl.PolicyResolve(&req, &resp)
	require.NoError(t, err)

	var retrievedPolicies []string

	for _, v := range resp.Policies {
		retrievedPolicies = append(retrievedPolicies, v.ID)
	}
	require.EqualValues(t, retrievedPolicies, policies)
}

// upsertTestToken creates a token for testing purposes
func upsertTestToken(codec rpc.ClientCodec, masterToken string, datacenter string,
	tokenModificationFn func(token *structs.ACLToken)) (*structs.ACLToken, error) {
	arg := structs.ACLTokenSetRequest{
		Datacenter: datacenter,
		ACLToken: structs.ACLToken{
			Description: "User token",
			Local:       false,
			Policies:    nil,
		},
		WriteRequest: structs.WriteRequest{Token: masterToken},
	}

	if tokenModificationFn != nil {
		tokenModificationFn(&arg.ACLToken)
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

func retrieveTestTokenAccessorForSecret(codec rpc.ClientCodec, masterToken string, datacenter string, id string) (string, error) {
	arg := structs.ACLTokenGetRequest{
		TokenID:      "root",
		TokenIDType:  structs.ACLTokenSecret,
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "root"},
	}

	var out structs.ACLTokenResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &arg, &out)

	if err != nil {
		return "", err
	}

	if out.Token == nil {
		return "", nil
	}

	return out.Token.AccessorID, nil
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

func deleteTestPolicy(codec rpc.ClientCodec, masterToken string, datacenter string, policyID string) error {
	arg := structs.ACLPolicyDeleteRequest{
		Datacenter:   datacenter,
		PolicyID:     policyID,
		WriteRequest: structs.WriteRequest{Token: masterToken},
	}

	var ignored string
	err := msgpackrpc.CallWithCodec(codec, "ACL.PolicyDelete", &arg, &ignored)
	return err
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

func requireTimeEquals(t *testing.T, expect, got time.Time) {
	t.Helper()
	if !expect.Equal(got) {
		t.Fatalf("expected=%q != got=%q", expect, got)
	}
}

func requireErrorContains(t *testing.T, err error, expectedErrorMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("An error is expected but got nil.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("unexpected error: %v", err)
	}
}
