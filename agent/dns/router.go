// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/armon/go-radix"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

const (
	addrLabel = "addr"

	arpaDomain = "in-addr.arpa."

	suffixFailover   = "failover."
	suffixNoFailover = "no-failover."
)

var (
	errInvalidQuestion = fmt.Errorf("invalid question")
	errNameNotFound    = fmt.Errorf("invalid question")
)

// TODO (v2-dns): metrics

// RouterDynamicConfig is the dynamic configuration that can be hot-reloaded
type RouterDynamicConfig struct {
	ARecordLimit          int
	DisableCompression    bool
	EnableDefaultFailover bool // TODO (v2-dns): plumbing required for this new V2 setting. This is the agent configured default
	EnableTruncate        bool
	NodeMetaTXT           bool
	NodeTTL               time.Duration
	Recursors             []string
	RecursorTimeout       time.Duration
	RecursorStrategy      structs.RecursorStrategy
	SOAConfig             SOAConfig
	// TTLRadix sets service TTLs by prefix, eg: "database-*"
	TTLRadix *radix.Tree
	// TTLStrict sets TTLs to service by full name match. It Has higher priority than TTLRadix
	TTLStrict      map[string]time.Duration
	UDPAnswerLimit int
}

type SOAConfig struct {
	Refresh uint32 // 3600 by default
	Retry   uint32 // 600
	Expire  uint32 // 86400
	Minttl  uint32 // 0
}

// DiscoveryQueryProcessor is an interface that can be used by any consumer requesting Service Discovery results.
// This could be attached to a gRPC endpoint in the future in addition to DNS.
// Making this an interface means testing the router with a mock is trivial.
type DiscoveryQueryProcessor interface {
	QueryByName(*discovery.Query, discovery.Context) ([]*discovery.Result, error)
	QueryByIP(net.IP, discovery.Context) ([]*discovery.Result, error)
}

// Router replaces miekg/dns.ServeMux with a simpler router that only checks for the 2-3 valid domains
// that Consul supports and forwards to a single DiscoveryQueryProcessor handler. If there is no match, it will recurse.
type Router struct {
	processor DiscoveryQueryProcessor
	domain    string
	altDomain string
	logger    hclog.Logger

	tokenFunc func() string

	defaultNamespace string
	defaultPartition string

	// TODO (v2-dns): default locality for request context?

	// dynamicConfig stores the config as an atomic value (for hot-reloading).
	// It is always of type *RouterDynamicConfig
	dynamicConfig atomic.Value
}

var _ = dns.Handler(&Router{})

func NewRouter(cfg Config) (*Router, error) {
	// Make sure domains are FQDN, make them case-insensitive for DNSRequestRouter
	domain := dns.CanonicalName(cfg.AgentConfig.DNSDomain)
	altDomain := dns.CanonicalName(cfg.AgentConfig.DNSAltDomain)

	// TODO (v2-dns): need to figure out tenancy information here in a way that work for V2 and V1

	router := &Router{
		processor: cfg.Processor,
		domain:    domain,
		altDomain: altDomain,
		logger:    cfg.Logger.Named(logging.DNS),
		tokenFunc: cfg.TokenFunc,
		// TODO (v2-dns): see tenancy question above
		//defaultPartition: ?,
		//defaultNamespace: ?,
	}

	if err := router.ReloadConfig(cfg.AgentConfig); err != nil {
		return nil, err
	}
	return router, nil
}

