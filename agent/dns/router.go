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

	"github.com/armon/go-metrics"
	"github.com/armon/go-radix"
	"github.com/miekg/dns"

	"github.com/hashicorp/go-hclog"

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
	errNotImplemented  = fmt.Errorf("not implemented")
	errRecursionFailed = fmt.Errorf("recursion failed")

	trailingSpacesRE = regexp.MustCompile(" +$")
)

// Context is used augment a DNS message with Consul-specific metadata.
type Context struct {
	Token             string
	DefaultPartition  string
	DefaultDatacenter string
}

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
	processor  DiscoveryQueryProcessor
	recursor   dnsRecursor
	domain     string
	altDomain  string
	datacenter string
	nodeName   string
	logger     hclog.Logger

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
		datacenter:                  cfg.AgentConfig.Datacenter,
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

	r.logger.Trace("received request", "question", req.Question[0].Name, "type", dns.Type(req.Question[0].Qtype).String())

	err := validateAndNormalizeRequest(req)
	if err != nil {
		r.logger.Error("error parsing DNS query", "error", err)
		if errors.Is(err, errInvalidQuestion) {
			return createRefusedResponse(req)
		}
		return createServerFailureResponse(req, configCtx, false)
	}

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

// getErrorFromECSNotGlobalError returns the underlying error from an ECSNotGlobalError, if it exists.
func getErrorFromECSNotGlobalError(err error) error {
	if errors.Is(err, discovery.ErrECSNotGlobal) {
		return err.(discovery.ECSNotGlobalError).Unwrap()
	}
	return err
}

// handleRequestRecursively is used to process an individual DNS request. It will recurse as needed
// a maximum number of times and returns a message in success or fail cases.
func (r *Router) handleRequestRecursively(req *dns.Msg, reqCtx Context, configCtx *RouterDynamicConfig,
	remoteAddress net.Addr, maxRecursionLevel int) *dns.Msg {

	r.logger.Trace(
		"received request",
		"question", req.Question[0].Name,
		"type", dns.Type(req.Question[0].Qtype).String(),
		"recursion_remaining", maxRecursionLevel)

	responseDomain, needRecurse := r.parseDomain(req.Question[0].Name)
	if needRecurse && !canRecurse(configCtx) {
		// This is the same error as an unmatched domain
		return createRefusedResponse(req)
	}

	if needRecurse {
		r.logger.Trace("checking recursors to handle request", "question", req.Question[0].Name, "type", dns.Type(req.Question[0].Qtype).String())

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

	// Need to pass the question name to properly support recursion and the
	// trimming of the domain suffixes.
	qName := dns.CanonicalName(req.Question[0].Name)
	if maxRecursionLevel < maxRecursionLevelDefault {
		// Get the QName without the domain suffix
		qName = r.trimDomain(qName)
	}

	reqType := parseRequestType(req)
	results, query, err := r.getQueryResults(req, reqCtx, reqType, qName, remoteAddress)

	// in case of the wrapped ECSNotGlobalError, extract the error from it.
	isECSGlobal := !errors.Is(err, discovery.ErrECSNotGlobal)
	err = getErrorFromECSNotGlobalError(err)
	if err != nil {
		return r.generateResponseFromError(req, err, qName, configCtx, responseDomain,
			isECSGlobal, query, canRecurse(configCtx))
	}

	r.logger.Trace("serializing results", "question", req.Question[0].Name, "results-found", len(results))

	// This needs the question information because it affects the serialization format.
	// e.g., the Consul service has the same "results" for both NS and A/AAAA queries, but the serialization differs.
	resp, err := r.serializeQueryResults(req, reqCtx, query, results, configCtx, responseDomain, remoteAddress, maxRecursionLevel)
	if err != nil {
		r.logger.Error("error serializing DNS results", "error", err)
		return r.generateResponseFromError(req, err, qName, configCtx, responseDomain,
			false, query, false)
	}

	// Switch to TCP if the client is
	network := "udp"
	if _, ok := remoteAddress.(*net.TCPAddr); ok {
		network = "tcp"
	}

	trimDNSResponse(configCtx, network, req, resp, r.logger)

	setEDNS(req, resp, isECSGlobal)
	return resp
}

// generateResponseFromError generates a response from an error.
func (r *Router) generateResponseFromError(req *dns.Msg, err error, qName string,
	configCtx *RouterDynamicConfig, responseDomain string, isECSGlobal bool,
	query *discovery.Query, canRecurse bool) *dns.Msg {
	switch {
	case errors.Is(err, errInvalidQuestion):
		r.logger.Error("invalid question", "name", qName)

		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNameError, isECSGlobal)
	case errors.Is(err, errNameNotFound):
		r.logger.Error("name not found", "name", qName)

		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNameError, isECSGlobal)
	case errors.Is(err, errNotImplemented):
		r.logger.Error("query not implemented", "name", qName, "type", dns.Type(req.Question[0].Qtype).String())

		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNotImplemented, isECSGlobal)
	case errors.Is(err, discovery.ErrNotSupported):
		r.logger.Debug("query name syntax not supported", "name", req.Question[0].Name)

		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNameError, isECSGlobal)
	case errors.Is(err, discovery.ErrNotFound):
		r.logger.Debug("query name not found", "name", req.Question[0].Name)

		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNameError, isECSGlobal)
	case errors.Is(err, discovery.ErrNoData):
		r.logger.Debug("no data available", "name", qName)

		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeSuccess, isECSGlobal)
	case errors.Is(err, discovery.ErrNoPathToDatacenter):
		dc := ""
		if query != nil {
			dc = query.QueryPayload.Tenancy.Datacenter
		}
		r.logger.Debug("no path to datacenter", "datacenter", dc)
		return createAuthoritativeResponse(req, configCtx, responseDomain, dns.RcodeNameError, isECSGlobal)
	}
	r.logger.Error("error processing discovery query", "error", err)
	return createServerFailureResponse(req, configCtx, canRecurse)
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

