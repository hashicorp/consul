package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/testutil"
)

func TestAgent_LoadKeyrings(t *testing.T) {
	t.Parallel()
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	a1 := NewTestAgent(t.Name(), nil)
	defer a1.Shutdown()

	c1 := a1.Config.ConsulConfig
	if c1.SerfLANConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c1.SerfLANConfig.KeyringFile)
	}
	if c1.SerfLANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}
	if c1.SerfWANConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c1.SerfLANConfig.KeyringFile)
	}
	if c1.SerfWANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}

	// Server should auto-load LAN and WAN keyring files
	a2 := &TestAgent{Name: t.Name(), Key: key}
	a2.Start()
	defer a2.Shutdown()

	c2 := a2.Config.ConsulConfig
	if c2.SerfLANConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c2.SerfLANConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}
	if c2.SerfWANConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c2.SerfWANConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}

	// Client should auto-load only the LAN keyring file
	cfg3 := TestConfig()
	cfg3.Server = false
	a3 := &TestAgent{Name: t.Name(), Config: cfg3, Key: key}
	a3.Start()
	defer a3.Shutdown()

	c3 := a3.Config.ConsulConfig
	if c3.SerfLANConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c3.SerfLANConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}
	if c3.SerfWANConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c3.SerfWANConfig.KeyringFile)
	}
	if c3.SerfWANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}
}

func TestAgent_InitKeyring(t *testing.T) {
	t.Parallel()
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	expected := fmt.Sprintf(`["%s"]`, key1)

	dir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "keyring")

	// First initialize the keyring
	if err := initKeyring(file, key1); err != nil {
		t.Fatalf("err: %s", err)
	}

	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != expected {
		t.Fatalf("bad: %s", content)
	}

	// Try initializing again with a different key
	if err := initKeyring(file, key2); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Content should still be the same
	content, err = ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != expected {
		t.Fatalf("bad: %s", content)
	}
}

func TestAgentKeyring_ACL(t *testing.T) {
	t.Parallel()
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="

	cfg := TestACLConfig()
	cfg.ACLDatacenter = "dc1"
	cfg.ACLMasterToken = "root"
	cfg.ACLDefaultPolicy = "deny"
	a := &TestAgent{Name: t.Name(), Config: cfg, Key: key1}
	a.Start()
	defer a.Shutdown()

	// List keys without access fails
	_, err := a.ListKeys("", 0)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got: %#v", err)
	}

	// List keys with access works
	_, err = a.ListKeys("root", 0)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Install without access fails
	_, err = a.InstallKey(key2, "", 0)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got: %#v", err)
	}

	// Install with access works
	_, err = a.InstallKey(key2, "root", 0)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Use without access fails
	_, err = a.UseKey(key2, "", 0)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got: %#v", err)
	}

	// Use with access works
	_, err = a.UseKey(key2, "root", 0)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Remove without access fails
	_, err = a.RemoveKey(key1, "", 0)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got: %#v", err)
	}

	// Remove with access works
	_, err = a.RemoveKey(key1, "root", 0)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}