// HandleRequest is used to process and individual DNS request. It returns a message in success or fail cases.
func (r *Router) HandleRequest(req *dns.Msg, reqCtx discovery.Context, remoteAddress net.Addr) *dns.Msg {
	cfg := r.dynamicConfig.Load().(*RouterDynamicConfig)

	err := validateAndNormalizeRequest(req)
	if err != nil {
		r.logger.Error("error parsing DNS query", "error", err)
		if errors.Is(err, errInvalidQuestion) {
			return createRefusedResponse(req)
		}
		return createServerFailureResponse(req, cfg, false)
	}

	reqType, responseDomain, needRecurse := r.parseDomain(req)

	if needRecurse && canRecurse(cfg) {
		// TODO (v2-dns): handle recursion
		r.logger.Error("recursion not implemented")
		return createServerFailureResponse(req, cfg, false)
	}

	var results []*discovery.Result
	switch reqType {
	case requestTypeName:
		//query, err := r.buildQuery(req, reqCtx)
		//results, err = r.processor.QueryByName(query, reqCtx)
		// TODO (v2-dns): implement requestTypeName
		// This will call discovery.QueryByName
		r.logger.Error("requestTypeName not implemented")
	case requestTypeIP:
		// TODO (v2-dns): implement requestTypeIP
		// This will call discovery.QueryByIP
		r.logger.Error("requestTypeIP not implemented")
	case requestTypeAddress:
		results, err = buildAddressResults(req)
	}
	if err != nil && errors.Is(err, errNameNotFound) {
		r.logger.Error("name not found", "name", req.Question[0].Name)
		return createNameErrorResponse(req, cfg, responseDomain)
	}
	if err != nil {
		r.logger.Error("error processing discovery query", "error", err)
		return createServerFailureResponse(req, cfg, false)
	}

	// This needs the question information because it affects the serialization format.
	// e.g., the Consul service has the same "results" for both NS and A/AAAA queries, but the serialization differs.
	resp, err := r.serializeQueryResults(req, results, cfg, responseDomain)
	if err != nil {
		r.logger.Error("error serializing DNS results", "error", err)
		return createServerFailureResponse(req, cfg, false)
	}
	return resp
}

// ServeDNS implements the miekg/dns.Handler interface.
// This is a standard DNS listener, so we inject a default request context based on the agent's config.
func (r *Router) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	reqCtx := r.defaultAgentDNSRequestContext()
	out := r.HandleRequest(req, reqCtx, w.RemoteAddr())
	w.WriteMsg(out)
}

// ReloadConfig hot-reloads the router config with new parameters
func (r *Router) ReloadConfig(newCfg *config.RuntimeConfig) error {
	cfg, err := getDynamicRouterConfig(newCfg)
	if err != nil {
		return fmt.Errorf("error loading DNS config: %w", err)
	}
	r.dynamicConfig.Store(cfg)
	return nil
}

func (r *Router) defaultAgentDNSRequestContext() discovery.Context {
	return discovery.Context{
		Token: r.tokenFunc(),
		// TODO (v2-dns): tenancy information; maybe we choose not to specify and use the default
		// attached to the Router (from the agent's config)
	}
}

func validateAndNormalizeRequest(req *dns.Msg) error {
	// like upstream miekg/dns, we require at least one question,
	// but we will only answer the first.
	if len(req.Question) == 0 {
		return errInvalidQuestion
	}

	// We mutate the request name to respond with the canonical name.
	// This is Consul convention.
	req.Question[0].Name = dns.CanonicalName(req.Question[0].Name)
	return nil
}

// Request type is similar to miekg/dns.Type, but correlates to the different query processors we might need to invoke.
type requestType string

const (
	requestTypeName    requestType = "NAME" // A/AAAA/CNAME/SRV/SOA
	requestTypeIP      requestType = "IP"
	requestTypeAddress requestType = "ADDR"
)

