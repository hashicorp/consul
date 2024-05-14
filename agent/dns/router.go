// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-radix"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"

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
	errNotImplemented  = fmt.Errorf("not implemented")
	errRecursionFailed = fmt.Errorf("recursion failed")

	trailingSpacesRE = regexp.MustCompile(" +$")
)

// RouterDynamicConfig is the dynamic configuration that can be hot-reloaded
type RouterDynamicConfig struct {
	ARecordLimit       int
	DisableCompression bool
	EnableTruncate     bool
	NodeMetaTXT        bool
	NodeTTL            time.Duration
	Recursors          []string
	RecursorTimeout    time.Duration
	RecursorStrategy   structs.RecursorStrategy
	SOAConfig          SOAConfig
	// TTLRadix sets service TTLs by prefix, eg: "database-*"
	TTLRadix *radix.Tree
	// TTLStrict sets TTLs to service by full name match. It Has higher priority than TTLRadix
	TTLStrict      map[string]time.Duration
	UDPAnswerLimit int
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
	handle(req *dns.Msg, cfgCtx *RouterDynamicConfig, remoteAddress net.Addr) (*dns.Msg, error)
}

// Router replaces miekg/dns.ServeMux with a simpler router that only checks for the 2-3 valid domains
// that Consul supports and forwards to a single DiscoveryQueryProcessor handler. If there is no match, it will recurse.
type Router struct {
	processor DiscoveryQueryProcessor
	recursor  dnsRecursor
	domain    string
	altDomain string
	nodeName  string
	logger    hclog.Logger

	tokenFunc                   func() string
	translateAddressFunc        func(dc string, addr string, taggedAddresses map[string]string, accept dnsutil.TranslateAddressAccept) string
	translateServiceAddressFunc func(dc string, address string, taggedAddresses map[string]structs.ServiceAddress, accept dnsutil.TranslateAddressAccept) string

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

	logger := cfg.Logger.Named(logging.DNS)

	router := &Router{
		processor:                   cfg.Processor,
		recursor:                    newRecursor(logger),
		domain:                      domain,
		altDomain:                   altDomain,
		logger:                      logger,
		nodeName:                    cfg.AgentConfig.NodeName,
		tokenFunc:                   cfg.TokenFunc,
		translateAddressFunc:        cfg.TranslateAddressFunc,
		translateServiceAddressFunc: cfg.TranslateServiceAddressFunc,
	}

	if err := router.ReloadConfig(cfg.AgentConfig); err != nil {
		return nil, err
	}
	return router, nil
}

// HandleRequest is used to process an individual DNS request. It returns a message in success or fail cases.
func (r *Router) HandleRequest(req *dns.Msg, reqCtx Context, remoteAddress net.Addr) *dns.Msg {
	configCtx := r.dynamicConfig.Load().(*RouterDynamicConfig)

	respGenerator := dnsResponseGenerator{}

	err := validateAndNormalizeRequest(req)
	if err != nil {
		r.logger.Error("error parsing DNS query", "error", err)
		if errors.Is(err, errInvalidQuestion) {
			return respGenerator.createRefusedResponse(req)
		}
		return respGenerator.createServerFailureResponse(req, configCtx, false)
	}

	r.logger.Trace("received request", "question", req.Question[0].Name, "type", dns.Type(req.Question[0].Qtype).String())
	r.normalizeContext(&reqCtx)

	defer func(s time.Time, q dns.Question) {
		metrics.MeasureSinceWithLabels([]string{"dns", "query"}, s,
			[]metrics.Label{
				{Name: "node", Value: r.nodeName},
				{Name: "type", Value: dns.Type(q.Qtype).String()},
			})

		r.logger.Trace("request served from client",
			"name", q.Name,
			"type", dns.Type(q.Qtype).String(),
			"class", dns.Class(q.Qclass).String(),
			"latency", time.Since(s).String(),
			"client", remoteAddress.String(),
			"client_network", remoteAddress.Network(),
		)
	}(time.Now(), req.Question[0])

	return r.handleRequestRecursively(req, reqCtx, configCtx, remoteAddress, maxRecursionLevelDefault)
}

