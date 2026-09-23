// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nftables

import (
	"errors"
	"net"
	"strconv"
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
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS IP provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT udp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT tcp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSPort:    8600,
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 127.0.0.1 udp dport 53 dnat to 127.0.0.1:8600",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 127.0.0.1 tcp dport 53 dnat to 127.0.0.1:8600",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 127.0.0.1 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 127.0.0.1 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS IP and port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				ConsulDNSPort:    8600,
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 udp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 tcp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"proxy outbound port is provided",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				NftablesProvider:  &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude inbound ports is set",
			Config{
				ProxyUserID:         "123",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   21000,
				ExcludeInboundPorts: []string{"22000", "22500"},
				NftablesProvider:    &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 22000 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 22500 return",
			},
		},
		{
			"exclude outbound ports is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundPorts: []string{"22000", "22500"},
				NftablesProvider:     &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT tcp dport 22000 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT tcp dport 22500 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude outbound CIDRs is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundCIDRs: []string{"1.1.1.1", "2.2.2.2/24"},
				NftablesProvider:     &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 1.1.1.1 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 2.2.2.2/24 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude UIDs is set",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				NftablesProvider:  &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"additional rules are passed",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				NftablesProvider:  &fakeNftablesProvider{},
			},
			[][]string{
				{"nft", "add", "rule", "inet", "consul_tproxy", "CONSUL_NAT_OUTPUT", "ip", "saddr", "192.0.2.0/24", "accept"},
				{"nft", "add", "rule", "inet", "consul_tproxy", "CONSUL_NAT_PREROUTING", "ip", "saddr", "192.0.2.0/24", "accept"},
			},
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip saddr 192.0.2.0/24 accept",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING ip saddr 192.0.2.0/24 accept",
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
			require.Equal(t, c.expectedRules, c.cfg.NftablesProvider.Rules())
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
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			// DNS IP is IPv4 — with inet family it is applied directly (no dualStack guard).
			"Consul DNS IP provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "10.0.34.16",
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT udp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT tcp dport 53 dnat ip to 10.0.34.16",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			// With dualStack=true and no explicit ConsulDNSIP, the inet family requires
			// rules for both loopback addresses: 127.0.0.1 (IPv4) and ::1 (IPv6).
			// This mirrors the iptables behaviour where SetupWithAdditionalRules wrote
			// the 127.0.0.1 rules via iptables and SetupWithAdditionalRulesIPv6 wrote
			// the ::1 rules via ip6tables.
			"Consul DNS port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSPort:    8600,
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				// IPv4 loopback rules (equivalent to iptables SetupWithAdditionalRules).
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 127.0.0.1 udp dport 53 dnat to 127.0.0.1:8600",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 127.0.0.1 tcp dport 53 dnat to 127.0.0.1:8600",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 127.0.0.1 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 127.0.0.1 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				// IPv6 loopback rules (equivalent to ip6tables SetupWithAdditionalRulesIPv6).
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip6 daddr ::1 udp dport 53 dnat to [::1]:8600",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip6 daddr ::1 tcp dport 53 dnat to [::1]:8600",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip6 daddr ::1 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip6 daddr ::1 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
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
				NftablesProvider: &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 udp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT ip daddr 10.0.34.16 tcp dport 53 dnat to 10.0.34.16:8600",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 udp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip daddr 10.0.34.16 tcp dport 53 jump CONSUL_DNS_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"proxy outbound port is provided",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				NftablesProvider:  &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude inbound ports is set",
			Config{
				ProxyUserID:         "123",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   21000,
				ExcludeInboundPorts: []string{"22000", "22500"},
				NftablesProvider:    &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 22000 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 22500 return",
			},
		},
		{
			"exclude outbound ports is set",
			Config{
				ProxyUserID:          "123",
				ProxyInboundPort:     20000,
				ProxyOutboundPort:    21000,
				ExcludeOutboundPorts: []string{"22000", "22500"},
				NftablesProvider:     &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT tcp dport 22000 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT tcp dport 22500 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
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
				NftablesProvider:     &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr 2406:da1a:23:5e05:e1c6::5 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr 2406:da1a:23:5e05:e1c6::ffff/24 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"exclude UIDs is set",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				NftablesProvider:  &fakeNftablesProvider{},
			},
			nil,
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"additional rules are passed",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				ExcludeUIDs:       []string{"456", "789"},
				NftablesProvider:  &fakeNftablesProvider{},
			},
			[][]string{
				{"nft", "add", "rule", "inet", "consul_tproxy", "CONSUL_NAT_OUTPUT", "ip", "saddr", "192.0.2.0/24", "accept"},
				{"nft", "add", "rule", "inet", "consul_tproxy", "CONSUL_NAT_PREROUTING", "ip", "saddr", "192.0.2.0/24", "accept"},
			},
			[]string{
				"nft add table inet consul_tproxy",
				"nft delete table inet consul_tproxy",
				"nft add table inet consul_tproxy",
				"nft add chain inet consul_tproxy CONSUL_PROXY_INBOUND",
				"nft add chain inet consul_tproxy CONSUL_PROXY_IN_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_OUTPUT",
				"nft add chain inet consul_tproxy CONSUL_PROXY_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_DNS_REDIRECT",
				"nft add chain inet consul_tproxy CONSUL_NAT_OUTPUT { type nat hook output priority -100 ; }",
				"nft add chain inet consul_tproxy CONSUL_NAT_PREROUTING { type nat hook prerouting priority -100 ; }",
				"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :21000",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta l4proto tcp jump CONSUL_PROXY_OUTPUT",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 123 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 127.0.0.1/32 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip6 daddr ::1/128 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_OUTPUT jump CONSUL_PROXY_REDIRECT",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 456 return",
				"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 789 return",
				"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta l4proto tcp jump CONSUL_PROXY_INBOUND",
				"nft add rule inet consul_tproxy CONSUL_PROXY_INBOUND meta l4proto tcp jump CONSUL_PROXY_IN_REDIRECT",
				"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT ip saddr 192.0.2.0/24 accept",
				"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING ip saddr 192.0.2.0/24 accept",
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
			require.Equal(t, c.expectedRules, c.cfg.NftablesProvider.Rules())
		})
	}
}

