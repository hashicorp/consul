// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iptables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetup_IPv4(t *testing.T) {
	cases := []struct {
		name            string
		cfg             Config
		additionalRules [][]string
		expectedRules   []string
	}{
		{
			"no proxy outbound port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS IP provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_DNS_REDIRECT udp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet nat CONSUL_DNS_REDIRECT tcp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet nat CONSUL_NAT_OUTPUT udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSPort:    8600,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip daddr 127.0.0.1 udp dport 53 dnat to 127.0.0.1:8600",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip daddr 127.0.0.1 tcp dport 53 dnat to 127.0.0.1:8600",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip daddr 127.0.0.1 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip daddr 127.0.0.1 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS IP and port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				ConsulDNSPort:    8600,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 udp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 tcp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"proxy outbound port is provided",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				IptablesProvider:  &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude inbound ports is set",
			Config{
				ProxyUserID:         "123",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   21000,
				ExcludeInboundPorts: []string{"22000", "22500"},
				IptablesProvider:    &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_INBOUND tcp dport 22000 return",
				"nft insert rule inet nat CONSUL_PROXY_INBOUND tcp dport 22500 return",
			},
		},
		{
			"exclude outbound ports is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundPorts: []string{"22000", "22500"},
				IptablesProvider:     &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT tcp dport 22000 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT tcp dport 22500 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude outbound CIDRs is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundCIDRs: []string{"1.1.1.1", "2.2.2.2/24"},
				IptablesProvider:     &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT ip daddr 1.1.1.1 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT ip daddr 2.2.2.2/24 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude UIDs is set",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				IptablesProvider:  &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"additional rules are passed",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				IptablesProvider:  &fakeIptablesProvider{},
			},
			[][]string{
				{"nft", "add", "rule", "inet", "nat", "CONSUL_NAT_OUTPUT", "ip", "saddr", "192.0.2.0/24", "accept"},
				{"nft", "add", "rule", "inet", "nat", "CONSUL_NAT_PREROUTING", "ip", "saddr", "192.0.2.0/24", "accept"},
			},
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip saddr 192.0.2.0/24 accept",
				"nft add rule inet nat CONSUL_NAT_PREROUTING ip saddr 192.0.2.0/24 accept",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var fn AdditionalRulesFn
			if c.additionalRules != nil {
				fn = func(provider Provider) {
					for _, rule := range c.additionalRules {
						provider.AddRule(rule[0], rule[1:]...)
					}
				}
			}

			err := SetupWithAdditionalRules(c.cfg, fn, false)
			require.NoError(t, err)
			require.Equal(t, c.expectedRules, c.cfg.IptablesProvider.Rules())
		})
	}
}

func TestSetup_IPv4_Dualstack(t *testing.T) {
	cases := []struct {
		name            string
		cfg             Config
		additionalRules [][]string
		expectedRules   []string
	}{
		{
			"no proxy outbound port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			// DNS IP is IPv4 — with inet family it is applied directly (no dualStack guard).
			"Consul DNS IP provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_DNS_REDIRECT udp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet nat CONSUL_DNS_REDIRECT tcp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet nat CONSUL_NAT_OUTPUT udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			// With dualStack=true and no explicit ConsulDNSIP, the default IPv6 address ::1 is used.
			"Consul DNS port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSPort:    8600,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip6 daddr ::1 udp dport 53 dnat to [::1]:8600",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip6 daddr ::1 tcp dport 53 dnat to [::1]:8600",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip6 daddr ::1 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip6 daddr ::1 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			// ConsulDNSIP is explicitly IPv4 so ipKw="ip" regardless of dualStack flag.
			"Consul DNS IP and port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				ConsulDNSPort:    8600,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 udp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet nat CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 tcp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"proxy outbound port is provided",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				IptablesProvider:  &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude inbound ports is set",
			Config{
				ProxyUserID:         "123",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   21000,
				ExcludeInboundPorts: []string{"22000", "22500"},
				IptablesProvider:    &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_INBOUND tcp dport 22000 return",
				"nft insert rule inet nat CONSUL_PROXY_INBOUND tcp dport 22500 return",
			},
		},
		{
			"exclude outbound ports is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundPorts: []string{"22000", "22500"},
				IptablesProvider:     &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT tcp dport 22000 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT tcp dport 22500 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			// IPv6 CIDRs — ipFamilyKeyword returns "ip6" for these addresses.
			"exclude outbound CIDRs is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundCIDRs: []string{"2406:da1a:23:5e05:e1c6::5", "2406:da1a:23:5e05:e1c6::ffff/24"},
				IptablesProvider:     &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr 2406:da1a:23:5e05:e1c6::5 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr 2406:da1a:23:5e05:e1c6::ffff/24 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude UIDs is set",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				IptablesProvider:  &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"additional rules are passed",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				IptablesProvider:  &fakeIptablesProvider{},
			},
			[][]string{
				{"nft", "add", "rule", "inet", "nat", "CONSUL_NAT_OUTPUT", "ip", "saddr", "192.0.2.0/24", "accept"},
				{"nft", "add", "rule", "inet", "nat", "CONSUL_NAT_PREROUTING", "ip", "saddr", "192.0.2.0/24", "accept"},
			},
			[]string{
				"nft add table inet nat",
				"nft add chain inet nat CONSUL_PROXY_INBOUND",
				"nft add chain inet nat CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet nat CONSUL_PROXY_OUTPUT",
				"nft add chain inet nat CONSUL_PROXY_REDIRECT",
				"nft add chain inet nat CONSUL_DNS_REDIRECT",
				"nft add chain inet nat CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet nat CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet nat CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet nat CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet nat CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet nat CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet nat CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet nat CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet nat CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft add rule inet nat CONSUL_NAT_OUTPUT ip saddr 192.0.2.0/24 accept",
				"nft add rule inet nat CONSUL_NAT_PREROUTING ip saddr 192.0.2.0/24 accept",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var fn AdditionalRulesFn
			if c.additionalRules != nil {
				fn = func(provider Provider) {
					for _, rule := range c.additionalRules {
						provider.AddRule(rule[0], rule[1:]...)
					}
				}
			}

			err := SetupWithAdditionalRules(c.cfg, fn, true)
			require.NoError(t, err)
			require.Equal(t, c.expectedRules, c.cfg.IptablesProvider.Rules())
		})
	}
}

