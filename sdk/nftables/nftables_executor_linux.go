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

	for _, pair := range []struct{ save, tables, nftFamily string }{
		{"iptables-save", "iptables", "ip"},
		{"ip6tables-save", "ip6tables", "ip6"},
	} {
		_, saveErr := exec.LookPath(pair.save)
		_, toolErr := exec.LookPath(pair.tables)
		if saveErr != nil || toolErr != nil {
			// Tool unavailable doesn't mean no legacy rules: on most modern
			// distros "iptables" is a compatibility shim (iptables-nft)
			// that stores rules as nftables objects in "ip"/"ip6", which
			// `nft` can still see. Fall back to that instead of assuming
			// the namespace is clean.
			if err := flushIPTablesWithNftShim(netNS, pair.nftFamily, consulChains); err != nil {
				errs = append(errs, fmt.Errorf("legacy iptables tools unavailable, nft fallback (%s family): %w", pair.nftFamily, err))
			}
			continue
		}

		// Dump the nat table and derive which Consul chains actually exist.
		// A chain that only shares a name prefix (e.g. an administrator's
		// own "CONSUL_PROXY_OUTPUT_CUSTOM") must not be treated as a match.
		out, err := runOutput(netNS, pair.save, "-t", "nat")
		if err != nil {
			// A real failure, not "no rules found" -- must not be treated
			// as clean, since we don't actually know.
			errs = append(errs, fmt.Errorf("%s -t nat: %w", pair.save, err))
			continue
		}
		existing := declaredChains(string(out), consulChains)
		if len(existing) == 0 {
			continue
		}

		// Legacy rules detected. Delete every jump into a Consul-owned
		// chain, then flush/delete only chains confirmed present. Chains
		// already removed by a prior attempt are simply absent from
		// `existing`, making a partial cleanup retryable.
		for _, line := range strings.Split(string(out), "\n") {
			// Lines like: -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT
			if !strings.HasPrefix(line, "-A ") {
				continue
			}
			target := jumpTarget(strings.Fields(line))
			if target == "" || !isOwnedChain(target, existing) {
				continue
			}
			// Convert "-A" to "-D" to delete the exact rule.
			delArgs := strings.Fields(strings.Replace(line, "-A ", "-D ", 1))
			if err := run(netNS, pair.tables, append([]string{"-t", "nat"}, delArgs...)...); err != nil {
				errs = append(errs, fmt.Errorf("%s -D (jump to %s): %w", pair.tables, target, err))
			}
		}

		for _, chain := range existing {
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

// flushIPTablesWithNftShim is the fallback used when iptables/ip6tables are
// unavailable. Most modern distros implement "iptables" via iptables-nft, a
// shim that stores its rules as nftables objects in the "ip"/"ip6" families
// -- still visible and removable via `nft` even without the tool itself.
func flushIPTablesWithNftShim(netNS, family string, candidates []string) error {
	out, err := runOutput(netNS, "nft", "-a", "list", "ruleset")
	if err != nil {
		return fmt.Errorf("nft -a list ruleset: %w", err)
	}

	chains, jumps := parseNftLegacyTable(string(out), family, candidates)
	if len(chains) == 0 {
		return nil
	}

	var errs []error
	for _, j := range jumps {
		if err := run(netNS, "nft", "delete", "rule", family, "nat", j.chain, "handle", j.handle); err != nil {
			errs = append(errs, fmt.Errorf("nft delete rule %s nat %s handle %s (jump to %s): %w", family, j.chain, j.handle, j.target, err))
		}
	}

	for _, chain := range chains {
		if err := run(netNS, "nft", "flush", "chain", family, "nat", chain); err != nil {
			errs = append(errs, fmt.Errorf("nft flush chain %s nat %s: %w", family, chain, err))
			continue
		}
		if err := run(netNS, "nft", "delete", "chain", family, "nat", chain); err != nil {
			errs = append(errs, fmt.Errorf("nft delete chain %s nat %s: %w", family, chain, err))
		}
	}

	return errors.Join(errs...)
}

// nftRuleRef identifies a rule inside a legacy nft-backed table that jumps to
// a Consul-owned chain, so it can be deleted (by handle) before that chain is
// removed.
type nftRuleRef struct {
	chain  string // the chain containing this rule
	handle string // this rule's handle, from a trailing "# handle N" comment
	target string // the Consul chain this rule jumps/gotos to
}

// parseNftLegacyTable scans `nft -a list ruleset` output, scoped strictly to
// "table <family> nat { ... }", for candidate chains declared there and any
// rule jumping/going to one. A chain in this package's own "inet
// consul_tproxy" table is never matched, even with the same name.
func parseNftLegacyTable(dump, family string, candidates []string) (chains []string, jumps []nftRuleRef) {
	wantHeader := "table " + family + " nat "
	inTable := false
	depth := 0
	curChain := ""
	declared := make(map[string]bool)

	isCandidate := func(name string) bool {
		for _, c := range candidates {
			if name == c {
				return true
			}
		}
		return false
	}

	for _, raw := range strings.Split(dump, "\n") {
		line := strings.TrimSpace(raw)

		if !inTable {
			if strings.HasPrefix(line, wantHeader) && strings.Contains(line, "{") {
				inTable = true
				depth = 1
			}
			continue
		}

		if depth == 1 && strings.HasPrefix(line, "chain ") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				curChain = fields[1]
				if isCandidate(curChain) {
					declared[curChain] = true
				}
			}
		}

		if depth >= 2 && curChain != "" {
			fields := strings.Fields(line)
			for i, f := range fields {
				if (f == "jump" || f == "goto") && i+1 < len(fields) && isCandidate(fields[i+1]) {
					if h := ruleHandle(fields); h != "" {
						jumps = append(jumps, nftRuleRef{chain: curChain, handle: h, target: fields[i+1]})
					}
				}
			}
		}

		depth += strings.Count(line, "{") - strings.Count(line, "}")
		switch {
		case depth <= 0:
			inTable = false
			curChain = ""
		case depth == 1:
			curChain = ""
		}
	}

	for _, c := range candidates {
		if declared[c] {
			chains = append(chains, c)
		}
	}
	return chains, jumps
}