// TestSetup_IPv6 is commented out because SetupWithAdditionalRulesIPv6 is commented out.
// With nftables the inet family handles both IPv4 and IPv6 in a single SetupWithAdditionalRules
// call, so a separate IPv6 pass is no longer required.
// func TestSetup_IPv6(t *testing.T) {
// 	cfg := Config{
// 		ProxyUserID:      "123",
// 		ProxyInboundPort: 20000,
// 		NftablesProvider: &fakeNftablesProvider{},
// 	}
// 	err := SetupWithAdditionalRulesIPv6(cfg, nil, true)
// 	require.NoError(t, err)
// 	require.Empty(t, cfg.NftablesProvider.Rules(),
// 		"SetupWithAdditionalRulesIPv6 should be a no-op: inet family in SetupWithAdditionalRules covers IPv6")
// }

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
				NftablesProvider: &nftablesExecutor{},
			},
			"ProxyUserID is required to set up traffic redirection",
		},
		{
			"no proxy inbound port",
			Config{
				ProxyUserID:       "123",
				ProxyOutboundPort: 21000,
				NftablesProvider:  &nftablesExecutor{},
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

type fakeNftablesProvider struct {
	rules []string

	// applyErr, when non-nil, is returned by ApplyRules to simulate a
	// failed nft batch (e.g. a kernel/parse-time rejection of the
	// generated script), without actually mutating any real nftables state.
	applyErr error
	// applyCalls counts how many times ApplyRules was invoked.
	applyCalls int
}

func (f *fakeNftablesProvider) AddRule(name string, args ...string) {
	var rule []string
	rule = append(rule, name)
	rule = append(rule, args...)

	f.rules = append(f.rules, strings.Join(rule, " "))
}

func (f *fakeNftablesProvider) ApplyRules(command string) error {
	f.applyCalls++
	return f.applyErr
}

func (f *fakeNftablesProvider) Rules() []string {
	return f.rules
}

func (f *fakeNftablesProvider) ClearAllRules() {
	f.rules = nil
}

// TestIpFamilyKeyword covers ipFamilyKeyword for plain IPs, CIDRs, and edge cases.
func TestIpFamilyKeyword(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		// Plain IPv4
		{"1.2.3.4", "ip"},
		{"127.0.0.1", "ip"},
		{"0.0.0.0", "ip"},
		// IPv4 CIDR
		{"10.0.0.0/8", "ip"},
		{"2.2.2.2/24", "ip"},
		// Plain IPv6
		{"::1", "ip6"},
		{"2001:db8::1", "ip6"},
		{"2406:da1a:23:5e05:e1c6::5", "ip6"},
		// IPv6 CIDR
		{"2406:da1a:23:5e05:e1c6::ffff/24", "ip6"},
		{"fe80::/10", "ip6"},
		// Unparseable/empty — falls back to ip6 (default branch)
		{"not-an-ip", "ip6"},
		{"", "ip6"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			require.Equal(t, c.expected, ipFamilyKeyword(c.input))
		})
	}
}

