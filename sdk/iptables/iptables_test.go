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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"iptables -t nat -A CONSUL_DNS_REDIRECT -p udp --dport 53 -j DNAT --to-destination 10.0.34.16",
				"iptables -t nat -A CONSUL_DNS_REDIRECT -p tcp --dport 53 -j DNAT --to-destination 10.0.34.16",
				"iptables -t nat -A OUTPUT -p udp --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"iptables -t nat -A CONSUL_DNS_REDIRECT -p udp -d 127.0.0.1 --dport 53 -j DNAT --to-destination 127.0.0.1:8600",
				"iptables -t nat -A CONSUL_DNS_REDIRECT -p tcp -d 127.0.0.1 --dport 53 -j DNAT --to-destination 127.0.0.1:8600",
				"iptables -t nat -A OUTPUT -p udp -d 127.0.0.1 --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -d 127.0.0.1 --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"iptables -t nat -A CONSUL_DNS_REDIRECT -p udp -d 10.0.34.16 --dport 53 -j DNAT --to-destination 10.0.34.16:8600",
				"iptables -t nat -A CONSUL_DNS_REDIRECT -p tcp -d 10.0.34.16 --dport 53 -j DNAT --to-destination 10.0.34.16:8600",
				"iptables -t nat -A OUTPUT -p udp -d 10.0.34.16 --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -d 10.0.34.16 --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_INBOUND -p tcp --dport 22000 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_INBOUND -p tcp --dport 22500 -j RETURN",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -p tcp --dport 22000 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -p tcp --dport 22500 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -d 1.1.1.1 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -d 2.2.2.2/24 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 456 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 789 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				{"iptables", "-t", "nat", "--policy", "POSTROUTING", "ACCEPT"},
				{"iptables", "-t", "nat", "--policy", "PREROUTING", "ACCEPT"},
			},
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 456 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 789 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat --policy POSTROUTING ACCEPT",
				"iptables -t nat --policy PREROUTING ACCEPT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				// "iptables -t nat -A CONSUL_DNS_REDIRECT -p udp --dport 53 -j DNAT --to-destination 10.0.34.16",
				// "iptables -t nat -A CONSUL_DNS_REDIRECT -p tcp --dport 53 -j DNAT --to-destination 10.0.34.16",
				// "iptables -t nat -A OUTPUT -p udp --dport 53 -j CONSUL_DNS_REDIRECT",
				// "iptables -t nat -A OUTPUT -p tcp --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				// "iptables -t nat -A CONSUL_DNS_REDIRECT -p udp -d 127.0.0.1 --dport 53 -j DNAT --to-destination 127.0.0.1:8600",
				// "iptables -t nat -A CONSUL_DNS_REDIRECT -p tcp -d 127.0.0.1 --dport 53 -j DNAT --to-destination 127.0.0.1:8600",
				// "iptables -t nat -A OUTPUT -p udp -d 127.0.0.1 --dport 53 -j CONSUL_DNS_REDIRECT",
				// "iptables -t nat -A OUTPUT -p tcp -d 127.0.0.1 --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				// "iptables -t nat -A CONSUL_DNS_REDIRECT -p udp -d 10.0.34.16 --dport 53 -j DNAT --to-destination 10.0.34.16:8600",
				// "iptables -t nat -A CONSUL_DNS_REDIRECT -p tcp -d 10.0.34.16 --dport 53 -j DNAT --to-destination 10.0.34.16:8600",
				// "iptables -t nat -A OUTPUT -p udp -d 10.0.34.16 --dport 53 -j CONSUL_DNS_REDIRECT",
				// "iptables -t nat -A OUTPUT -p tcp -d 10.0.34.16 --dport 53 -j CONSUL_DNS_REDIRECT",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_INBOUND -p tcp --dport 22000 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_INBOUND -p tcp --dport 22500 -j RETURN",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -p tcp --dport 22000 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -p tcp --dport 22500 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -d 1.1.1.1 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -d 2.2.2.2/24 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 456 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 789 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				{"iptables", "-t", "nat", "--policy", "POSTROUTING", "ACCEPT"},
				{"iptables", "-t", "nat", "--policy", "PREROUTING", "ACCEPT"},
			},
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
				"iptables -t nat -N CONSUL_DNS_REDIRECT",
				"iptables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"iptables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 456 -j RETURN",
				"iptables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 789 -j RETURN",
				"iptables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"iptables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"iptables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat --policy POSTROUTING ACCEPT",
				"iptables -t nat --policy PREROUTING ACCEPT",
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

