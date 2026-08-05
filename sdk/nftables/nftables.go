// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nftables

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	// ProxyInboundChain is the chain to intercept inbound traffic.
	ProxyInboundChain = "CONSUL_PROXY_INBOUND"

	// ProxyInboundRedirectChain is the chain to redirect inbound traffic to the proxy.
	ProxyInboundRedirectChain = "CONSUL_PROXY_IN_REDIRECT"

	// ProxyOutputChain is the chain to intercept outbound traffic.
	ProxyOutputChain = "CONSUL_PROXY_OUTPUT"

	// ProxyOutputRedirectChain is the chain to redirect outbound traffic to the proxy
	ProxyOutputRedirectChain = "CONSUL_PROXY_REDIRECT"

	// DNSChain is the chain to redirect outbound DNS traffic to Consul DNS.
	DNSChain = "CONSUL_DNS_REDIRECT"

	// consulNATOutputChain is the nftables base chain hooked into the output path.
	consulNATOutputChain = "CONSUL_NAT_OUTPUT"

	// consulNATPreRoutingChain is the nftables base chain hooked into the prerouting path.
	consulNATPreRoutingChain = "CONSUL_NAT_PREROUTING"

	DefaultTProxyOutboundPort = 15001
)

// Config is used to configure which traffic interception and redirection
// rules should be applied with the nftables commands.
type Config struct {
	// ConsulDNSIP is the IP for Consul DNS to direct DNS queries to.
	ConsulDNSIP string

	// ConsulDNSPort is the port for Consul DNS to direct DNS queries to.
	ConsulDNSPort int

	// ProxyUserID is the user ID of the proxy process.
	ProxyUserID string

	// ProxyInboundPort is the port of the proxy's inbound listener.
	ProxyInboundPort int

	// ProxyInboundPort is the port of the proxy's outbound listener.
	ProxyOutboundPort int

	// ExcludeInboundPorts is the list of ports that should be excluded
	// from inbound traffic redirection.
	ExcludeInboundPorts []string

	// ExcludeOutboundPorts is the list of ports that should be excluded
	// from outbound traffic redirection.
	ExcludeOutboundPorts []string

	// ExcludeOutboundCIDRs is the list of IP CIDRs that should be excluded
	// from outbound traffic redirection.
	ExcludeOutboundCIDRs []string

	// ExcludeUIDs is the list of additional user IDs to exclude
	// from traffic redirection.
	ExcludeUIDs []string

	// NetNS is the network namespace where the traffic redirection rules
	// should be applied. This must be a path to the network namespace,
	// e.g. /var/run/netns/foo.
	NetNS string

	// NftablesProvider is the Provider that will apply nftables rules.
	NftablesProvider Provider
}

// AdditionalRulesFn can be implemented by the caller to
// add environment specific rules (like ECS) that needs to
// be executed for traffic redirection to work properly.
//
// This gets called by the Setup function after all the
// first class nftables rules are added. The implemented
// function should only call the `AddRule` and optionally
// the `Rules` method of the provider.
type AdditionalRulesFn func(nftablesProvider Provider)

// Provider is an interface for executing nftables rules.
type Provider interface {
	// AddRule adds a rule without executing it.
	AddRule(name string, args ...string)
	// ApplyRules executes rules that have been added via AddRule.
	// The current executor applies the accumulated nftables rules atomically.
	// ApplyRules should not be called twice on the same instance in order to avoid
	// duplicate rule application.
	ApplyRules(string) error
	// Rules returns the list of rules that have been added (including those not yet
	// applied).
	Rules() []string

	// ClearAllRules clears all rules that are added
	ClearAllRules()
}

func verifyDualStackConfig(cfg Config, dualStack bool) error {
	if dualStack {
		if cfg.ConsulDNSIP != "" {
			ip := net.ParseIP(cfg.ConsulDNSIP)
			if ip == nil {
				return fmt.Errorf("unable to parse consulDNSIP")
			}
			if ip.To4() != nil {
				return fmt.Errorf("for dual stack ipv6 consulDNSIP required")
			}
		}
	} else {
		if cfg.ConsulDNSIP != "" {
			ip := net.ParseIP(cfg.ConsulDNSIP)
			if ip == nil {
				return fmt.Errorf("unable to parse consulDNSIP")
			}
			if ip.To4() == nil {
				return fmt.Errorf("for non dual stack setup ipv4 consulDNSIP required")
			}
		}
	}
	return nil
}