// TestSetupWithAdditionalRulesIPv6_IsNoop verifies the backward-compat stub is a no-op.
func TestSetupWithAdditionalRulesIPv6_IsNoop(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		ProxyInboundPort: 20000,
		NftablesProvider: &fakeNftablesProvider{},
	}
	err := SetupWithAdditionalRulesIPv6(cfg, nil, true)
	require.NoError(t, err)
	require.Empty(t, cfg.NftablesProvider.Rules(),
		"SetupWithAdditionalRulesIPv6 must be a no-op: inet family covers IPv6")

	err = SetupWithAdditionalRulesIPv6(cfg, nil, false)
	require.NoError(t, err)
	require.Empty(t, cfg.NftablesProvider.Rules())
}

// TestSetup_DelegatesToSetupWithAdditionalRules verifies Setup produces the same rules as SetupWithAdditionalRules.
func TestSetup_DelegatesToSetupWithAdditionalRules(t *testing.T) {
	cfgA := Config{
		ProxyUserID:       "42",
		ProxyInboundPort:  20000,
		ProxyOutboundPort: 21000,
		NftablesProvider:  &fakeNftablesProvider{},
	}
	cfgB := Config{
		ProxyUserID:       "42",
		ProxyInboundPort:  20000,
		ProxyOutboundPort: 21000,
		NftablesProvider:  &fakeNftablesProvider{},
	}

	require.NoError(t, Setup(cfgA, false))
	require.NoError(t, SetupWithAdditionalRules(cfgB, nil, false))
	require.Equal(t, cfgB.NftablesProvider.Rules(), cfgA.NftablesProvider.Rules())
}

// TestSetup_ReturnsVerifyDualStackConfigError verifies Setup propagates verifyDualStackConfig errors.
func TestSetup_ReturnsVerifyDualStackConfigError(t *testing.T) {
	cases := []struct {
		name      string
		cfg       Config
		dualStack bool
		expErr    string
	}{
		{
			name: "dualStack=true with IPv4 DNS IP",
			cfg: Config{
				ProxyUserID:      "1",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "192.168.1.1",
				NftablesProvider: &fakeNftablesProvider{},
			},
			dualStack: true,
			expErr:    "for dual stack ipv6 consulDNSIP required",
		},
		{
			name: "dualStack=false with IPv6 DNS IP",
			cfg: Config{
				ProxyUserID:      "1",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "2001:db8::1",
				NftablesProvider: &fakeNftablesProvider{},
			},
			dualStack: false,
			expErr:    "for non dual stack setup ipv4 consulDNSIP required",
		},
		{
			name: "invalid DNS IP",
			cfg: Config{
				ProxyUserID:      "1",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "bad-ip",
				NftablesProvider: &fakeNftablesProvider{},
			},
			dualStack: false,
			expErr:    "unable to parse consulDNSIP",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Setup(c.cfg, c.dualStack)
			require.EqualError(t, err, c.expErr)
			require.Empty(t, c.cfg.NftablesProvider.Rules())
		})
	}
}