// getTTLForResult returns the TTL for a given result.
func getTTLForResult(name string, overrideTTL *uint32, query *discovery.Query, cfg *RouterDynamicConfig) uint32 {
	// In the case we are not making a discovery query, such as addr. or arpa. lookups,
	// use the node TTL by convention
	if query == nil {
		return uint32(cfg.NodeTTL / time.Second)
	}

	if overrideTTL != nil {
		// If a result was provided with an override, use that. This is the case for some prepared queries.
		return *overrideTTL
	}

	switch query.QueryType {
	case discovery.QueryTypeService, discovery.QueryTypePreparedQuery:
		ttl, ok := cfg.getTTLForService(name)
		if ok {
			return uint32(ttl / time.Second)
		}
		fallthrough
	default:
		return uint32(cfg.NodeTTL / time.Second)
	}
}

// getQueryResults returns a discovery.Result from a DNS message.
func (r *Router) getQueryResults(req *dns.Msg, reqCtx Context, reqType requestType,
	qName string, remoteAddress net.Addr) ([]*discovery.Result, *discovery.Query, error) {
	switch reqType {
	case requestTypeConsul:
		// This is a special case of discovery.QueryByName where we know that we need to query the consul service
		// regardless of the question name.
		query := &discovery.Query{
			QueryType: discovery.QueryTypeService,
			QueryPayload: discovery.QueryPayload{
				Name: structs.ConsulServiceName,
				Tenancy: discovery.QueryTenancy{
					// We specify the partition here so that in the case we are a client agent in a non-default partition.
					// We don't want the query processors default partition to be used.
					// This is a small hack because for V1 CE, this is not the correct default partition name, but we
					// need to add something to disambiguate the empty field.
					Partition: acl.DefaultPartitionName, //NOTE: note this won't work if we ever have V2 client agents
				},
				Limit: 3,
			},
		}

		results, err := r.processor.QueryByName(query, discovery.Context{Token: reqCtx.Token})
		return results, query, err
	case requestTypeName:
		query, err := buildQueryFromDNSMessage(req, reqCtx, r.domain, r.altDomain, remoteAddress)
		if err != nil {
			r.logger.Error("error building discovery query from DNS request", "error", err)
			return nil, query, err
		}
		results, err := r.processor.QueryByName(query, discovery.Context{Token: reqCtx.Token})

		if getErrorFromECSNotGlobalError(err) != nil {
			r.logger.Error("error processing discovery query", "error", err)
			return nil, query, err
		}
		return results, query, err
	case requestTypeIP:
		ip := dnsutil.IPFromARPA(qName)
		if ip == nil {
			r.logger.Error("error building IP from DNS request", "name", qName)
			return nil, nil, errNameNotFound
		}
		results, err := r.processor.QueryByIP(ip, discovery.Context{Token: reqCtx.Token})
		return results, nil, err
	case requestTypeAddress:
		results, err := buildAddressResults(req)
		if err != nil {
			r.logger.Error("error processing discovery query", "error", err)
			return nil, nil, err
		}
		return results, nil, nil
	}

	r.logger.Error("error parsing discovery query type", "requestType", reqType)
	return nil, nil, errInvalidQuestion
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

// getTTLForService Find the TTL for a given service.
// return ttl, true if found, 0, false otherwise
func (cfg *RouterDynamicConfig) getTTLForService(service string) (time.Duration, bool) {
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

func getPortsFromResult(result *discovery.Result) []discovery.Port {
	if len(result.Ports) > 0 {
		return result.Ports
	}
	// return one record.
	return []discovery.Port{{}}
}

// serializeQueryResults converts a discovery.Result into a DNS message.
func (r *Router) serializeQueryResults(req *dns.Msg, reqCtx Context,
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
			for _, port := range getPortsFromResult(result) {
				ans, ex, ns := r.getAnswerExtraAndNs(result, port, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
				resp.Answer = append(resp.Answer, ans...)
				resp.Extra = append(resp.Extra, ex...)
				resp.Ns = append(resp.Ns, ns...)
			}
		}
	case reqType == requestTypeAddress:
		for _, result := range results {
			for _, port := range getPortsFromResult(result) {
				ans, ex, ns := r.getAnswerExtraAndNs(result, port, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
				resp.Answer = append(resp.Answer, ans...)
				resp.Extra = append(resp.Extra, ex...)
				resp.Ns = append(resp.Ns, ns...)
			}
		}
	case qType == dns.TypeSRV:
		handled := make(map[string]struct{})
		for _, result := range results {
			for _, port := range getPortsFromResult(result) {

				// Avoid duplicate entries, possible if a node has
				// the same service the same port, etc.

				// The datacenter should be empty during translation if it is a peering lookup.
				// This should be fine because we should always prefer the WAN address.

				address := ""
				if result.Service != nil {
					address = result.Service.Address
				} else {
					address = result.Node.Address
				}
				tuple := fmt.Sprintf("%s:%s:%d", result.Node.Name, address, port.Number)
				if _, ok := handled[tuple]; ok {
					continue
				}
				handled[tuple] = struct{}{}

				ans, ex, ns := r.getAnswerExtraAndNs(result, port, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
				resp.Answer = append(resp.Answer, ans...)
				resp.Extra = append(resp.Extra, ex...)
				resp.Ns = append(resp.Ns, ns...)
			}
		}
	default:
		// default will send it to where it does some de-duping while it calls getAnswerExtraAndNs and recurses.
		r.appendResultsToDNSResponse(req, reqCtx, query, resp, results, cfg, responseDomain, remoteAddress, maxRecursionLevel)
	}

	if query != nil && query.QueryType != discovery.QueryTypeVirtual &&
		len(resp.Answer) == 0 && len(resp.Extra) == 0 {
		return nil, discovery.ErrNoData
	}

	return resp, nil
}

// getServiceAddressMapFromLocationMap converts a map of Location to a map of ServiceAddress.
func getServiceAddressMapFromLocationMap(taggedAddresses map[string]*discovery.TaggedAddress) map[string]structs.ServiceAddress {
	taggedServiceAddresses := make(map[string]structs.ServiceAddress, len(taggedAddresses))
	for k, v := range taggedAddresses {
		taggedServiceAddresses[k] = structs.ServiceAddress{
			Address: v.Address,
			Port:    int(v.Port.Number),
		}
	}
	return taggedServiceAddresses
}

// getStringAddressMapFromTaggedAddressMap converts a map of Location to a map of string.
func getStringAddressMapFromTaggedAddressMap(taggedAddresses map[string]*discovery.TaggedAddress) map[string]string {
	taggedServiceAddresses := make(map[string]string, len(taggedAddresses))
	for k, v := range taggedAddresses {
		taggedServiceAddresses[k] = v.Address
	}
	return taggedServiceAddresses
}

// appendResultsToDNSResponse builds dns message from the discovery results and
// appends them to the dns response.
func (r *Router) appendResultsToDNSResponse(req *dns.Msg, reqCtx Context,
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
		for _, port := range getPortsFromResult(result) {

			// Add the node record
			had_answer := false
			ans, extra, _ := r.getAnswerExtraAndNs(result, port, req, reqCtx, query, cfg, responseDomain, remoteAddress, maxRecursionLevel)
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
	}
	if len(resp.Answer) == 0 && len(answerCNAME) > 0 {
		resp.Answer = answerCNAME
	}
}

// defaultAgentDNSRequestContext returns a default request context based on the agent's config.
func (r *Router) defaultAgentDNSRequestContext() Context {
	return Context{
		Token:             r.tokenFunc(),
		DefaultDatacenter: r.datacenter,
		// We don't need to specify the agent's partition here because that will be handled further down the stack
		// in the query processor.
	}
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
			Node: &discovery.Location{
				Address: ip.String(),
			},
			Type: discovery.ResultTypeNode, // We choose node by convention since we do not know the origin of the IP
		},
	}, nil
}

// getAnswerAndExtra creates the dns answer and extra from discovery results.
func (r *Router) getAnswerExtraAndNs(result *discovery.Result, port discovery.Port, req *dns.Msg, reqCtx Context,
	query *discovery.Query, cfg *RouterDynamicConfig, domain string, remoteAddress net.Addr,
	maxRecursionLevel int) (answer []dns.RR, extra []dns.RR, ns []dns.RR) {
	serviceAddress, nodeAddress := r.getServiceAndNodeAddresses(result, req)
	qName := req.Question[0].Name
	ttlLookupName := qName
	if query != nil {
		ttlLookupName = query.QueryPayload.Name
	}

	ttl := getTTLForResult(ttlLookupName, result.DNS.TTL, query, cfg)

	qType := req.Question[0].Qtype

	// TODO (v2-dns): skip records that refer to a workload/node that don't have a valid DNS name.

	// Special case responses
	switch {
	// PTR requests are first since they are a special case of domain overriding question type
	case parseRequestType(req) == requestTypeIP:
		ptrTarget := ""
		if result.Type == discovery.ResultTypeNode {
			ptrTarget = result.Node.Name
		} else if result.Type == discovery.ResultTypeService {
			ptrTarget = result.Service.Name
		}

		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: qName, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0},
			Ptr: canonicalNameForResult(result.Type, ptrTarget, domain, result.Tenancy, port.Name),
		}
		answer = append(answer, ptr)
	case qType == dns.TypeNS:
		resultType := result.Type
		target := result.Node.Name
		if parseRequestType(req) == requestTypeConsul && resultType == discovery.ResultTypeService {
			resultType = discovery.ResultTypeNode
		}
		fqdn := canonicalNameForResult(resultType, target, domain, result.Tenancy, port.Name)
		extraRecord := makeIPBasedRecord(fqdn, nodeAddress, ttl)

		answer = append(answer, makeNSRecord(domain, fqdn, ttl))
		extra = append(extra, extraRecord)
	case qType == dns.TypeSOA:
		fqdn := canonicalNameForResult(result.Type, result.Node.Name, domain, result.Tenancy, port.Name)
		extraRecord := makeIPBasedRecord(fqdn, nodeAddress, ttl)

		ns = append(ns, makeNSRecord(domain, fqdn, ttl))
		extra = append(extra, extraRecord)
	case qType == dns.TypeSRV:
		// We put A/AAAA/CNAME records in the additional section for SRV requests
		a, e := r.getAnswerExtrasForAddressAndTarget(nodeAddress, serviceAddress, req, reqCtx,
			result, port, ttl, remoteAddress, cfg, domain, maxRecursionLevel)
		answer = append(answer, a...)
		extra = append(extra, e...)

	default:
		a, e := r.getAnswerExtrasForAddressAndTarget(nodeAddress, serviceAddress, req, reqCtx,
			result, port, ttl, remoteAddress, cfg, domain, maxRecursionLevel)
		answer = append(answer, a...)
		extra = append(extra, e...)
	}

	a, e := getAnswerAndExtraTXT(req, cfg, qName, result, ttl, domain, query, &port)
	answer = append(answer, a...)
	extra = append(extra, e...)
	return
}

