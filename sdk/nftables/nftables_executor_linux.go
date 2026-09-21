// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package nftables

import (
	"bytes"
	"errors"
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

// flushLegacyIPTablesRules deletes Consul-managed iptables/ip6tables rules and
// chains left behind by a previous (pre-nftables) installation, preventing
// double-NAT or inconsistent redirection alongside the new nftables rules. It
// is a no-op when no legacy Consul chains are detected, and returns an error
// aggregating every failed deletion rather than ignoring cleanup failures.
func flushLegacyIPTablesRules(netNS string) error {
	consulChains := []string{
		"CONSUL_PROXY_INBOUND",
		"CONSUL_PROXY_IN_REDIRECT",
		"CONSUL_PROXY_OUTPUT",
		"CONSUL_PROXY_REDIRECT",
		"CONSUL_DNS_REDIRECT",
	}

	var errs []error

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
		// the Consul chains themselves. Failures are collected rather than
		// ignored, so the caller learns cleanup was incomplete.
		for _, line := range strings.Split(string(out), "\n") {
			// Lines like: -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
			if !strings.HasPrefix(line, "-A ") {
				continue
			}
			for _, chain := range consulChains {
				if strings.Contains(line, "-j "+chain) {
					// Convert "-A" to "-D" to delete the exact rule.
					delArgs := strings.Fields(strings.Replace(line, "-A ", "-D ", 1))
					if err := run(netNS, pair.tables, append([]string{"-t", "nat"}, delArgs...)...); err != nil {
						errs = append(errs, fmt.Errorf("%s -D %s: %w", pair.tables, chain, err))
					}
				}
			}
		}

		for _, chain := range consulChains {
			if err := run(netNS, pair.tables, "-t", "nat", "-F", chain); err != nil {
				errs = append(errs, fmt.Errorf("%s -F %s: %w", pair.tables, chain, err))
				continue
			}
			if err := run(netNS, pair.tables, "-t", "nat", "-X", chain); err != nil {
				errs = append(errs, fmt.Errorf("%s -X %s: %w", pair.tables, chain, err))
			}
		}
	}

	return errors.Join(errs...)
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

// run executes a command and returns an error (including captured output) if it fails.
func run(netNS, bin string, args ...string) error {
	var cmd *exec.Cmd
	if netNS != "" {
		nsArgs := append([]string{fmt.Sprintf("--net=%s", netNS), "--", bin}, args...)
		cmd = exec.Command("nsenter", nsArgs...)
	} else {
		cmd = exec.Command(bin, args...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w, output: %s", err, out.String())
	}
	return nil
}

// ApplyRules builds a script from all collected lines and pipes it atomically to
// `nft -f -` (or `nsenter --net=<ns> -- nft -f -` when a network namespace is
// configured).  The command argument is unused; nft is always the binary.
func (n *nftablesExecutor) ApplyRules(_ string) error {
	if _, err := exec.LookPath("nft"); err != nil {
		return fmt.Errorf("nft binary not found: %w", err)
	}

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
		// The native nftables rules failed to apply. Any pre-existing legacy
		// iptables rules are left untouched (cleanup only runs below, after
		// a successful install), so traffic keeps flowing through them
		// instead of being left with no interception at all.
		return fmt.Errorf("failed to apply nftables rules: %w, output: %s", err, out.String())
	}

	// The new nftables rules are active. Only now attempt to remove any
	// legacy iptables/ip6tables rules from a previous install — never
	// before the new rules are confirmed working.
	if err := flushLegacyIPTablesRules(n.cfg.NetNS); err != nil {
		return fmt.Errorf("nftables rules applied successfully, but failed to remove legacy iptables rules: %w", err)
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