// TestSetup_IPv4_Dualstack_IPv6DNSRedirect verifies DNS DNAT uses ip6 keyword for an IPv6 ConsulDNSIP.
func TestSetup_IPv4_Dualstack_IPv6DNSRedirect(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		ProxyInboundPort: 20000,
		ConsulDNSIP:      "2001:db8::68",
		NftablesProvider: &fakeNftablesProvider{},
	}

	err := SetupWithAdditionalRules(cfg, nil, true)
	require.NoError(t, err)

	rules := cfg.NftablesProvider.Rules()
	require.Contains(t, rules,
		"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT udp dport 53 dnat ip6 to 2001:db8::68")
	require.Contains(t, rules,
		"nft add rule inet consul_tproxy CONSUL_DNS_REDIRECT tcp dport 53 dnat ip6 to 2001:db8::68")
	require.Contains(t, rules,
		"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT udp dport 53 jump CONSUL_DNS_REDIRECT")
	require.Contains(t, rules,
		"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT tcp dport 53 jump CONSUL_DNS_REDIRECT")
}

// TestSetup_CombinedExclusions verifies all four exclusion types work together.
func TestSetup_CombinedExclusions(t *testing.T) {
	cfg := Config{
		ProxyUserID:          "123",
		ProxyInboundPort:     20000,
		ProxyOutboundPort:    21000,
		ExcludeInboundPorts:  []string{"8080"},
		ExcludeOutboundPorts: []string{"9090"},
		ExcludeOutboundCIDRs: []string{"10.10.0.0/16"},
		ExcludeUIDs:          []string{"999"},
		NftablesProvider:     &fakeNftablesProvider{},
	}

	err := SetupWithAdditionalRules(cfg, nil, false)
	require.NoError(t, err)

	rules := cfg.NftablesProvider.Rules()
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 8080 return")
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT tcp dport 9090 return")
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 10.10.0.0/16 return")
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 999 return")
}

// TestSetup_NilProviderUsesDefaultExecutor ensures a nil provider is assigned without panicking.
func TestSetup_NilProviderUsesDefaultExecutor(t *testing.T) {
	cfg := Config{
		ProxyUserID: "", // invalid — error returned before nft is exec'd
	}
	err := SetupWithAdditionalRules(cfg, nil, false)
	require.EqualError(t, err, "ProxyUserID is required to set up traffic redirection")
}

// TestSetup_ReapplyClearsRules ensures a second call clears and repopulates rather than appending.
func TestSetup_ReapplyClearsRules(t *testing.T) {
	provider := &fakeNftablesProvider{}
	cfg := Config{
		ProxyUserID:      "1",
		ProxyInboundPort: 20000,
		NftablesProvider: provider,
	}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))
	firstCount := len(provider.Rules())
	require.Greater(t, firstCount, 0)

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))
	require.Equal(t, firstCount, len(provider.Rules()), "second call must not double-accumulate rules")
}

// TestSetup_ReapplyWithChangedConfig ensures that reapplying with a different
// configuration (e.g. after exclusions are added/removed between agent
// restarts) fully replaces the previous rule set rather than layering the new
// rules on top of stale ones. A naive implementation that forgets to clear
// state before rebuilding could otherwise leave a since-removed exclusion's
// "return" rule active alongside the newly configured ones.
func TestSetup_ReapplyWithChangedConfig(t *testing.T) {
	provider := &fakeNftablesProvider{}
	cfg := Config{
		ProxyUserID:          "1",
		ProxyInboundPort:     20000,
		ExcludeUIDs:          []string{"999"},
		ExcludeOutboundCIDRs: []string{"10.10.0.0/16"},
		NftablesProvider:     provider,
	}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))
	rules := provider.Rules()
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 999 return")
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 10.10.0.0/16 return")

	// Reapply with a changed configuration: the old UID exclusion is dropped
	// and replaced with a different UID and a new port exclusion.
	cfg.ExcludeUIDs = []string{"111"}
	cfg.ExcludeOutboundCIDRs = nil
	cfg.ExcludeInboundPorts = []string{"8080"}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))
	rules = provider.Rules()

	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 111 return")
	require.Contains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 8080 return")
	require.NotContains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT skuid 999 return",
		"stale UID exclusion from the previous config must not survive a reapply")
	require.NotContains(t, rules, "nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT ip daddr 10.10.0.0/16 return",
		"stale CIDR exclusion removed from the config must not survive a reapply")
}