// Setup will set up nftables interception and redirection rules
// based on the configuration provided in cfg.
// The inet address family is used so that a single rule set covers both
// IPv4 and IPv6 traffic — no separate ip6tables pass is required.
func Setup(cfg Config, dualStack bool) error {

	if err := verifyDualStackConfig(cfg, dualStack); err != nil {
		return err
	}

	return SetupWithAdditionalRules(cfg, nil, dualStack)
}

// SetupWithAdditionalRules will set up nftables interception and redirection rules
// based on the configuration provided in cfg. The additionalRulesFn will be applied
// after the normal set of rules. This implementation was inspired by OSM's traffic
// redirection rule setup.
//
// The nftables inet family is used so a single rule set covers both IPv4 and IPv6;
// there is no longer a need for a separate IPv6-only pass.
func SetupWithAdditionalRules(cfg Config, additionalRulesFn AdditionalRulesFn, dualStack bool) error {

	if cfg.NftablesProvider == nil {
		cfg.NftablesProvider = &nftablesExecutor{cfg: cfg}
	} else {
		cfg.NftablesProvider.ClearAllRules()
	}

	err := validateConfig(cfg)
	if err != nil {
		return err
	}

	// Set the default outbound port if it's not already set.
	if cfg.ProxyOutboundPort == 0 {
		cfg.ProxyOutboundPort = DefaultTProxyOutboundPort
	}

	// Create the inet nat table. The inet family processes both IPv4 and IPv6.
	cfg.NftablesProvider.AddRule("nft", "add", "table", "inet", "nat")

	// Create regular (non-hook) chains used for traffic redirection.
	chains := []string{ProxyInboundChain, ProxyInboundRedirectChain, ProxyOutputChain, ProxyOutputRedirectChain, DNSChain}
	for _, chain := range chains {
		cfg.NftablesProvider.AddRule("nft", "add", "chain", "inet", "nat", chain)
	}

	// Create base chains that hook into the kernel packet-processing pipeline,
	// replacing the traditional PREROUTING and OUTPUT entry points.
	cfg.NftablesProvider.AddRule("nft", "add", "chain", "inet", "nat", consulNATOutputChain,
		"{ type nat hook output priority -100 ; }")
	cfg.NftablesProvider.AddRule("nft", "add", "chain", "inet", "nat", consulNATPreRoutingChain,
		"{ type nat hook prerouting priority -100 ; }")

	// Configure outbound rules.
	{
		// Redirect all TCP traffic hitting PROXY_REDIRECT chain to Envoy's outbound listener.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyOutputRedirectChain,
			"meta", "l4proto", "tcp", "redirect", "to", ":"+strconv.Itoa(cfg.ProxyOutboundPort))

		// DNS redirection rules. With the inet family these cover both IPv4 and IPv6;
		// a separate IPv4-only guard is no longer needed.
		if cfg.ConsulDNSIP != "" && cfg.ConsulDNSPort == 0 {
			// In the inet address family, dnat requires an explicit ip/ip6 keyword
			// alongside the destination address to identify the protocol family.
			dnsIPKw := ipFamilyKeyword(cfg.ConsulDNSIP)
			// Direct all DNS traffic in the DNS chain to the Consul DNS Service IP.
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", DNSChain,
				"udp", "dport", "53", "dnat", dnsIPKw, "to", cfg.ConsulDNSIP)
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", DNSChain,
				"tcp", "dport", "53", "dnat", dnsIPKw, "to", cfg.ConsulDNSIP)

			// Jump outbound port-53 traffic into the DNS chain.
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", consulNATOutputChain,
				"udp", "dport", "53", "jump", DNSChain)
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", consulNATOutputChain,
				"tcp", "dport", "53", "jump", DNSChain)
		} else if cfg.ConsulDNSPort != 0 {
			// Determine the DNS IP and IP-family keyword for address matching.
			consulDNSIP := "127.0.0.1"
			if dualStack {
				consulDNSIP = "::1"
			}
			if cfg.ConsulDNSIP != "" {
				consulDNSIP = cfg.ConsulDNSIP
			}
			dnsIPKw := ipFamilyKeyword(consulDNSIP)
			consulDNSHostPort := net.JoinHostPort(consulDNSIP, strconv.Itoa(cfg.ConsulDNSPort))

			// Direct DNS traffic destined for the Consul DNS IP to the right port.
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", DNSChain,
				dnsIPKw, "daddr", consulDNSIP, "udp", "dport", "53", "dnat", "to", consulDNSHostPort)
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", DNSChain,
				dnsIPKw, "daddr", consulDNSIP, "tcp", "dport", "53", "dnat", "to", consulDNSHostPort)

			// Jump outbound port-53 traffic destined for the Consul DNS IP into the DNS chain.
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", consulNATOutputChain,
				dnsIPKw, "daddr", consulDNSIP, "udp", "dport", "53", "jump", DNSChain)
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", consulNATOutputChain,
				dnsIPKw, "daddr", consulDNSIP, "tcp", "dport", "53", "jump", DNSChain)
		}

		// Jump all outbound TCP traffic from the output hook into PROXY_OUTPUT.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", consulNATOutputChain,
			"meta", "l4proto", "tcp", "jump", ProxyOutputChain)

		// Don't redirect the proxy's own traffic back to itself.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyOutputChain,
			"skuid", cfg.ProxyUserID, "return")

		// Skip localhost traffic (IPv4 and IPv6) — it doesn't need proxy routing.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyOutputChain,
			"ip", "daddr", "127.0.0.1/32", "return")
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyOutputChain,
			"ip6", "daddr", "::1/128", "return")

		// Redirect all remaining outbound traffic to Envoy.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyOutputChain,
			"jump", ProxyOutputRedirectChain)

		// insert (prepend) rules so they take precedence over the defaults above.
		for _, outboundPort := range cfg.ExcludeOutboundPorts {
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", "nat", ProxyOutputChain,
				"tcp", "dport", outboundPort, "return")
		}

		for _, outboundCIDR := range cfg.ExcludeOutboundCIDRs {
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", "nat", ProxyOutputChain,
				ipFamilyKeyword(outboundCIDR), "daddr", outboundCIDR, "return")
		}

		for _, uid := range cfg.ExcludeUIDs {
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", "nat", ProxyOutputChain,
				"skuid", uid, "return")
		}
	}

	// Configure inbound rules.
	{
		// Redirect all TCP traffic in PROXY_IN_REDIRECT to Envoy's inbound listener.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyInboundRedirectChain,
			"meta", "l4proto", "tcp", "redirect", "to", ":"+strconv.Itoa(cfg.ProxyInboundPort))

		// Jump inbound TCP traffic from the prerouting hook into PROXY_INBOUND.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", consulNATPreRoutingChain,
			"meta", "l4proto", "tcp", "jump", ProxyInboundChain)

		// Redirect remaining inbound traffic to Envoy.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", "nat", ProxyInboundChain,
			"meta", "l4proto", "tcp", "jump", ProxyInboundRedirectChain)

		for _, inboundPort := range cfg.ExcludeInboundPorts {
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", "nat", ProxyInboundChain,
				"tcp", "dport", inboundPort, "return")
		}
	}

	// Call function to add any additional rules passed on by the caller.
	if additionalRulesFn != nil {
		additionalRulesFn(cfg.NftablesProvider)
	}
	return cfg.NftablesProvider.ApplyRules("nft")
}

