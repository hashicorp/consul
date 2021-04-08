package iptables

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
)

const (
	// Chain to intercept inbound traffic
	ProxyInboundChain = "CONSUL_PROXY_INBOUND"

	// Chain to redirect inbound traffic to the proxy
	ProxyInboundRedirectChain = "CONSUL_PROXY_IN_REDIRECT"

	// Chain to intercept outbound traffic
	ProxyOutputChain = "CONSUL_PROXY_OUTPUT"

	// Chain to redirect outbound traffic to the proxy
	ProxyOutputRedirectChain = "CONSUL_PROXY_REDIRECT"
)

// Config is used to configure which traffic interception and redirection
// rules should be applied with the iptables commands.
type Config struct {
	// ProxyUserID is the user ID of the proxy process.
	ProxyUserID string

	// ProxyInboundPort is the port of the proxy's inbound listener.
	ProxyInboundPort int

	// ProxyInboundPort is the port of the proxy's outbound listener.
	ProxyOutboundPort int

	// IptablesProvider is the Provider that will apply iptables rules.
	IptablesProvider Provider
}

// Provider is an interface for executing iptables rules.
type Provider interface {
	// AddRule adds a rule without executing it.
	AddRule(name string, args ...string)
	// ApplyRules executes rules that have been added via AddRule.
	// This operation is currently not atomic, and if there's an error applying rules,
	// you may be left in a state where partial rules were applied.
	ApplyRules() error
	// Rules returns the list of rules that have been added but not applied yet.
	Rules() []string
}

// iptablesExecutor implements IptablesProvider using exec.Cmd.
type iptablesExecutor struct {
	commands []*exec.Cmd
}

func (i *iptablesExecutor) AddRule(name string, args ...string) {
	i.commands = append(i.commands, exec.Command(name, args...))
}

func (i *iptablesExecutor) ApplyRules() error {
	_, err := exec.LookPath("iptables")
	if err != nil {
		return err
	}

	for _, cmd := range i.commands {
		err := cmd.Run()
		if err != nil {
			output, err := cmd.CombinedOutput()

			if err != nil {
				return fmt.Errorf("failed to run command: %s, err: %v", cmd.String(), err)
			}

			return fmt.Errorf("failed to run command: %s, err: %v, output: %s", cmd.String(), err, string(output))
		}
	}

	return nil
}

func (i *iptablesExecutor) Rules() []string {
	var rules []string
	for _, cmd := range i.commands {
		rules = append(rules, cmd.String())
	}

	return rules
}

// Setup will set up iptables interception and redirection rules
// based on the configuration provided in cfg.
// This implementation was inspired by
// https://github.com/openservicemesh/osm/blob/650a1a1dcf081ae90825f3b5dba6f30a0e532725/pkg/injector/iptables.go
func Setup(cfg Config) error {
	if cfg.IptablesProvider == nil {
		cfg.IptablesProvider = &iptablesExecutor{}
	}

	err := validateConfig(cfg)
	if err != nil {
		return err
	}

	// Create chains we will use for redirection.
	chains := []string{ProxyInboundChain, ProxyInboundRedirectChain, ProxyOutputChain, ProxyOutputRedirectChain}
	for _, chain := range chains {
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-N", chain)
	}

	// Configure outbound rules.
	{
		// Redirects outbound TCP traffic hitting PROXY_REDIRECT chain to Envoy's outbound listener port.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", ProxyOutputRedirectChain, "-p", "tcp", "-j", "REDIRECT", "--to-port", strconv.Itoa(cfg.ProxyOutboundPort))

		// For outbound TCP traffic jump from OUTPUT chain to PROXY_OUTPUT chain.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "-j", ProxyOutputChain)

		// Don't redirect Envoy traffic back to itself, return it to the next chain for processing.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", ProxyOutputChain, "-m", "owner", "--uid-owner", cfg.ProxyUserID, "-j", "RETURN")

		// Skip localhost traffic, doesn't need to be routed via the proxy.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", ProxyOutputChain, "-d", "127.0.0.1/32", "-j", "RETURN")

		// Redirect remaining outbound traffic to Envoy.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", ProxyOutputChain, "-j", ProxyOutputRedirectChain)
	}

	// Configure inbound rules.
	{
		// Redirects inbound TCP traffic hitting the PROXY_IN_REDIRECT chain to Envoy's inbound listener port.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", ProxyInboundRedirectChain, "-p", "tcp", "-j", "REDIRECT", "--to-port", strconv.Itoa(cfg.ProxyInboundPort))

		// For inbound traffic jump from PREROUTING chain to PROXY_INBOUND chain.
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "-j", ProxyInboundChain)

		// Redirect remaining inbound traffic to Envoy.
		// todo: figure out why does inbound have tcp protocol but outbound does not
		cfg.IptablesProvider.AddRule("iptables", "-t", "nat", "-A", ProxyInboundChain, "-p", "tcp", "-j", ProxyInboundRedirectChain)
	}

	return cfg.IptablesProvider.ApplyRules()
}

func validateConfig(cfg Config) error {
	if cfg.ProxyUserID == "" {
		return errors.New("ProxyUserID is required to set up traffic redirection")
	}

	if cfg.ProxyOutboundPort == 0 {
		return errors.New("ProxyOutboundPort is required to set up traffic redirection")
	}

	if cfg.ProxyInboundPort == 0 {
		return errors.New("ProxyInboundPort is required to set up traffic redirection")
	}

	return nil
}
