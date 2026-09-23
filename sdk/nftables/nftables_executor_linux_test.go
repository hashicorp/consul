// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package nftables

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJumpTarget(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		expected string
	}{
		{"jump via -j", "-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT", "CONSUL_PROXY_OUTPUT"},
		{"jump via --jump", "-A OUTPUT -p tcp --jump CONSUL_PROXY_OUTPUT", "CONSUL_PROXY_OUTPUT"},
		{"jump target is not the first flag", "-A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN", "RETURN"},
		{"trailing -j with no argument", "-A OUTPUT -p tcp -j", ""},
		{"no -j at all", "-A OUTPUT -p tcp -m owner --uid-owner 123", ""},
		{"empty line", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, jumpTarget(strings.Fields(c.line)))
		})
	}
}

func TestIsOwnedChain(t *testing.T) {
	consulChains := []string{
		"CONSUL_PROXY_INBOUND",
		"CONSUL_PROXY_IN_REDIRECT",
		"CONSUL_PROXY_OUTPUT",
		"CONSUL_PROXY_REDIRECT",
		"CONSUL_DNS_REDIRECT",
	}

	cases := []struct {
		name     string
		target   string
		expected bool
	}{
		{"exact match", "CONSUL_PROXY_OUTPUT", true},
		{"exact match, another chain", "CONSUL_DNS_REDIRECT", true},
		// Regression test: a chain whose name merely has a Consul chain name
		// as a prefix must NOT be treated as owned. Substring matching would
		// incorrectly delete jumps to administrator-managed chains like this.
		{"chain name with owned chain as prefix is not owned", "CONSUL_PROXY_OUTPUT_CUSTOM", false},
		{"unrelated chain", "OUTPUT", false},
		{"empty target", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, isOwnedChain(c.target, consulChains))
		})
	}
}

func TestDeclaredChains(t *testing.T) {
	consulChains := []string{
		"CONSUL_PROXY_INBOUND",
		"CONSUL_PROXY_IN_REDIRECT",
		"CONSUL_PROXY_OUTPUT",
		"CONSUL_PROXY_REDIRECT",
		"CONSUL_DNS_REDIRECT",
	}

	t.Run("all chains declared", func(t *testing.T) {
		dump := `*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:CONSUL_PROXY_INBOUND - [0:0]
:CONSUL_PROXY_IN_REDIRECT - [0:0]
:CONSUL_PROXY_OUTPUT - [0:0]
:CONSUL_PROXY_REDIRECT - [0:0]
:CONSUL_DNS_REDIRECT - [0:0]
-A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND
-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
COMMIT
`
		require.ElementsMatch(t, consulChains, declaredChains(dump, consulChains))
	})

	t.Run("no chains declared", func(t *testing.T) {
		dump := `*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
COMMIT
`
		require.Empty(t, declaredChains(dump, consulChains))
	})

	// Regression test: a chain that merely has a Consul chain name as a
	// prefix must not be treated as an existing Consul chain. Before this
	// fix, a substring check over the raw dump would incorrectly detect
	// "legacy rules" here and then attempt (and fail) to flush/delete all
	// five canonical chain names, none of which actually exist.
	t.Run("similarly named chain is not mistaken for owned chain", func(t *testing.T) {
		dump := `*nat
:PREROUTING ACCEPT [0:0]
:CONSUL_PROXY_OUTPUT_CUSTOM - [0:0]
-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT_CUSTOM
COMMIT
`
		require.Empty(t, declaredChains(dump, consulChains))
	})

	// Regression test: only chains still actually present are returned, so a
	// retry after a partially-successful cleanup only targets what remains.
	t.Run("partial cleanup — only remaining chains returned", func(t *testing.T) {
		dump := `*nat
:PREROUTING ACCEPT [0:0]
:CONSUL_PROXY_OUTPUT - [0:0]
:CONSUL_DNS_REDIRECT - [0:0]
COMMIT
`
		require.ElementsMatch(t, []string{"CONSUL_PROXY_OUTPUT", "CONSUL_DNS_REDIRECT"}, declaredChains(dump, consulChains))
	})

	t.Run("empty dump", func(t *testing.T) {
		require.Empty(t, declaredChains("", consulChains))
	})
}

