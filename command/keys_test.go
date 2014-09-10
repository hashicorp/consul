package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
)

func TestKeysCommand_implements(t *testing.T) {
	var _ cli.Command = &KeysCommand{}
}

func TestKeysCommandRun(t *testing.T) {
	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "kZyFABeAmc64UMTrm9XuKA=="
	key3 := "2k5VRlBIIKUPc1v77rsswg=="

	// Begin with a single key
	conf := agent.Config{EncryptKey: key1}
	a1 := testAgentWithConfig(&conf, t)
	defer a1.Shutdown()

	// The keyring was initialized with only the provided key
	out := listKeys(t, a1.addr, false)
	if !strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
	if strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}

	// The key was installed on the WAN gossip layer also
	out = listKeys(t, a1.addr, true)
	if !strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
	if strings.Contains(out, key3) {
		t.Fatalf("bad: %#v", out)
	}

	// Install the second key onto the keyring
	installKey(t, a1.addr, false, key2)

	// Both keys should be present
	out = listKeys(t, a1.addr, false)
	for _, key := range []string{key1, key2} {
		if !strings.Contains(out, key) {
			t.Fatalf("bad: %#v", out)
		}
	}

	// Second key should not be installed on WAN
	out = listKeys(t, a1.addr, true)
	if strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}

	// Change out the primary key
	useKey(t, a1.addr, false, key2)

	// Remove the original key
	removeKey(t, a1.addr, false, key1)

	// Make sure only the new key is present
	out = listKeys(t, a1.addr, false)
	if strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
	if !strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}

	// Original key still remains on WAN keyring
	out = listKeys(t, a1.addr, true)
	if !strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}

	// Install second key on WAN keyring
	installKey(t, a1.addr, true, key3)

	// Two keys now present on WAN keyring
	out = listKeys(t, a1.addr, true)
	for _, key := range []string{key1, key3} {
		if !strings.Contains(out, key) {
			t.Fatalf("bad: %#v", out)
		}
	}

	// Change WAN primary key
	useKey(t, a1.addr, true, key3)

	// Remove original key from WAN keyring
	removeKey(t, a1.addr, true, key1)

	// Only new key should exist on WAN keyring
	out = listKeys(t, a1.addr, true)
	if !strings.Contains(out, key3) {
		t.Fatalf("bad: %#v", out)
	}
	if strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestKeysCommandRun_help(t *testing.T) {
	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}
	code := c.Run(nil)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.ErrorWriter.String(), "Usage:") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}

func listKeys(t *testing.T, addr string, wan bool) string {
	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	args := []string{"-list", "-rpc-addr=" + addr}
	if wan {
		args = append(args, "-wan")
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	return ui.OutputWriter.String()
}

func installKey(t *testing.T, addr string, wan bool, key string) {
	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	args := []string{"-install=" + key, "-rpc-addr=" + addr}
	if wan {
		args = append(args, "-wan")
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func useKey(t *testing.T, addr string, wan bool, key string) {
	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	args := []string{"-use=" + key, "-rpc-addr=" + addr}
	if wan {
		args = append(args, "-wan")
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func removeKey(t *testing.T, addr string, wan bool, key string) {
	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	args := []string{"-remove=" + key, "-rpc-addr=" + addr}
	if wan {
		args = append(args, "-wan")
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}
