package token

import (
	"os"
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
			DataDir:               dataDir,
			ACLAgentToken:         "alfa",
			ACLAgentRecoveryToken: "bravo",
			ACLDefaultToken:       "charlie",
			ACLReplicationToken:   "delta",
		}
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "alfa", store.AgentToken())
		require.Equal(t, "bravo", store.AgentRecoveryToken())
		require.Equal(t, "charlie", store.UserToken())
		require.Equal(t, "delta", store.ReplicationToken())
	})

	t.Run("updated from Config", func(t *testing.T) {
		cfg := Config{
			DataDir:               dataDir,
			ACLDefaultToken:       "echo",
			ACLAgentToken:         "foxtrot",
			ACLAgentRecoveryToken: "golf",
			ACLReplicationToken:   "hotel",
		}
		// ensures no error for missing persisted tokens file
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "echo", store.UserToken())
		require.Equal(t, "foxtrot", store.AgentToken())
		require.Equal(t, "golf", store.AgentRecoveryToken())
		require.Equal(t, "hotel", store.ReplicationToken())
	})

	t.Run("with persisted tokens", func(t *testing.T) {
		cfg := Config{
			DataDir:               dataDir,
			ACLDefaultToken:       "echo",
			ACLAgentToken:         "foxtrot",
			ACLAgentRecoveryToken: "golf",
			ACLReplicationToken:   "hotel",
		}

		tokens := `{
			"agent" : "india",
			"agent_recovery" : "juliett",
			"default": "kilo",
			"replication" : "lima"
		}`

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		// no updates since token persistence is not enabled
		require.Equal(t, "echo", store.UserToken())
		require.Equal(t, "foxtrot", store.AgentToken())
		require.Equal(t, "golf", store.AgentRecoveryToken())
		require.Equal(t, "hotel", store.ReplicationToken())

		cfg.EnablePersistence = true
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "india", store.AgentToken())
		require.Equal(t, "juliett", store.AgentRecoveryToken())
		require.Equal(t, "kilo", store.UserToken())
		require.Equal(t, "lima", store.ReplicationToken())

		// check store persistence was enabled
		require.NotNil(t, store.persistence)
	})

	t.Run("persisted tokens include pre-1.11 agent_master naming", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:     true,
			DataDir:               dataDir,
			ACLAgentRecoveryToken: "golf",
		}

		tokens := `{"agent_master": "juliett"}`
		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "juliett", store.AgentRecoveryToken())
	})

	t.Run("with persisted tokens, persisted tokens override config", func(t *testing.T) {
		tokens := `{
			"agent" : "mike",
			"agent_recovery" : "november",
			"default": "oscar",
			"replication" : "papa"
		}`

		cfg := Config{
			EnablePersistence:     true,
			DataDir:               dataDir,
			ACLDefaultToken:       "quebec",
			ACLAgentToken:         "romeo",
			ACLAgentRecoveryToken: "sierra",
			ACLReplicationToken:   "tango",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "mike", store.AgentToken())
		require.Equal(t, "november", store.AgentRecoveryToken())
		require.Equal(t, "oscar", store.UserToken())
		require.Equal(t, "papa", store.ReplicationToken())
	})

	t.Run("with some persisted tokens", func(t *testing.T) {
		tokens := `{
			"agent" : "uniform",
			"agent_recovery" : "victor"
		}`

		cfg := Config{
			EnablePersistence:     true,
			DataDir:               dataDir,
			ACLDefaultToken:       "whiskey",
			ACLAgentToken:         "xray",
			ACLAgentRecoveryToken: "yankee",
			ACLReplicationToken:   "zulu",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "uniform", store.AgentToken())
		require.Equal(t, "victor", store.AgentRecoveryToken())
		require.Equal(t, "whiskey", store.UserToken())
		require.Equal(t, "zulu", store.ReplicationToken())
	})

	t.Run("persisted file contains invalid data", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:     true,
			DataDir:               dataDir,
			ACLDefaultToken:       "one",
			ACLAgentToken:         "two",
			ACLAgentRecoveryToken: "three",
			ACLReplicationToken:   "four",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "one", store.UserToken())
		require.Equal(t, "two", store.AgentToken())
		require.Equal(t, "three", store.AgentRecoveryToken())
		require.Equal(t, "four", store.ReplicationToken())
	})

	t.Run("persisted file contains invalid json", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:     true,
			DataDir:               dataDir,
			ACLDefaultToken:       "alfa",
			ACLAgentToken:         "bravo",
			ACLAgentRecoveryToken: "charlie",
			ACLReplicationToken:   "foxtrot",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte("[1,2,3]"), 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "alfa", store.UserToken())
		require.Equal(t, "bravo", store.AgentToken())
		require.Equal(t, "charlie", store.AgentRecoveryToken())
		require.Equal(t, "foxtrot", store.ReplicationToken())
	})
}

func TestStore_WithPersistenceLock(t *testing.T) {
	dataDir := testutil.TempDir(t, "datadir")
	store := new(Store)
	cfg := Config{
		EnablePersistence:     true,
		DataDir:               dataDir,
		ACLDefaultToken:       "default-token",
		ACLAgentToken:         "agent-token",
		ACLAgentRecoveryToken: "recovery-token",
		ACLReplicationToken:   "replication-token",
	}
	err := store.Load(cfg, hclog.New(nil))
	require.NoError(t, err)

	f := func() error {
		updated := store.UpdateUserToken("the-new-token", TokenSourceAPI)
		require.True(t, updated)

		updated = store.UpdateAgentRecoveryToken("the-new-recovery-token", TokenSourceAPI)
		require.True(t, updated)
		return nil
	}

	err = store.WithPersistenceLock(f)
	require.NoError(t, err)

	tokens, err := readPersistedFromFile(filepath.Join(dataDir, tokensPath))
	require.NoError(t, err)
	expected := persistedTokens{
		Default:       "the-new-token",
		AgentRecovery: "the-new-recovery-token",
	}
	require.Equal(t, expected, tokens)
}