// writeFakeBin creates an executable POSIX shell script named name inside
// dir, for use as a stand-in for a real system binary (nft, nsenter,
// iptables, iptables-save, ...) invoked by the executor via exec.Command.
// Scripts communicate with the test through environment variables (set via
// t.Setenv) rather than embedded paths, so the same script bodies stay
// simple regardless of where the temp directory lives.
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
// a wrapped fake command (nft, iptables, ...) on PATH still actually runs.
// This lets tests verify namespace forwarding end-to-end rather than just
// asserting nsenter was called.
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

// TestRun_Success verifies run() returns nil when the underlying command
// exits zero.
func TestRun_Success(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "ok-bin", `exit 0`)
	usePATH(t, dir)

	require.NoError(t, run("", "ok-bin"))
}

// TestRun_FailureIncludesOutput verifies run() surfaces the failing
// command's captured stdout/stderr in the returned error, not just the exit
// status.
func TestRun_FailureIncludesOutput(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "fail-bin", `echo "boom detail"; exit 3`)
	usePATH(t, dir)

	err := run("", "fail-bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom detail")
}

// TestRun_NamespaceForwarding verifies that a non-empty netNS causes run()
// to invoke "nsenter --net=<netNS> -- <bin> <args...>" rather than the bin
// directly.
func TestRun_NamespaceForwarding(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nsenter.log")
	t.Setenv("NSENTER_LOG", logPath)
	writeFakeNsenter(t, dir)
	writeFakeBin(t, dir, "inner-bin", `exit 0`)
	usePATH(t, dir)

	require.NoError(t, run("/var/run/netns/foo", "inner-bin", "arg1", "arg2"))

	logged, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Equal(t, "--net=/var/run/netns/foo -- inner-bin arg1 arg2\n", string(logged))
}

// TestRunOutput_Success verifies runOutput() returns the command's captured
// output.
func TestRunOutput_Success(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "outputter", `echo "hello from fake"`)
	usePATH(t, dir)

	out, err := runOutput("", "outputter")
	require.NoError(t, err)
	require.Equal(t, "hello from fake\n", string(out))
}

// TestRunOutput_NamespaceForwarding verifies runOutput() also forwards
// through nsenter when a network namespace is configured, and still
// returns the wrapped command's output.
func TestRunOutput_NamespaceForwarding(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nsenter.log")
	t.Setenv("NSENTER_LOG", logPath)
	writeFakeNsenter(t, dir)
	writeFakeBin(t, dir, "dumper", `echo "dump output"`)
	usePATH(t, dir)

	out, err := runOutput("myns", "dumper", "-t", "nat")
	require.NoError(t, err)
	require.Equal(t, "dump output\n", string(out))

	logged, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Equal(t, "--net=myns -- dumper -t nat\n", string(logged))
}

// TestFlushLegacyIPTablesRules_ToolsNotOnPath verifies the cleanup is a
// silent no-op (not an error) when iptables-save/iptables aren't installed,
// e.g. on a host that never had the legacy iptables integration.
func TestFlushLegacyIPTablesRules_ToolsNotOnPath(t *testing.T) {
	dir := t.TempDir() // deliberately left empty
	// Fully isolated PATH: no fallback to the host's real PATH, so a host
	// that happens to have iptables/nft installed can't mask this case.
	t.Setenv("PATH", dir)

	require.NoError(t, flushLegacyIPTablesRules(""))
}

