package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/memberlist"
)

func checkForKey(key string, keyring *memberlist.Keyring) error {
	rk, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}

	pk := keyring.GetPrimaryKey()
	if !bytes.Equal(rk, pk) {
		return fmt.Errorf("got %q want %q", pk, rk)
	}
	return nil
}

func TestAgent_LoadKeyrings(t *testing.T) {
	t.Parallel()
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	t.Run("no keys", func(t *testing.T) {
		a1 := NewTestAgent(t.Name(), "")
		defer a1.Shutdown()

		c1 := a1.consulConfig()
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
	})

	// Server should auto-load LAN and WAN keyring files
	t.Run("server with keys", func(t *testing.T) {
		a2 := &TestAgent{Name: t.Name(), Key: key}
		a2.Start()
		defer a2.Shutdown()

		c2 := a2.consulConfig()
		if c2.SerfLANConfig.KeyringFile == "" {
			t.Fatalf("should have keyring file")
		}
		if c2.SerfLANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c2.SerfLANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
		if c2.SerfWANConfig.KeyringFile == "" {
			t.Fatalf("should have keyring file")
		}
		if c2.SerfWANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c2.SerfWANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Client should auto-load only the LAN keyring file
	t.Run("client with keys", func(t *testing.T) {
		a3 := &TestAgent{Name: t.Name(), HCL: `
			server = false
			bootstrap = false
		`, Key: key}
		a3.Start()
		defer a3.Shutdown()

		c3 := a3.consulConfig()
		if c3.SerfLANConfig.KeyringFile == "" {
			t.Fatalf("should have keyring file")
		}
		if c3.SerfLANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c3.SerfLANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
		if c3.SerfWANConfig.KeyringFile != "" {
			t.Fatalf("bad: %#v", c3.SerfWANConfig.KeyringFile)
		}
		if c3.SerfWANConfig.MemberlistConfig.Keyring != nil {
			t.Fatalf("keyring should not be loaded")
		}
	})
}

func TestAgent_InmemKeyrings(t *testing.T) {
	t.Parallel()
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	t.Run("no keys", func(t *testing.T) {
		a1 := NewTestAgent(t.Name(), "")
		defer a1.Shutdown()

		c1 := a1.consulConfig()
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
	})

	// Server should auto-load LAN and WAN keyring
	t.Run("server with keys", func(t *testing.T) {
		a2 := &TestAgent{Name: t.Name(), HCL: `
			encrypt = "` + key + `"
			disable_keyring_file = true
		`}
		a2.Start()
		defer a2.Shutdown()

		c2 := a2.consulConfig()
		if c2.SerfLANConfig.KeyringFile != "" {
			t.Fatalf("should not have keyring file")
		}
		if c2.SerfLANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c2.SerfLANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
		if c2.SerfWANConfig.KeyringFile != "" {
			t.Fatalf("should not have keyring file")
		}
		if c2.SerfWANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c2.SerfWANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Client should auto-load only the LAN keyring
	t.Run("client with keys", func(t *testing.T) {
		a3 := &TestAgent{Name: t.Name(), HCL: `
			encrypt = "` + key + `"
			server = false
			bootstrap = false
			disable_keyring_file = true
		`}
		a3.Start()
		defer a3.Shutdown()

		c3 := a3.consulConfig()
		if c3.SerfLANConfig.KeyringFile != "" {
			t.Fatalf("should not have keyring file")
		}
		if c3.SerfLANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c3.SerfLANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
		if c3.SerfWANConfig.KeyringFile != "" {
			t.Fatalf("bad: %#v", c3.SerfWANConfig.KeyringFile)
		}
		if c3.SerfWANConfig.MemberlistConfig.Keyring != nil {
			t.Fatalf("keyring should not be loaded")
		}
	})

	// Any keyring files should be ignored
	t.Run("ignore files", func(t *testing.T) {
		dir := testutil.TempDir(t, "consul")
		defer os.RemoveAll(dir)

		badKey := "unUzC2X3JgMKVJlZna5KVg=="
		if err := initKeyring(filepath.Join(dir, SerfLANKeyring), badKey); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := initKeyring(filepath.Join(dir, SerfWANKeyring), badKey); err != nil {
			t.Fatalf("err: %v", err)
		}

		a4 := &TestAgent{Name: t.Name(), HCL: `
			encrypt = "` + key + `"
			disable_keyring_file = true
			data_dir = "` + dir + `"
		`}
		a4.Start()
		defer a4.Shutdown()

		c4 := a4.consulConfig()
		if c4.SerfLANConfig.KeyringFile != "" {
			t.Fatalf("should not have keyring file")
		}
		if c4.SerfLANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c4.SerfLANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
		if c4.SerfWANConfig.KeyringFile != "" {
			t.Fatalf("should not have keyring file")
		}
		if c4.SerfWANConfig.MemberlistConfig.Keyring == nil {
			t.Fatalf("keyring should be loaded")
		}
		if err := checkForKey(key, c4.SerfWANConfig.MemberlistConfig.Keyring); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
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

	a := &TestAgent{Name: t.Name(), HCL: TestACLConfig() + `
		acl_datacenter = "dc1"
		acl_master_token = "root"
		acl_default_policy = "deny"
	`, Key: key1}
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
