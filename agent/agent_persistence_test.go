package agent

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestAgent_persistCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// Create the TTL check to persist
	check := &checks.CheckTTL{
		CheckID: "check1",
		TTL:     10 * time.Minute,
	}

	// Persist some check state for the check
	err := a.persistCheckState(check, api.HealthCritical, "nope")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the persisted file exists and has the content
	file := filepath.Join(a.Config.DataDir, checkStateDir, stringHash("check1"))
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Decode the state
	var p persistedCheckState
	if err := json.Unmarshal(buf, &p); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the fields
	if p.CheckID != "check1" {
		t.Fatalf("bad: %#v", p)
	}
	if p.Output != "nope" {
		t.Fatalf("bad: %#v", p)
	}
	if p.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", p)
	}

	// Check the expiration time was set
	if p.Expires < time.Now().Unix() {
		t.Fatalf("bad: %#v", p)
	}
}

func TestAgent_loadCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// Create a check whose state will expire immediately
	check := &checks.CheckTTL{
		CheckID: "check1",
		TTL:     0,
	}

	// Persist the check state
	err := a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try to load the state
	health := &structs.HealthCheck{
		CheckID: "check1",
		Status:  api.HealthCritical,
	}
	if err := a.loadCheckState(health); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should not have restored the status due to expiration
	if health.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", health)
	}
	if health.Output != "" {
		t.Fatalf("bad: %#v", health)
	}

	// Should have purged the state
	file := filepath.Join(a.Config.DataDir, checksDir, stringHash("check1"))
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should have purged state")
	}

	// Set a TTL which will not expire before we check it
	check.TTL = time.Minute
	err = a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try to load
	if err := a.loadCheckState(health); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should have restored
	if health.Status != api.HealthPassing {
		t.Fatalf("bad: %#v", health)
	}
	if health.Output != "yup" {
		t.Fatalf("bad: %#v", health)
	}
}

func TestAgent_purgeCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// No error if the state does not exist
	if err := a.purgeCheckState("check1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Persist some state to the data dir
	check := &checks.CheckTTL{
		CheckID: "check1",
		TTL:     time.Minute,
	}
	err := a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Purge the check state
	if err := a.purgeCheckState("check1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed the file
	file := filepath.Join(a.Config.DataDir, checkStateDir, stringHash("check1"))
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should have removed file")
	}
}

func TestAgent_loadTokens(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		acl = {
			enabled = true
			tokens = {
				agent = "alfa"
				agent_master = "bravo",
				default = "charlie"
				replication = "delta"
			}
		}

	`)
	defer a.Shutdown()
	require := require.New(t)

	tokensFullPath := filepath.Join(a.config.DataDir, tokensPath)

	t.Run("original-configuration", func(t *testing.T) {
		require.Equal("alfa", a.tokens.AgentToken())
		require.Equal("bravo", a.tokens.AgentMasterToken())
		require.Equal("charlie", a.tokens.UserToken())
		require.Equal("delta", a.tokens.ReplicationToken())
	})

	t.Run("updated-configuration", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "echo",
			ACLAgentToken:       "foxtrot",
			ACLAgentMasterToken: "golf",
			ACLReplicationToken: "hotel",
		}
		// ensures no error for missing persisted tokens file
		require.NoError(a.loadTokens(cfg))
		require.Equal("echo", a.tokens.UserToken())
		require.Equal("foxtrot", a.tokens.AgentToken())
		require.Equal("golf", a.tokens.AgentMasterToken())
		require.Equal("hotel", a.tokens.ReplicationToken())
	})

	t.Run("persisted-tokens", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "echo",
			ACLAgentToken:       "foxtrot",
			ACLAgentMasterToken: "golf",
			ACLReplicationToken: "hotel",
		}

		tokens := `{
			"agent" : "india",
			"agent_master" : "juliett",
			"default": "kilo",
			"replication" : "lima"
		}`

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte(tokens), 0600))
		require.NoError(a.loadTokens(cfg))

		// no updates since token persistence is not enabled
		require.Equal("echo", a.tokens.UserToken())
		require.Equal("foxtrot", a.tokens.AgentToken())
		require.Equal("golf", a.tokens.AgentMasterToken())
		require.Equal("hotel", a.tokens.ReplicationToken())

		a.config.ACLEnableTokenPersistence = true
		require.NoError(a.loadTokens(cfg))

		require.Equal("india", a.tokens.AgentToken())
		require.Equal("juliett", a.tokens.AgentMasterToken())
		require.Equal("kilo", a.tokens.UserToken())
		require.Equal("lima", a.tokens.ReplicationToken())
	})

	t.Run("persisted-tokens-override", func(t *testing.T) {
		tokens := `{
			"agent" : "mike",
			"agent_master" : "november",
			"default": "oscar",
			"replication" : "papa"
		}`

		cfg := &config.RuntimeConfig{
			ACLToken:            "quebec",
			ACLAgentToken:       "romeo",
			ACLAgentMasterToken: "sierra",
			ACLReplicationToken: "tango",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte(tokens), 0600))
		require.NoError(a.loadTokens(cfg))

		require.Equal("mike", a.tokens.AgentToken())
		require.Equal("november", a.tokens.AgentMasterToken())
		require.Equal("oscar", a.tokens.UserToken())
		require.Equal("papa", a.tokens.ReplicationToken())
	})

	t.Run("partial-persisted", func(t *testing.T) {
		tokens := `{
			"agent" : "uniform",
			"agent_master" : "victor"
		}`

		cfg := &config.RuntimeConfig{
			ACLToken:            "whiskey",
			ACLAgentToken:       "xray",
			ACLAgentMasterToken: "yankee",
			ACLReplicationToken: "zulu",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte(tokens), 0600))
		require.NoError(a.loadTokens(cfg))

		require.Equal("uniform", a.tokens.AgentToken())
		require.Equal("victor", a.tokens.AgentMasterToken())
		require.Equal("whiskey", a.tokens.UserToken())
		require.Equal("zulu", a.tokens.ReplicationToken())
	})

	t.Run("persistence-error-not-json", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "one",
			ACLAgentToken:       "two",
			ACLAgentMasterToken: "three",
			ACLReplicationToken: "four",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0600))
		err := a.loadTokens(cfg)
		require.Error(err)

		require.Equal("one", a.tokens.UserToken())
		require.Equal("two", a.tokens.AgentToken())
		require.Equal("three", a.tokens.AgentMasterToken())
		require.Equal("four", a.tokens.ReplicationToken())
	})

	t.Run("persistence-error-wrong-top-level", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "alfa",
			ACLAgentToken:       "bravo",
			ACLAgentMasterToken: "charlie",
			ACLReplicationToken: "foxtrot",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte("[1,2,3]"), 0600))
		err := a.loadTokens(cfg)
		require.Error(err)

		require.Equal("alfa", a.tokens.UserToken())
		require.Equal("bravo", a.tokens.AgentToken())
		require.Equal("charlie", a.tokens.AgentMasterToken())
		require.Equal("foxtrot", a.tokens.ReplicationToken())
	})
}