// SetupWithAdditionalRulesIPv6 is a no-op. With nftables the inet address
// family handles both IPv4 and IPv6 in a single SetupWithAdditionalRules call,
// so a separate IPv6 pass is no longer required.
//
// Deprecated: callers that previously invoked this for dual-stack support will
// receive full dual-stack coverage automatically from SetupWithAdditionalRules.
func SetupWithAdditionalRulesIPv6(_ Config, _ AdditionalRulesFn, _ bool) error {
	return nil
}

// ipFamilyKeyword returns "ip" for IPv4 addresses/CIDRs and "ip6" for IPv6.
// It accepts plain IP addresses ("1.2.3.4") and CIDR notation ("1.2.3.4/24").
func ipFamilyKeyword(cidrOrIP string) string {
	ipStr := cidrOrIP
	if i := strings.IndexByte(cidrOrIP, '/'); i >= 0 {
		ipStr = cidrOrIP[:i]
	}
	if ip := net.ParseIP(ipStr); ip != nil && ip.To4() != nil {
		return "ip"
	}
	return "ip6"
}

func validateConfig(cfg Config) error {
	if cfg.ProxyUserID == "" {
		return errors.New("ProxyUserID is required to set up traffic redirection")
	}

	if cfg.ProxyInboundPort == 0 {
		return errors.New("ProxyInboundPort is required to set up traffic redirection")
	}

	return nil
}
