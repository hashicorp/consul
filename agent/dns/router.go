// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"regexp"
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
	"github.com/hashicorp/consul/internal/dnsutil"
	"github.com/hashicorp/consul/logging"
)

const (
	addrLabel = "addr"

	arpaDomain = "arpa."
	arpaLabel  = "arpa"

	suffixFailover           = "failover."
	suffixNoFailover         = "no-failover."
	maxRecursionLevelDefault = 3 // This field comes from the V1 DNS server and affects V1 catalog lookups
	maxRecurseRecords        = 5
)

var (
	errInvalidQuestion = fmt.Errorf("invalid question")
	errNameNotFound    = fmt.Errorf("name not found")
	errRecursionFailed = fmt.Errorf("recursion failed")

	trailingSpacesRE = regexp.MustCompile(" +$")
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

	discovery.EnterpriseDNSConfig
}

// GetTTLForService Find the TTL for a given service.
// return ttl, true if found, 0, false otherwise
func (cfg *RouterDynamicConfig) GetTTLForService(service string) (time.Duration, bool) {
	if cfg.TTLStrict != nil {
		ttl, ok := cfg.TTLStrict[service]
		if ok {
			return ttl, true
		}
	}
	if cfg.TTLRadix != nil {
		_, ttlRaw, ok := cfg.TTLRadix.LongestPrefix(service)
		if ok {
			return ttlRaw.(time.Duration), true
		}
	}
	return 0, false
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
	return r.handleRequestRecursively(req, reqCtx, remoteAddress, maxRecursionLevelDefault)
}

// handleRequestRecursively is used to process an individual DNS request. It will recurse as needed
// a maximum number of times and returns a message in success or fail cases.
func (r *Router) handleRequestRecursively(req *dns.Msg, reqCtx discovery.Context,
	remoteAddress net.Addr, maxRecursionLevel int) *dns.Msg {
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
	results, query, err := r.getQueryResults(req, reqCtx, reqType, configCtx)
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
	resp, err := r.serializeQueryResults(req, reqCtx, query, results, configCtx, responseDomain, remoteAddress, maxRecursionLevel)
	if err != nil {
		r.logger.Error("error serializing DNS results", "error", err)
		return createServerFailureResponse(req, configCtx, false)
	}
	return resp
}

// getTTLForResult returns the TTL for a given result.
func getTTLForResult(name string, query *discovery.Query, cfg *RouterDynamicConfig) uint32 {
	switch {
	// TODO (v2-dns): currently have to do this related to the results type being changed to node whe
	// the v1 data fetcher encounters a blank service address and uses the node address instead.
	// we will revisiting this when look at modifying the discovery result struct to
	// possibly include additional metadata like the node address.
	case query != nil && query.QueryType == discovery.QueryTypeService:
		ttl, ok := cfg.GetTTLForService(name)
		if ok {
			return uint32(ttl / time.Second)
		}
		fallthrough
	default:
		return uint32(cfg.NodeTTL / time.Second)
	}
}

