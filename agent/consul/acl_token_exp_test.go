package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestACLTokenReap_Primary(t *testing.T) {
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
	t.Helper()
	require.NotEqual(t, local, global)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	clock := newStoppedClock(time.Now())
	s1.clock = clock

	acl := ACL{s1}

	masterTokenAccessorID, err := retrieveTestTokenAccessorForSecret(codec, "root", "dc1", "root")
	require.NoError(t, err)

	listTokens := func() (localTokens, globalTokens []string, err error) {
		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		var res structs.ACLTokenListResponse
		err = acl.TokenList(&req, &res)
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
		// The master token and the anonymous token are always going to be
		// present and global.
		expectGlobal = append(expectGlobal, masterTokenAccessorID)
		expectGlobal = append(expectGlobal, structs.ACLTokenAnonymousID)

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

	clock.Reset()

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

	clock.Reset()

	// 2 expiring
	token3, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTime = clock.Now().Add(1 * time.Minute)
		token.Local = local
	})
	require.NoError(t, err)
	token4, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTime = clock.Now().Add(2 * time.Minute)
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

	t.Run("one should be reaped", func(t *testing.T) {
		prevTime := clock.Add(1*time.Minute + 1*time.Second)

		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// rewind time to actually list the expired tokens
		clock.Set(prevTime)

		requireTokenMatch(t, []string{
			token1.AccessorID,
			token2.AccessorID,
			// token3.AccessorID,
			token4.AccessorID,
			token5.AccessorID,
			token6.AccessorID,
		})
	})

	t.Run("one should be reaped", func(t *testing.T) {
		prevTime := clock.Add(25 * time.Hour)

		n, err := s1.reapExpiredACLTokens(local, global)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// rewind time to actually list the expired tokens
		clock.Set(prevTime)

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