// TestFlushLegacyIPTablesRules_SaveToolWithoutMutateTool verifies that a
// dump/mutate tool pair is skipped entirely when only the save half is
// installed (e.g. iptables-save present without iptables), since cleanup
// could not be applied even if legacy chains were found.
func TestFlushLegacyIPTablesRules_SaveToolWithoutMutateTool(t *testing.T) {
	dir := t.TempDir()
	saveCalled := filepath.Join(dir, "save-called")
	// If this ran, it would mean the missing "iptables" mutate tool didn't
	// short-circuit the pair before the dump was even attempted.
	writeFakeBin(t, dir, "iptables-save", `touch "`+saveCalled+`"; echo "*nat"; echo "COMMIT"`)
	t.Setenv("PATH", dir)

	require.NoError(t, flushLegacyIPTablesRules(""))

	_, statErr := os.Stat(saveCalled)
	require.True(t, os.IsNotExist(statErr), "iptables-save must not run when iptables itself is missing")
}

// TestFlushLegacyIPTablesRules_SaveCommandFails verifies that a failing
// iptables-save invocation is treated like "no legacy chains" (skip, no
// error) rather than aborting cleanup entirely — a transient dump failure
// on one family (e.g. ip6tables) must not block iptables cleanup.
func TestFlushLegacyIPTablesRules_SaveCommandFails(t *testing.T) {
	dir := t.TempDir()
	iptablesLog := filepath.Join(dir, "iptables.log")
	writeFakeBin(t, dir, "iptables-save", `echo "dump failed" >&2; exit 1`)
	writeFakeBin(t, dir, "iptables", `echo "$@" >> "`+iptablesLog+`"; exit 0`)
	t.Setenv("PATH", dir)

	require.NoError(t, flushLegacyIPTablesRules(""))

	_, statErr := os.Stat(iptablesLog)
	require.True(t, os.IsNotExist(statErr), "iptables must not be invoked when the dump command fails")
}

// TestFlushLegacyIPTablesRules_JumpRuleDeletionFailurePropagates verifies
// that a failing "-D" (delete jump rule) call is aggregated into the
// returned error just like flush/delete-chain failures, rather than being
// silently ignored.
func TestFlushLegacyIPTablesRules_JumpRuleDeletionFailurePropagates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IPTABLES_FAIL_SUBSTR", "-D OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT")

	writeFakeBin(t, dir, "iptables-save", `cat <<'IPSAVE_EOF'
*nat
:CONSUL_PROXY_OUTPUT - [0:0]
-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `
case "$*" in
  *"$IPTABLES_FAIL_SUBSTR"*)
    echo "simulated jump-rule deletion failure" >&2
    exit 1
    ;;
esac
exit 0
`)
	usePATH(t, dir)

	err := flushLegacyIPTablesRules("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated jump-rule deletion failure")
	require.Contains(t, err.Error(), "-D (jump to CONSUL_PROXY_OUTPUT)")
}

// TestFlushLegacyIPTablesRules_NoLegacyChains verifies that when no
// Consul-owned chains are declared, iptables-save is consulted but iptables
// is never invoked to delete/flush anything.
func TestFlushLegacyIPTablesRules_NoLegacyChains(t *testing.T) {
	dir := t.TempDir()
	saveLog := filepath.Join(dir, "iptables-save.log")
	iptablesLog := filepath.Join(dir, "iptables.log")
	t.Setenv("IPTABLES_SAVE_LOG", saveLog)

	writeFakeBin(t, dir, "iptables-save", `
echo "$@" >> "$IPTABLES_SAVE_LOG"
cat <<'IPSAVE_EOF'
*nat
:PREROUTING ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `echo "$@" >> "`+iptablesLog+`"; exit 0`)
	usePATH(t, dir)

	require.NoError(t, flushLegacyIPTablesRules(""))

	saveCalls, err := os.ReadFile(saveLog)
	require.NoError(t, err)
	require.Equal(t, "-t nat\n", string(saveCalls))

	_, statErr := os.Stat(iptablesLog)
	require.True(t, os.IsNotExist(statErr), "iptables must not be invoked when no legacy chains are declared")
}