// getQueryResults returns a discovery.Result from a DNS message.
func (r *Router) getQueryResults(req *dns.Msg, reqCtx discovery.Context, reqType requestType, cfg *RouterDynamicConfig) ([]*discovery.Result, *discovery.Query, error) {
	var query *discovery.Query
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

		results, err := r.processor.QueryByName(query, reqCtx)
		return results, query, err
	case requestTypeName:
		query, err := buildQueryFromDNSMessage(req, r.domain, r.altDomain, cfg, r.defaultEntMeta, r.datacenter)
		if err != nil {
			r.logger.Error("error building discovery query from DNS request", "error", err)
			return nil, query, err
		}
		results, err := r.processor.QueryByName(query, reqCtx)
		if err != nil {
			r.logger.Error("error processing discovery query", "error", err)
			return nil, query, err
		}
		return results, query, nil
	case requestTypeIP:
		ip := dnsutil.IPFromARPA(req.Question[0].Name)
		if ip == nil {
			r.logger.Error("error building IP from DNS request", "name", req.Question[0].Name)
			return nil, nil, errNameNotFound
		}
		results, err := r.processor.QueryByIP(ip, reqCtx)
		return results, query, err
	case requestTypeAddress:
		results, err := buildAddressResults(req)
		if err != nil {
			r.logger.Error("error processing discovery query", "error", err)
			return nil, query, err
		}
		return results, query, nil
	}
	return nil, query, errors.New("invalid request type")
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
func (r *Router) serializeQueryResults(req *dns.Msg, reqCtx discovery.Context,
	query *discovery.Query, results []*discovery.Result, cfg *RouterDynamicConfig,
	responseDomain string, remoteAddress net.Addr, maxRecursionLevel int) (*dns.Msg, error) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Compress = !cfg.DisableCompression
	resp.Authoritative = true
	resp.RecursionAvailable = canRecurse(cfg)

	qType := req.Question[0].Qtype
	reqType := parseRequestType(req)

	// Always add the SOA record if requested.
	switch {
	case qType == dns.TypeSOA:
		resp.Answer = append(resp.Answer, makeSOARecord(responseDomain, cfg))
		for _, result := range results {
			ans, ex, ns := r.getAnswerExtraAndNs(result, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
			resp.Answer = append(resp.Answer, ans...)
			resp.Extra = append(resp.Extra, ex...)
			resp.Ns = append(resp.Ns, ns...)
		}
	case qType == dns.TypeSRV, reqType == requestTypeAddress:
		for _, result := range results {
			ans, ex, ns := r.getAnswerExtraAndNs(result, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
			resp.Answer = append(resp.Answer, ans...)
			resp.Extra = append(resp.Extra, ex...)
			resp.Ns = append(resp.Ns, ns...)
		}
	default:
		// default will send it to where it does some de-duping while it calls getAnswerExtraAndNs and recurses.
		r.appendResultsToDNSResponse(req, reqCtx, query, resp, results, cfg, responseDomain, remoteAddress, maxRecursionLevel)
	}

	return resp, nil
}

// appendResultsToDNSResponse builds dns message from the discovery results and
// appends them to the dns response.
func (r *Router) appendResultsToDNSResponse(req *dns.Msg, reqCtx discovery.Context,
	query *discovery.Query, resp *dns.Msg, results []*discovery.Result, cfg *RouterDynamicConfig,
	responseDomain string, remoteAddress net.Addr, maxRecursionLevel int) {

	// Always add the SOA record if requested.
	if req.Question[0].Qtype == dns.TypeSOA {
		resp.Answer = append(resp.Answer, makeSOARecord(responseDomain, cfg))
	}

	handled := make(map[string]struct{})
	var answerCNAME []dns.RR = nil

	count := 0
	for _, result := range results {
		// Add the node record
		had_answer := false
		ans, extra, _ := r.getAnswerExtraAndNs(result, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
		resp.Extra = append(resp.Extra, extra...)

		if len(ans) == 0 {
			continue
		}

		// Avoid duplicate entries, possible if a node has
		// the same service on multiple ports, etc.
		if _, ok := handled[ans[0].String()]; ok {
			continue
		}
		handled[ans[0].String()] = struct{}{}

		switch ans[0].(type) {
		case *dns.CNAME:
			// keep track of the first CNAME + associated RRs but don't add to the resp.Answer yet
			// this will only be added if no non-CNAME RRs are found
			if len(answerCNAME) == 0 {
				answerCNAME = ans
			}
		default:
			resp.Answer = append(resp.Answer, ans...)
			had_answer = true
		}

		if had_answer {
			count++
			if count == cfg.ARecordLimit {
				// We stop only if greater than 0 or we reached the limit
				return
			}
		}
	}

	if len(resp.Answer) == 0 && len(answerCNAME) > 0 {
		resp.Answer = answerCNAME
	}
}

// defaultAgentDNSRequestContext returns a default request context based on the agent's config.
func (r *Router) defaultAgentDNSRequestContext() discovery.Context {
	return discovery.Context{
		Token: r.tokenFunc(),
		// TODO (v2-dns): tenancy information; maybe we choose not to specify and use the default
		// attached to the Router (from the agent's config)
	}
}

// resolveCNAME is used to recursively resolve CNAME records
func (r *Router) resolveCNAME(cfg *RouterDynamicConfig, name string, reqCtx discovery.Context,
	remoteAddress net.Addr, maxRecursionLevel int) []dns.RR {
	// If the CNAME record points to a Consul address, resolve it internally
	// Convert query to lowercase because DNS is case insensitive; d.domain and
	// d.altDomain are already converted

	if ln := strings.ToLower(name); strings.HasSuffix(ln, "."+r.domain) || strings.HasSuffix(ln, "."+r.altDomain) {
		if maxRecursionLevel < 1 {
			//d.logger.Error("Infinite recursion detected for name, won't perform any CNAME resolution.", "name", name)
			return nil
		}
		req := &dns.Msg{}

		req.SetQuestion(name, dns.TypeANY)
		// TODO: handle error response
		resp := r.handleRequestRecursively(req, reqCtx, nil, maxRecursionLevel-1)

		return resp.Answer
	}

	// Do nothing if we don't have a recursor
	if !canRecurse(cfg) {
		return nil
	}

	// Ask for any A records
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)

	// Make a DNS lookup request
	recursorResponse, err := r.recursor.handle(m, cfg, remoteAddress)
	if err == nil {
		return recursorResponse.Answer
	}

	r.logger.Error("all resolvers failed for name", "name", name)
	return nil
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

	// We keep this check brief so we can have more specific error handling later.
	if labelCount < 1 {
		return false
	}

	return labels[labelCount-1] == arpaLabel
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
		EnterpriseDNSConfig: discovery.GetEnterpriseDNSConfig(conf),
	}

	if conf.DNSServiceTTL != nil {
		cfg.TTLRadix = radix.New()
		cfg.TTLStrict = make(map[string]time.Duration)

		for key, ttl := range conf.DNSServiceTTL {
			// All suffix with '*' are put in radix
			// This include '*' that will match anything
			if strings.HasSuffix(key, "*") {
				cfg.TTLRadix.Insert(key[:len(key)-1], ttl)
			} else {
				cfg.TTLStrict[key] = ttl
			}
		}
	} else {
		cfg.TTLRadix = nil
		cfg.TTLStrict = nil
	}

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

// getAnswerAndExtra creates the dns answer and extra from discovery results.
func (r *Router) getAnswerExtraAndNs(result *discovery.Result, req *dns.Msg, reqCtx discovery.Context,
	query *discovery.Query, cfg *RouterDynamicConfig, domain string, remoteAddress net.Addr, maxRecursionLevel int) (answer []dns.RR, extra []dns.RR, ns []dns.RR) {
	address, target := getAddressAndTargetFromDiscoveryResult(result, r.domain)
	qName := req.Question[0].Name
	ttlLookupName := qName
	if query != nil {
		ttlLookupName = query.QueryPayload.Name
	}
	ttl := getTTLForResult(ttlLookupName, query, cfg)
	qType := req.Question[0].Qtype

	// TODO (v2-dns): skip records that refer to a workload/node that don't have a valid DNS name.

	// Special case responses
	switch {
	// PTR requests are first since they are a special case of domain overriding question type
	case parseRequestType(req) == requestTypeIP:
		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: qName, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0},
			Ptr: canonicalNameForResult(result, domain),
		}
		answer = append(answer, ptr)
	case qType == dns.TypeNS:
		// TODO (v2-dns): fqdn in V1 has the datacenter included, this would need to be added to discovery.Result
		fqdn := canonicalNameForResult(result, domain)
		extraRecord := makeIPBasedRecord(fqdn, address, ttl) // TODO (v2-dns): this is not sufficient, because recursion and CNAMES are supported

		answer = append(answer, makeNSRecord(domain, fqdn, ttl))
		extra = append(extra, extraRecord)
	case qType == dns.TypeSOA:
		// TODO (v2-dns): fqdn in V1 has the datacenter included, this would need to be added to discovery.Result
		// to be returned in the result.
		fqdn := canonicalNameForResult(result, domain)
		extraRecord := makeIPBasedRecord(fqdn, address, ttl) // TODO (v2-dns): this is not sufficient, because recursion and CNAMES are supported

		ns = append(ns, makeNSRecord(domain, fqdn, ttl))
		extra = append(extra, extraRecord)
	case qType == dns.TypeSRV:
		// We put A/AAAA/CNAME records in the additional section for SRV requests
		a, e := r.getAnswerExtrasForAddressAndTarget(address, target, req, reqCtx,
			result, ttl, remoteAddress, maxRecursionLevel)
		answer = append(answer, a...)
		extra = append(extra, e...)

		cfg := r.dynamicConfig.Load().(*RouterDynamicConfig)
		if cfg.NodeMetaTXT {
			extra = append(extra, makeTXTRecord(target.FQDN(), result, ttl)...)
		}
	default:
		a, e := r.getAnswerExtrasForAddressAndTarget(address, target, req, reqCtx,
			result, ttl, remoteAddress, maxRecursionLevel)
		answer = append(answer, a...)
		extra = append(extra, e...)
	}
	return
}

