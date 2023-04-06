// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestACLTokenReap_Primary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	t.Run("global", func(t *testing.T) {
		t.Parallel()
		testACLTokenReap_Primary(t, false, true)
	})
	t.Run("local", func(t *testing.T) {
		t.Parallel()
		testACLTokenReap_Primary(t, true, false)
	})
}

func testACLTokenReap_Primary(t *testing.T, local, global bool) {
	// -------------------------------------------
	// A word of caution when testing reapExpiredACLTokens():
	//
	// The underlying memdb index used for reaping has a minimum granularity of
	// 1 second as it delegates to `time.Unix()`. This test will have to be
	// deliberately slow to allow for necessary sleeps.  If you try to make it
	// operate faster (using expiration ttls of milliseconds) it will be flaky.
	// -------------------------------------------

	t.Helper()
	require.NotEqual(t, local, global)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 8 * time.Second
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	aclEp := ACL{srv: s1}

	initialManagementTokenAccessorID, err := retrieveTestTokenAccessorForSecret(codec, "root", "dc1", "root")
	require.NoError(t, err)

	listTokens := func() (localTokens, globalTokens []string, err error) {
		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		var res structs.ACLTokenListResponse
		err = aclEp.TokenList(&req, &res)
		if err != nil {
			return nil, nil, err
		}

		for _, tok := range res.Tokens {
			if tok.Local {
				localTokens = append(localTokens, tok.AccessorID)
			} else {
				globalTokens = append(globalTokens, tok.AccessorID)
			}
		}

		return localTokens, globalTokens, nil
	}

	requireTokenMatch := func(t *testing.T, expect []string) {
		t.Helper()

		var expectLocal, expectGlobal []string
		// The initial management token and the anonymous token are always
		// going to be present and global.
		expectGlobal = append(expectGlobal, initialManagementTokenAccessorID)
		expectGlobal = append(expectGlobal, acl.AnonymousTokenID)

		if local {
			expectLocal = append(expectLocal, expect...)
		} else {
			expectGlobal = append(expectGlobal, expect...)
		}

		localTokens, globalTokens, err := listTokens()
		require.NoError(t, err)
		require.ElementsMatch(t, expectLocal, localTokens)
		require.ElementsMatch(t, expectGlobal, globalTokens)
	}

	// initial sanity check
	requireTokenMatch(t, []string{})

	t.Run("no tokens", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		requireTokenMatch(t, []string{})
	})

	// 2 normal
	token1, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.Local = local
	})
	require.NoError(t, err)
	token2, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.Local = local
	})
	require.NoError(t, err)

	requireTokenMatch(t, []string{
		token1.AccessorID,
		token2.AccessorID,
	})

	t.Run("only normal tokens", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		requireTokenMatch(t, []string{
			token1.AccessorID,
			token2.AccessorID,
		})
	})

	// 2 expiring
	token3, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTTL = 1 * time.Second
		token.Local = local
	})
	require.NoError(t, err)
	token4, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTTL = 5 * time.Second
		token.Local = local
	})
	require.NoError(t, err)

	// 2 more normal
	token5, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.Local = local
	})
	require.NoError(t, err)
	token6, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.Local = local
	})
	require.NoError(t, err)

	requireTokenMatch(t, []string{
		token1.AccessorID,
		token2.AccessorID,
		token3.AccessorID,
		token4.AccessorID,
		token5.AccessorID,
		token6.AccessorID,
	})

	t.Run("mixed but nothing expired yet", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		requireTokenMatch(t, []string{
			token1.AccessorID,
			token2.AccessorID,
			token3.AccessorID,
			token4.AccessorID,
			token5.AccessorID,
			token6.AccessorID,
		})
	})

	time.Sleep(token3.ExpirationTime.Sub(time.Now()) + 10*time.Millisecond)

	t.Run("one should be reaped", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		requireTokenMatch(t, []string{
			token1.AccessorID,
			token2.AccessorID,
			// token3.AccessorID,
			token4.AccessorID,
			token5.AccessorID,
			token6.AccessorID,
		})
	})

	time.Sleep(token4.ExpirationTime.Sub(time.Now()) + 10*time.Millisecond)

	t.Run("two should be reaped", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		requireTokenMatch(t, []string{
			token1.AccessorID,
			token2.AccessorID,
			// token3.AccessorID,
			// token4.AccessorID,
			token5.AccessorID,
			token6.AccessorID,
		})
	})
}
