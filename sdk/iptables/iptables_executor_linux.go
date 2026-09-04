// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package iptables

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