// getAnswerExtrasForAddressAndTarget creates the dns answer and extra from address and target dnsAddress pairs.
func (r *Router) getAnswerExtrasForAddressAndTarget(address *dnsAddress, target *dnsAddress, req *dns.Msg,
	reqCtx discovery.Context, result *discovery.Result, ttl uint32, remoteAddress net.Addr,
	maxRecursionLevel int) (answer []dns.RR, extra []dns.RR) {
	qName := req.Question[0].Name
	reqType := parseRequestType(req)

	cfg := r.dynamicConfig.Load().(*RouterDynamicConfig)
	switch {

	// There is no target and the address is a FQDN (external service)
	case address.IsFQDN():
		a, e := r.makeRecordFromFQDN(address.FQDN(), result, req, reqCtx,
			cfg, ttl, remoteAddress, maxRecursionLevel)
		answer = append(a, answer...)
		extra = append(e, extra...)

	// The target is a FQDN (internal or external service name)
	case result.Type != discovery.ResultTypeNode && target.IsFQDN():
		a, e := r.makeRecordFromFQDN(target.FQDN(), result, req, reqCtx,
			cfg, ttl, remoteAddress, maxRecursionLevel)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// There is no target and the address is an IP
	case address.IsIP():
		// TODO (v2-dns): Do not CNAME node address in case of WAN address.
		ipRecordName := target.FQDN()
		if maxRecursionLevel < maxRecursionLevelDefault || ipRecordName == "" {
			ipRecordName = qName
		}
		a, e := getAnswerExtrasForIP(ipRecordName, address, req.Question[0], reqType, result, ttl)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The target is an IP
	case target.IsIP():
		a, e := getAnswerExtrasForIP(qName, target, req.Question[0], reqType, result, ttl)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The target is a CNAME for the service we are looking
	// for. So we use the address.
	case target.FQDN() == req.Question[0].Name && address.IsIP():
		a, e := getAnswerExtrasForIP(qName, address, req.Question[0], reqType, result, ttl)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The target is a FQDN (internal or external service name)
	default:
		a, e := r.makeRecordFromFQDN(target.FQDN(), result, req, reqCtx, cfg, ttl, remoteAddress, maxRecursionLevel)
		answer = append(a, answer...)
		extra = append(e, extra...)
	}
	return
}

