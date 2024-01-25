// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/armon/go-radix"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"

	"github.com/hashicorp/consul/acl"
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
	errNameNotFound    = fmt.Errorf("name not found")
	errRecursionFailed = fmt.Errorf("recursion failed")
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

	enterpriseDNSConfig
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

// dnsRecursor is an interface that can be used to mock calls to external DNS servers for unit testing.
//
//go:generate mockery --name dnsRecursor --inpackage
type dnsRecursor interface {
	handle(req *dns.Msg, cfgCtx *RouterDynamicConfig, remoteAddr net.Addr) (*dns.Msg, error)
}

// Router replaces miekg/dns.ServeMux with a simpler router that only checks for the 2-3 valid domains
// that Consul supports and forwards to a single DiscoveryQueryProcessor handler. If there is no match, it will recurse.
type Router struct {
	processor  DiscoveryQueryProcessor
	recursor   dnsRecursor
	domain     string
	altDomain  string
	datacenter string
	logger     hclog.Logger

	tokenFunc func() string

	defaultEntMeta acl.EnterpriseMeta

	// TODO (v2-dns): default locality for request context?

	// dynamicConfig stores the config as an atomic value (for hot-reloading).
	// It is always of type *RouterDynamicConfig
	dynamicConfig atomic.Value
}

var _ = dns.Handler(&Router{})
var _ = DNSRouter(&Router{})

func NewRouter(cfg Config) (*Router, error) {
	// Make sure domains are FQDN, make them case-insensitive for DNSRequestRouter
	domain := dns.CanonicalName(cfg.AgentConfig.DNSDomain)
	altDomain := dns.CanonicalName(cfg.AgentConfig.DNSAltDomain)

	// TODO (v2-dns): need to figure out tenancy information here in a way that work for V2 and V1

	logger := cfg.Logger.Named(logging.DNS)

	router := &Router{
		processor:      cfg.Processor,
		recursor:       newRecursor(logger),
		domain:         domain,
		altDomain:      altDomain,
		logger:         logger,
		tokenFunc:      cfg.TokenFunc,
		defaultEntMeta: cfg.EntMeta,
	}

	if err := router.ReloadConfig(cfg.AgentConfig); err != nil {
		return nil, err
	}
	return router, nil
}

// HandleRequest is used to process an individual DNS request. It returns a message in success or fail cases.
func (r *Router) HandleRequest(req *dns.Msg, reqCtx discovery.Context, remoteAddress net.Addr) *dns.Msg {
	configCtx := r.dynamicConfig.Load().(*RouterDynamicConfig)

	err := validateAndNormalizeRequest(req)
	if err != nil {
		r.logger.Error("error parsing DNS query", "error", err)
		if errors.Is(err, errInvalidQuestion) {
			return createRefusedResponse(req)
		}
		return createServerFailureResponse(req, configCtx, false)
	}

	responseDomain, needRecurse := r.parseDomain(req)
	if needRecurse && !canRecurse(configCtx) {
		// This is the same error as an unmatched domain
		return createRefusedResponse(req)
	}

	if needRecurse {
		// This assumes `canRecurse(configCtx)` is true above
		resp, err := r.recursor.handle(req, configCtx, remoteAddress)
		if err != nil && !errors.Is(err, errRecursionFailed) {
			r.logger.Error("unhandled error recursing DNS query", "error", err)
		}
		if err != nil {
			return createServerFailureResponse(req, configCtx, true)
		}
		return resp
	}

	reqType := parseRequestType(req)
	results, err := r.getQueryResults(req, reqCtx, reqType, configCtx)
	switch {
	case errors.Is(err, errNameNotFound):
		r.logger.Error("name not found", "name", req.Question[0].Name)

		ecsGlobal := !errors.Is(err, discovery.ErrECSNotGlobal)
		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNameError, ecsGlobal)
	// TODO (v2-dns): there is another case here where the discovery service returns "name not found"
	case errors.Is(err, discovery.ErrNoData):
		r.logger.Debug("no data available", "name", req.Question[0].Name)

		ecsGlobal := !errors.Is(err, discovery.ErrECSNotGlobal)
		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeSuccess, ecsGlobal)
	case err != nil:
		r.logger.Error("error processing discovery query", "error", err)
		return createServerFailureResponse(req, configCtx, canRecurse(configCtx))
	}

	// This needs the question information because it affects the serialization format.
	// e.g., the Consul service has the same "results" for both NS and A/AAAA queries, but the serialization differs.
	resp, err := r.serializeQueryResults(req, results, configCtx, responseDomain)
	if err != nil {
		r.logger.Error("error serializing DNS results", "error", err)
		return createServerFailureResponse(req, configCtx, false)
	}
	return resp
}