// ruleHandle returns the numeric handle from a rule line's trailing
// "# handle N" comment (as produced by `nft -a`), or "" if absent.
func ruleHandle(fields []string) string {
	for i, f := range fields {
		if f == "handle" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// declaredChains parses an iptables-save dump and returns the subset of
// candidates actually declared as chains (lines of the form
// ":ChainName POLICY [packets:bytes]"). Matching is exact, so
// "CONSUL_PROXY_OUTPUT_CUSTOM" is never mistaken for "CONSUL_PROXY_OUTPUT".
func declaredChains(dump string, candidates []string) []string {
	declared := make(map[string]bool)
	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ":") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		declared[strings.TrimPrefix(fields[0], ":")] = true
	}

	var existing []string
	for _, c := range candidates {
		if declared[c] {
			existing = append(existing, c)
		}
	}
	return existing
}

// jumpTarget returns the chain name a rule jumps to (the argument following
// "-j" or "--jump"), or "" if the rule has no jump target.
func jumpTarget(fields []string) string {
	for i, f := range fields {
		if (f == "-j" || f == "--jump") && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// isOwnedChain reports whether target is exactly one of the given chain
// names. Callers must use exact equality rather than substring matching: a
// chain named e.g. "CONSUL_PROXY_OUTPUT_CUSTOM" is not owned by Consul even
// though "CONSUL_PROXY_OUTPUT" is a prefix of its name, and deleting jumps to
// it would remove an administrator-managed rule Consul never created.
func isOwnedChain(target string, chains []string) bool {
	for _, c := range chains {
		if target == c {
			return true
		}
	}
	return false
}

// runOutput runs a command and returns its stdout. On failure, the returned
// error wraps the captured stderr, so a failed inspection carries an
// actionable diagnostic message instead of a bare exit status.
func runOutput(netNS, bin string, args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	if netNS != "" {
		nsArgs := append([]string{fmt.Sprintf("--net=%s", netNS), "--", bin}, args...)
		cmd = exec.Command("nsenter", nsArgs...)
	} else {
		cmd = exec.Command(bin, args...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w, output: %s", err, stderr.String())
	}
	return out, nil
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