// getAddressAndTargetFromDiscoveryResult returns the address and target from a discovery result.
func getAnswerExtrasForIP(name string, addr *dnsAddress, question dns.Question, reqType requestType, result *discovery.Result, ttl uint32) (answer []dns.RR, extra []dns.RR) {
	record := makeIPBasedRecord(name, addr, ttl)
	qType := question.Qtype

	isARecordWhenNotExplicitlyQueried := record.Header().Rrtype == dns.TypeA && qType != dns.TypeA && qType != dns.TypeANY
	isAAAARecordWhenNotExplicitlyQueried := record.Header().Rrtype == dns.TypeAAAA && qType != dns.TypeAAAA && qType != dns.TypeANY

	// For explicit A/AAAA queries, we must only return those records in the answer section.
	if isARecordWhenNotExplicitlyQueried ||
		isAAAARecordWhenNotExplicitlyQueried {
		extra = append(extra, record)
	} else {
		answer = append(answer, record)
	}

	if reqType != requestTypeAddress && qType == dns.TypeSRV {
		srv := makeSRVRecord(name, name, result, ttl)
		answer = append(answer, srv)
	}
	return
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

// makeIPBasedRecord an A or AAAA record for the given name and IP.
// Note: we might want to pass in the Query Name here, which is used in addr. and virtual. queries
// since there is only ever one result. Right now choosing to leave it off for simplification.
func makeIPBasedRecord(name string, addr *dnsAddress, ttl uint32) dns.RR {

	if addr.IsIPV4() {
		// check if the query type is  A for IPv4 or ANY
		return &dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: addr.IP(),
		}
	}

	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		AAAA: addr.IP(),
	}
}