// TestSetup_FailedApply_ErrorPropagatesAndRetrySucceeds simulates a failed
// nft batch (e.g. the kernel/nft parser rejecting the generated script) and
// verifies that: (1) the error is surfaced to the caller rather than
// swallowed, so Setup is never mistaken for a success, and (2) the failure
// doesn't leave the provider in a state that blocks a subsequent, successful
// retry — mirroring nft's own transactional all-or-nothing semantics, where a
// rejected batch leaves the previously active table untouched and retryable.
func TestSetup_FailedApply_ErrorPropagatesAndRetrySucceeds(t *testing.T) {
	provider := &fakeNftablesProvider{applyErr: errors.New("simulated nft batch failure")}
	cfg := Config{
		ProxyUserID:      "1",
		ProxyInboundPort: 20000,
		NftablesProvider: provider,
	}

	err := SetupWithAdditionalRules(cfg, nil, false)
	require.EqualError(t, err, "simulated nft batch failure")
	require.Equal(t, 1, provider.applyCalls)
	require.NotEmpty(t, provider.Rules(), "the rejected batch's staged rules remain visible for diagnostics")

	// Clear the simulated failure and retry: the earlier failed attempt must
	// not prevent a clean, successful reapply.
	provider.applyErr = nil
	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))
	require.Equal(t, 2, provider.applyCalls)
	require.Contains(t, provider.Rules(),
		"nft add rule inet consul_tproxy CONSUL_PROXY_IN_REDIRECT meta l4proto tcp redirect to :20000")
}

// TestSetup_DefaultOutboundPort verifies ProxyOutboundPort=0 falls back to DefaultTProxyOutboundPort (15001).
func TestSetup_DefaultOutboundPort(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "1",
		ProxyInboundPort: 20000,
		NftablesProvider: &fakeNftablesProvider{},
	}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))

	require.Contains(t, cfg.NftablesProvider.Rules(),
		"nft add rule inet consul_tproxy CONSUL_PROXY_REDIRECT meta l4proto tcp redirect to :15001")
}

// TestNormalizePortRange covers normalizePortRange for single ports, ranges, and edge cases.
func TestNormalizePortRange(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		// single ports — unchanged
		{"8080", "8080"},
		{"22", "22"},
		{"0", "0"},
		// iptables colon-range → nftables dash-range
		{"8080:9000", "8080-9000"},
		{"1000:2000", "1000-2000"},
		{"0:65535", "0-65535"},
		// open-ended ranges — iptables' omitted-bound semantics preserved
		{":19000", "0-19000"},
		{"18000:", "18000-65535"},
		{":", "0-65535"},
		// already dash-separated — unchanged
		{"8080-9000", "8080-9000"},
		// TCP service names — resolved to their well-known numeric port via
		// /etc/services, matching iptables' --dport behavior.
		{"ssh", "22"},
		{"http", "80"},
		{"http:https", "80-443"},
		{"ssh:", "22-65535"},
		{":ssh", "0-22"},
		{"ssh-https", "22-443"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := normalizePortRange(c.input)
			require.NoError(t, err)
			require.Equal(t, c.expected, got)
		})
	}
}