// TestFlushLegacyIPTablesRules_RemovesOnlyDiscoveredChains verifies that
// cleanup deletes the jump rule and flushes/deletes each declared
// Consul-owned chain, while leaving a similarly-named but un-owned chain
// (a prefix match, not an exact match) completely untouched.
func TestFlushLegacyIPTablesRules_RemovesOnlyDiscoveredChains(t *testing.T) {
	dir := t.TempDir()
	iptablesLog := filepath.Join(dir, "iptables.log")

	writeFakeBin(t, dir, "iptables-save", `cat <<'IPSAVE_EOF'
*nat
:PREROUTING ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:CONSUL_PROXY_OUTPUT - [0:0]
:CONSUL_DNS_REDIRECT - [0:0]
:CONSUL_PROXY_OUTPUT_CUSTOM - [0:0]
-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT_CUSTOM
-A OUTPUT -p udp -j CONSUL_DNS_REDIRECT
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `echo "$@" >> "`+iptablesLog+`"; exit 0`)
	usePATH(t, dir)

	require.NoError(t, flushLegacyIPTablesRules(""))

	logged, err := os.ReadFile(iptablesLog)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(logged)), "\n")

	require.Contains(t, lines, "-t nat -D OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT")
	require.Contains(t, lines, "-t nat -F CONSUL_PROXY_OUTPUT")
	require.Contains(t, lines, "-t nat -X CONSUL_PROXY_OUTPUT")
	require.Contains(t, lines, "-t nat -D OUTPUT -p udp -j CONSUL_DNS_REDIRECT")
	require.Contains(t, lines, "-t nat -F CONSUL_DNS_REDIRECT")
	require.Contains(t, lines, "-t nat -X CONSUL_DNS_REDIRECT")

	for _, l := range lines {
		require.NotContains(t, l, "CONSUL_PROXY_OUTPUT_CUSTOM",
			"a chain that merely shares a name prefix with a Consul chain must never be targeted")
	}
}

// TestFlushLegacyIPTablesRules_DeletionFailurePropagates verifies that a
// failing chain deletion is not swallowed: the error is returned, and
// includes which command failed.
func TestFlushLegacyIPTablesRules_DeletionFailurePropagates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IPTABLES_FAIL_SUBSTR", "-X CONSUL_PROXY_OUTPUT")

	writeFakeBin(t, dir, "iptables-save", `cat <<'IPSAVE_EOF'
*nat
:CONSUL_PROXY_OUTPUT - [0:0]
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `
case "$*" in
  *"$IPTABLES_FAIL_SUBSTR"*)
    echo "simulated deletion failure" >&2
    exit 1
    ;;
esac
exit 0
`)
	usePATH(t, dir)

	err := flushLegacyIPTablesRules("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated deletion failure")
	require.Contains(t, err.Error(), "CONSUL_PROXY_OUTPUT")
}

