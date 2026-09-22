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
