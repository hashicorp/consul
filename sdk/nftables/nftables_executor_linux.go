// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package nftables

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// nftablesExecutor implements Provider by collecting nft(8) script lines and
// applying them atomically via `nft -f -` (reading from stdin).
type nftablesExecutor struct {
	lines []string
	cfg   Config
}

// AddRule collects one nft script line. The first argument (the binary name) is
// ignored; all remaining arguments form the nft command that will be written to
// the script (e.g. "add", "rule", "inet", "nat", ...).
func (n *nftablesExecutor) AddRule(_ string, args ...string) {
	n.lines = append(n.lines, strings.Join(args, " "))
}

// flushLegacyIPTablesRules removes Consul-managed iptables/ip6tables rules and
// chains left over from a previous installation to prevent double-NAT and broken connectivity.
// It is a no-op when no legacy chains are detected. All errors are ignored.
func flushLegacyIPTablesRules(netNS string) {
	consulChains := []string{
		"CONSUL_PROXY_INBOUND",
		"CONSUL_PROXY_IN_REDIRECT",
		"CONSUL_PROXY_OUTPUT",
		"CONSUL_PROXY_REDIRECT",
		"CONSUL_DNS_REDIRECT",
	}

	for _, pair := range []struct{ save, tables string }{
		{"iptables-save", "iptables"},
		{"ip6tables-save", "ip6tables"},
	} {
		if _, err := exec.LookPath(pair.save); err != nil {
			continue
		}
		if _, err := exec.LookPath(pair.tables); err != nil {
			continue
		}

		// Dump the nat table. If none of the Consul chain names appear, skip
		// entirely — zero extra forks in the normal (no legacy rules) case.
		out, err := runOutput(netNS, pair.save, "-t", "nat")
		if err != nil || !containsAny(string(out), consulChains) {
			continue
		}

		// Legacy rules detected. Walk the dump and delete every rule that
		// jumps to a Consul chain from a built-in chain, then flush/delete
		// the Consul chains themselves.
		for _, line := range strings.Split(string(out), "\n") {
			// Lines like: -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
			if !strings.HasPrefix(line, "-A ") {
				continue
			}
			for _, chain := range consulChains {
				if strings.Contains(line, "-j "+chain) {
					// Convert "-A" to "-D" to delete the exact rule.
					delArgs := strings.Fields(strings.Replace(line, "-A ", "-D ", 1))
					runSilent(netNS, pair.tables, append([]string{"-t", "nat"}, delArgs...)...)
				}
			}
		}

		for _, chain := range consulChains {
			runSilent(netNS, pair.tables, "-t", "nat", "-F", chain)
			runSilent(netNS, pair.tables, "-t", "nat", "-X", chain)
		}
	}
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// runOutput runs a command and returns its combined output.
func runOutput(netNS, bin string, args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	if netNS != "" {
		nsArgs := append([]string{fmt.Sprintf("--net=%s", netNS), "--", bin}, args...)
		cmd = exec.Command("nsenter", nsArgs...)
	} else {
		cmd = exec.Command(bin, args...)
	}
	return cmd.Output()
}

// runSilent executes a command, silently ignoring errors — for best-effort cleanup.
func runSilent(netNS, bin string, args ...string) {
	var cmd *exec.Cmd
	if netNS != "" {
		nsArgs := append([]string{fmt.Sprintf("--net=%s", netNS), "--", bin}, args...)
		cmd = exec.Command("nsenter", nsArgs...)
	} else {
		cmd = exec.Command(bin, args...)
	}
	_ = cmd.Run()
}

// ApplyRules builds a script from all collected lines and pipes it atomically to
// `nft -f -` (or `nsenter --net=<ns> -- nft -f -` when a network namespace is
// configured).  The command argument is unused; nft is always the binary.
func (n *nftablesExecutor) ApplyRules(_ string) error {
	if _, err := exec.LookPath("nft"); err != nil {
		return fmt.Errorf("nft binary not found: %w", err)
	}

	//cleanup of any legacy iptables rules from a previous Consul version.
	flushLegacyIPTablesRules(n.cfg.NetNS)

	script := strings.Join(n.lines, "\n")

	var cmd *exec.Cmd
	if n.cfg.NetNS != "" {
		if _, err := exec.LookPath("nsenter"); err != nil {
			return fmt.Errorf("nsenter binary not found: %w", err)
		}
		cmd = exec.Command("nsenter", fmt.Sprintf("--net=%s", n.cfg.NetNS), "--", "nft", "-f", "-")
	} else {
		cmd = exec.Command("nft", "-f", "-")
	}
	cmd.Stdin = strings.NewReader(script)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply nftables rules: %w, output: %s", err, out.String())
	}
	return nil
}

// Rules returns the accumulated nft commands prefixed with "nft " for
// inspection (e.g. in tests and the Rules() API).
func (n *nftablesExecutor) Rules() []string {
	var rules []string
	for _, line := range n.lines {
		rules = append(rules, "nft "+line)
	}
	return rules
}

func (n *nftablesExecutor) ClearAllRules() {
	n.lines = nil
}