// TestFlushLegacyIPTablesRules_NamespaceForwarding verifies that both the
// discovery (iptables-save) and mutation (iptables) commands are routed
// through nsenter when a network namespace is configured.
func TestFlushLegacyIPTablesRules_NamespaceForwarding(t *testing.T) {
	dir := t.TempDir()
	nsenterLog := filepath.Join(dir, "nsenter.log")
	t.Setenv("NSENTER_LOG", nsenterLog)
	writeFakeNsenter(t, dir)

	writeFakeBin(t, dir, "iptables-save", `cat <<'IPSAVE_EOF'
*nat
:CONSUL_DNS_REDIRECT - [0:0]
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `exit 0`)
	usePATH(t, dir)

	require.NoError(t, flushLegacyIPTablesRules("/var/run/netns/test"))

	logged, err := os.ReadFile(nsenterLog)
	require.NoError(t, err)
	require.Contains(t, string(logged), "--net=/var/run/netns/test -- iptables-save -t nat")
	require.Contains(t, string(logged), "--net=/var/run/netns/test -- iptables -t nat -F CONSUL_DNS_REDIRECT")
	require.Contains(t, string(logged), "--net=/var/run/netns/test -- iptables -t nat -X CONSUL_DNS_REDIRECT")
}

// TestApplyRules_WritesScriptToNftStdin verifies that ApplyRules joins the
// collected AddRule lines and pipes them to `nft -f -` via stdin, exactly as
// documented.
func TestApplyRules_WritesScriptToNftStdin(t *testing.T) {
	dir := t.TempDir()
	stdinCapture := filepath.Join(dir, "nft.stdin")
	writeFakeBin(t, dir, "nft", `cat > "`+stdinCapture+`"`)
	usePATH(t, dir) // no iptables* present, so legacy cleanup silently no-ops

	n := &nftablesExecutor{}
	n.AddRule("nft", "add", "table", "inet", "consul_tproxy")
	n.AddRule("nft", "add", "chain", "inet", "consul_tproxy", "PROXY_INBOUND")

	require.NoError(t, n.ApplyRules("nft"))

	got, err := os.ReadFile(stdinCapture)
	require.NoError(t, err)
	require.Equal(t, "add table inet consul_tproxy\nadd chain inet consul_tproxy PROXY_INBOUND", string(got))
}

// TestApplyRules_NftFailure_PreventsCleanupAndReturnsError verifies that
// when the native `nft -f -` apply fails, ApplyRules returns an error
// including nft's output, and never attempts legacy iptables cleanup —
// leaving any pre-existing legacy rules (and their interception) intact.
func TestApplyRules_NftFailure_PreventsCleanupAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "nft", `echo "nft: syntax error" >&2; exit 1`)

	cleanupMarker := filepath.Join(dir, "cleanup-invoked")
	writeFakeBin(t, dir, "iptables-save", `touch "`+cleanupMarker+`"; exit 1`)
	usePATH(t, dir)

	n := &nftablesExecutor{}
	n.AddRule("nft", "add", "table", "inet", "consul_tproxy")

	err := n.ApplyRules("nft")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to apply nftables rules")
	require.Contains(t, err.Error(), "nft: syntax error")

	_, statErr := os.Stat(cleanupMarker)
	require.True(t, os.IsNotExist(statErr), "legacy cleanup must not run when the native nft apply fails")
}

// TestApplyRules_SuccessRunsLegacyCleanup verifies that once nft applies
// successfully, ApplyRules removes only the discovered legacy Consul
// chains, tying the nft-apply and iptables-cleanup steps together.
func TestApplyRules_SuccessRunsLegacyCleanup(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "nft", `exit 0`)

	iptablesLog := filepath.Join(dir, "iptables.log")
	writeFakeBin(t, dir, "iptables-save", `cat <<'IPSAVE_EOF'
*nat
:CONSUL_PROXY_OUTPUT - [0:0]
-A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `echo "$@" >> "`+iptablesLog+`"; exit 0`)
	usePATH(t, dir)

	n := &nftablesExecutor{}
	require.NoError(t, n.ApplyRules("nft"))

	logged, err := os.ReadFile(iptablesLog)
	require.NoError(t, err)
	require.Contains(t, string(logged), "-t nat -D OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT")
	require.Contains(t, string(logged), "-t nat -F CONSUL_PROXY_OUTPUT")
	require.Contains(t, string(logged), "-t nat -X CONSUL_PROXY_OUTPUT")
}

// TestApplyRules_CleanupFailure_ErrorPropagates verifies that a legacy
// cleanup failure is surfaced from ApplyRules (rather than swallowed),
// while still reporting that the new nftables rules did apply.
func TestApplyRules_CleanupFailure_ErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "nft", `exit 0`)

	writeFakeBin(t, dir, "iptables-save", `cat <<'IPSAVE_EOF'
*nat
:CONSUL_PROXY_OUTPUT - [0:0]
COMMIT
IPSAVE_EOF
`)
	writeFakeBin(t, dir, "iptables", `echo "simulated failure" >&2; exit 1`)
	usePATH(t, dir)

	n := &nftablesExecutor{}
	err := n.ApplyRules("nft")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nftables rules applied successfully, but failed to remove legacy iptables rules")
	require.Contains(t, err.Error(), "simulated failure")
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