// getServiceAndNodeAddresses returns the service and node addresses from a discovery result.
func (r *Router) getServiceAndNodeAddresses(result *discovery.Result, req *dns.Msg) (*dnsAddress, *dnsAddress) {
	addrTranslate := dnsutil.TranslateAddressAcceptDomain
	if req.Question[0].Qtype == dns.TypeA {
		addrTranslate |= dnsutil.TranslateAddressAcceptIPv4
	} else if req.Question[0].Qtype == dns.TypeAAAA {
		addrTranslate |= dnsutil.TranslateAddressAcceptIPv6
	} else {
		addrTranslate |= dnsutil.TranslateAddressAcceptAny
	}

	// The datacenter should be empty during translation if it is a peering lookup.
	// This should be fine because we should always prefer the WAN address.
	serviceAddress := newDNSAddress("")
	if result.Service != nil {
		sa := r.translateServiceAddressFunc(result.Tenancy.Datacenter,
			result.Service.Address, getServiceAddressMapFromLocationMap(result.Service.TaggedAddresses),
			addrTranslate)
		serviceAddress = newDNSAddress(sa)
	}
	nodeAddress := newDNSAddress("")
	if result.Node != nil {
		na := r.translateAddressFunc(result.Tenancy.Datacenter, result.Node.Address,
			getStringAddressMapFromTaggedAddressMap(result.Node.TaggedAddresses), addrTranslate)
		nodeAddress = newDNSAddress(na)
	}
	return serviceAddress, nodeAddress
}