// zara zara fix me fix me fix me
func TestSetup_IPv6(t *testing.T) {
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"ip6tables -t nat -A CONSUL_DNS_REDIRECT -p udp --dport 53 -j DNAT --to-destination 10.0.34.16",
				"ip6tables -t nat -A CONSUL_DNS_REDIRECT -p tcp --dport 53 -j DNAT --to-destination 10.0.34.16",
				"ip6tables -t nat -A OUTPUT -p udp --dport 53 -j CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A OUTPUT -p tcp --dport 53 -j CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"ip6tables -t nat -A CONSUL_DNS_REDIRECT -p udp -d ::1 --dport 53 -j DNAT --to-destination [::1]:8600",
				"ip6tables -t nat -A CONSUL_DNS_REDIRECT -p tcp -d ::1 --dport 53 -j DNAT --to-destination [::1]:8600",
				"ip6tables -t nat -A OUTPUT -p udp -d ::1 --dport 53 -j CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A OUTPUT -p tcp -d ::1 --dport 53 -j CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
			"Consul DNS IP and port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				ConsulDNSIP:      "2406:da1a:23:5e05:e1c6::5",
				ConsulDNSPort:    8600,
				IptablesProvider: &fakeIptablesProvider{},
			},
			nil,
			[]string{
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
				"ip6tables -t nat -A CONSUL_DNS_REDIRECT -p udp -d 2406:da1a:23:5e05:e1c6::5 --dport 53 -j DNAT --to-destination [2406:da1a:23:5e05:e1c6::5]:8600",
				"ip6tables -t nat -A CONSUL_DNS_REDIRECT -p tcp -d 2406:da1a:23:5e05:e1c6::5 --dport 53 -j DNAT --to-destination [2406:da1a:23:5e05:e1c6::5]:8600",
				"ip6tables -t nat -A OUTPUT -p udp -d 2406:da1a:23:5e05:e1c6::5 --dport 53 -j CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A OUTPUT -p tcp -d 2406:da1a:23:5e05:e1c6::5 --dport 53 -j CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -I CONSUL_PROXY_INBOUND -p tcp --dport 22000 -j RETURN",
				"ip6tables -t nat -I CONSUL_PROXY_INBOUND -p tcp --dport 22500 -j RETURN",
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -p tcp --dport 22000 -j RETURN",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -p tcp --dport 22500 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
			},
		},
		{
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -d 2406:da1a:23:5e05:e1c6::5 -j RETURN",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -d 2406:da1a:23:5e05:e1c6::ffff/24 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 456 -j RETURN",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 789 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
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
				{"ip6tables", "-t", "nat", "--policy", "POSTROUTING", "ACCEPT"},
				{"ip6tables", "-t", "nat", "--policy", "PREROUTING", "ACCEPT"},
			},
			[]string{
				"ip6tables -t nat -N CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat -N CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -N CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -N CONSUL_DNS_REDIRECT",
				"ip6tables -t nat -A CONSUL_PROXY_REDIRECT -p tcp -j REDIRECT --to-port 21000",
				"ip6tables -t nat -A OUTPUT -p tcp -j CONSUL_PROXY_OUTPUT",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -d ::1/128 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_OUTPUT -j CONSUL_PROXY_REDIRECT",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 456 -j RETURN",
				"ip6tables -t nat -I CONSUL_PROXY_OUTPUT -m owner --uid-owner 789 -j RETURN",
				"ip6tables -t nat -A CONSUL_PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 20000",
				"ip6tables -t nat -A PREROUTING -p tcp -j CONSUL_PROXY_INBOUND",
				"ip6tables -t nat -A CONSUL_PROXY_INBOUND -p tcp -j CONSUL_PROXY_IN_REDIRECT",
				"ip6tables -t nat --policy POSTROUTING ACCEPT",
				"ip6tables -t nat --policy PREROUTING ACCEPT",
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

			err := SetupWithAdditionalRulesIPv6(c.cfg, fn, true)
			require.NoError(t, err)
			require.Equal(t, c.expectedRules, c.cfg.IptablesProvider.Rules())
		})
	}
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
				IptablesProvider: &iptablesExecutor{},
			},
			"ProxyUserID is required to set up traffic redirection",
		},
		{
			"no proxy inbound port",
			Config{
				ProxyUserID:       "123",
				ProxyOutboundPort: 21000,
				IptablesProvider:  &iptablesExecutor{},
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
