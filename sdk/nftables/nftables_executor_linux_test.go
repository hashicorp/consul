// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package nftables

import (
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

func TestRuleHandle(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		expected string
	}{
		{"handle present", "tcp dport 22 jump CONSUL_PROXY_INBOUND # handle 5", "5"},
		{"no handle", "tcp dport 22 jump CONSUL_PROXY_INBOUND", ""},
		{"trailing handle with no number", "tcp dport 22 jump CONSUL_PROXY_INBOUND # handle", ""},
		{"empty line", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, ruleHandle(strings.Fields(c.line)))
		})
	}
}

func TestParseNftLegacyTable(t *testing.T) {
	consulChains := []string{
		"CONSUL_PROXY_INBOUND",
		"CONSUL_PROXY_IN_REDIRECT",
		"CONSUL_PROXY_OUTPUT",
		"CONSUL_PROXY_REDIRECT",
		"CONSUL_DNS_REDIRECT",
	}

	t.Run("legacy ip nat table with jumps", func(t *testing.T) {
		dump := `
table ip nat {
	chain PREROUTING {
		type nat hook prerouting priority -100; policy accept;
		tcp dport 22 jump CONSUL_PROXY_INBOUND # handle 5
	}
	chain CONSUL_PROXY_INBOUND {
		tcp dport 20000 redirect to :20000 # handle 6
	}
	chain OUTPUT {
		type nat hook output priority -100; policy accept;
		meta l4proto tcp jump CONSUL_PROXY_OUTPUT # handle 9
	}
	chain CONSUL_PROXY_OUTPUT {
		jump CONSUL_PROXY_REDIRECT # handle 10
	}
	chain CONSUL_PROXY_REDIRECT {
		meta l4proto tcp redirect to :21000 # handle 11
	}
}
table inet consul_tproxy {
	chain CONSUL_PROXY_OUTPUT {
		jump CONSUL_PROXY_REDIRECT # handle 20
	}
}
`
		chains, jumps := parseNftLegacyTable(dump, "ip", consulChains)
		require.ElementsMatch(t, []string{"CONSUL_PROXY_INBOUND", "CONSUL_PROXY_OUTPUT", "CONSUL_PROXY_REDIRECT"}, chains)
		require.ElementsMatch(t, []nftRuleRef{
			{chain: "PREROUTING", handle: "5", target: "CONSUL_PROXY_INBOUND"},
			{chain: "OUTPUT", handle: "9", target: "CONSUL_PROXY_OUTPUT"},
			{chain: "CONSUL_PROXY_OUTPUT", handle: "10", target: "CONSUL_PROXY_REDIRECT"},
		}, jumps)
	})

	// Regression test: this package's own new table declares chains with the
	// exact same names (CONSUL_PROXY_OUTPUT, etc.) in the "inet"
	// "consul_tproxy" table. Those must never be mistaken for legacy
	// iptables-nft-backed chains in the "ip"/"ip6" "nat" table.
	t.Run("own inet consul_tproxy table is never matched", func(t *testing.T) {
		dump := `
table inet consul_tproxy {
	chain CONSUL_PROXY_OUTPUT {
		jump CONSUL_PROXY_REDIRECT # handle 20
	}
	chain CONSUL_PROXY_REDIRECT {
		meta l4proto tcp redirect to :21000 # handle 21
	}
}
`
		chains, jumps := parseNftLegacyTable(dump, "ip", consulChains)
		require.Empty(t, chains)
		require.Empty(t, jumps)
	})

	t.Run("no matching table", func(t *testing.T) {
		dump := `
table ip filter {
	chain INPUT {
		type filter hook input priority 0; policy accept;
	}
}
`
		chains, jumps := parseNftLegacyTable(dump, "ip", consulChains)
		require.Empty(t, chains)
		require.Empty(t, jumps)
	})

	t.Run("ip6 family is scoped independently from ip", func(t *testing.T) {
		dump := `
table ip nat {
	chain CONSUL_PROXY_OUTPUT {
	}
}
table ip6 nat {
	chain CONSUL_DNS_REDIRECT {
	}
}
`
		ipChains, _ := parseNftLegacyTable(dump, "ip", consulChains)
		ip6Chains, _ := parseNftLegacyTable(dump, "ip6", consulChains)
		require.ElementsMatch(t, []string{"CONSUL_PROXY_OUTPUT"}, ipChains)
		require.ElementsMatch(t, []string{"CONSUL_DNS_REDIRECT"}, ip6Chains)
	})

	t.Run("empty dump", func(t *testing.T) {
		chains, jumps := parseNftLegacyTable("", "ip", consulChains)
		require.Empty(t, chains)
		require.Empty(t, jumps)
	})
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
