package iptables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		IptablesProvider: &fakeIptablesProvider{},
	}

	expectedRules := []string{
		"iptables -t nat -N PROXY_INBOUND",
		"iptables -t nat -N PROXY_IN_REDIRECT",
		"iptables -t nat -N PROXY_OUTPUT",
		"iptables -t nat -N PROXY_REDIRECT",
		"iptables -t nat -A PROXY_REDIRECT -p tcp -j REDIRECT --to-port 15001",
		"iptables -t nat -A OUTPUT -p tcp -j PROXY_OUTPUT",
		"iptables -t nat -A PROXY_OUTPUT -m owner --uid-owner 123 -j RETURN",
		"iptables -t nat -A PROXY_OUTPUT -d 127.0.0.1/32 -j RETURN",
		"iptables -t nat -A PROXY_OUTPUT -j PROXY_REDIRECT",
		"iptables -t nat -A PROXY_IN_REDIRECT -p tcp -j REDIRECT --to-port 15006",
		"iptables -t nat -A PREROUTING -p tcp -j PROXY_INBOUND",
		"iptables -t nat -A PROXY_INBOUND -p tcp -j PROXY_IN_REDIRECT",
	}

	err := Setup(cfg)
	require.NoError(t, err)
	require.Equal(t, expectedRules, cfg.IptablesProvider.Rules())
}

func TestSetup_errors(t *testing.T) {
	cfg := Config{
		ProxyUserID:      "123",
		IptablesProvider: &iptablesExecutor{},
	}

	err := Setup(cfg)
	require.EqualError(t, err, "exec: \"iptables\": executable file not found in $PATH")
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
