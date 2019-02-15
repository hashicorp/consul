package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestACLTokenReap_Primary_Global(t *testing.T) {
	t.Parallel()

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

	listTokens := func() ([]string, error) {
		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}

		var res structs.ACLTokenListResponse
		err := acl.TokenList(&req, &res)
		if err != nil {
			return nil, err
		}

		var out []string
		for _, tok := range res.Tokens {
			out = append(out, tok.AccessorID)
		}

		return out, nil
	}

	requireTokenMatch := func(t *testing.T, expect []string) {
		t.Helper()
		tokens, err := listTokens()
		require.NoError(t, err)
		require.ElementsMatch(t, expect, tokens)
	}

	// initial sanity check
	requireTokenMatch(t, []string{
		masterTokenAccessorID,
		structs.ACLTokenAnonymousID,
	})

	t.Run("no tokens", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(false, true, clock.Now)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		requireTokenMatch(t, []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
		})
	})

	clock.Reset()

	// 2 normal
	token1, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)
	token2, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	requireTokenMatch(t, []string{
		masterTokenAccessorID,
		structs.ACLTokenAnonymousID,
		token1.AccessorID,
		token2.AccessorID,
	})

	t.Run("only normal tokens", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(false, true, clock.Now)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		requireTokenMatch(t, []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
			token1.AccessorID,
			token2.AccessorID,
		})
	})

	clock.Reset()

	// 2 expiring
	token3, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTime = clock.Now().Add(1 * time.Minute)
	})
	require.NoError(t, err)
	token4, err := upsertTestToken(codec, "root", "dc1", func(token *structs.ACLToken) {
		token.ExpirationTime = clock.Now().Add(2 * time.Minute)
	})
	require.NoError(t, err)

	// 2 more normal
	token5, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)
	token6, err := upsertTestToken(codec, "root", "dc1", nil)
	require.NoError(t, err)

	requireTokenMatch(t, []string{
		masterTokenAccessorID,
		structs.ACLTokenAnonymousID,
		token1.AccessorID,
		token2.AccessorID,
		token3.AccessorID,
		token4.AccessorID,
		token5.AccessorID,
		token6.AccessorID,
	})

	t.Run("mixed but nothing expired yet", func(t *testing.T) {
		n, err := s1.reapExpiredACLTokens(false, true, clock.Now)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		requireTokenMatch(t, []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
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

		n, err := s1.reapExpiredACLTokens(false, true, clock.Now)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// rewind time to actually list the expired tokens
		clock.Set(prevTime)

		requireTokenMatch(t, []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
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

		n, err := s1.reapExpiredACLTokens(false, true, clock.Now)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// rewind time to actually list the expired tokens
		clock.Set(prevTime)

		requireTokenMatch(t, []string{
			masterTokenAccessorID,
			structs.ACLTokenAnonymousID,
			token1.AccessorID,
			token2.AccessorID,
			// token3.AccessorID,
			// token4.AccessorID,
			token5.AccessorID,
			token6.AccessorID,
		})
	})
}