// getQueryResults returns a discovery.Result from a DNS message.
func (r *Router) getQueryResults(req *dns.Msg, reqCtx discovery.Context, reqType requestType, cfgCtx *RouterDynamicConfig) ([]*discovery.Result, error) {
	switch reqType {
	case requestTypeConsul:
		// This is a special case of discovery.QueryByName where we know that we need to query the consul service
		// regardless of the question name.
		query := &discovery.Query{
			QueryType: discovery.QueryTypeService,
			QueryPayload: discovery.QueryPayload{
				Name: structs.ConsulServiceName,
			},
			Limit: 3, // TODO (v2-dns): need to thread this through to the backend and make sure we shuffle the results
		}
		return r.processor.QueryByName(query, reqCtx)
	case requestTypeName:
		query, err := buildQueryFromDNSMessage(req, r.domain, r.altDomain, cfgCtx, r.defaultEntMeta)
		if err != nil {
			r.logger.Error("error building discovery query from DNS request", "error", err)
			return nil, err
		}
		return r.processor.QueryByName(query, reqCtx)
	case requestTypeIP:
		// TODO (v2-dns): implement requestTypeIP
		// This will call discovery.QueryByIP
		return nil, errors.New("requestTypeIP not implemented")
	case requestTypeAddress:
		return buildAddressResults(req)
	}
	return nil, errors.New("invalid request type")
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

// Request type is similar to miekg/dns.Type, but correlates to the different query processors we might need to invoke.
type requestType string

const (
	requestTypeName    requestType = "NAME"   // A/AAAA/CNAME/SRV
	requestTypeIP      requestType = "IP"     // PTR
	requestTypeAddress requestType = "ADDR"   // Custom addr. A/AAAA lookups
	requestTypeConsul  requestType = "CONSUL" // SOA/NS
)

// parseDomain converts a DNS message into a generic discovery request.
// If the request domain does not match "consul." or the alternative domain,
// it will return true for needRecurse. The logic is based on miekg/dns.ServeDNS matcher.
// The implementation assumes that the only valid domains are "consul." and the alternative domain, and
// that DS query types are not supported.
func (r *Router) parseDomain(req *dns.Msg) (string, bool) {
	target := dns.CanonicalName(req.Question[0].Name)
	target, _ = stripSuffix(target)

	for offset, overflow := 0, false; !overflow; offset, overflow = dns.NextLabel(target, offset) {
		subdomain := target[offset:]
		switch subdomain {
		case ".":
			// We don't support consul having a domain or altdomain attached to the root.
			return "", true
		case r.domain:
			return r.domain, false
		case r.altDomain:
			return r.altDomain, false
		case arpaDomain:
			// PTR queries always respond with the primary domain.
			return r.domain, false
			// Default: fallthrough
		}
	}
	// No match found; recurse if possible
	return "", true
}

// parseRequestType inspects the DNS message type and question name to determine the requestType of request.
// We assume by the time this is called, we are responding to a question with a domain we serve.
// This is used internally to determine which query processor method (if any) to invoke.
func parseRequestType(req *dns.Msg) requestType {
	switch {
	case req.Question[0].Qtype == dns.TypeSOA || req.Question[0].Qtype == dns.TypeNS:
		// SOA and NS type supersede the domain
		// NOTE!: In V1 of the DNS server it was possible to serve a PTR lookup using the arpa domain but a SOA question type.
		// This also included the SOA record. This seemed inconsistent and unnecessary - it was removed for simplicity.
		return requestTypeConsul
	case isPTRSubdomain(req.Question[0].Name):
		return requestTypeIP
	case isAddrSubdomain(req.Question[0].Name):
		return requestTypeAddress
	default:
		return requestTypeName
	}
}

// serializeQueryResults converts a discovery.Result into a DNS message.
func (r *Router) serializeQueryResults(req *dns.Msg, results []*discovery.Result, cfg *RouterDynamicConfig, responseDomain string) (*dns.Msg, error) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Compress = !cfg.DisableCompression
	resp.Authoritative = true
	resp.RecursionAvailable = canRecurse(cfg)

	// Always add the SOA record if requested.
	if req.Question[0].Qtype == dns.TypeSOA {
		resp.Answer = append(resp.Answer, makeSOARecord(responseDomain, cfg))
	}

	for _, result := range results {
		appendResultToDNSResponse(result, req, resp, responseDomain, cfg)
	}

	return resp, nil
}