// getAnswerExtrasForAddressAndTarget creates the dns answer and extra from nodeAddress and serviceAddress dnsAddress pairs.
func (r *Router) getAnswerExtrasForAddressAndTarget(nodeAddress *dnsAddress, serviceAddress *dnsAddress, req *dns.Msg,
	reqCtx Context, result *discovery.Result, port discovery.Port, ttl uint32, remoteAddress net.Addr,
	cfg *RouterDynamicConfig, domain string, maxRecursionLevel int) (answer []dns.RR, extra []dns.RR) {
	qName := req.Question[0].Name
	reqType := parseRequestType(req)

	switch {
	case (reqType == requestTypeAddress || result.Type == discovery.ResultTypeVirtual) &&
		serviceAddress.IsEmptyString() && nodeAddress.IsIP():
		a, e := getAnswerExtrasForIP(qName, nodeAddress, req.Question[0], reqType, result, ttl, domain, &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	case result.Type == discovery.ResultTypeNode && nodeAddress.IsIP():
		canonicalNodeName := canonicalNameForResult(result.Type, result.Node.Name, domain, result.Tenancy, port.Name)
		a, e := getAnswerExtrasForIP(canonicalNodeName, nodeAddress, req.Question[0], reqType,
			result, ttl, domain, &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	case result.Type == discovery.ResultTypeNode && !nodeAddress.IsIP():
		a, e := r.makeRecordFromFQDN(result, req, reqCtx, cfg,
			ttl, remoteAddress, maxRecursionLevel, serviceAddress.FQDN(), &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	case serviceAddress.IsEmptyString() && nodeAddress.IsEmptyString():
		return nil, nil

	// There is no service address and the node address is an IP
	case serviceAddress.IsEmptyString() && nodeAddress.IsIP():
		resultType := discovery.ResultTypeNode
		if result.Type == discovery.ResultTypeWorkload {
			resultType = discovery.ResultTypeWorkload
		}
		canonicalNodeName := canonicalNameForResult(resultType, result.Node.Name, domain, result.Tenancy, port.Name)
		a, e := getAnswerExtrasForIP(canonicalNodeName, nodeAddress, req.Question[0], reqType, result, ttl, domain, &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// There is no service address and the node address is a FQDN (external service)
	case serviceAddress.IsEmptyString():
		a, e := r.makeRecordFromFQDN(result, req, reqCtx, cfg, ttl, remoteAddress, maxRecursionLevel, nodeAddress.FQDN(), &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The service address is an IP
	case serviceAddress.IsIP():
		canonicalServiceName := canonicalNameForResult(discovery.ResultTypeService, result.Service.Name, domain, result.Tenancy, port.Name)
		a, e := getAnswerExtrasForIP(canonicalServiceName, serviceAddress, req.Question[0], reqType, result, ttl, domain, &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// If the service address is a CNAME for the service we are looking
	// for then use the node address.
	case serviceAddress.FQDN() == req.Question[0].Name && nodeAddress.IsIP():
		canonicalNodeName := canonicalNameForResult(discovery.ResultTypeNode, result.Node.Name, domain, result.Tenancy, port.Name)
		a, e := getAnswerExtrasForIP(canonicalNodeName, nodeAddress, req.Question[0], reqType, result, ttl, domain, &port)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The service address is a FQDN (internal or external service name)
	default:
		a, e := r.makeRecordFromFQDN(result, req, reqCtx, cfg, ttl, remoteAddress, maxRecursionLevel, serviceAddress.FQDN(), &port)
		answer = append(answer, a...)
		extra = append(extra, e...)
	}

	return
}

// getAnswerAndExtraTXT determines whether a TXT needs to be create and then
// returns the TXT record in the answer or extra depending on the question type.
func getAnswerAndExtraTXT(req *dns.Msg, cfg *RouterDynamicConfig, qName string,
	result *discovery.Result, ttl uint32, domain string, query *discovery.Query, port *discovery.Port) (answer []dns.RR, extra []dns.RR) {
	if !shouldAppendTXTRecord(query, cfg, req) {
		return
	}
	recordHeaderName := qName
	serviceAddress := newDNSAddress("")
	if result.Service != nil {
		serviceAddress = newDNSAddress(result.Service.Address)
	}
	if result.Type != discovery.ResultTypeNode &&
		result.Type != discovery.ResultTypeVirtual &&
		!serviceAddress.IsInternalFQDN(domain) &&
		!serviceAddress.IsExternalFQDN(domain) {
		recordHeaderName = canonicalNameForResult(discovery.ResultTypeNode, result.Node.Name,
			domain, result.Tenancy, port.Name)
	}
	qType := req.Question[0].Qtype
	generateMeta := false
	metaInAnswer := false
	if qType == dns.TypeANY || qType == dns.TypeTXT {
		generateMeta = true
		metaInAnswer = true
	} else if cfg.NodeMetaTXT {
		generateMeta = true
	}

	// Do not generate txt records if we don't have to: https://github.com/hashicorp/consul/pull/5272
	if generateMeta {
		meta := makeTXTRecord(recordHeaderName, result, ttl)
		if metaInAnswer {
			answer = append(answer, meta...)
		} else {
			extra = append(extra, meta...)
		}
	}
	return answer, extra
}

// shouldAppendTXTRecord determines whether a TXT record should be appended to the response.
func shouldAppendTXTRecord(query *discovery.Query, cfg *RouterDynamicConfig, req *dns.Msg) bool {
	qType := req.Question[0].Qtype
	switch {
	// Node records
	case query != nil && query.QueryType == discovery.QueryTypeNode && (cfg.NodeMetaTXT || qType == dns.TypeANY || qType == dns.TypeTXT):
		return true
	// Service records
	case query != nil && query.QueryType == discovery.QueryTypeService && cfg.NodeMetaTXT && qType == dns.TypeSRV:
		return true
	// Prepared query records
	case query != nil && query.QueryType == discovery.QueryTypePreparedQuery && cfg.NodeMetaTXT && qType == dns.TypeSRV:
		return true
	}
	return false
}

// getAnswerExtrasForIP creates the dns answer and extra from IP dnsAddress pairs.
func getAnswerExtrasForIP(name string, addr *dnsAddress, question dns.Question,
	reqType requestType, result *discovery.Result, ttl uint32, domain string, port *discovery.Port) (answer []dns.RR, extra []dns.RR) {
	qType := question.Qtype
	canReturnARecord := qType == dns.TypeSRV || qType == dns.TypeA || qType == dns.TypeANY || qType == dns.TypeNS || qType == dns.TypeTXT
	canReturnAAAARecord := qType == dns.TypeSRV || qType == dns.TypeAAAA || qType == dns.TypeANY || qType == dns.TypeNS || qType == dns.TypeTXT
	if reqType != requestTypeAddress && result.Type != discovery.ResultTypeVirtual {
		switch {
		// check IPV4
		case addr.IsIP() && addr.IsIPV4() && !canReturnARecord,
			// check IPV6
			addr.IsIP() && !addr.IsIPV4() && !canReturnAAAARecord:
			return
		}
	}

	// Have to pass original question name here even if the system has recursed
	// and stripped off the domain suffix.
	recHdrName := question.Name
	if qType == dns.TypeSRV {
		nameSplit := strings.Split(name, ".")
		if len(nameSplit) > 1 && nameSplit[1] == addrLabel {
			recHdrName = name
		} else {
			recHdrName = name
		}
		name = question.Name
	}

	if reqType != requestTypeAddress && qType == dns.TypeSRV {
		if result.Type == discovery.ResultTypeService && addr.IsIP() && result.Node.Address != addr.String() {
			// encode the ip to be used in the header of the A/AAAA record
			// as well as the target of the SRV record.
			recHdrName = encodeIPAsFqdn(result, addr.IP(), domain)
		}
		if result.Type == discovery.ResultTypeWorkload {
			recHdrName = canonicalNameForResult(result.Type, result.Node.Name, domain, result.Tenancy, port.Name)
		}
		srv := makeSRVRecord(name, recHdrName, result, ttl, port)
		answer = append(answer, srv)
	}

	record := makeIPBasedRecord(recHdrName, addr, ttl)

	isARecordWhenNotExplicitlyQueried := record.Header().Rrtype == dns.TypeA && qType != dns.TypeA && qType != dns.TypeANY
	isAAAARecordWhenNotExplicitlyQueried := record.Header().Rrtype == dns.TypeAAAA && qType != dns.TypeAAAA && qType != dns.TypeANY

	// For explicit A/AAAA queries, we must only return those records in the answer section.
	if isARecordWhenNotExplicitlyQueried ||
		isAAAARecordWhenNotExplicitlyQueried {
		extra = append(extra, record)
	} else {
		answer = append(answer, record)
	}

	return
}

// encodeIPAsFqdn encodes an IP address as a FQDN.
func encodeIPAsFqdn(result *discovery.Result, ip net.IP, responseDomain string) string {
	ipv4 := ip.To4()
	ipStr := hex.EncodeToString(ip)
	if ipv4 != nil {
		ipStr = ipStr[len(ipStr)-(net.IPv4len*2):]
	}
	if result.Tenancy.PeerName != "" {
		// Exclude the datacenter from the FQDN on the addr for peers.
		// This technically makes no difference, since the addr endpoint ignores the DC
		// component of the request, but do it anyway for a less confusing experience.
		return fmt.Sprintf("%s.addr.%s", ipStr, responseDomain)
	}
	return fmt.Sprintf("%s.addr.%s.%s", ipStr, result.Tenancy.Datacenter, responseDomain)
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

func (r *Router) makeRecordFromFQDN(result *discovery.Result, req *dns.Msg, reqCtx Context, cfg *RouterDynamicConfig, ttl uint32, remoteAddress net.Addr, maxRecursionLevel int, fqdn string, port *discovery.Port) ([]dns.RR, []dns.RR) {
	edns := req.IsEdns0() != nil
	q := req.Question[0]

	more := r.resolveCNAME(cfg, dns.Fqdn(fqdn), reqCtx, remoteAddress, maxRecursionLevel)
	var additional []dns.RR
	extra := 0
MORE_REC:
	for _, rr := range more {
		switch rr.Header().Rrtype {
		case dns.TypeCNAME, dns.TypeA, dns.TypeAAAA, dns.TypeTXT:
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
		answer := makeSRVRecord(q.Name, fqdn, result, ttl, port)
		return []dns.RR{answer}, additional
	}

	address := ""
	if result.Service != nil && result.Service.Address != "" {
		address = result.Service.Address
	} else if result.Node != nil {
		address = result.Node.Address
	}

	answers := []dns.RR{
		makeCNAMERecord(q.Name, address, ttl),
	}
	answers = append(answers, additional...)

	return answers, nil
}

// makeCNAMERecord returns a CNAME record for the given name and target.
func makeCNAMERecord(name string, target string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: dns.Fqdn(target),
	}
}

// func makeSRVRecord returns an SRV record for the given name and target.
func makeSRVRecord(name, target string, result *discovery.Result, ttl uint32, port *discovery.Port) *dns.SRV {
	return &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 1,
		Weight:   uint16(result.DNS.Weight),
		Port:     uint16(port.Number),
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

// canonicalNameForResult returns the canonical name for a discovery result.
func canonicalNameForResult(resultType discovery.ResultType, target, domain string,
	tenancy discovery.ResultTenancy, portName string) string {
	switch resultType {
	case discovery.ResultTypeService:
		if tenancy.Namespace != "" {
			return fmt.Sprintf("%s.%s.%s.%s.%s", target, "service", tenancy.Namespace, tenancy.Datacenter, domain)
		}
		return fmt.Sprintf("%s.%s.%s.%s", target, "service", tenancy.Datacenter, domain)
	case discovery.ResultTypeNode:
		if tenancy.PeerName != "" && tenancy.Partition != "" {
			// We must return a more-specific DNS name for peering so
			// that there is no ambiguity with lookups.
			// Nodes are always registered in the default namespace, so
			// the `.ns` qualifier is not required.
			return fmt.Sprintf("%s.node.%s.peer.%s.ap.%s",
				target,
				tenancy.PeerName,
				tenancy.Partition,
				domain)
		}
		if tenancy.PeerName != "" {
			// We must return a more-specific DNS name for peering so
			// that there is no ambiguity with lookups.
			return fmt.Sprintf("%s.node.%s.peer.%s",
				target,
				tenancy.PeerName,
				domain)
		}
		// Return a simpler format for non-peering nodes.
		return fmt.Sprintf("%s.node.%s.%s", target, tenancy.Datacenter, domain)
	case discovery.ResultTypeWorkload:
		// TODO (v2-dns): it doesn't appear this is being used to return a result. Need to investigate and refactor
		if portName != "" {
			return fmt.Sprintf("%s.port.%s.workload.%s.ns.%s.ap.%s", portName, target, tenancy.Namespace, tenancy.Partition, domain)
		}
		return fmt.Sprintf("%s.workload.%s.ns.%s.ap.%s", target, tenancy.Namespace, tenancy.Partition, domain)
	}
	return ""
}
