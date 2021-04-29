package iptables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	cases := []struct {
		name          string
		cfg           Config
		expectedRules []string
	}{
		{
			"no proxy outbound port provided",
			Config{
				ProxyUserID:      "123",
				ProxyInboundPort: 20000,
				IptablesProvider: &fakeIptablesProvider{},
			},
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
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
			"proxy outbound port is provided",
			Config{
				ProxyUserID:       "123",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
				IptablesProvider:  &fakeIptablesProvider{},
			},
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
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
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
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
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
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
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
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
			[]string{
				"iptables -t nat -N CONSUL_PROXY_INBOUND",
				"iptables -t nat -N CONSUL_PROXY_IN_REDIRECT",
				"iptables -t nat -N CONSUL_PROXY_OUTPUT",
				"iptables -t nat -N CONSUL_PROXY_REDIRECT",
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Setup(c.cfg)
			require.NoError(t, err)
			require.Equal(t, c.expectedRules, c.cfg.IptablesProvider.Rules())
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
			err := Setup(c.cfg)
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

func (f *fakeIptablesProvider) ApplyRules() error {
	return nil
}

func (f *fakeIptablesProvider) Rules() []string {
	return f.rules
}