// defaultAgentDNSRequestContext returns a default request context based on the agent's config.
func (r *Router) defaultAgentDNSRequestContext() discovery.Context {
	return discovery.Context{
		Token: r.tokenFunc(),
		// TODO (v2-dns): tenancy information; maybe we choose not to specify and use the default
		// attached to the Router (from the agent's config)
	}
}

// validateAndNormalizeRequest validates the DNS request and normalizes the request name.
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

// stripSuffix strips off the suffixes that may have been added to the request name.
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

// isAddrSubdomain returns true if the domain is a valid addr subdomain.
func isAddrSubdomain(domain string) bool {
	labels := dns.SplitDomainName(domain)

	// Looking for <hexadecimal-encoded IP>.addr.<optional datacenter>.consul.
	if len(labels) > 2 {
		return labels[1] == addrLabel
	}
	return false
}

// isPTRSubdomain returns true if the domain ends in the PTR domain, "in-addr.arpa.".
func isPTRSubdomain(domain string) bool {
	labels := dns.SplitDomainName(domain)
	labelCount := len(labels)

	if labelCount < 3 {
		return false
	}

	return fmt.Sprintf("%s.%s.", labels[labelCount-2], labels[labelCount-1]) == arpaDomain
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
		enterpriseDNSConfig: getEnterpriseDNSConfig(conf),
	}

	// TODO (v2-dns): add service TTL recalculation

	for _, r := range conf.DNSRecursors {
		ra, err := formatRecursorAddress(r)
		if err != nil {
			return nil, fmt.Errorf("invalid recursor address: %w", err)
		}
		cfg.Recursors = append(cfg.Recursors, ra)
	}

	return cfg, nil
}

// canRecurse returns true if the router can recurse on the request.
func canRecurse(cfg *RouterDynamicConfig) bool {
	return len(cfg.Recursors) > 0
}

// createServerFailureResponse returns a SERVFAIL message.
func createServerFailureResponse(req *dns.Msg, cfg *RouterDynamicConfig, recursionAvailable bool) *dns.Msg {
	// Return a SERVFAIL message
	m := &dns.Msg{}
	m.SetReply(req)
	m.Compress = !cfg.DisableCompression
	m.SetRcode(req, dns.RcodeServerFailure)
	m.RecursionAvailable = recursionAvailable
	if edns := req.IsEdns0(); edns != nil {
		setEDNS(req, m, true)
	}
	return m
}

// setEDNS is used to set the responses EDNS size headers and
// possibly the ECS headers as well if they were present in the
// original request
func setEDNS(request *dns.Msg, response *dns.Msg, ecsGlobal bool) {
	edns := request.IsEdns0()
	if edns == nil {
		return
	}

	// cannot just use the SetEdns0 function as we need to embed
	// the ECS option as well
	ednsResp := new(dns.OPT)
	ednsResp.Hdr.Name = "."
	ednsResp.Hdr.Rrtype = dns.TypeOPT
	ednsResp.SetUDPSize(edns.UDPSize())

	// Set up the ECS option if present
	if subnet := ednsSubnetForRequest(request); subnet != nil {
		subOp := new(dns.EDNS0_SUBNET)
		subOp.Code = dns.EDNS0SUBNET
		subOp.Family = subnet.Family
		subOp.Address = subnet.Address
		subOp.SourceNetmask = subnet.SourceNetmask
		if c := response.Rcode; ecsGlobal || c == dns.RcodeNameError || c == dns.RcodeServerFailure || c == dns.RcodeRefused || c == dns.RcodeNotImplemented {
			// reply is globally valid and should be cached accordingly
			subOp.SourceScope = 0
		} else {
			// reply is only valid for the subnet it was queried with
			subOp.SourceScope = subnet.SourceNetmask
		}
		ednsResp.Option = append(ednsResp.Option, subOp)
	}

	response.Extra = append(response.Extra, ednsResp)
}

// ednsSubnetForRequest looks through the request to find any EDS subnet options
func ednsSubnetForRequest(req *dns.Msg) *dns.EDNS0_SUBNET {
	// IsEdns0 returns the EDNS RR if present or nil otherwise
	edns := req.IsEdns0()
	if edns == nil {
		return nil
	}

	for _, o := range edns.Option {
		if subnet, ok := o.(*dns.EDNS0_SUBNET); ok {
			return subnet
		}
	}
	return nil
}

