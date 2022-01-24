package keyring

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	consulapi "github.com/hashicorp/consul/api"
)

func TestKeyringCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKeyringCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "kZyFABeAmc64UMTrm9XuKA=="

	// Begin with a single key
	a1 := agent.NewTestAgent(t, `
		encrypt = "`+key1+`"
	`)
	defer a1.Shutdown()

	// The LAN and WAN keyrings were initialized with key1
	out := listKeys(t, a1.HTTPAddr())
	if !strings.Contains(out, "dc1 (LAN):\n  "+key1) {
		t.Fatalf("bad: %#v", out)
	}
	if !strings.Contains(out, "WAN:\n  "+key1) {
		t.Fatalf("bad: %#v", out)
	}
	if strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}

	// Install the second key onto the keyring
	installKey(t, a1.HTTPAddr(), key2)

	// Both keys should be present
	out = listKeys(t, a1.HTTPAddr())
	for _, key := range []string{key1, key2} {
		if !strings.Contains(out, key) {
			t.Fatalf("bad: %#v", out)
		}
	}

	// Rotate to key2, remove key1
	useKey(t, a1.HTTPAddr(), key2)

	// New key should be present
	out = listPrimaryKeys(t, a1.HTTPAddr())
	if strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
	if !strings.Contains(out, key2) {
		t.Fatalf("bad: %#v", out)
	}

	removeKey(t, a1.HTTPAddr(), key1)

	// Only key2 is present now
	out = listKeys(t, a1.HTTPAddr())
	if !strings.Contains(out, "dc1 (LAN):\n  "+key2) {
		t.Fatalf("bad: %#v", out)
	}
	if !strings.Contains(out, "WAN:\n  "+key2) {
		t.Fatalf("bad: %#v", out)
	}
	if strings.Contains(out, key1) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestKeyringCommand_help(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run(nil)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Test that we didn't actually try to dial the RPC server.
	if !strings.Contains(ui.ErrorWriter.String(), "Usage:") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}

func TestKeyringCommand_failedConnection(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-list", "-http-addr=127.0.0.1:0"}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d, %#v", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.ErrorWriter.String(), "dial") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestKeyringCommand_invalidLocalOnly(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-install=blah", "-local-only=true"}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func TestKeyringCommand_invalidRelayFactor(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-list", "-relay-factor=6"}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func listKeys(t *testing.T, addr string) string {
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-list", "-http-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	return ui.OutputWriter.String()
}

func listPrimaryKeys(t *testing.T, addr string) string {
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-list-primary", "-http-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	return ui.OutputWriter.String()
}

func installKey(t *testing.T, addr string, key string) {
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-install=" + key, "-http-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func useKey(t *testing.T, addr string, key string) {
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-use=" + key, "-http-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func removeKey(t *testing.T, addr string, key string) {
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-remove=" + key, "-http-addr=" + addr}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func TestKeyringCommand_poolName(t *testing.T) {
	require.Equal(t, "dc1 (LAN):\n", poolName("dc1", false, "", ""))
	require.Equal(t, "WAN:\n", poolName("dc1", true, "", ""))
}

func TestKeyringCommand_formatKeys(t *testing.T) {
	require.Equal(t, "", formatKeys(map[string]int{}, 0))
	keys := formatKeys(map[string]int{"key1": 1, "key2": 2}, 2)
	require.Contains(t, keys, "  key1 [1/2]\n")
	require.Contains(t, keys, "  key2 [2/2]\n")
}

func TestKeyringCommand_formatMessages(t *testing.T) {
	require.Equal(t, "", formatMessages(map[string]string{}))
	messages := formatMessages(map[string]string{"n1": "hello", "n2": "world"})
	require.Contains(t, messages, "  ===> n1: hello\n")
	require.Contains(t, messages, "  ===> n2: world\n")
}

func TestKeyringCommand_formatResponse(t *testing.T) {
	response := &consulapi.KeyringResponse{Datacenter: "dc1", NumNodes: 1}
	keys := map[string]int{"key1": 1}
	require.Equal(t, "\ndc1 (LAN):\n  key1 [1/1]", formatResponse(response, keys))

	response = &consulapi.KeyringResponse{WAN: true, Datacenter: "dc1", NumNodes: 1}
	keys = map[string]int{"key1": 1}
	require.Equal(t, "\nWAN:\n  key1 [1/1]", formatResponse(response, keys))
}

func TestKeyringCommand_numActions(t *testing.T) {
	require.Equal(t, 0, numberActions(false, false, "", "", ""))
	require.Equal(t, 1, numberActions(true, false, "", "", ""))
	require.Equal(t, 1, numberActions(false, true, "", "", ""))
	require.Equal(t, 1, numberActions(false, false, "1", "", ""))
	require.Equal(t, 2, numberActions(true, false, "1", "", ""))
	require.Equal(t, 2, numberActions(false, false, "1", "1", ""))
}
