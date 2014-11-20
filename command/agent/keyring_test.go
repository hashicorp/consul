package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAgent_LoadKeyrings(t *testing.T) {
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	conf1 := nextConfig()
	dir1, agent1 := makeAgent(t, conf1)
	defer os.RemoveAll(dir1)
	defer agent1.Shutdown()

	c := agent1.config.ConsulConfig
	if c.SerfLANConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c.SerfLANConfig.KeyringFile)
	}
	if c.SerfLANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}
	if c.SerfWANConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c.SerfLANConfig.KeyringFile)
	}
	if c.SerfWANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}

	// Server should auto-load LAN and WAN keyring files
	conf2 := nextConfig()
	dir2, agent2 := makeAgentKeyring(t, conf2, key)
	defer os.RemoveAll(dir2)
	defer agent2.Shutdown()

	c = agent2.config.ConsulConfig
	if c.SerfLANConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c.SerfLANConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}
	if c.SerfWANConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c.SerfWANConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}

	// Client should auto-load only the LAN keyring file
	conf3 := nextConfig()
	conf3.Server = false
	dir3, agent3 := makeAgentKeyring(t, conf3, key)
	defer os.RemoveAll(dir3)
	defer agent3.Shutdown()

	c = agent3.config.ConsulConfig
	if c.SerfLANConfig.KeyringFile == "" {
		t.Fatalf("should have keyring file")
	}
	if c.SerfLANConfig.MemberlistConfig.Keyring == nil {
		t.Fatalf("keyring should be loaded")
	}
	if c.SerfWANConfig.KeyringFile != "" {
		t.Fatalf("bad: %#v", c.SerfWANConfig.KeyringFile)
	}
	if c.SerfWANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}
}

func TestAgent_InitKeyring(t *testing.T) {
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	expected := fmt.Sprintf(`["%s"]`, key1)

	dir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
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