// createRefusedResponse returns a REFUSED message. This is the default behavior for unmatched queries in
// upstream miekg/dns.
func createRefusedResponse(req *dns.Msg) *dns.Msg {
	// Return a REFUSED message
	m := &dns.Msg{}
	m.SetRcode(req, dns.RcodeRefused)
	return m
}

// createAuthoritativeResponse returns an authoritative message that contains the SOA in the event that data is
// not return for a query. There can be multiple reasons for not returning data, hence the rcode argument.
func createAuthoritativeResponse(req *dns.Msg, cfg *RouterDynamicConfig, domain string, rcode int, ecsGlobal bool) *dns.Msg {
	m := &dns.Msg{}
	m.SetRcode(req, rcode)
	m.Compress = !cfg.DisableCompression
	m.Authoritative = true
	m.RecursionAvailable = canRecurse(cfg)
	if edns := req.IsEdns0(); edns != nil {
		setEDNS(req, m, ecsGlobal)
	}

	// We add the SOA on NameErrors
	soa := makeSOARecord(domain, cfg)
	m.Ns = append(m.Ns, soa)

	return m
}

// buildAddressResults returns a discovery.Result from a DNS request for addr. records.
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

// buildQueryFromDNSMessage appends the discovery result to the dns message.
func appendResultToDNSResponse(result *discovery.Result, req *dns.Msg, resp *dns.Msg, domain string, cfg *RouterDynamicConfig) {
	ip, ok := convertToIp(result)

	// if the result is not an IP, we can try to recurse on the hostname.
	// TODO (v2-dns): hostnames are valid for workloads in V2, do we just want to return the CNAME?
	if !ok {
		// TODO (v2-dns): recurse on HandleRequest()
		panic("not implemented")
	}

	var ttl uint32
	switch result.Type {
	case discovery.ResultTypeNode, discovery.ResultTypeVirtual, discovery.ResultTypeWorkload:
		ttl = uint32(cfg.NodeTTL / time.Second)
	case discovery.ResultTypeService:
		// TODO (v2-dns): implement service TTL using the radix tree
	}

	qName := dns.CanonicalName(req.Question[0].Name)
	qType := req.Question[0].Qtype

	record, isIPV4 := makeRecord(qName, ip, ttl)

	// TODO (v2-dns): skip records that refer to a workload/node that don't have a valid DNS name.

	// Special case responses
	switch qType {
	case dns.TypeSOA:
		// TODO (v2-dns): fqdn in V1 has the datacenter included, this would need to be added to discovery.Result
		// to be returned in the result.
		fqdn := fmt.Sprintf("%s.%s.%s", result.Target, strings.ToLower(string(result.Type)), domain)
		extraRecord, _ := makeRecord(fqdn, ip, ttl) // TODO (v2-dns): this is not sufficient, because recursion and CNAMES are supported

		resp.Ns = append(resp.Ns, makeNSRecord(domain, fqdn, ttl))
		resp.Extra = append(resp.Extra, extraRecord)
		return
	case dns.TypeNS:
		// TODO (v2-dns): fqdn in V1 has the datacenter included, this would need to be added to discovery.Result
		fqdn := fmt.Sprintf("%s.%s.%s.", result.Target, strings.ToLower(string(result.Type)), domain)
		extraRecord, _ := makeRecord(fqdn, ip, ttl) // TODO (v2-dns): this is not sufficient, because recursion and CNAMES are supported

		resp.Answer = append(resp.Ns, makeNSRecord(domain, fqdn, ttl))
		resp.Extra = append(resp.Extra, extraRecord)
		return
	case dns.TypeSRV:
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

// convertToIp converts a discovery.Result to a net.IP.
func convertToIp(result *discovery.Result) (net.IP, bool) {
	ip := net.ParseIP(result.Address)
	if ip == nil {
		return nil, false
	}
	return ip, true
}

func makeSOARecord(domain string, cfg *RouterDynamicConfig) dns.RR {
	return &dns.SOA{
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
}

func makeNSRecord(domain, fqdn string, ttl uint32) dns.RR {
	return &dns.NS{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns: fqdn,
	}
}

// makeRecord an A or AAAA record for the given name and IP.
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