func (r *Router) makeRecordFromFQDN(fqdn string, result *discovery.Result,
	req *dns.Msg, reqCtx discovery.Context, cfg *RouterDynamicConfig, ttl uint32,
	remoteAddress net.Addr, maxRecursionLevel int) ([]dns.RR, []dns.RR) {
	edns := req.IsEdns0() != nil
	q := req.Question[0]

	more := r.resolveCNAME(cfg, dns.Fqdn(fqdn), reqCtx, remoteAddress, maxRecursionLevel)
	var additional []dns.RR
	extra := 0
MORE_REC:
	for _, rr := range more {
		switch rr.Header().Rrtype {
		case dns.TypeCNAME, dns.TypeA, dns.TypeAAAA:
			// set the TTL manually
			rr.Header().Ttl = ttl
			additional = append(additional, rr)

			extra++
			if extra == maxRecurseRecords && !edns {
				break MORE_REC
			}
		}
	}

	if q.Qtype == dns.TypeSRV {
		answers := []dns.RR{
			makeSRVRecord(q.Name, fqdn, result, ttl),
		}
		return answers, additional
	}

	answers := []dns.RR{
		makeCNAMERecord(result, q.Name, ttl),
	}
	answers = append(answers, additional...)

	return answers, nil
}

// makeCNAMERecord returns a CNAME record for the given name and target.
func makeCNAMERecord(result *discovery.Result, qName string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   qName,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: dns.Fqdn(result.Target),
	}
}

// func makeSRVRecord returns an SRV record for the given name and target.
func makeSRVRecord(name, target string, result *discovery.Result, ttl uint32) *dns.SRV {
	return &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 1,
		Weight:   uint16(result.Weight),
		Port:     uint16(result.Port),
		Target:   target,
	}
}

// encodeKVasRFC1464 encodes a key-value pair according to RFC1464
func encodeKVasRFC1464(key, value string) (txt string) {
	// For details on these replacements c.f. https://www.ietf.org/rfc/rfc1464.txt
	key = strings.Replace(key, "`", "``", -1)
	key = strings.Replace(key, "=", "`=", -1)

	// Backquote the leading spaces
	leadingSpacesRE := regexp.MustCompile("^ +")
	numLeadingSpaces := len(leadingSpacesRE.FindString(key))
	key = leadingSpacesRE.ReplaceAllString(key, strings.Repeat("` ", numLeadingSpaces))

	// Backquote the trailing spaces
	numTrailingSpaces := len(trailingSpacesRE.FindString(key))
	key = trailingSpacesRE.ReplaceAllString(key, strings.Repeat("` ", numTrailingSpaces))

	value = strings.Replace(value, "`", "``", -1)

	return key + "=" + value
}

// makeTXTRecord returns a TXT record for the given name and result metadata.
func makeTXTRecord(name string, result *discovery.Result, ttl uint32) []dns.RR {
	extra := make([]dns.RR, 0, len(result.Metadata))
	for key, value := range result.Metadata {
		txt := value
		if !strings.HasPrefix(strings.ToLower(key), "rfc1035-") {
			txt = encodeKVasRFC1464(key, value)
		}

		extra = append(extra, &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Txt: []string{txt},
		})
	}
	return extra
}

// getAddressAndTargetFromCheckServiceNode returns the address and target for a given discovery.Result
func getAddressAndTargetFromDiscoveryResult(result *discovery.Result, domain string) (*dnsAddress, *dnsAddress) {
	target := newDNSAddress(result.Target)
	if !target.IsEmptyString() && !target.IsInternalFQDNOrIP(domain) {
		target.SetAddress(canonicalNameForResult(result, domain))
	}
	address := newDNSAddress(result.Address)
	return address, target
}
