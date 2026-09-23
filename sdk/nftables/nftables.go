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

	// tproxyTable is the nftables table used for all Consul-managed traffic
	// redirection rules. A dedicated name (rather than the generic "nat") is
	// used to avoid colliding with tables created by other tools on the same
	// host, and to make Consul-managed rules easy to identify via
	// `nft list ruleset`.
	tproxyTable = "consul_tproxy"

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

	// ProxyOutboundPort is the port of the proxy's outbound listener.
	ProxyOutboundPort int

	// ExcludeInboundPorts is the list of ports that should be excluded
	// from inbound traffic redirection. Each entry may be a numeric port
	// ("8080"), a range ("8080:9000"), or a TCP service name resolved via
	// /etc/services ("ssh"), matching iptables' --dport syntax.
	ExcludeInboundPorts []string

	// ExcludeOutboundPorts is the list of ports that should be excluded
	// from outbound traffic redirection. Accepts the same formats as
	// ExcludeInboundPorts.
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

	// Canonicalize the proxy's own UID the same way excluded UIDs are
	// canonicalized below: it is interpolated directly into the nft script
	// (see "skuid" rule further down) and must never carry raw, unvalidated
	// text into that script.
	cfg.ProxyUserID, err = validateUID(cfg.ProxyUserID)
	if err != nil {
		return fmt.Errorf("ProxyUserID: %w", err)
	}

	// ConsulDNSIP is also interpolated directly into the nft script (as a
	// "dnat to" destination below). Setup() checks it via
	// verifyDualStackConfig, but SetupWithAdditionalRules is exported and
	// can be called directly (e.g. by ECS mesh-init) without going through
	// Setup(), so canonicalize it here too rather than relying on callers.
	if cfg.ConsulDNSIP != "" {
		cfg.ConsulDNSIP, err = validateDNSIP(cfg.ConsulDNSIP)
		if err != nil {
			return fmt.Errorf("ConsulDNSIP: %w", err)
		}
	}

	// Set the default outbound port if it's not already set.
	if cfg.ProxyOutboundPort == 0 {
		cfg.ProxyOutboundPort = DefaultTProxyOutboundPort
	}

	// Create the inet table. The inet family processes both IPv4 and IPv6.
	//
	// "add rule" always appends, unlike "add table"/"add chain" which are
	// no-ops if already present. So on a retried Setup (e.g. CNI ADD retry),
	// we force-reset the table first: add (ensure it exists, since delete
	// errors otherwise), delete (wipes it and all chains/rules), add (recreate
	// empty) before repopulating below.
	cfg.NftablesProvider.AddRule("nft", "add", "table", "inet", tproxyTable)
	cfg.NftablesProvider.AddRule("nft", "delete", "table", "inet", tproxyTable)
	cfg.NftablesProvider.AddRule("nft", "add", "table", "inet", tproxyTable)

	// Create regular (non-hook) chains used for traffic redirection.
	chains := []string{ProxyInboundChain, ProxyInboundRedirectChain, ProxyOutputChain, ProxyOutputRedirectChain, DNSChain}
	for _, chain := range chains {
		cfg.NftablesProvider.AddRule("nft", "add", "chain", "inet", tproxyTable, chain)
	}

	// Create base chains that hook into the kernel packet-processing pipeline,
	// replacing the traditional PREROUTING and OUTPUT entry points.
	cfg.NftablesProvider.AddRule("nft", "add", "chain", "inet", tproxyTable, consulNATOutputChain,
		"{ type nat hook output priority -100 ; }")
	cfg.NftablesProvider.AddRule("nft", "add", "chain", "inet", tproxyTable, consulNATPreRoutingChain,
		"{ type nat hook prerouting priority -100 ; }")

	// The inet family intercepts both IPv4 and IPv6. When dual-stack is disabled,
	// preserve IPv4-only behaviour by returning IPv6 packets immediately — matching
	// the old iptables behaviour where ip6tables was never invoked.
	if !dualStack {
		for _, chain := range []string{consulNATOutputChain, consulNATPreRoutingChain} {
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, chain,
				"meta", "nfproto", "ipv6", "return")
		}
	}

	// Configure outbound rules.
	{
		// Redirect all TCP traffic hitting PROXY_REDIRECT chain to Envoy's outbound listener.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyOutputRedirectChain,
			"meta", "l4proto", "tcp", "redirect", "to", ":"+strconv.Itoa(cfg.ProxyOutboundPort))

		// DNS redirection rules. With the inet family these cover both IPv4 and IPv6;
		// a separate IPv4-only guard is no longer needed.
		if cfg.ConsulDNSIP != "" && cfg.ConsulDNSPort == 0 {
			// In the inet address family, dnat requires an explicit ip/ip6 keyword
			// alongside the destination address to identify the protocol family.
			dnsIPKw := ipFamilyKeyword(cfg.ConsulDNSIP)
			// Direct all DNS traffic in the DNS chain to the Consul DNS Service IP.
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, DNSChain,
				"udp", "dport", "53", "dnat", dnsIPKw, "to", cfg.ConsulDNSIP)
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, DNSChain,
				"tcp", "dport", "53", "dnat", dnsIPKw, "to", cfg.ConsulDNSIP)

			// Jump outbound port-53 traffic into the DNS chain.
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, consulNATOutputChain,
				"udp", "dport", "53", "jump", DNSChain)
			cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, consulNATOutputChain,
				"tcp", "dport", "53", "jump", DNSChain)
		} else if cfg.ConsulDNSPort != 0 {
			// Build the list of DNS IPs to write rules for.
			//
			// iptables equivalent (main branch):
			//   - SetupWithAdditionalRules  (iptables,  IPv4): uses 127.0.0.1, guarded by !dualStack
			//   - SetupWithAdditionalRulesIPv6 (ip6tables, IPv6): uses ::1,       runs only when dualStack
			//
			// nftables uses the inet family (single pass covering both IPv4 and IPv6), so
			// when dualStack=true we must emit rules for BOTH loopback addresses here.
			// When an explicit ConsulDNSIP is given, only that one address is used (unchanged).
			var dnsIPs []string
			if cfg.ConsulDNSIP != "" {
				dnsIPs = []string{cfg.ConsulDNSIP}
			} else if dualStack {
				dnsIPs = []string{"127.0.0.1", "::1"}
			} else {
				dnsIPs = []string{"127.0.0.1"}
			}

			for _, consulDNSIP := range dnsIPs {
				dnsIPKw := ipFamilyKeyword(consulDNSIP)
				consulDNSHostPort := net.JoinHostPort(consulDNSIP, strconv.Itoa(cfg.ConsulDNSPort))

				// Direct DNS traffic destined for the Consul DNS IP to the right port.
				cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, DNSChain,
					dnsIPKw, "daddr", consulDNSIP, "udp", "dport", "53", "dnat", "to", consulDNSHostPort)
				cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, DNSChain,
					dnsIPKw, "daddr", consulDNSIP, "tcp", "dport", "53", "dnat", "to", consulDNSHostPort)

				// Jump outbound port-53 traffic destined for the Consul DNS IP into the DNS chain.
				cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, consulNATOutputChain,
					dnsIPKw, "daddr", consulDNSIP, "udp", "dport", "53", "jump", DNSChain)
				cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, consulNATOutputChain,
					dnsIPKw, "daddr", consulDNSIP, "tcp", "dport", "53", "jump", DNSChain)
			}
		}

		// Jump all outbound TCP traffic from the output hook into PROXY_OUTPUT.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, consulNATOutputChain,
			"meta", "l4proto", "tcp", "jump", ProxyOutputChain)

		// Don't redirect the proxy's own traffic back to itself.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyOutputChain,
			"skuid", cfg.ProxyUserID, "return")

		// Skip localhost traffic (IPv4 and IPv6) — it doesn't need proxy routing.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyOutputChain,
			"ip", "daddr", "127.0.0.1/32", "return")
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyOutputChain,
			"ip6", "daddr", "::1/128", "return")

		// Redirect all remaining outbound traffic to Envoy.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyOutputChain,
			"jump", ProxyOutputRedirectChain)

		// insert (prepend) rules so they take precedence over the defaults above.
		for _, outboundPort := range cfg.ExcludeOutboundPorts {
			normalizedPort, err := normalizePortRange(outboundPort)
			if err != nil {
				return fmt.Errorf("ExcludeOutboundPorts: %w", err)
			}
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", tproxyTable, ProxyOutputChain,
				"tcp", "dport", normalizedPort, "return")
		}

		for _, outboundCIDR := range cfg.ExcludeOutboundCIDRs {
			normalizedCIDR, err := validateCIDR(outboundCIDR)
			if err != nil {
				return fmt.Errorf("ExcludeOutboundCIDRs: %w", err)
			}
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", tproxyTable, ProxyOutputChain,
				ipFamilyKeyword(normalizedCIDR), "daddr", normalizedCIDR, "return")
		}

		for _, uid := range cfg.ExcludeUIDs {
			normalizedUID, err := validateUID(uid)
			if err != nil {
				return fmt.Errorf("ExcludeUIDs: %w", err)
			}
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", tproxyTable, ProxyOutputChain,
				"skuid", normalizedUID, "return")
		}
	}

	// Configure inbound rules.
	{
		// Redirect all TCP traffic in PROXY_IN_REDIRECT to Envoy's inbound listener.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyInboundRedirectChain,
			"meta", "l4proto", "tcp", "redirect", "to", ":"+strconv.Itoa(cfg.ProxyInboundPort))

		// Jump inbound TCP traffic from the prerouting hook into PROXY_INBOUND.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, consulNATPreRoutingChain,
			"meta", "l4proto", "tcp", "jump", ProxyInboundChain)

		// Redirect remaining inbound traffic to Envoy.
		cfg.NftablesProvider.AddRule("nft", "add", "rule", "inet", tproxyTable, ProxyInboundChain,
			"meta", "l4proto", "tcp", "jump", ProxyInboundRedirectChain)

		for _, inboundPort := range cfg.ExcludeInboundPorts {
			normalizedPort, err := normalizePortRange(inboundPort)
			if err != nil {
				return fmt.Errorf("ExcludeInboundPorts: %w", err)
			}
			cfg.NftablesProvider.AddRule("nft", "insert", "rule", "inet", tproxyTable, ProxyInboundChain,
				"tcp", "dport", normalizedPort, "return")
		}
	}

	// Call function to add any additional rules passed on by the caller.
	if additionalRulesFn != nil {
		additionalRulesFn(cfg.NftablesProvider)
	}

	return cfg.NftablesProvider.ApplyRules("nft")
}