// parseQuery converts a DNS message into a generic discovery request.
// If the request domain does not match "consul." or the alternative domain,
// it will return true for needRecurse. The logic is based on miekg/dns.ServeDNS matcher.
// The implementation assumes that the only valid domains are "consul." and the alternative domain, and
// that DS query types are not supported.
func (r *Router) parseDomain(req *dns.Msg) (requestType, string, bool) {
	target := dns.CanonicalName(req.Question[0].Name)
	target, _ = stripSuffix(target)

	for offset, overflow := 0, false; !overflow; offset, overflow = dns.NextLabel(target, offset) {
		subdomain := target[offset:]
		switch subdomain {
		case r.domain:
			if isAddrSubdomain(target) {
				return requestTypeAddress, r.domain, false
			}
			return requestTypeName, r.domain, false

		case r.altDomain:
			// TODO (v2-dns): the default, unspecified alt domain should be ".". Next label should never return this
			// but write a test to verify that.
			if isAddrSubdomain(target) {
				return requestTypeAddress, r.altDomain, false
			}
			return requestTypeName, r.altDomain, false
		case arpaDomain:
			// PTR queries always respond with the primary domain.
			return requestTypeIP, r.domain, false
			// Default: fallthrough
		}
	}
	// No match found; recurse if possible
	return "", "", true
}

func (r *Router) serializeQueryResults(req *dns.Msg, results []*discovery.Result, cfg *RouterDynamicConfig, responseDomain string) (*dns.Msg, error) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Compress = !cfg.DisableCompression
	resp.Authoritative = true
	resp.RecursionAvailable = canRecurse(cfg)

	// TODO (v2-dns): add SOA if that is the question type

	for _, result := range results {
		appendResultToDNSResponse(result, req, resp, responseDomain, cfg)
	}

	return resp, nil
}

func stripSuffix(target string) (string, bool) {
	enableFailover := false

	// Strip off any suffixes that may have been added.
	offset, underflow := dns.PrevLabel(target, 1)
	if !underflow {
		maybeSuffix := target[offset:]
		switch maybeSuffix {
		case suffixFailover:
			target = target[:offset]
			enableFailover = true
		case suffixNoFailover:
			target = target[:offset]
		}
	}
	return target, enableFailover
}

func isAddrSubdomain(domain string) bool {
	labels := dns.SplitDomainName(domain)

	// Looking for <hexadecimal-encoded IP>.addr.<optional datacenter>.consul.
	if len(labels) > 2 {
		return labels[1] == addrLabel
	}
	return false
}

// getDynamicRouterConfig takes agent config and creates/resets the config used by DNS Router
func getDynamicRouterConfig(conf *config.RuntimeConfig) (*RouterDynamicConfig, error) {
	cfg := &RouterDynamicConfig{
		ARecordLimit:       conf.DNSARecordLimit,
		EnableTruncate:     conf.DNSEnableTruncate,
		NodeTTL:            conf.DNSNodeTTL,
		RecursorStrategy:   conf.DNSRecursorStrategy,
		RecursorTimeout:    conf.DNSRecursorTimeout,
		UDPAnswerLimit:     conf.DNSUDPAnswerLimit,
		NodeMetaTXT:        conf.DNSNodeMetaTXT,
		DisableCompression: conf.DNSDisableCompression,
		SOAConfig: SOAConfig{
			Expire:  conf.DNSSOA.Expire,
			Minttl:  conf.DNSSOA.Minttl,
			Refresh: conf.DNSSOA.Refresh,
			Retry:   conf.DNSSOA.Retry,
		},
	}

	// TODO (v2-dns): add service TTL recalculation

	// TODO (v2-dns): add recursor address formatting
	return cfg, nil
}

func canRecurse(cfg *RouterDynamicConfig) bool {
	return len(cfg.Recursors) > 0
}

func createServerFailureResponse(req *dns.Msg, cfg *RouterDynamicConfig, recursionAvailable bool) *dns.Msg {
	// Return a SERVFAIL message
	m := &dns.Msg{}
	m.SetReply(req)
	m.Compress = !cfg.DisableCompression
	m.SetRcode(req, dns.RcodeServerFailure)
	// TODO (2-dns): set EDNS
	m.RecursionAvailable = recursionAvailable
	return m
}

func createRefusedResponse(req *dns.Msg) *dns.Msg {
	// Return a REFUSED message
	m := &dns.Msg{}
	m.SetRcode(req, dns.RcodeRefused)
	return m
}

