// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package nftables

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeFakeBin creates an executable POSIX shell script named name inside
// dir, for use as a stand-in for a real system binary (nft, nsenter, ...)
// invoked by the executor via exec.Command. Scripts communicate with the
// test through environment variables (set via t.Setenv) rather than
// embedded paths, so the same script bodies stay simple regardless of
// where the temp directory lives.
func writeFakeBin(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

// usePATH prepends dir to PATH for the duration of the test, so
// exec.LookPath finds any fake binaries installed there before falling back
// to the host's real PATH (needed for the shell scripts themselves to use
// ordinary utilities like cat/echo/touch).
func usePATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// writeFakeNsenter installs a fake "nsenter" that records its own argv (via
// the NSENTER_LOG env var) and then execs whatever command follows "--", so
// a wrapped fake command (e.g. nft) on PATH still actually runs. This lets
// tests verify namespace forwarding end-to-end rather than just asserting
// nsenter was called.
func writeFakeNsenter(t *testing.T, dir string) {
	t.Helper()
	writeFakeBin(t, dir, "nsenter", `
echo "$@" >> "$NSENTER_LOG"
while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do
  shift
done
shift
exec "$@"
`)
}

// TestNftablesExecutor_AddRuleAndRules verifies that AddRule joins its
// arguments (ignoring the leading binary name) into a script line, and that
// Rules() re-prefixes each stored line with "nft " for display/inspection.
func TestNftablesExecutor_AddRuleAndRules(t *testing.T) {
	n := &nftablesExecutor{}
	n.AddRule("nft", "add", "table", "inet", "consul_tproxy")
	n.AddRule("nft", "add", "chain", "inet", "consul_tproxy", "PROXY_INBOUND")

	require.Equal(t, []string{
		"nft add table inet consul_tproxy",
		"nft add chain inet consul_tproxy PROXY_INBOUND",
	}, n.Rules())

	n.ClearAllRules()
	require.Empty(t, n.Rules())
}

// TestApplyRules_WritesScriptToNftStdin verifies that ApplyRules joins the
// collected AddRule lines and pipes them to `nft -f -` via stdin, exactly as
// documented.
func TestApplyRules_WritesScriptToNftStdin(t *testing.T) {
	dir := t.TempDir()
	stdinCapture := filepath.Join(dir, "nft.stdin")
	writeFakeBin(t, dir, "nft", `cat > "`+stdinCapture+`"`)
	usePATH(t, dir)

	n := &nftablesExecutor{}
	n.AddRule("nft", "add", "table", "inet", "consul_tproxy")
	n.AddRule("nft", "add", "chain", "inet", "consul_tproxy", "PROXY_INBOUND")

	require.NoError(t, n.ApplyRules("nft"))

	got, err := os.ReadFile(stdinCapture)
	require.NoError(t, err)
	require.Equal(t, "add table inet consul_tproxy\nadd chain inet consul_tproxy PROXY_INBOUND", string(got))
}

// TestApplyRules_NftFailure_ReturnsError verifies that when the native
// `nft -f -` apply fails, ApplyRules returns an error including nft's
// captured output.
func TestApplyRules_NftFailure_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "nft", `echo "nft: syntax error" >&2; exit 1`)
	usePATH(t, dir)

	n := &nftablesExecutor{}
	n.AddRule("nft", "add", "table", "inet", "consul_tproxy")

	err := n.ApplyRules("nft")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to apply nftables rules")
	require.Contains(t, err.Error(), "nft: syntax error")
}

// TestApplyRules_NamespaceForwarding verifies that a configured NetNS causes
// ApplyRules to invoke "nsenter --net=<ns> -- nft -f -" rather than
// executing nft directly, while the script still reaches nft's stdin
// unchanged.
func TestApplyRules_NamespaceForwarding(t *testing.T) {
	dir := t.TempDir()
	nsenterLog := filepath.Join(dir, "nsenter.log")
	t.Setenv("NSENTER_LOG", nsenterLog)
	writeFakeNsenter(t, dir)

	stdinCapture := filepath.Join(dir, "nft.stdin")
	writeFakeBin(t, dir, "nft", `cat > "`+stdinCapture+`"`)
	usePATH(t, dir)

	n := &nftablesExecutor{cfg: Config{NetNS: "/var/run/netns/consul-test"}}
	n.AddRule("nft", "add", "table", "inet", "consul_tproxy")

	require.NoError(t, n.ApplyRules("nft"))

	nsLogged, err := os.ReadFile(nsenterLog)
	require.NoError(t, err)
	require.Equal(t, "--net=/var/run/netns/consul-test -- nft -f -\n", string(nsLogged))

	stdinLogged, err := os.ReadFile(stdinCapture)
	require.NoError(t, err)
	require.Equal(t, "add table inet consul_tproxy", string(stdinLogged))
}

// TestApplyRules_NftBinaryNotFound verifies ApplyRules fails fast with a
// clear error when the nft binary isn't installed at all.
func TestApplyRules_NftBinaryNotFound(t *testing.T) {
	dir := t.TempDir() // deliberately empty
	// Fully isolated PATH: no fallback to the host's real PATH, so a host
	// that happens to have nft installed can't mask this case.
	t.Setenv("PATH", dir)

	n := &nftablesExecutor{}
	err := n.ApplyRules("nft")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nft binary not found")
}

// TestApplyRules_NsenterBinaryNotFound verifies ApplyRules fails fast when a
// network namespace is configured but nsenter isn't installed, even though
// nft itself is present.
func TestApplyRules_NsenterBinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "nft", `exit 0`)
	// Use an isolated PATH (no fallback to the host's real PATH) so a
	// genuinely installed nsenter on the test host can't mask this case.
	t.Setenv("PATH", dir)

	n := &nftablesExecutor{cfg: Config{NetNS: "/var/run/netns/missing"}}
	err := n.ApplyRules("nft")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nsenter binary not found")
}
