// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
			DataDir:                        dataDir,
			ACLAgentToken:                  "alfa",
			ACLAgentRecoveryToken:          "bravo",
			ACLDefaultToken:                "charlie",
			ACLReplicationToken:            "delta",
			ACLConfigFileRegistrationToken: "echo",
		}
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "alfa", store.AgentToken())
		require.Equal(t, "bravo", store.AgentRecoveryToken())
		require.Equal(t, "charlie", store.UserToken())
		require.Equal(t, "delta", store.ReplicationToken())
		require.Equal(t, "echo", store.ConfigFileRegistrationToken())
	})

	t.Run("updated from Config", func(t *testing.T) {
		cfg := Config{
			DataDir:                        dataDir,
			ACLDefaultToken:                "echo",
			ACLAgentToken:                  "foxtrot",
			ACLAgentRecoveryToken:          "golf",
			ACLReplicationToken:            "hotel",
			ACLConfigFileRegistrationToken: "india",
		}
		// ensures no error for missing persisted tokens file
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "echo", store.UserToken())
		require.Equal(t, "foxtrot", store.AgentToken())
		require.Equal(t, "golf", store.AgentRecoveryToken())
		require.Equal(t, "hotel", store.ReplicationToken())
		require.Equal(t, "india", store.ConfigFileRegistrationToken())
	})

	t.Run("with persisted tokens", func(t *testing.T) {
		cfg := Config{
			DataDir:                        dataDir,
			ACLDefaultToken:                "echo",
			ACLAgentToken:                  "foxtrot",
			ACLAgentRecoveryToken:          "golf",
			ACLReplicationToken:            "hotel",
			ACLConfigFileRegistrationToken: "delta",
		}

		tokens := `{
			"agent" : "india",
			"agent_recovery" : "juliett",
			"default": "kilo",
			"replication": "lima",
			"config_file_service_registration": "mike"
		}`

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		// no updates since token persistence is not enabled
		require.Equal(t, "echo", store.UserToken())
		require.Equal(t, "foxtrot", store.AgentToken())
		require.Equal(t, "golf", store.AgentRecoveryToken())
		require.Equal(t, "hotel", store.ReplicationToken())
		require.Equal(t, "delta", store.ConfigFileRegistrationToken())

		cfg.EnablePersistence = true
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "india", store.AgentToken())
		require.Equal(t, "juliett", store.AgentRecoveryToken())
		require.Equal(t, "kilo", store.UserToken())
		require.Equal(t, "lima", store.ReplicationToken())
		require.Equal(t, "mike", store.ConfigFileRegistrationToken())

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
			"replication" : "papa",
			"config_file_service_registration" : "lima"
		}`

		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "quebec",
			ACLAgentToken:                  "romeo",
			ACLAgentRecoveryToken:          "sierra",
			ACLReplicationToken:            "tango",
			ACLConfigFileRegistrationToken: "uniform",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "mike", store.AgentToken())
		require.Equal(t, "november", store.AgentRecoveryToken())
		require.Equal(t, "oscar", store.UserToken())
		require.Equal(t, "papa", store.ReplicationToken())
		require.Equal(t, "lima", store.ConfigFileRegistrationToken())
	})

	t.Run("with some persisted tokens", func(t *testing.T) {
		tokens := `{
			"agent" : "uniform",
			"agent_recovery" : "victor"
		}`

		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "whiskey",
			ACLAgentToken:                  "xray",
			ACLAgentRecoveryToken:          "yankee",
			ACLReplicationToken:            "zulu",
			ACLConfigFileRegistrationToken: "victor",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "uniform", store.AgentToken())
		require.Equal(t, "victor", store.AgentRecoveryToken())
		require.Equal(t, "whiskey", store.UserToken())
		require.Equal(t, "zulu", store.ReplicationToken())
		require.Equal(t, "victor", store.ConfigFileRegistrationToken())
	})

	t.Run("persisted file contains invalid data", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "one",
			ACLAgentToken:                  "two",
			ACLAgentRecoveryToken:          "three",
			ACLReplicationToken:            "four",
			ACLConfigFileRegistrationToken: "five",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "one", store.UserToken())
		require.Equal(t, "two", store.AgentToken())
		require.Equal(t, "three", store.AgentRecoveryToken())
		require.Equal(t, "four", store.ReplicationToken())
		require.Equal(t, "five", store.ConfigFileRegistrationToken())
	})

	t.Run("persisted file contains invalid json", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "alfa",
			ACLAgentToken:                  "bravo",
			ACLAgentRecoveryToken:          "charlie",
			ACLReplicationToken:            "foxtrot",
			ACLConfigFileRegistrationToken: "golf",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte("[1,2,3]"), 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "alfa", store.UserToken())
		require.Equal(t, "bravo", store.AgentToken())
		require.Equal(t, "charlie", store.AgentRecoveryToken())
		require.Equal(t, "foxtrot", store.ReplicationToken())
		require.Equal(t, "golf", store.ConfigFileRegistrationToken())
	})
}

func TestStore_WithPersistenceLock(t *testing.T) {
	setupStore := func() (string, *Store) {
		dataDir := testutil.TempDir(t, "datadir")
		store := new(Store)
		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "default-token",
			ACLAgentToken:                  "agent-token",
			ACLAgentRecoveryToken:          "recovery-token",
			ACLReplicationToken:            "replication-token",
			ACLConfigFileRegistrationToken: "registration-token",
		}
		err := store.Load(cfg, hclog.New(nil))
		require.NoError(t, err)

		return dataDir, store
	}

	requirePersistedTokens := func(t *testing.T, dataDir string, expected persistedTokens) {
		t.Helper()
		tokens, err := readPersistedFromFile(filepath.Join(dataDir, tokensPath))
		require.NoError(t, err)
		require.Equal(t, expected, tokens)
	}

	t.Run("persist some tokens", func(t *testing.T) {
		dataDir, store := setupStore()
		err := store.WithPersistenceLock(func() error {
			require.True(t, store.UpdateUserToken("the-new-default-token", TokenSourceAPI))
			require.True(t, store.UpdateAgentRecoveryToken("the-new-recovery-token", TokenSourceAPI))
			return nil
		})
		require.NoError(t, err)

		// Only API-sourced tokens are persisted.
		requirePersistedTokens(t, dataDir, persistedTokens{
			Default:       "the-new-default-token",
			AgentRecovery: "the-new-recovery-token",
		})
	})

	t.Run("persist all tokens", func(t *testing.T) {
		dataDir, store := setupStore()
		err := store.WithPersistenceLock(func() error {
			require.True(t, store.UpdateUserToken("the-new-default-token", TokenSourceAPI))
			require.True(t, store.UpdateAgentToken("the-new-agent-token", TokenSourceAPI))
			require.True(t, store.UpdateAgentRecoveryToken("the-new-recovery-token", TokenSourceAPI))
			require.True(t, store.UpdateReplicationToken("the-new-replication-token", TokenSourceAPI))
			require.True(t, store.UpdateConfigFileRegistrationToken("the-new-registration-token", TokenSourceAPI))
			return nil
		})
		require.NoError(t, err)

		requirePersistedTokens(t, dataDir, persistedTokens{
			Default:                "the-new-default-token",
			Agent:                  "the-new-agent-token",
			AgentRecovery:          "the-new-recovery-token",
			Replication:            "the-new-replication-token",
			ConfigFileRegistration: "the-new-registration-token",
		})
	})

}
