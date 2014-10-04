package agent

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
		t.Fatalf("bad: %#v", c.SerfLANConfig.KeyringFile)
	}
	if c.SerfWANConfig.MemberlistConfig.Keyring != nil {
		t.Fatalf("keyring should not be loaded")
	}
}

func TestAgent_InitKeyring(t *testing.T) {
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="

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

	content1, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(string(content1), key1) {
		t.Fatalf("bad: %s", content1)
	}
	if strings.Contains(string(content1), key2) {
		t.Fatalf("bad: %s", content1)
	}

	// Now initialize again with the same key
	if err := initKeyring(file, key1); err != nil {
		t.Fatalf("err: %s", err)
	}

	content2, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(content1, content2) {
		t.Fatalf("bad: %s", content2)
	}

	// Initialize an existing keyring with a new key
	if err := initKeyring(file, key2); err != nil {
		t.Fatalf("err: %s", err)
	}

	content3, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(string(content3), key1) {
		t.Fatalf("bad: %s", content3)
	}
	if !strings.Contains(string(content3), key2) {
		t.Fatalf("bad: %s", content3)
	}

	// Unmarshal and make sure that key1 is still primary
	var keys []string
	if err := json.Unmarshal(content3, &keys); err != nil {
		t.Fatalf("err: %s", err)
	}
	if keys[0] != key1 {
		t.Fatalf("bad: %#v", keys)
	}
}
