package token

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestStore_Load(t *testing.T) {
	dataDir := testutil.TempDir(t, "datadir")
	tokenFile := filepath.Join(dataDir, tokensPath)
	logger := hclog.New(nil)
	store := new(Store)

	t.Run("with empty store", func(t *testing.T) {
		cfg := Config{
			DataDir:             dataDir,
			ACLAgentToken:       "alfa",
			ACLAgentRootToken:   "bravo",
			ACLDefaultToken:     "charlie",
			ACLReplicationToken: "delta",
		}
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "alfa", store.AgentToken())
		require.Equal(t, "bravo", store.AgentRootToken())
		require.Equal(t, "charlie", store.UserToken())
		require.Equal(t, "delta", store.ReplicationToken())
	})

	t.Run("updated from Config", func(t *testing.T) {
		cfg := Config{
			DataDir:             dataDir,
			ACLDefaultToken:     "echo",
			ACLAgentToken:       "foxtrot",
			ACLAgentRootToken:   "golf",
			ACLReplicationToken: "hotel",
		}
		// ensures no error for missing persisted tokens file
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "echo", store.UserToken())
		require.Equal(t, "foxtrot", store.AgentToken())
		require.Equal(t, "golf", store.AgentRootToken())
		require.Equal(t, "hotel", store.ReplicationToken())
	})

	t.Run("with persisted tokens", func(t *testing.T) {
		cfg := Config{
			DataDir:             dataDir,
			ACLDefaultToken:     "echo",
			ACLAgentToken:       "foxtrot",
			ACLAgentRootToken:   "golf",
			ACLReplicationToken: "hotel",
		}

		tokens := `{
			"agent" : "india",
			"agent_root" : "juliett",
			"default": "kilo",
			"replication" : "lima"
		}`

		require.NoError(t, ioutil.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		// no updates since token persistence is not enabled
		require.Equal(t, "echo", store.UserToken())
		require.Equal(t, "foxtrot", store.AgentToken())
		require.Equal(t, "golf", store.AgentRootToken())
		require.Equal(t, "hotel", store.ReplicationToken())

		cfg.EnablePersistence = true
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "india", store.AgentToken())
		require.Equal(t, "juliett", store.AgentRootToken())
		require.Equal(t, "kilo", store.UserToken())
		require.Equal(t, "lima", store.ReplicationToken())

		// check store persistence was enabled
		require.NotNil(t, store.persistence)
	})

	t.Run("with persisted tokens, persisted tokens override config", func(t *testing.T) {
		tokens := `{
			"agent" : "mike",
			"agent_root" : "november",
			"default": "oscar",
			"replication" : "papa"
		}`

		cfg := Config{
			EnablePersistence:   true,
			DataDir:             dataDir,
			ACLDefaultToken:     "quebec",
			ACLAgentToken:       "romeo",
			ACLAgentRootToken:   "sierra",
			ACLReplicationToken: "tango",
		}

		require.NoError(t, ioutil.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "mike", store.AgentToken())
		require.Equal(t, "november", store.AgentRootToken())
		require.Equal(t, "oscar", store.UserToken())
		require.Equal(t, "papa", store.ReplicationToken())
	})

	t.Run("with some persisted tokens", func(t *testing.T) {
		tokens := `{
			"agent" : "uniform",
			"agent_root" : "victor"
		}`

		cfg := Config{
			EnablePersistence:   true,
			DataDir:             dataDir,
			ACLDefaultToken:     "whiskey",
			ACLAgentToken:       "xray",
			ACLAgentRootToken:   "yankee",
			ACLReplicationToken: "zulu",
		}

		require.NoError(t, ioutil.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "uniform", store.AgentToken())
		require.Equal(t, "victor", store.AgentRootToken())
		require.Equal(t, "whiskey", store.UserToken())
		require.Equal(t, "zulu", store.ReplicationToken())
	})

	t.Run("persisted file contains invalid data", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:   true,
			DataDir:             dataDir,
			ACLDefaultToken:     "one",
			ACLAgentToken:       "two",
			ACLAgentRootToken:   "three",
			ACLReplicationToken: "four",
		}

		require.NoError(t, ioutil.WriteFile(tokenFile, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "one", store.UserToken())
		require.Equal(t, "two", store.AgentToken())
		require.Equal(t, "three", store.AgentRootToken())
		require.Equal(t, "four", store.ReplicationToken())
	})

	t.Run("persisted file contains invalid json", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:   true,
			DataDir:             dataDir,
			ACLDefaultToken:     "alfa",
			ACLAgentToken:       "bravo",
			ACLAgentRootToken:   "charlie",
			ACLReplicationToken: "foxtrot",
		}

		require.NoError(t, ioutil.WriteFile(tokenFile, []byte("[1,2,3]"), 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "alfa", store.UserToken())
		require.Equal(t, "bravo", store.AgentToken())
		require.Equal(t, "charlie", store.AgentRootToken())
		require.Equal(t, "foxtrot", store.ReplicationToken())
	})
}

func TestStore_WithPersistenceLock(t *testing.T) {
	dataDir := testutil.TempDir(t, "datadir")
	store := new(Store)
	cfg := Config{
		EnablePersistence:   true,
		DataDir:             dataDir,
		ACLDefaultToken:     "default-token",
		ACLAgentToken:       "agent-token",
		ACLAgentRootToken:   "root-token",
		ACLReplicationToken: "replication-token",
	}
	err := store.Load(cfg, hclog.New(nil))
	require.NoError(t, err)

	f := func() error {
		updated := store.UpdateUserToken("the-new-token", TokenSourceAPI)
		require.True(t, updated)

		updated = store.UpdateAgentRootToken("the-new-root-token", TokenSourceAPI)
		require.True(t, updated)
		return nil
	}

	err = store.WithPersistenceLock(f)
	require.NoError(t, err)

	tokens, err := readPersistedFromFile(filepath.Join(dataDir, tokensPath))
	require.NoError(t, err)
	expected := persistedTokens{
		Default:   "the-new-token",
		AgentRoot: "the-new-root-token",
	}
	require.Equal(t, expected, tokens)
}