// handleRequestRecursively is used to process an individual DNS request. It will recurse as needed
// a maximum number of times and returns a message in success or fail cases.
func (r *Router) handleRequestRecursively(req *dns.Msg, reqCtx Context, configCtx *RouterDynamicConfig,
	remoteAddress net.Addr, maxRecursionLevel int) *dns.Msg {
	respGenerator := dnsResponseGenerator{}

	r.logger.Trace(
		"received request",
		"question", req.Question[0].Name,
		"type", dns.Type(req.Question[0].Qtype).String(),
		"recursion_remaining", maxRecursionLevel)

	responseDomain, needRecurse := r.parseDomain(req.Question[0].Name)
	if needRecurse && !canRecurse(configCtx) {
		// This is the same error as an unmatched domain
		return respGenerator.createRefusedResponse(req)
	}

	if needRecurse {
		r.logger.Trace("checking recursors to handle request", "question", req.Question[0].Name, "type", dns.Type(req.Question[0].Qtype).String())

		// This assumes `canRecurse(configCtx)` is true above
		resp, err := r.recursor.handle(req, configCtx, remoteAddress)
		if err != nil && !errors.Is(err, errRecursionFailed) {
			r.logger.Error("unhandled error recursing DNS query", "error", err)
		}
		if err != nil {
			return respGenerator.createServerFailureResponse(req, configCtx, true)
		}
		return resp
	}

	// Need to pass the question name to properly support recursion and the
	// trimming of the domain suffixes.
	qName := dns.CanonicalName(req.Question[0].Name)
	if maxRecursionLevel < maxRecursionLevelDefault {
		// Get the QName without the domain suffix
		qName = r.trimDomain(qName)
	}

	results, query, err := discoveryResultsFetcher{}.getQueryResults(&getQueryOptions{
		req:           req,
		reqCtx:        reqCtx,
		qName:         qName,
		remoteAddress: remoteAddress,
		processor:     r.processor,
		logger:        r.logger,
		domain:        r.domain,
		altDomain:     r.altDomain,
	})

	// in case of the wrapped ECSNotGlobalError, extract the error from it.
	isECSGlobal := !errors.Is(err, discovery.ErrECSNotGlobal)
	err = getErrorFromECSNotGlobalError(err)
	if err != nil {
		return respGenerator.generateResponseFromError(&generateResponseFromErrorOpts{
			req:            req,
			err:            err,
			qName:          qName,
			configCtx:      configCtx,
			responseDomain: responseDomain,
			isECSGlobal:    isECSGlobal,
			query:          query,
			canRecurse:     canRecurse(configCtx),
			logger:         r.logger,
		})
	}

	r.logger.Trace("serializing results", "question", req.Question[0].Name, "results-found", len(results))

	// This needs the question information because it affects the serialization format.
	// e.g., the Consul service has the same "results" for both NS and A/AAAA queries, but the serialization differs.
	serializedOpts := &serializeOptions{
		req:                         req,
		reqCtx:                      reqCtx,
		query:                       query,
		results:                     results,
		cfg:                         configCtx,
		responseDomain:              responseDomain,
		remoteAddress:               remoteAddress,
		maxRecursionLevel:           maxRecursionLevel,
		translateAddressFunc:        r.translateAddressFunc,
		translateServiceAddressFunc: r.translateServiceAddressFunc,
		resolveCnameFunc:            r.resolveCNAME,
	}
	resp, err := messageSerializer{}.serialize(serializedOpts)
	if err != nil {
		r.logger.Error("error serializing DNS results", "error", err)
		return respGenerator.generateResponseFromError(&generateResponseFromErrorOpts{
			req:            req,
			err:            err,
			qName:          qName,
			configCtx:      configCtx,
			responseDomain: responseDomain,
			isECSGlobal:    isECSGlobal,
			query:          query,
			canRecurse:     false,
			logger:         r.logger,
		})
	}

	respGenerator.trimDNSResponse(configCtx, remoteAddress, req, resp, r.logger)
	respGenerator.setEDNS(req, resp, isECSGlobal)
	return resp
}

// trimDomain trims the domain from the question name.
func (r *Router) trimDomain(questionName string) string {
	longer := r.domain
	shorter := r.altDomain

	if len(shorter) > len(longer) {
		longer, shorter = shorter, longer
	}

	if strings.HasSuffix(questionName, "."+strings.TrimLeft(longer, ".")) {
		return strings.TrimSuffix(questionName, longer)
	}
	return strings.TrimSuffix(questionName, shorter)
}

// ServeDNS implements the miekg/dns.Handler interface.
// This is a standard DNS listener.
func (r *Router) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	out := r.HandleRequest(req, Context{}, w.RemoteAddr())
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

// resolveCNAME is used to recursively resolve CNAME records
func (r *Router) resolveCNAME(cfgContext *RouterDynamicConfig, name string, reqCtx Context,
	remoteAddress net.Addr, maxRecursionLevel int) []dns.RR {
	// If the CNAME record points to a Consul address, resolve it internally
	// Convert query to lowercase because DNS is case-insensitive; r.domain and
	// r.altDomain are already converted

	if ln := strings.ToLower(name); strings.HasSuffix(ln, "."+r.domain) || strings.HasSuffix(ln, "."+r.altDomain) {
		if maxRecursionLevel < 1 {
			r.logger.Error("Infinite recursion detected for name, won't perform any CNAME resolution.", "name", name)
			return nil
		}
		req := &dns.Msg{}

		req.SetQuestion(name, dns.TypeANY)
		// TODO: handle error response (this is a comment from the V1 DNS Server)
		resp := r.handleRequestRecursively(req, reqCtx, cfgContext, nil, maxRecursionLevel-1)

		return resp.Answer
	}

	// Do nothing if we don't have a recursor
	if !canRecurse(cfgContext) {
		return nil
	}

	// Ask for any A records
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)

	// Make a DNS lookup request
	recursorResponse, err := r.recursor.handle(m, cfgContext, remoteAddress)
	if err == nil {
		return recursorResponse.Answer
	}

	r.logger.Error("all resolvers failed for name", "name", name)
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
func (r *Router) parseDomain(questionName string) (string, bool) {
	target := dns.CanonicalName(questionName)
	target, _ = stripAnyFailoverSuffix(target)

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

// GetConfig returns the current router config
func (r *Router) GetConfig() *RouterDynamicConfig {
	return r.dynamicConfig.Load().(*RouterDynamicConfig)
}

// getErrorFromECSNotGlobalError returns the underlying error from an ECSNotGlobalError, if it exists.
func getErrorFromECSNotGlobalError(err error) error {
	if errors.Is(err, discovery.ErrECSNotGlobal) {
		return err.(discovery.ECSNotGlobalError).Unwrap()
	}
	return err
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

// normalizeContext makes sure context information is populated with agent defaults as needed.
// Right now this is just the ACL token. We do this in the router with the token because DNS doesn't
// allow a token to be passed in the request, and we expect ACL tokens upfront in APIs when they are enabled.
// Tenancy information is left out because it is safe/expected to assume agent defaults in the backend lookup.
func (r *Router) normalizeContext(ctx *Context) {
	if ctx.Token == "" {
		ctx.Token = r.tokenFunc()
	}
}

// stripAnyFailoverSuffix strips off the suffixes that may have been added to the request name.
func stripAnyFailoverSuffix(target string) (string, bool) {
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