func createNameErrorResponse(req *dns.Msg, cfg *RouterDynamicConfig, domain string) *dns.Msg {
	// Return a NXDOMAIN message
	m := &dns.Msg{}
	m.SetRcode(req, dns.RcodeNameError)
	m.Compress = !cfg.DisableCompression
	m.Authoritative = true

	// We add the SOA on NameErrors
	// TODO (v2-dns): refactor into a common function
	soa := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			// Has to be consistent with MinTTL to avoid invalidation
			Ttl: cfg.SOAConfig.Minttl,
		},
		Ns:      "ns." + domain,
		Serial:  uint32(time.Now().Unix()),
		Mbox:    "hostmaster." + domain,
		Refresh: cfg.SOAConfig.Refresh,
		Retry:   cfg.SOAConfig.Retry,
		Expire:  cfg.SOAConfig.Expire,
		Minttl:  cfg.SOAConfig.Minttl,
	}
	m.Ns = append(m.Ns, soa)

	return m
}

func buildAddressResults(req *dns.Msg) ([]*discovery.Result, error) {
	domain := dns.CanonicalName(req.Question[0].Name)
	labels := dns.SplitDomainName(domain)
	hexadecimal := labels[0]

	if len(hexadecimal)/2 != 4 && len(hexadecimal)/2 != 16 {
		return nil, errNameNotFound
	}

	var ip net.IP
	ip, err := hex.DecodeString(hexadecimal)
	if err != nil {
		return nil, errNameNotFound
	}

	return []*discovery.Result{
		{
			Address: ip.String(),
			Type:    discovery.ResultTypeNode, // We choose node by convention since we do not know the origin of the IP
		},
	}, nil
}

func appendResultToDNSResponse(result *discovery.Result, req *dns.Msg, resp *dns.Msg, _ string, cfg *RouterDynamicConfig) {
	ip, ok := convertToIp(result)

	// if the result is not an IP, we can try to recurse on the hostname.
	// TODO (v2-dns): hostnames are valid for workloads in V2, do we just want to return the CNAME?
	if !ok {
		// TODO (v2-dns): recurse on HandleRequest()
		panic("not implemented")
	}

	var ttl uint32
	switch result.Type {
	case discovery.ResultTypeNode:
		ttl = uint32(cfg.NodeTTL / time.Second)
	case discovery.ResultTypeService:
		// TODO (v2-dns): implement service TTL using the radix tree
	}

	qName := dns.CanonicalName(req.Question[0].Name)
	qType := req.Question[0].Qtype

	record, isIPV4 := makeRecord(qName, ip, ttl)

	if qType == dns.TypeSRV {
		// We put A/AAAA/CNAME records in the additional section for SRV requests
		resp.Extra = append(resp.Extra, record)

		// TODO (v2-dns): implement SRV records for the answer section

		return
	}

	// For explicit A/AAAA queries, we must only return those records in the answer section.
	if isIPV4 && qType != dns.TypeA && qType != dns.TypeANY {
		resp.Extra = append(resp.Extra, record)
		return
	}
	if !isIPV4 && qType != dns.TypeAAAA && qType != dns.TypeANY {
		resp.Extra = append(resp.Extra, record)
		return
	}

	resp.Answer = append(resp.Answer, record)
}

func convertToIp(result *discovery.Result) (net.IP, bool) {
	ip := net.ParseIP(result.Address)
	if ip == nil {
		return nil, false
	}
	return ip, true
}

// Note: we might want to pass in the Query Name here, which is used in addr. and virtual. queries
// since there is only ever one result. Right now choosing to leave it off for simplification.
func makeRecord(name string, ip net.IP, ttl uint32) (dns.RR, bool) {

	isIPV4 := ip.To4() != nil

	if isIPV4 {
		// check if the query type is  A for IPv4 or ANY
		return &dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: ip,
		}, true
	}

	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		AAAA: ip,
	}, false
}
