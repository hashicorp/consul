package command

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
)

func TestKeyringCommand_implements(t *testing.T) {
	var _ cli.Command = &KeyringCommand{}
}

func TestKeyringCommandRun(t *testing.T) {
	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "kZyFABeAmc64UMTrm9XuKA=="

	// Begin with a single key
	conf := agent.Config{EncryptKey: key1}
	a1 := testAgentWithConfig(&conf, t)
	defer a1.Shutdown()

	// The keyring was initialized with only the provided key
	out := listKeys(t, a1.addr)
	if !strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
	if strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}

	// Install the second key onto the keyring
	installKey(t, a1.addr, key2)

	// Both keys should be present
	out = listKeys(t, a1.addr)
	for _, key := range []string{key1, key2} {
		if !strings.Contains(out, key) {
			t.Fatalf("bad: %#v", out)
		}
	}

	// Change out the primary key
	useKey(t, a1.addr, key2)

	// Remove the original key
	removeKey(t, a1.addr, key1)

	// Make sure only the new key is present
	out = listKeys(t, a1.addr)
	if strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
	if !strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestKeyringCommandRun_help(t *testing.T) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}
	code := c.Run(nil)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Test that we didn't actually try to dial the RPC server.
	if !strings.Contains(ui.ErrorWriter.String(), "Usage:") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}

func TestKeyringCommandRun_failedConnection(t *testing.T) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}
	args := []string{"-list", "-rpc-addr=127.0.0.1:0"}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d, %#v", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.ErrorWriter.String(), "dial") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestKeyCommandRun_initKeyringFail(t *testing.T) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}

	// Should error if no data-dir given
	args := []string{"-init=HS5lJ+XuTlYKWaeGYyG+/A=="}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Errors on invalid key
	args = []string{"-init=xyz", "-data-dir=/tmp"}
	code = c.Run(args)
	if code != 1 {
		t.Fatalf("should have errored")
	}
}

func TestKeyCommandRun_initKeyring(t *testing.T) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}

	tempDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tempDir)

	args := []string{
		"-init=HS5lJ+XuTlYKWaeGYyG+/A==",
		"-data-dir=" + tempDir,
	}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	fileLAN := filepath.Join(tempDir, agent.SerfLANKeyring)
	fileWAN := filepath.Join(tempDir, agent.SerfWANKeyring)
	if _, err := os.Stat(fileLAN); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(fileWAN); err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := "[\n  \"HS5lJ+XuTlYKWaeGYyG+/A==\"\n]"

	contentLAN, err := ioutil.ReadFile(fileLAN)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(contentLAN) != expected {
		t.Fatalf("bad: %#v", string(contentLAN))
	}

	contentWAN, err := ioutil.ReadFile(fileWAN)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(contentWAN) != expected {
		t.Fatalf("bad: %#v", string(contentWAN))
	}
}

func listKeys(t *testing.T, addr string) string {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}

	args := []string{"-list", "-rpc-addr=" + addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	return ui.OutputWriter.String()
}

func installKey(t *testing.T, addr string, key string) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}

	args := []string{"-install=" + key, "-rpc-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func useKey(t *testing.T, addr string, key string) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}

	args := []string{"-use=" + key, "-rpc-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func removeKey(t *testing.T, addr string, key string) {
	ui := new(cli.MockUi)
	c := &KeyringCommand{Ui: ui}

	args := []string{"-remove=" + key, "-rpc-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}
