// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package token

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
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
			ACLDNSToken:                    "foxtrot",
		}
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "alfa", store.AgentToken())
		require.Equal(t, "bravo", store.AgentRecoveryToken())
		require.Equal(t, "charlie", store.UserToken())
		require.Equal(t, "delta", store.ReplicationToken())
		require.Equal(t, "echo", store.ConfigFileRegistrationToken())
		require.Equal(t, "foxtrot", store.DNSToken())
	})

	t.Run("updated from Config", func(t *testing.T) {
		cfg := Config{
			DataDir:                        dataDir,
			ACLDefaultToken:                "sierra",
			ACLAgentToken:                  "tango",
			ACLAgentRecoveryToken:          "uniform",
			ACLReplicationToken:            "victor",
			ACLConfigFileRegistrationToken: "xray",
			ACLDNSToken:                    "zulu",
		}
		// ensures no error for missing persisted tokens file
		require.NoError(t, store.Load(cfg, logger))
		require.Equal(t, "sierra", store.UserToken())
		require.Equal(t, "tango", store.AgentToken())
		require.Equal(t, "uniform", store.AgentRecoveryToken())
		require.Equal(t, "victor", store.ReplicationToken())
		require.Equal(t, "xray", store.ConfigFileRegistrationToken())
		require.Equal(t, "zulu", store.DNSToken())
	})

	t.Run("with persisted tokens", func(t *testing.T) {
		cfg := Config{
			DataDir:                        dataDir,
			ACLDefaultToken:                "alpha",
			ACLAgentToken:                  "bravo",
			ACLAgentRecoveryToken:          "charlie",
			ACLReplicationToken:            "delta",
			ACLConfigFileRegistrationToken: "echo",
			ACLDNSToken:                    "foxtrot",
		}

		tokens := `{
			"agent" : "golf",
			"agent_recovery" : "hotel",
			"default": "india",
			"replication": "juliet",
			"config_file_service_registration": "kilo",
			"dns": "lima"
		}`

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		// no updates since token persistence is not enabled
		require.Equal(t, "alpha", store.UserToken())
		require.Equal(t, "bravo", store.AgentToken())
		require.Equal(t, "charlie", store.AgentRecoveryToken())
		require.Equal(t, "delta", store.ReplicationToken())
		require.Equal(t, "echo", store.ConfigFileRegistrationToken())
		require.Equal(t, "foxtrot", store.DNSToken())

		cfg.EnablePersistence = true
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "golf", store.AgentToken())
		require.Equal(t, "hotel", store.AgentRecoveryToken())
		require.Equal(t, "india", store.UserToken())
		require.Equal(t, "juliet", store.ReplicationToken())
		require.Equal(t, "kilo", store.ConfigFileRegistrationToken())
		require.Equal(t, "lima", store.DNSToken())

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
			"config_file_service_registration" : "lima",
			"dns": "kilo"
		}`

		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "quebec",
			ACLAgentToken:                  "romeo",
			ACLAgentRecoveryToken:          "sierra",
			ACLReplicationToken:            "tango",
			ACLConfigFileRegistrationToken: "uniform",
			ACLDNSToken:                    "victor",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "mike", store.AgentToken())
		require.Equal(t, "november", store.AgentRecoveryToken())
		require.Equal(t, "oscar", store.UserToken())
		require.Equal(t, "papa", store.ReplicationToken())
		require.Equal(t, "lima", store.ConfigFileRegistrationToken())
		require.Equal(t, "kilo", store.DNSToken())
	})

	t.Run("with some persisted tokens", func(t *testing.T) {
		tokens := `{
			"agent" : "xray",
			"agent_recovery" : "zulu"
		}`

		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "alpha",
			ACLAgentToken:                  "bravo",
			ACLAgentRecoveryToken:          "charlie",
			ACLReplicationToken:            "delta",
			ACLConfigFileRegistrationToken: "echo",
			ACLDNSToken:                    "foxtrot",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte(tokens), 0600))
		require.NoError(t, store.Load(cfg, logger))

		require.Equal(t, "xray", store.AgentToken())
		require.Equal(t, "zulu", store.AgentRecoveryToken())

		require.Equal(t, "alpha", store.UserToken())
		require.Equal(t, "delta", store.ReplicationToken())
		require.Equal(t, "echo", store.ConfigFileRegistrationToken())
		require.Equal(t, "foxtrot", store.DNSToken())
	})

	t.Run("persisted file contains invalid data", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "alpha",
			ACLAgentToken:                  "bravo",
			ACLAgentRecoveryToken:          "charlie",
			ACLReplicationToken:            "delta",
			ACLConfigFileRegistrationToken: "echo",
			ACLDNSToken:                    "foxtrot",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "alpha", store.UserToken())
		require.Equal(t, "bravo", store.AgentToken())
		require.Equal(t, "charlie", store.AgentRecoveryToken())
		require.Equal(t, "delta", store.ReplicationToken())
		require.Equal(t, "echo", store.ConfigFileRegistrationToken())
		require.Equal(t, "foxtrot", store.DNSToken())
	})

	t.Run("persisted file contains invalid json", func(t *testing.T) {
		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "alfa",
			ACLAgentToken:                  "bravo",
			ACLAgentRecoveryToken:          "charlie",
			ACLReplicationToken:            "delta",
			ACLConfigFileRegistrationToken: "echo",
			ACLDNSToken:                    "foxtrot",
		}

		require.NoError(t, os.WriteFile(tokenFile, []byte("[1,2,3]"), 0600))
		err := store.Load(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode tokens file")

		require.Equal(t, "alfa", store.UserToken())
		require.Equal(t, "bravo", store.AgentToken())
		require.Equal(t, "charlie", store.AgentRecoveryToken())
		require.Equal(t, "delta", store.ReplicationToken())
		require.Equal(t, "echo", store.ConfigFileRegistrationToken())
		require.Equal(t, "foxtrot", store.DNSToken())
	})
}

func TestStore_WithPersistenceLock(t *testing.T) {
	// ACLDefaultToken:                 alpha   --> sierra
	// ACLAgentToken:                   bravo   --> tango
	// ACLAgentRecoveryToken:           charlie --> uniform
	// ACLReplicationToken:             delta   --> victor
	// ACLConfigFileRegistrationToken:  echo    --> xray
	// ACLDNSToken:                     foxtrot --> zulu
	setupStore := func() (string, *Store) {
		dataDir := testutil.TempDir(t, "datadir")
		store := new(Store)
		cfg := Config{
			EnablePersistence:              true,
			DataDir:                        dataDir,
			ACLDefaultToken:                "alpha",
			ACLAgentToken:                  "bravo",
			ACLAgentRecoveryToken:          "charlie",
			ACLReplicationToken:            "delta",
			ACLConfigFileRegistrationToken: "echo",
			ACLDNSToken:                    "foxtrot",
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
			require.True(t, store.UpdateUserToken("sierra", TokenSourceAPI))
			require.True(t, store.UpdateAgentRecoveryToken("tango", TokenSourceAPI))
			return nil
		})
		require.NoError(t, err)

		// Only API-sourced tokens are persisted.
		requirePersistedTokens(t, dataDir, persistedTokens{
			Default:       "sierra",
			AgentRecovery: "tango",
		})
	})

	t.Run("persist all tokens", func(t *testing.T) {
		dataDir, store := setupStore()
		err := store.WithPersistenceLock(func() error {
			require.True(t, store.UpdateUserToken("sierra", TokenSourceAPI))
			require.True(t, store.UpdateAgentToken("tango", TokenSourceAPI))
			require.True(t, store.UpdateAgentRecoveryToken("uniform", TokenSourceAPI))
			require.True(t, store.UpdateReplicationToken("victor", TokenSourceAPI))
			require.True(t, store.UpdateConfigFileRegistrationToken("xray", TokenSourceAPI))
			require.True(t, store.UpdateDNSToken("zulu", TokenSourceAPI))
			return nil
		})
		require.NoError(t, err)

		requirePersistedTokens(t, dataDir, persistedTokens{
			Default:                "sierra",
			Agent:                  "tango",
			AgentRecovery:          "uniform",
			Replication:            "victor",
			ConfigFileRegistration: "xray",
			DNS:                    "zulu",
		})
	})

}
