package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/memberlist"
	"github.com/stretchr/testify/require"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	t.Run("no keys", func(t *testing.T) {
		a1 := NewTestAgent(t, "")
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
		dataDir := testutil.TempDir(t, "keyfile")
		writeKeyRings(t, key, dataDir)

		a2 := StartTestAgent(t, TestAgent{DataDir: dataDir})
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
		dataDir := testutil.TempDir(t, "keyfile")
		writeKeyRings(t, key, dataDir)

		a3 := StartTestAgent(t, TestAgent{
			HCL: `
			server = false
			bootstrap = false
			`,
			DataDir: dataDir,
		})
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

func writeKeyRings(t *testing.T, key string, dataDir string) {
	t.Helper()
	writeKey := func(key, filename string) {
		path := filepath.Join(dataDir, filename)
		require.NoError(t, initKeyring(path, key), "Error creating keyring %s", path)
	}
	writeKey(key, SerfLANKeyring)
	writeKey(key, SerfWANKeyring)
}

func TestAgent_InmemKeyrings(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	// Should be no configured keyring file by default
	t.Run("no keys", func(t *testing.T) {
		a1 := NewTestAgent(t, "")
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
		a2 := NewTestAgent(t, `
			encrypt = "`+key+`"
			disable_keyring_file = true
		`)
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
		a3 := NewTestAgent(t, `
			encrypt = "`+key+`"
			server = false
			bootstrap = false
			disable_keyring_file = true
		`)
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

		badKey := "unUzC2X3JgMKVJlZna5KVg=="
		if err := initKeyring(filepath.Join(dir, SerfLANKeyring), badKey); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := initKeyring(filepath.Join(dir, SerfWANKeyring), badKey); err != nil {
			t.Fatalf("err: %v", err)
		}

		a4 := NewTestAgent(t, `
			encrypt = "`+key+`"
			disable_keyring_file = true
			data_dir = "`+dir+`"
		`)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="

	dataDir := testutil.TempDir(t, "keyfile")
	writeKeyRings(t, key1, dataDir)

	a := StartTestAgent(t, TestAgent{HCL: `
		primary_datacenter = "dc1"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
			}
		}
	`, DataDir: dataDir})
	defer a.Shutdown()

	// List keys without access fails
	_, err := a.ListKeys("", false, 0)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got: %#v", err)
	}

	// List keys with access works
	_, err = a.ListKeys("root", false, 0)
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

func TestValidateLocalOnly(t *testing.T) {
	require.NoError(t, ValidateLocalOnly(false, false))
	require.NoError(t, ValidateLocalOnly(true, true))

	require.Error(t, ValidateLocalOnly(true, false))
}

func TestAgent_KeyringIsMissingKey(t *testing.T) {
	key1 := "tbLJg26ZJyJ9pK3qhc9jig=="
	key2 := "4leC33rgtXKIVUr9Nr0snQ=="
	decoded1, err := decodeStringKey(key1)
	require.NoError(t, err)
	keyring, err := memberlist.NewKeyring([][]byte{}, decoded1)
	require.NoError(t, err)

	require.True(t, keyringIsMissingKey(keyring, key2))
	require.False(t, keyringIsMissingKey(keyring, key1))
}