// SetupWithAdditionalRulesIPv6 was previously called internally by Setup() to apply
// ip6tables rules for dual-stack pods. With nftables the inet address family covers
// both IPv4 and IPv6 in a single SetupWithAdditionalRules call, making a separate
// IPv6 pass unnecessary. The function is retained here as no-op for backward compatibility as a reference
// for migrated from iptables to nftables.
func SetupWithAdditionalRulesIPv6(_ Config, _ AdditionalRulesFn, _ bool) error {
	return nil
}

// normalizePortRange converts an iptables-style port spec to nftables format.
// Colon ranges become dash ranges ("8080:9000" -> "8080-9000"), with an
// omitted endpoint defaulting to the full port space ("1024:" -> "1024-65535").
// Like iptables, any port or range endpoint may also be a TCP service name
// (e.g. "ssh", or "http:https" for a range) -- nft has no such lookup, so
// these are resolved to numeric ports via /etc/services beforehand.
func normalizePortRange(port string) (string, error) {
	if idx := strings.IndexByte(port, ':'); idx >= 0 {
		lo, hi := port[:idx], port[idx+1:]
		loNum, hiNum := "0", "65535"
		var err error
		if lo != "" {
			if loNum, err = resolvePort(lo); err != nil {
				return "", fmt.Errorf("invalid port range %q: %w", port, err)
			}
		}
		if hi != "" {
			if hiNum, err = resolvePort(hi); err != nil {
				return "", fmt.Errorf("invalid port range %q: %w", port, err)
			}
		}
		return loNum + "-" + hiNum, nil
	}

	if idx := strings.IndexByte(port, '-'); idx >= 0 {
		// A whole TCP service name may itself contain a hyphen (e.g.
		// "http-alt"). Try resolving the full string as a single service
		// name first, before assuming the hyphen is a range separator.
		if resolved, err := resolvePort(port); err == nil {
			return resolved, nil
		}

		lo, hi := port[:idx], port[idx+1:]
		loNum, err := resolvePort(lo)
		if err != nil {
			return "", fmt.Errorf("invalid port range %q: %w", port, err)
		}
		hiNum, err := resolvePort(hi)
		if err != nil {
			return "", fmt.Errorf("invalid port range %q: %w", port, err)
		}
		return loNum + "-" + hiNum, nil
	}

	resolved, err := resolvePort(port)
	if err != nil {
		return "", fmt.Errorf("invalid port %q: %w", port, err)
	}
	return resolved, nil
}