// TestNormalizePortRange_HyphenatedServiceName verifies that a whole TCP
// service name containing a hyphen (e.g. "http-alt") is resolved as a single
// service, not misread as a range between two shorter, possibly-unresolvable
// names (e.g. "http" and "alt", where "alt" is not a registered service).
// The expected port is derived from the resolver itself rather than
// hardcoded, since the exact assignment for "http-alt" varies by host.
func TestNormalizePortRange_HyphenatedServiceName(t *testing.T) {
	wantPort, err := net.LookupPort("tcp", "http-alt")
	if err != nil {
		t.Skipf("skipping: %q is not a resolvable TCP service name on this host", "http-alt")
	}

	got, err := normalizePortRange("http-alt")
	require.NoError(t, err)
	require.NotContains(t, got, "-", "a whole service name must resolve to a single port, not a range")
	require.Equal(t, strconv.Itoa(wantPort), got)
}

// TestNormalizePortRange_InvalidInput covers inputs normalizePortRange must
// reject: unresolvable names, out-of-range numbers, and empty endpoints.
func TestNormalizePortRange_InvalidInput(t *testing.T) {
	cases := []string{
		"",                        // empty
		"not-a-real-service-name", // not numeric, not a known service name
		"8080:not-a-service",      // non-resolvable upper bound
		"-1",                      // negative
		"70000",                   // above 65535
		"8080-",                   // dash range with empty upper bound
		"-9000",                   // dash range with empty lower bound
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := normalizePortRange(input)
			require.Error(t, err)
		})
	}
}

// TestSetup_PortRangeNormalization verifies that iptables-style port ranges
// ("8080:9000") in ExcludeInboundPorts and ExcludeOutboundPorts are
// normalised to nftables syntax ("8080-9000") in the generated rules.
func TestSetup_PortRangeNormalization(t *testing.T) {
	cfg := Config{
		ProxyUserID:          "123",
		ProxyInboundPort:     20000,
		ProxyOutboundPort:    21000,
		ExcludeInboundPorts:  []string{"8080:9000"},
		ExcludeOutboundPorts: []string{"1000:2000"},
		NftablesProvider:     &fakeNftablesProvider{},
	}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))

	rules := cfg.NftablesProvider.Rules()
	// Colon ranges must be converted to dash ranges.
	require.Contains(t, rules,
		"nft insert rule inet consul_tproxy CONSUL_PROXY_INBOUND tcp dport 8080-9000 return")
	require.Contains(t, rules,
		"nft insert rule inet consul_tproxy CONSUL_PROXY_OUTPUT tcp dport 1000-2000 return")
	// Colon syntax must not appear in any rule.
	for _, r := range rules {
		require.NotContains(t, r, "8080:9000", "colon range must be normalised to dash")
		require.NotContains(t, r, "1000:2000", "colon range must be normalised to dash")
	}
}

// TestSetup_IPv4Only_IPv6EarlyReturnRulesPresent verifies that when dualStack=false,
// IPv6 early-return rules are added to both base chains so IPv6 is not intercepted.
func TestSetup_IPv4Only_IPv6EarlyReturnRulesPresent(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		ProxyInboundPort: 20000,
		NftablesProvider: &fakeNftablesProvider{},
	}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, false))

	rules := cfg.NftablesProvider.Rules()
	// The presence of these return rules is what PREVENTS IPv6 interception.
	require.Contains(t, rules,
		"nft add rule inet consul_tproxy CONSUL_NAT_OUTPUT meta nfproto ipv6 return")
	require.Contains(t, rules,
		"nft add rule inet consul_tproxy CONSUL_NAT_PREROUTING meta nfproto ipv6 return")
}

// TestSetup_DualStack_NoIPv6EarlyReturn verifies that when dualStack=true,
// no "meta nfproto ipv6 return" rules are added, so IPv6 traffic passes through
// and is intercepted by the proxy as intended.
func TestSetup_DualStack_NoIPv6EarlyReturn(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		ProxyInboundPort: 20000,
		NftablesProvider: &fakeNftablesProvider{},
	}

	require.NoError(t, SetupWithAdditionalRules(cfg, nil, true))

	rules := cfg.NftablesProvider.Rules()
	for _, r := range rules {
		require.NotContains(t, r, "nfproto ipv6 return",
			"dual-stack mode must not add IPv6 early-return rules")
	}
}