// TestSetup_IPv6 verifies that SetupWithAdditionalRulesIPv6 is a no-op.
// With nftables inet family, IPv6 is handled automatically by SetupWithAdditionalRules.
func TestSetup_IPv6(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		ProxyInboundPort: 20000,
		IptablesProvider: &fakeIptablesProvider{},
	}
	err := SetupWithAdditionalRulesIPv6(cfg, nil, true)
	require.NoError(t, err)
	require.Empty(t, cfg.IptablesProvider.Rules(),
		"SetupWithAdditionalRulesIPv6 should be a no-op: inet family in SetupWithAdditionalRules covers IPv6")
}

func TestVerifyDualStackConfig(t *testing.T) {
	// Define various test cases to cover all branches of the function.
	testCases := []struct {
		name        string
		cfg         Config
		dualStack   bool
		expectError bool
		errorMsg    string
	}{
		// --- Dual Stack Enabled (dualStack = true) ---
		{
			name:        "Dual Stack: Valid IPv6",
			cfg:         Config{ConsulDNSIP: "2001:db8::68"},
			dualStack:   true,
			expectError: false,
		},
		{
			name:        "Dual Stack: Valid IPv4 (should fail)",
			cfg:         Config{ConsulDNSIP: "192.0.2.1"},
			dualStack:   true,
			expectError: true,
			errorMsg:    "for dual stack ipv6 consulDNSIP required",
		},
		{
			name:        "Dual Stack: Empty IP",
			cfg:         Config{ConsulDNSIP: ""},
			dualStack:   true,
			expectError: false,
		},
		{
			name:        "Dual Stack: Invalid IP",
			cfg:         Config{ConsulDNSIP: "not-an-ip"},
			dualStack:   true,
			expectError: true,
			errorMsg:    "unable to parse consulDNSIP",
		},
		// --- Dual Stack Disabled (dualStack = false) ---
		{
			name:        "Non-Dual Stack: Valid IPv4",
			cfg:         Config{ConsulDNSIP: "192.0.2.1"},
			dualStack:   false,
			expectError: false,
		},
		{
			name:        "Non-Dual Stack: Valid IPv6 (should fail)",
			cfg:         Config{ConsulDNSIP: "2001:db8::68"},
			dualStack:   false,
			expectError: true,
			errorMsg:    "for non dual stack setup ipv4 consulDNSIP required",
		},
		{
			name:        "Non-Dual Stack: Empty IP",
			cfg:         Config{ConsulDNSIP: ""},
			dualStack:   false,
			expectError: false,
		},
		{
			name:        "Non-Dual Stack: Invalid IP",
			cfg:         Config{ConsulDNSIP: "not-an-ip"},
			dualStack:   false,
			expectError: true,
			errorMsg:    "unable to parse consulDNSIP",
		},
	}

	// Iterate over the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyDualStackConfig(tc.cfg, tc.dualStack)

			if tc.expectError {
				// We expect an error
				if err == nil {
					t.Errorf("expected an error, but got nil")
				} else if err.Error() != tc.errorMsg {
					t.Errorf("expected error message '%s', but got '%s'", tc.errorMsg, err.Error())
				}
			} else {
				// We do not expect an error
				if err != nil {
					t.Errorf("did not expect an error, but got: %v", err)
				}
			}
		})
	}
}

func TestSetup_errors(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		expErr string
	}{
		{
			"no proxy UID",
			Config{
				IptablesProvider: &nftablesExecutor{},
			},
			"ProxyUserID is required to set up traffic redirection",
		},
		{
			"no proxy inbound port",
			Config{
				ProxyUserID:       "123",
				ProxyOutboundPort: 21000,
				IptablesProvider:  &nftablesExecutor{},
			},
			"ProxyInboundPort is required to set up traffic redirection",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Setup(c.cfg, true)
			require.EqualError(t, err, c.expErr)
			err = Setup(c.cfg, false)
			require.EqualError(t, err, c.expErr)

		})
	}
}

type fakeIptablesProvider struct {
	rules []string
}

func (f *fakeIptablesProvider) AddRule(name string, args ...string) {
	var rule []string
	rule = append(rule, name)
	rule = append(rule, args...)

	f.rules = append(f.rules, strings.Join(rule, " "))
}

func (f *fakeIptablesProvider) ApplyRules(command string) error {
	return nil
}

func (f *fakeIptablesProvider) Rules() []string {
	return f.rules
}

func (f *fakeIptablesProvider) ClearAllRules() {
	f.rules = nil
}