// resolvePort returns the numeric port for s: numeric input is used as-is,
// and non-numeric input is resolved as a TCP service name via /etc/services
// (e.g. "ssh" -> "22"), matching iptables' --dport semantics.
func resolvePort(s string) (string, error) {
	if s == "" {
		return "", errors.New("port value must not be empty")
	}
	if err := validatePortNumber(s); err == nil {
		return s, nil
	}
	port, err := net.LookupPort("tcp", s)
	if err != nil {
		return "", fmt.Errorf("port must be numeric or a resolvable TCP service name, got %q", s)
	}
	return strconv.Itoa(port), nil
}

// validatePortNumber returns an error unless s is a decimal port number in
// 0-65535.
func validatePortNumber(s string) error {
	if s == "" {
		return errors.New("port value must not be empty")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("port must be numeric, got %q", s)
	}
	if n < 0 || n > 65535 {
		return fmt.Errorf("port %d out of range (0-65535)", n)
	}
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

// validateCIDR parses cidr as either a bare IP address ("1.2.3.4") or an
// IP/prefix-length CIDR ("1.2.3.4/24", "::1/128") and returns its canonical
// string form.
//
// This value is later interpolated directly into an nft script (see the
// ExcludeOutboundCIDRs rule in SetupWithAdditionalRules), so it must never be
// passed through as raw, attacker-influenced text: a value containing nft
// statement separators, control characters, or additional nft keywords could
// otherwise inject extra statements into the script rather than remaining
// inert rule data. Rebuilding the value strictly from the parsed IP/prefix
// components (rather than merely pattern-matching the input) guarantees any
// such embedded syntax cannot survive into the reconstructed value.
//
// The host bits of the address are preserved as given (the value is not
// masked down to its network address), since exclusions are matched against
// the exact address/CIDR the operator configured.
func validateCIDR(cidr string) (string, error) {
	if cidr == "" {
		return "", errors.New("must not be empty")
	}

	ipPart, prefixPart, hasPrefix := strings.Cut(cidr, "/")
	ip := net.ParseIP(ipPart)
	if ip == nil {
		return "", fmt.Errorf("must be a valid IP address or CIDR, got %q", cidr)
	}
	if !hasPrefix {
		return ip.String(), nil
	}

	maxPrefix := 32
	if ip.To4() == nil {
		maxPrefix = 128
	}
	prefix, err := strconv.Atoi(prefixPart)
	if err != nil || prefix < 0 || prefix > maxPrefix {
		return "", fmt.Errorf("invalid CIDR prefix length in %q", cidr)
	}
	return ip.String() + "/" + strconv.Itoa(prefix), nil
}

// validateUID parses uid as a non-negative Linux user ID and returns its
// canonical decimal string form.
//
// Like validateCIDR, this exists because the value is interpolated directly
// into an nft script (as an "skuid" match), so raw text must never reach it:
// parsing the value strictly as an unsigned integer, then reconstructing the
// canonical decimal string from the parsed number, guarantees embedded nft
// syntax (statement separators, control characters, extra keywords) cannot
// survive into the value actually written to the script.
func validateUID(uid string) (string, error) {
	if uid == "" {
		return "", errors.New("must not be empty")
	}
	n, err := strconv.ParseUint(uid, 10, 32)
	if err != nil {
		return "", fmt.Errorf("must be a valid numeric user ID, got %q", uid)
	}
	return strconv.FormatUint(n, 10), nil
}

// validateDNSIP parses ip as a plain IP address (no CIDR notation — unlike
// ExcludeOutboundCIDRs, ConsulDNSIP identifies a single destination, not a
// range) and returns its canonical string form.
//
// Like validateCIDR and validateUID, this exists because the value is
// interpolated directly into the nft script (as a "dnat to" destination in
// the ConsulDNSIP rules in SetupWithAdditionalRules), so raw text must never
// reach it.
func validateDNSIP(ip string) (string, error) {
	if ip == "" {
		return "", errors.New("must not be empty")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("must be a valid IP address, got %q", ip)
	}
	return parsed.String(), nil
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
