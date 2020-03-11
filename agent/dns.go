package agent

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"regexp"

	metrics "github.com/armon/go-metrics"
	radix "github.com/armon/go-radix"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
)

const (
	// UDP can fit ~25 A records in a 512B response, and ~14 AAAA
	// records. Limit further to prevent unintentional configuration
	// abuse that would have a negative effect on application response
	// times.
	maxUDPAnswerLimit        = 8
	maxRecurseRecords        = 5
	maxRecursionLevelDefault = 3

	// Increment a counter when requests staler than this are served
	staleCounterThreshold = 5 * time.Second

	defaultMaxUDPSize = 512

	MaxDNSLabelLength = 63
)

var InvalidDnsRe = regexp.MustCompile(`[^A-Za-z0-9\\-]+`)

type dnsSOAConfig struct {
	Refresh uint32 // 3600 by default
	Retry   uint32 // 600
	Expire  uint32 // 86400
	Minttl  uint32 // 0
}

type dnsConfig struct {
	AllowStale      bool
	Datacenter      string
	EnableTruncate  bool
	MaxStale        time.Duration
	UseCache        bool
	CacheMaxAge     time.Duration
	NodeName        string
	NodeTTL         time.Duration
	OnlyPassing     bool
	RecursorTimeout time.Duration
	Recursors       []string
	SegmentName     string
	UDPAnswerLimit  int
	ARecordLimit    int
	NodeMetaTXT     bool
	SOAConfig       dnsSOAConfig
	// TTLRadix sets service TTLs by prefix, eg: "database-*"
	TTLRadix *radix.Tree
	// TTLStict sets TTLs to service by full name match. It Has higher priority than TTLRadix
	TTLStrict          map[string]time.Duration
	DisableCompression bool

	enterpriseDNSConfig
}

// DNSServer is used to wrap an Agent and expose various
// service discovery endpoints using a DNS interface.
type DNSServer struct {
	*dns.Server
	agent     *Agent
	mux       *dns.ServeMux
	domain    string
	altDomain string
	logger    hclog.Logger

	// config stores the config as an atomic value (for hot-reloading). It is always of type *dnsConfig
	config atomic.Value

	// recursorEnabled stores whever the recursor handler is enabled as an atomic flag.
	// the recursor handler is only enabled if recursors are configured. This flag is used during config hot-reloading
	recursorEnabled uint32
}

func NewDNSServer(a *Agent) (*DNSServer, error) {
	// Make sure domains are FQDN, make them case insensitive for ServeMux
	domain := dns.Fqdn(strings.ToLower(a.config.DNSDomain))
	altDomain := dns.Fqdn(strings.ToLower(a.config.DNSAltDomain))

	srv := &DNSServer{
		agent:     a,
		domain:    domain,
		altDomain: altDomain,
		logger:    a.logger.Named(logging.DNS),
	}
	cfg, err := GetDNSConfig(a.config)
	if err != nil {
		return nil, err
	}
	srv.config.Store(cfg)

	return srv, nil
}

// GetDNSConfig takes global config and creates the config used by DNS server
func GetDNSConfig(conf *config.RuntimeConfig) (*dnsConfig, error) {
	cfg := &dnsConfig{
		AllowStale:         conf.DNSAllowStale,
		ARecordLimit:       conf.DNSARecordLimit,
		Datacenter:         conf.Datacenter,
		EnableTruncate:     conf.DNSEnableTruncate,
		MaxStale:           conf.DNSMaxStale,
		NodeName:           conf.NodeName,
		NodeTTL:            conf.DNSNodeTTL,
		OnlyPassing:        conf.DNSOnlyPassing,
		RecursorTimeout:    conf.DNSRecursorTimeout,
		SegmentName:        conf.SegmentName,
		UDPAnswerLimit:     conf.DNSUDPAnswerLimit,
		NodeMetaTXT:        conf.DNSNodeMetaTXT,
		DisableCompression: conf.DNSDisableCompression,
		UseCache:           conf.DNSUseCache,
		CacheMaxAge:        conf.DNSCacheMaxAge,
		SOAConfig: dnsSOAConfig{
			Expire:  conf.DNSSOA.Expire,
			Minttl:  conf.DNSSOA.Minttl,
			Refresh: conf.DNSSOA.Refresh,
			Retry:   conf.DNSSOA.Retry,
		},
		enterpriseDNSConfig: getEnterpriseDNSConfig(conf),
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
	}
	for _, r := range conf.DNSRecursors {
		ra, err := recursorAddr(r)
		if err != nil {
			return nil, fmt.Errorf("Invalid recursor address: %v", err)
		}
		cfg.Recursors = append(cfg.Recursors, ra)
	}

	return cfg, nil
}

// GetTTLForService Find the TTL for a given service.
// return ttl, true if found, 0, false otherwise
func (cfg *dnsConfig) GetTTLForService(service string) (time.Duration, bool) {
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

func (d *DNSServer) ListenAndServe(network, addr string, notif func()) error {
	cfg := d.config.Load().(*dnsConfig)

	d.mux = dns.NewServeMux()
	d.mux.HandleFunc("arpa.", d.handlePtr)
	d.mux.HandleFunc(d.domain, d.handleQuery)
	// this is not an empty string check because NewDNSServer will have
	// converted the configured alt domain into an FQDN which will ensure that
	// the value ends with a ".". Therefore "." is the empty string equivalent
	// for originally having no alternate domain set. If there is a reason
	// why consul should be configured to handle the root zone I have yet
	// to think of it.
	if d.altDomain != "." {
		d.mux.HandleFunc(d.altDomain, d.handleQuery)
	}
	d.toggleRecursorHandlerFromConfig(cfg)

	d.Server = &dns.Server{
		Addr:              addr,
		Net:               network,
		Handler:           d.mux,
		NotifyStartedFunc: notif,
	}
	if network == "udp" {
		d.UDPSize = 65535
	}
	return d.Server.ListenAndServe()
}

// toggleRecursorHandlerFromConfig enables or disables the recursor handler based on config idempotently
func (d *DNSServer) toggleRecursorHandlerFromConfig(cfg *dnsConfig) {
	shouldEnable := len(cfg.Recursors) > 0

	if shouldEnable && atomic.CompareAndSwapUint32(&d.recursorEnabled, 0, 1) {
		d.mux.HandleFunc(".", d.handleRecurse)
		d.logger.Debug("recursor enabled")
		return
	}

	if !shouldEnable && atomic.CompareAndSwapUint32(&d.recursorEnabled, 1, 0) {
		d.mux.HandleRemove(".")
		d.logger.Debug("recursor disabled")
		return
	}
}

// ReloadConfig hot-reloads the server config with new parameters under config.RuntimeConfig.DNS*
func (d *DNSServer) ReloadConfig(newCfg *config.RuntimeConfig) error {
	cfg, err := GetDNSConfig(newCfg)
	if err != nil {
		return err
	}
	d.config.Store(cfg)
	d.toggleRecursorHandlerFromConfig(cfg)
	return nil
}

// setEDNS is used to set the responses EDNS size headers and
// possibly the ECS headers as well if they were present in the
// original request
func setEDNS(request *dns.Msg, response *dns.Msg, ecsGlobal bool) {
	// Enable EDNS if enabled
	if edns := request.IsEdns0(); edns != nil {
		// cannot just use the SetEdns0 function as we need to embed
		// the ECS option as well
		ednsResp := new(dns.OPT)
		ednsResp.Hdr.Name = "."
		ednsResp.Hdr.Rrtype = dns.TypeOPT
		ednsResp.SetUDPSize(edns.UDPSize())

		// Setup the ECS option if present
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
}

// recursorAddr is used to add a port to the recursor if omitted.
func recursorAddr(recursor string) (string, error) {
	// Add the port if none
START:
	_, _, err := net.SplitHostPort(recursor)
	if ae, ok := err.(*net.AddrError); ok {
		if ae.Err == "missing port in address" {
			recursor = ipaddr.FormatAddressPort(recursor, 53)
			goto START
		} else if ae.Err == "too many colons in address" {
			if ip := net.ParseIP(recursor); ip != nil && ip.To4() == nil {
				recursor = ipaddr.FormatAddressPort(recursor, 53)
				goto START
			}
		}
	}
	if err != nil {
		return "", err
	}

	// Get the address
	addr, err := net.ResolveTCPAddr("tcp", recursor)
	if err != nil {
		return "", err
	}

	// Return string
	return addr.String(), nil
}

func serviceNodeCanonicalDNSName(sn *structs.ServiceNode, domain string) string {
	return serviceCanonicalDNSName(sn.ServiceName, sn.Datacenter, domain, &sn.EnterpriseMeta)
}

// handlePtr is used to handle "reverse" DNS queries
func (d *DNSServer) handlePtr(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	defer func(s time.Time) {
		metrics.MeasureSinceWithLabels([]string{"dns", "ptr_query"}, s,
			[]metrics.Label{{Name: "node", Value: d.agent.config.NodeName}})
		d.logger.Debug("request served from client",
			"question", q,
			"latency", time.Since(s).String(),
			"client", resp.RemoteAddr().String(),
			"client_network", resp.RemoteAddr().Network(),
		)
	}(time.Now())

	cfg := d.config.Load().(*dnsConfig)

	// Setup the message response
	m := new(dns.Msg)
	m.SetReply(req)
	m.Compress = !cfg.DisableCompression
	m.Authoritative = true
	m.RecursionAvailable = (len(cfg.Recursors) > 0)

	// Only add the SOA if requested
	if req.Question[0].Qtype == dns.TypeSOA {
		d.addSOA(cfg, m)
	}

	datacenter := d.agent.config.Datacenter

	// Get the QName without the domain suffix
	qName := strings.ToLower(dns.Fqdn(req.Question[0].Name))

	args := structs.DCSpecificRequest{
		Datacenter: datacenter,
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.tokens.UserToken(),
			AllowStale: cfg.AllowStale,
		},
	}
	var out structs.IndexedNodes

	// TODO: Replace ListNodes with an internal RPC that can do the filter
	// server side to avoid transferring the entire node list.
	if err := d.agent.RPC("Catalog.ListNodes", &args, &out); err == nil {
		for _, n := range out.Nodes {
			arpa, _ := dns.ReverseAddr(n.Address)
			if arpa == qName {
				ptr := &dns.PTR{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0},
					Ptr: fmt.Sprintf("%s.node.%s.%s", n.Node, datacenter, d.domain),
				}
				m.Answer = append(m.Answer, ptr)
				break
			}
		}
	}

	// only look into the services if we didn't find a node
	if len(m.Answer) == 0 {
		// lookup the service address
		serviceAddress := dnsutil.ExtractAddressFromReverse(qName)
		sargs := structs.ServiceSpecificRequest{
			Datacenter: datacenter,
			QueryOptions: structs.QueryOptions{
				Token:      d.agent.tokens.UserToken(),
				AllowStale: cfg.AllowStale,
			},
			ServiceAddress: serviceAddress,
			EnterpriseMeta: *structs.WildcardEnterpriseMeta(),
		}

		var sout structs.IndexedServiceNodes
		if err := d.agent.RPC("Catalog.ServiceNodes", &sargs, &sout); err == nil {
			for _, n := range sout.ServiceNodes {
				if n.ServiceAddress == serviceAddress {
					ptr := &dns.PTR{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0},
						Ptr: serviceNodeCanonicalDNSName(n, d.domain),
					}
					m.Answer = append(m.Answer, ptr)
					break
				}
			}
		}
	}

	// nothing found locally, recurse
	if len(m.Answer) == 0 {
		d.handleRecurse(resp, req)
		return
	}

	// ptr record responses are globally valid
	setEDNS(req, m, true)

	// Write out the complete response
	if err := resp.WriteMsg(m); err != nil {
		d.logger.Warn("failed to respond", "error", err)
	}
}

// handleQuery is used to handle DNS queries in the configured domain
func (d *DNSServer) handleQuery(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	defer func(s time.Time) {
		metrics.MeasureSinceWithLabels([]string{"dns", "domain_query"}, s,
			[]metrics.Label{{Name: "node", Value: d.agent.config.NodeName}})
		d.logger.Debug("request served from client",
			"name", q.Name,
			"type", dns.Type(q.Qtype),
			"class", dns.Class(q.Qclass),
			"latency", time.Since(s).String(),
			"client", resp.RemoteAddr().String(),
			"client_network", resp.RemoteAddr().Network(),
		)
	}(time.Now())

	// Switch to TCP if the client is
	network := "udp"
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	cfg := d.config.Load().(*dnsConfig)

	// Setup the message response
	m := new(dns.Msg)
	m.SetReply(req)
	m.Compress = !cfg.DisableCompression
	m.Authoritative = true
	m.RecursionAvailable = (len(cfg.Recursors) > 0)

	ecsGlobal := true

	switch req.Question[0].Qtype {
	case dns.TypeSOA:
		ns, glue := d.nameservers(cfg, req.IsEdns0() != nil, maxRecursionLevelDefault, req)
		m.Answer = append(m.Answer, d.soa(cfg))
		m.Ns = append(m.Ns, ns...)
		m.Extra = append(m.Extra, glue...)
		m.SetRcode(req, dns.RcodeSuccess)

	case dns.TypeNS:
		ns, glue := d.nameservers(cfg, req.IsEdns0() != nil, maxRecursionLevelDefault, req)
		m.Answer = ns
		m.Extra = glue
		m.SetRcode(req, dns.RcodeSuccess)

	case dns.TypeAXFR:
		m.SetRcode(req, dns.RcodeNotImplemented)

	default:
		ecsGlobal = d.dispatch(network, resp.RemoteAddr(), req, m)
	}

	setEDNS(req, m, ecsGlobal)

	// Write out the complete response
	if err := resp.WriteMsg(m); err != nil {
		d.logger.Warn("failed to respond", "error", err)
	}
}

func (d *DNSServer) soa(cfg *dnsConfig) *dns.SOA {
	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   d.domain,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			// Has to be consistent with MinTTL to avoid invalidation
			Ttl: cfg.SOAConfig.Minttl,
		},
		Ns:      "ns." + d.domain,
		Serial:  uint32(time.Now().Unix()),
		Mbox:    "hostmaster." + d.domain,
		Refresh: cfg.SOAConfig.Refresh,
		Retry:   cfg.SOAConfig.Retry,
		Expire:  cfg.SOAConfig.Expire,
		Minttl:  cfg.SOAConfig.Minttl,
	}
}

// addSOA is used to add an SOA record to a message for the given domain
func (d *DNSServer) addSOA(cfg *dnsConfig, msg *dns.Msg) {
	msg.Ns = append(msg.Ns, d.soa(cfg))
}

// nameservers returns the names and ip addresses of up to three random servers
// in the current cluster which serve as authoritative name servers for zone.

func (d *DNSServer) nameservers(cfg *dnsConfig, edns bool, maxRecursionLevel int, req *dns.Msg) (ns []dns.RR, extra []dns.RR) {
	out, err := d.lookupServiceNodes(cfg, d.agent.config.Datacenter, structs.ConsulServiceName, "", structs.DefaultEnterpriseMeta(), false, maxRecursionLevel)
	if err != nil {
		d.logger.Warn("Unable to get list of servers", "error", err)
		return nil, nil
	}

	if len(out.Nodes) == 0 {
		d.logger.Warn("no servers found")
		return
	}

	// shuffle the nodes to randomize the output
	out.Nodes.Shuffle()

	for _, o := range out.Nodes {
		name, dc := o.Node.Node, o.Node.Datacenter

		if InvalidDnsRe.MatchString(name) {
			d.logger.Warn("Skipping invalid node for NS records", "node", name)
			continue
		}

		fqdn := name + ".node." + dc + "." + d.domain
		fqdn = dns.Fqdn(strings.ToLower(fqdn))

		// NS record
		nsrr := &dns.NS{
			Hdr: dns.RR_Header{
				Name:   d.domain,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    uint32(cfg.NodeTTL / time.Second),
			},
			Ns: fqdn,
		}
		ns = append(ns, nsrr)

		extra = append(extra, d.makeRecordFromNode(dc, o.Node, dns.TypeANY, fqdn, cfg.NodeTTL, maxRecursionLevel)...)

		// don't provide more than 3 servers
		if len(ns) >= 3 {
			return
		}
	}

	return
}

// dispatch is used to parse a request and invoke the correct handler
func (d *DNSServer) dispatch(network string, remoteAddr net.Addr, req, resp *dns.Msg) (ecsGlobal bool) {
	return d.doDispatch(network, remoteAddr, req, resp, maxRecursionLevelDefault)
}

func (d *DNSServer) invalidQuery(req, resp *dns.Msg, cfg *dnsConfig, qName string) {
	d.logger.Warn("QName invalid", "qname", qName)
	d.addSOA(cfg, resp)
	resp.SetRcode(req, dns.RcodeNameError)
}

func (d *DNSServer) parseDatacenter(labels []string, datacenter *string) bool {
	switch len(labels) {
	case 1:
		*datacenter = labels[0]
		return true
	case 0:
		return true
	default:
		return false
	}
}

// doDispatch is used to parse a request and invoke the correct handler.
// parameter maxRecursionLevel will handle whether recursive call can be performed
func (d *DNSServer) doDispatch(network string, remoteAddr net.Addr, req, resp *dns.Msg, maxRecursionLevel int) (ecsGlobal bool) {
	ecsGlobal = true
	// By default the query is in the default datacenter
	datacenter := d.agent.config.Datacenter

	// have to deref to clone it so we don't modify
	var entMeta structs.EnterpriseMeta

	// Get the QName without the domain suffix
	qName := strings.ToLower(dns.Fqdn(req.Question[0].Name))
	qName = d.trimDomain(qName)

	// Split into the label parts
	labels := dns.SplitDomainName(qName)

	cfg := d.config.Load().(*dnsConfig)

	var queryKind string
	var queryParts []string
	var querySuffixes []string

	done := false
	for i := len(labels) - 1; i >= 0 && !done; i-- {
		switch labels[i] {
		case "service", "connect", "node", "query", "addr":
			queryParts = labels[:i]
			querySuffixes = labels[i+1:]
			queryKind = labels[i]
			done = true
		default:
			// If this is a SRV query the "service" label is optional, we add it back to use the
			// existing code-path.
			if req.Question[0].Qtype == dns.TypeSRV && strings.HasPrefix(labels[i], "_") {
				queryKind = "service"
				queryParts = labels[:i+1]
				querySuffixes = labels[i+1:]
				done = true
			}
		}
	}

	if queryKind == "" {
		goto INVALID
	}

	switch queryKind {
	case "service":
		n := len(queryParts)
		if n < 1 {
			goto INVALID
		}

		if !d.parseDatacenterAndEnterpriseMeta(querySuffixes, cfg, &datacenter, &entMeta) {
			goto INVALID
		}

		// Support RFC 2782 style syntax
		if n == 2 && strings.HasPrefix(queryParts[1], "_") && strings.HasPrefix(queryParts[0], "_") {

			// Grab the tag since we make nuke it if it's tcp
			tag := queryParts[1][1:]

			// Treat _name._tcp.service.consul as a default, no need to filter on that tag
			if tag == "tcp" {
				tag = ""
			}

			// _name._tag.service.consul
			d.serviceLookup(cfg, network, datacenter, queryParts[0][1:], tag, &entMeta, false, req, resp, maxRecursionLevel)

			// Consul 0.3 and prior format for SRV queries
		} else {

			// Support "." in the label, re-join all the parts
			tag := ""
			if n >= 2 {
				tag = strings.Join(queryParts[:n-1], ".")
			}

			// tag[.tag].name.service.consul
			d.serviceLookup(cfg, network, datacenter, queryParts[n-1], tag, &entMeta, false, req, resp, maxRecursionLevel)
		}
	case "connect":
		if len(queryParts) < 1 {
			goto INVALID
		}

		if !d.parseDatacenterAndEnterpriseMeta(querySuffixes, cfg, &datacenter, &entMeta) {
			goto INVALID
		}

		// name.connect.consul
		d.serviceLookup(cfg, network, datacenter, queryParts[len(queryParts)-1], "", &entMeta, true, req, resp, maxRecursionLevel)
	case "node":
		if len(queryParts) < 1 {
			goto INVALID
		}

		if !d.parseDatacenter(querySuffixes, &datacenter) {
			goto INVALID
		}

		// Allow a "." in the node name, just join all the parts
		node := strings.Join(queryParts, ".")
		d.nodeLookup(cfg, network, datacenter, node, req, resp, maxRecursionLevel)
	case "query":
		// ensure we have a query name
		if len(queryParts) < 1 {
			goto INVALID
		}

		if !d.parseDatacenter(querySuffixes, &datacenter) {
			goto INVALID
		}

		// Allow a "." in the query name, just join all the parts.
		query := strings.Join(queryParts, ".")
		ecsGlobal = false
		d.preparedQueryLookup(cfg, network, datacenter, query, remoteAddr, req, resp, maxRecursionLevel)

	case "addr":
		// <address>.addr.<suffixes>.<domain> - addr must be the second label, datacenter is optional
		if len(queryParts) != 1 {
			goto INVALID
		}

		switch len(queryParts[0]) / 2 {
		// IPv4
		case 4:
			ip, err := hex.DecodeString(queryParts[0])
			if err != nil {
				goto INVALID
			}

			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   qName + d.domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(cfg.NodeTTL / time.Second),
				},
				A: ip,
			})
		// IPv6
		case 16:
			ip, err := hex.DecodeString(queryParts[0])
			if err != nil {
				goto INVALID
			}

			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   qName + d.domain,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(cfg.NodeTTL / time.Second),
				},
				AAAA: ip,
			})
		}
	}
	// early return without error
	return

INVALID:
	d.logger.Warn("QName invalid", "qname", qName)
	d.addSOA(cfg, resp)
	resp.SetRcode(req, dns.RcodeNameError)
	return
}

func (d *DNSServer) trimDomain(query string) string {
	longer := d.domain
	shorter := d.altDomain

	if len(shorter) > len(longer) {
		longer, shorter = shorter, longer
	}

	if strings.HasSuffix(query, longer) {
		return strings.TrimSuffix(query, longer)
	}
	return strings.TrimSuffix(query, shorter)
}

// nodeLookup is used to handle a node query
func (d *DNSServer) nodeLookup(cfg *dnsConfig, network, datacenter, node string, req, resp *dns.Msg, maxRecursionLevel int) {
	// Only handle ANY, A, AAAA, and TXT type requests
	qType := req.Question[0].Qtype
	if qType != dns.TypeANY && qType != dns.TypeA && qType != dns.TypeAAAA && qType != dns.TypeTXT {
		return
	}

	// Make an RPC request
	args := &structs.NodeSpecificRequest{
		Datacenter: datacenter,
		Node:       node,
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.tokens.UserToken(),
			AllowStale: cfg.AllowStale,
		},
	}
	out, err := d.lookupNode(cfg, args)
	if err != nil {
		d.logger.Error("rpc error", "error", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// If we have no out.NodeServices.Nodeaddress, return not found!
	if out.NodeServices == nil {
		d.addSOA(cfg, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Add the node record
	n := out.NodeServices.Node

	metaTarget := &resp.Extra
	if qType == dns.TypeTXT || qType == dns.TypeANY {
		metaTarget = &resp.Answer
	}

	q := req.Question[0]
	// Only compute A and CNAME record if query is not TXT type
	if qType != dns.TypeTXT {
		records := d.makeRecordFromNode(n.Datacenter, n, q.Qtype, q.Name, cfg.NodeTTL, maxRecursionLevel)
		resp.Answer = append(resp.Answer, records...)
	}

	if cfg.NodeMetaTXT || qType == dns.TypeTXT || qType == dns.TypeANY {
		metas := d.generateMeta(n.Datacenter, q.Name, n, cfg.NodeTTL)
		*metaTarget = append(*metaTarget, metas...)
	}
}

func (d *DNSServer) lookupNode(cfg *dnsConfig, args *structs.NodeSpecificRequest) (*structs.IndexedNodeServices, error) {
	var out structs.IndexedNodeServices

	useCache := cfg.UseCache
RPC:
	if useCache {
		raw, _, err := d.agent.cache.Get(cachetype.NodeServicesName, args)
		if err != nil {
			return nil, err
		}
		reply, ok := raw.(*structs.IndexedNodeServices)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		out = *reply
	} else {
		if err := d.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
			return nil, err
		}
	}

	// Verify that request is not too stale, redo the request
	if args.AllowStale {
		if out.LastContact > cfg.MaxStale {
			args.AllowStale = false
			useCache = false
			d.logger.Warn("Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"dns", "stale_queries"}, 1)
		}
	}

	return &out, nil
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
	trailingSpacesRE := regexp.MustCompile(" +$")
	numTrailingSpaces := len(trailingSpacesRE.FindString(key))
	key = trailingSpacesRE.ReplaceAllString(key, strings.Repeat("` ", numTrailingSpaces))

	value = strings.Replace(value, "`", "``", -1)

	return key + "=" + value
}

// indexRRs populates a map which indexes a given list of RRs by name. NOTE that
// the names are all squashed to lower case so we can perform case-insensitive
// lookups; the RRs are not modified.
func indexRRs(rrs []dns.RR, index map[string]dns.RR) {
	for _, rr := range rrs {
		name := strings.ToLower(rr.Header().Name)
		if _, ok := index[name]; !ok {
			index[name] = rr
		}
	}
}

// syncExtra takes a DNS response message and sets the extra data to the most
// minimal set needed to cover the answer data. A pre-made index of RRs is given
// so that can be re-used between calls. This assumes that the extra data is
// only used to provide info for SRV records. If that's not the case, then this
// will wipe out any additional data.
func syncExtra(index map[string]dns.RR, resp *dns.Msg) {
	extra := make([]dns.RR, 0, len(resp.Answer))
	resolved := make(map[string]struct{}, len(resp.Answer))
	for _, ansRR := range resp.Answer {
		srv, ok := ansRR.(*dns.SRV)
		if !ok {
			continue
		}

		// Note that we always use lower case when using the index so
		// that compares are not case-sensitive. We don't alter the actual
		// RRs we add into the extra section, however.
		target := strings.ToLower(srv.Target)

	RESOLVE:
		if _, ok := resolved[target]; ok {
			continue
		}
		resolved[target] = struct{}{}

		extraRR, ok := index[target]
		if ok {
			extra = append(extra, extraRR)
			if cname, ok := extraRR.(*dns.CNAME); ok {
				target = strings.ToLower(cname.Target)
				goto RESOLVE
			}
		}
	}
	resp.Extra = extra
}

// dnsBinaryTruncate find the optimal number of records using a fast binary search and return
// it in order to return a DNS answer lower than maxSize parameter.
func dnsBinaryTruncate(resp *dns.Msg, maxSize int, index map[string]dns.RR, hasExtra bool) int {
	originalAnswser := resp.Answer
	startIndex := 0
	endIndex := len(resp.Answer) + 1
	for endIndex-startIndex > 1 {
		median := startIndex + (endIndex-startIndex)/2

		resp.Answer = originalAnswser[:median]
		if hasExtra {
			syncExtra(index, resp)
		}
		aLen := resp.Len()
		if aLen <= maxSize {
			if maxSize-aLen < 10 {
				// We are good, increasing will go out of bounds
				return median
			}
			startIndex = median
		} else {
			endIndex = median
		}
	}
	return startIndex
}

// trimTCPResponse limit the MaximumSize of messages to 64k as it is the limit
// of DNS responses
func (d *DNSServer) trimTCPResponse(req, resp *dns.Msg) (trimmed bool) {
	hasExtra := len(resp.Extra) > 0
	// There is some overhead, 65535 does not work
	maxSize := 65523 // 64k - 12 bytes DNS raw overhead

	// We avoid some function calls and allocations by only handling the
	// extra data when necessary.
	var index map[string]dns.RR
	originalSize := resp.Len()
	originalNumRecords := len(resp.Answer)

	// It is not possible to return more than 4k records even with compression
	// Since we are performing binary search it is not a big deal, but it
	// improves a bit performance, even with binary search
	truncateAt := 4096
	if req.Question[0].Qtype == dns.TypeSRV {
		// More than 1024 SRV records do not fit in 64k
		truncateAt = 1024
	}
	if len(resp.Answer) > truncateAt {
		resp.Answer = resp.Answer[:truncateAt]
	}
	if hasExtra {
		index = make(map[string]dns.RR, len(resp.Extra))
		indexRRs(resp.Extra, index)
	}
	truncated := false

	// This enforces the given limit on 64k, the max limit for DNS messages
	for len(resp.Answer) > 1 && resp.Len() > maxSize {
		truncated = true
		// More than 100 bytes, find with a binary search
		if resp.Len()-maxSize > 100 {
			bestIndex := dnsBinaryTruncate(resp, maxSize, index, hasExtra)
			resp.Answer = resp.Answer[:bestIndex]
		} else {
			resp.Answer = resp.Answer[:len(resp.Answer)-1]
		}
		if hasExtra {
			syncExtra(index, resp)
		}
	}
	if truncated {
		d.logger.Debug("TCP answer to question too large, truncated",
			"question", req.Question,
			"records", fmt.Sprintf("%d/%d", len(resp.Answer), originalNumRecords),
			"size", fmt.Sprintf("%d/%d", resp.Len(), originalSize),
		)
	}
	return truncated
}

// trimUDPResponse makes sure a UDP response is not longer than allowed by RFC
// 1035. Enforce an arbitrary limit that can be further ratcheted down by
// config, and then make sure the response doesn't exceed 512 bytes. Any extra
// records will be trimmed along with answers.
func trimUDPResponse(req, resp *dns.Msg, udpAnswerLimit int) (trimmed bool) {
	numAnswers := len(resp.Answer)
	hasExtra := len(resp.Extra) > 0
	maxSize := defaultMaxUDPSize

	// Update to the maximum edns size
	if edns := req.IsEdns0(); edns != nil {
		if size := edns.UDPSize(); size > uint16(maxSize) {
			maxSize = int(size)
		}
	}

	// We avoid some function calls and allocations by only handling the
	// extra data when necessary.
	var index map[string]dns.RR
	if hasExtra {
		index = make(map[string]dns.RR, len(resp.Extra))
		indexRRs(resp.Extra, index)
	}

	// This cuts UDP responses to a useful but limited number of responses.
	maxAnswers := lib.MinInt(maxUDPAnswerLimit, udpAnswerLimit)
	compress := resp.Compress
	if maxSize == defaultMaxUDPSize && numAnswers > maxAnswers {
		// We disable computation of Len ONLY for non-eDNS request (512 bytes)
		resp.Compress = false
		resp.Answer = resp.Answer[:maxAnswers]
		if hasExtra {
			syncExtra(index, resp)
		}
	}

	// This enforces the given limit on the number bytes. The default is 512 as
	// per the RFC, but EDNS0 allows for the user to specify larger sizes. Note
	// that we temporarily switch to uncompressed so that we limit to a response
	// that will not exceed 512 bytes uncompressed, which is more conservative and
	// will allow our responses to be compliant even if some downstream server
	// uncompresses them.
	// Even when size is too big for one single record, try to send it anyway
	// (useful for 512 bytes messages)
	for len(resp.Answer) > 1 && resp.Len() > maxSize-7 {
		// More than 100 bytes, find with a binary search
		if resp.Len()-maxSize > 100 {
			bestIndex := dnsBinaryTruncate(resp, maxSize, index, hasExtra)
			resp.Answer = resp.Answer[:bestIndex]
		} else {
			resp.Answer = resp.Answer[:len(resp.Answer)-1]
		}
		if hasExtra {
			syncExtra(index, resp)
		}
	}
	// For 512 non-eDNS responses, while we compute size non-compressed,
	// we send result compressed
	resp.Compress = compress
	return len(resp.Answer) < numAnswers
}

// trimDNSResponse will trim the response for UDP and TCP
func (d *DNSServer) trimDNSResponse(cfg *dnsConfig, network string, req, resp *dns.Msg) (trimmed bool) {
	if network != "tcp" {
		trimmed = trimUDPResponse(req, resp, cfg.UDPAnswerLimit)
	} else {
		trimmed = d.trimTCPResponse(req, resp)
	}
	// Flag that there are more records to return in the UDP response
	if trimmed && cfg.EnableTruncate {
		resp.Truncated = true
	}
	return trimmed
}

// lookupServiceNodes returns nodes with a given service.
func (d *DNSServer) lookupServiceNodes(cfg *dnsConfig, datacenter, service, tag string, entMeta *structs.EnterpriseMeta, connect bool, maxRecursionLevel int) (structs.IndexedCheckServiceNodes, error) {
	args := structs.ServiceSpecificRequest{
		Connect:     connect,
		Datacenter:  datacenter,
		ServiceName: service,
		ServiceTags: []string{tag},
		TagFilter:   tag != "",
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.tokens.UserToken(),
			AllowStale: cfg.AllowStale,
			MaxAge:     cfg.CacheMaxAge,
		},
	}

	if entMeta != nil {
		args.EnterpriseMeta = *entMeta
	}

	var out structs.IndexedCheckServiceNodes

	if cfg.UseCache {
		raw, m, err := d.agent.cache.Get(cachetype.HealthServicesName, &args)
		if err != nil {
			return out, err
		}
		reply, ok := raw.(*structs.IndexedCheckServiceNodes)
		if !ok {
			// This should never happen, but we want to protect against panics
			return out, fmt.Errorf("internal error: response type not correct")
		}
		d.logger.Trace("cache results for service",
			"cache_hit", m.Hit,
			"service", service,
		)

		out = *reply
	} else {
		if err := d.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
			return out, err
		}
	}

	if args.AllowStale && out.LastContact > staleCounterThreshold {
		metrics.IncrCounter([]string{"dns", "stale_queries"}, 1)
	}

	// redo the request the response was too stale
	if args.AllowStale && out.LastContact > cfg.MaxStale {
		args.AllowStale = false
		d.logger.Warn("Query results too stale, re-requesting")

		if err := d.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
			return structs.IndexedCheckServiceNodes{}, err
		}
	}

	// Filter out any service nodes due to health checks
	// We copy the slice to avoid modifying the result if it comes from the cache
	nodes := make(structs.CheckServiceNodes, len(out.Nodes))
	copy(nodes, out.Nodes)
	out.Nodes = nodes.Filter(cfg.OnlyPassing)
	return out, nil
}

// serviceLookup is used to handle a service query
func (d *DNSServer) serviceLookup(cfg *dnsConfig, network, datacenter, service, tag string, entMeta *structs.EnterpriseMeta, connect bool, req, resp *dns.Msg, maxRecursionLevel int) {
	out, err := d.lookupServiceNodes(cfg, datacenter, service, tag, entMeta, connect, maxRecursionLevel)
	if err != nil {
		d.logger.Error("rpc error", "error", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		d.addSOA(cfg, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Perform a random shuffle
	out.Nodes.Shuffle()

	// Determine the TTL
	ttl, _ := cfg.GetTTLForService(service)

	// Add various responses depending on the request
	qType := req.Question[0].Qtype
	if qType == dns.TypeSRV {
		d.serviceSRVRecords(cfg, datacenter, out.Nodes, req, resp, ttl, maxRecursionLevel)
	} else {
		d.serviceNodeRecords(cfg, datacenter, out.Nodes, req, resp, ttl, maxRecursionLevel)
	}

	d.trimDNSResponse(cfg, network, req, resp)

	// If the answer is empty and the response isn't truncated, return not found
	if len(resp.Answer) == 0 && !resp.Truncated {
		d.addSOA(cfg, resp)
		return
	}
}

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

// preparedQueryLookup is used to handle a prepared query.
func (d *DNSServer) preparedQueryLookup(cfg *dnsConfig, network, datacenter, query string, remoteAddr net.Addr, req, resp *dns.Msg, maxRecursionLevel int) {
	// Execute the prepared query.
	args := structs.PreparedQueryExecuteRequest{
		Datacenter:    datacenter,
		QueryIDOrName: query,
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.tokens.UserToken(),
			AllowStale: cfg.AllowStale,
			MaxAge:     cfg.CacheMaxAge,
		},

		// Always pass the local agent through. In the DNS interface, there
		// is no provision for passing additional query parameters, so we
		// send the local agent's data through to allow distance sorting
		// relative to ourself on the server side.
		Agent: structs.QuerySource{
			Datacenter: d.agent.config.Datacenter,
			Segment:    d.agent.config.SegmentName,
			Node:       d.agent.config.NodeName,
		},
	}

	subnet := ednsSubnetForRequest(req)

	if subnet != nil {
		args.Source.Ip = subnet.Address.String()
	} else {
		switch v := remoteAddr.(type) {
		case *net.UDPAddr:
			args.Source.Ip = v.IP.String()
		case *net.TCPAddr:
			args.Source.Ip = v.IP.String()
		case *net.IPAddr:
			args.Source.Ip = v.IP.String()
		}
	}

	out, err := d.lookupPreparedQuery(cfg, args)

	// If they give a bogus query name, treat that as a name error,
	// not a full on server error. We have to use a string compare
	// here since the RPC layer loses the type information.
	if err != nil && err.Error() == consul.ErrQueryNotFound.Error() {
		d.addSOA(cfg, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	} else if err != nil {
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// TODO (slackpad) - What's a safe limit we can set here? It seems like
	// with dup filtering done at this level we need to get everything to
	// match the previous behavior. We can optimize by pushing more filtering
	// into the query execution, but for now I think we need to get the full
	// response. We could also choose a large arbitrary number that will
	// likely work in practice, like 10*maxUDPAnswerLimit which should help
	// reduce bandwidth if there are thousands of nodes available.

	// Determine the TTL. The parse should never fail since we vet it when
	// the query is created, but we check anyway. If the query didn't
	// specify a TTL then we will try to use the agent's service-specific
	// TTL configs.
	var ttl time.Duration
	if out.DNS.TTL != "" {
		var err error
		ttl, err = time.ParseDuration(out.DNS.TTL)
		if err != nil {
			d.logger.Warn("Failed to parse TTL for prepared query , ignoring",
				"ttl", out.DNS.TTL,
				"prepared_query", query,
			)
		}
	} else {
		ttl, _ = cfg.GetTTLForService(out.Service)
	}

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		d.addSOA(cfg, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Add various responses depending on the request.
	qType := req.Question[0].Qtype
	if qType == dns.TypeSRV {
		d.serviceSRVRecords(cfg, out.Datacenter, out.Nodes, req, resp, ttl, maxRecursionLevel)
	} else {
		d.serviceNodeRecords(cfg, out.Datacenter, out.Nodes, req, resp, ttl, maxRecursionLevel)
	}

	d.trimDNSResponse(cfg, network, req, resp)

	// If the answer is empty and the response isn't truncated, return not found
	if len(resp.Answer) == 0 && !resp.Truncated {
		d.addSOA(cfg, resp)
		return
	}
}

func (d *DNSServer) lookupPreparedQuery(cfg *dnsConfig, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
	var out structs.PreparedQueryExecuteResponse

RPC:
	if cfg.UseCache {
		raw, m, err := d.agent.cache.Get(cachetype.PreparedQueryName, &args)
		if err != nil {
			return nil, err
		}
		reply, ok := raw.(*structs.PreparedQueryExecuteResponse)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, err
		}

		d.logger.Trace("cache results for prepared query",
			"cache_hit", m.Hit,
			"prepared_query", args.QueryIDOrName,
		)

		out = *reply
	} else {
		if err := d.agent.RPC("PreparedQuery.Execute", &args, &out); err != nil {
			return nil, err
		}
	}

	// Verify that request is not too stale, redo the request.
	if args.AllowStale {
		if out.LastContact > cfg.MaxStale {
			args.AllowStale = false
			d.logger.Warn("Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"dns", "stale_queries"}, 1)
		}
	}

	return &out, nil
}

// serviceNodeRecords is used to add the node records for a service lookup
func (d *DNSServer) serviceNodeRecords(cfg *dnsConfig, dc string, nodes structs.CheckServiceNodes, req, resp *dns.Msg, ttl time.Duration, maxRecursionLevel int) {
	handled := make(map[string]struct{})
	var answerCNAME []dns.RR = nil

	count := 0
	for _, node := range nodes {
		// Add the node record
		had_answer := false
		records, _ := d.nodeServiceRecords(dc, node, req, ttl, cfg, maxRecursionLevel)
		if len(records) == 0 {
			continue
		}

		// Avoid duplicate entries, possible if a node has
		// the same service on multiple ports, etc.
		if _, ok := handled[records[0].String()]; ok {
			continue
		}
		handled[records[0].String()] = struct{}{}

		if records != nil {
			switch records[0].(type) {
			case *dns.CNAME:
				// keep track of the first CNAME + associated RRs but don't add to the resp.Answer yet
				// this will only be added if no non-CNAME RRs are found
				if len(answerCNAME) == 0 {
					answerCNAME = records
				}
			default:
				resp.Answer = append(resp.Answer, records...)
				had_answer = true
			}
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

func findWeight(node structs.CheckServiceNode) int {
	// By default, when only_passing is false, warning and passing nodes are returned
	// Those values will be used if using a client with support while server has no
	// support for weights
	weightPassing := 1
	weightWarning := 1
	if node.Service.Weights != nil {
		weightPassing = node.Service.Weights.Passing
		weightWarning = node.Service.Weights.Warning
	}
	serviceChecks := make(api.HealthChecks, 0)
	for _, c := range node.Checks {
		if c.ServiceName == node.Service.Service || c.ServiceName == "" {
			healthCheck := &api.HealthCheck{
				Node:        c.Node,
				CheckID:     string(c.CheckID),
				Name:        c.Name,
				Status:      c.Status,
				Notes:       c.Notes,
				Output:      c.Output,
				ServiceID:   c.ServiceID,
				ServiceName: c.ServiceName,
				ServiceTags: c.ServiceTags,
			}
			serviceChecks = append(serviceChecks, healthCheck)
		}
	}
	status := serviceChecks.AggregatedStatus()
	switch status {
	case api.HealthWarning:
		return weightWarning
	case api.HealthPassing:
		return weightPassing
	case api.HealthMaint:
		// Not used in theory
		return 0
	case api.HealthCritical:
		// Should not happen since already filtered
		return 0
	default:
		// When non-standard status, return 1
		return 1
	}
}

func (d *DNSServer) encodeIPAsFqdn(dc string, ip net.IP) string {
	ipv4 := ip.To4()
	if ipv4 != nil {
		ipStr := hex.EncodeToString(ip)
		return fmt.Sprintf("%s.addr.%s.%s", ipStr[len(ipStr)-(net.IPv4len*2):], dc, d.domain)
	} else {
		return fmt.Sprintf("%s.addr.%s.%s", hex.EncodeToString(ip), dc, d.domain)
	}
}

func makeARecord(qType uint16, ip net.IP, ttl time.Duration) dns.RR {

	var ipRecord dns.RR
	ipv4 := ip.To4()
	if ipv4 != nil {
		if qType == dns.TypeSRV || qType == dns.TypeA || qType == dns.TypeANY || qType == dns.TypeNS || qType == dns.TypeTXT {
			ipRecord = &dns.A{
				Hdr: dns.RR_Header{
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl / time.Second),
				},
				A: ipv4,
			}
		}
	} else if qType == dns.TypeSRV || qType == dns.TypeAAAA || qType == dns.TypeANY || qType == dns.TypeNS || qType == dns.TypeTXT {
		ipRecord = &dns.AAAA{
			Hdr: dns.RR_Header{
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			AAAA: ip,
		}
	}
	return ipRecord
}

// Craft dns records for a node
// In case of an SRV query the answer will be a IN SRV and additional data will store an IN A to the node IP
// Otherwise it will return a IN A record
func (d *DNSServer) makeRecordFromNode(dc string, node *structs.Node, qType uint16, qName string, ttl time.Duration, maxRecursionLevel int) []dns.RR {
	addrTranslate := TranslateAddressAcceptDomain
	if qType == dns.TypeA {
		addrTranslate |= TranslateAddressAcceptIPv4
	} else if qType == dns.TypeAAAA {
		addrTranslate |= TranslateAddressAcceptIPv6
	} else {
		addrTranslate |= TranslateAddressAcceptAny
	}

	addr := d.agent.TranslateAddress(node.Datacenter, node.Address, node.TaggedAddresses, addrTranslate)
	ip := net.ParseIP(addr)

	var res []dns.RR

	if ip == nil {
		res = append(res, &dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			Target: dns.Fqdn(node.Address),
		})

		res = append(res,
			d.resolveCNAME(d.config.Load().(*dnsConfig), dns.Fqdn(node.Address), maxRecursionLevel)...,
		)

		return res
	}

	ipRecord := makeARecord(qType, ip, ttl)
	if ipRecord == nil {
		return nil
	}

	ipRecord.Header().Name = qName
	return []dns.RR{ipRecord}
}

// Craft dns records for a service
// In case of an SRV query the answer will be a IN SRV and additional data will store an IN A to the node IP
// Otherwise it will return a IN A record
func (d *DNSServer) makeRecordFromServiceNode(dc string, serviceNode structs.CheckServiceNode, addr net.IP, req *dns.Msg, ttl time.Duration) ([]dns.RR, []dns.RR) {
	q := req.Question[0]
	ipRecord := makeARecord(q.Qtype, addr, ttl)
	if ipRecord == nil {
		return nil, nil
	}

	if q.Qtype == dns.TypeSRV {
		nodeFQDN := fmt.Sprintf("%s.node.%s.%s", serviceNode.Node.Node, dc, d.domain)
		answers := []dns.RR{
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl / time.Second),
				},
				Priority: 1,
				Weight:   uint16(findWeight(serviceNode)),
				Port:     uint16(d.agent.TranslateServicePort(dc, serviceNode.Service.Port, serviceNode.Service.TaggedAddresses)),
				Target:   nodeFQDN,
			},
		}

		ipRecord.Header().Name = nodeFQDN
		return answers, []dns.RR{ipRecord}
	}

	ipRecord.Header().Name = q.Name
	return []dns.RR{ipRecord}, nil
}

// Craft dns records for an IP
// In case of an SRV query the answer will be a IN SRV and additional data will store an IN A to the IP
// Otherwise it will return a IN A record
func (d *DNSServer) makeRecordFromIP(dc string, addr net.IP, serviceNode structs.CheckServiceNode, req *dns.Msg, ttl time.Duration) ([]dns.RR, []dns.RR) {
	q := req.Question[0]
	ipRecord := makeARecord(q.Qtype, addr, ttl)
	if ipRecord == nil {
		return nil, nil
	}

	if q.Qtype == dns.TypeSRV {
		ipFQDN := d.encodeIPAsFqdn(dc, addr)
		answers := []dns.RR{
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl / time.Second),
				},
				Priority: 1,
				Weight:   uint16(findWeight(serviceNode)),
				Port:     uint16(d.agent.TranslateServicePort(dc, serviceNode.Service.Port, serviceNode.Service.TaggedAddresses)),
				Target:   ipFQDN,
			},
		}

		ipRecord.Header().Name = ipFQDN
		return answers, []dns.RR{ipRecord}
	}

	ipRecord.Header().Name = q.Name
	return []dns.RR{ipRecord}, nil
}

// Craft dns records for an FQDN
// In case of an SRV query the answer will be a IN SRV and additional data will store an IN A to the IP
// Otherwise it will return a CNAME and a IN A record
func (d *DNSServer) makeRecordFromFQDN(dc string, fqdn string, serviceNode structs.CheckServiceNode, req *dns.Msg, ttl time.Duration, cfg *dnsConfig, maxRecursionLevel int) ([]dns.RR, []dns.RR) {
	edns := req.IsEdns0() != nil
	q := req.Question[0]

	more := d.resolveCNAME(cfg, dns.Fqdn(fqdn), maxRecursionLevel)
	var additional []dns.RR
	extra := 0
MORE_REC:
	for _, rr := range more {
		switch rr.Header().Rrtype {
		case dns.TypeCNAME, dns.TypeA, dns.TypeAAAA:
			// set the TTL manually
			rr.Header().Ttl = uint32(ttl / time.Second)
			additional = append(additional, rr)

			extra++
			if extra == maxRecurseRecords && !edns {
				break MORE_REC
			}
		}
	}

	if q.Qtype == dns.TypeSRV {
		answers := []dns.RR{
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl / time.Second),
				},
				Priority: 1,
				Weight:   uint16(findWeight(serviceNode)),
				Port:     uint16(d.agent.TranslateServicePort(dc, serviceNode.Service.Port, serviceNode.Service.TaggedAddresses)),
				Target:   dns.Fqdn(fqdn),
			},
		}
		return answers, additional
	}

	answers := []dns.RR{
		&dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			Target: dns.Fqdn(fqdn),
		}}
	answers = append(answers, additional...)

	return answers, nil
}

func (d *DNSServer) nodeServiceRecords(dc string, node structs.CheckServiceNode, req *dns.Msg, ttl time.Duration, cfg *dnsConfig, maxRecursionLevel int) ([]dns.RR, []dns.RR) {
	addrTranslate := TranslateAddressAcceptDomain
	if req.Question[0].Qtype == dns.TypeA {
		addrTranslate |= TranslateAddressAcceptIPv4
	} else if req.Question[0].Qtype == dns.TypeAAAA {
		addrTranslate |= TranslateAddressAcceptIPv6
	} else {
		addrTranslate |= TranslateAddressAcceptAny
	}

	serviceAddr := d.agent.TranslateServiceAddress(dc, node.Service.Address, node.Service.TaggedAddresses, addrTranslate)
	nodeAddr := d.agent.TranslateAddress(node.Node.Datacenter, node.Node.Address, node.Node.TaggedAddresses, addrTranslate)
	if serviceAddr == "" && nodeAddr == "" {
		return nil, nil
	}

	nodeIPAddr := net.ParseIP(nodeAddr)
	serviceIPAddr := net.ParseIP(serviceAddr)

	// There is no service address and the node address is an IP
	if serviceAddr == "" && nodeIPAddr != nil {
		if node.Node.Address != nodeAddr {
			// Do not CNAME node address in case of WAN address
			return d.makeRecordFromIP(dc, nodeIPAddr, node, req, ttl)
		}

		return d.makeRecordFromServiceNode(dc, node, nodeIPAddr, req, ttl)
	}

	// There is no service address and the node address is a FQDN (external service)
	if serviceAddr == "" {
		return d.makeRecordFromFQDN(dc, nodeAddr, node, req, ttl, cfg, maxRecursionLevel)
	}

	// The service address is an IP
	if serviceIPAddr != nil {
		return d.makeRecordFromIP(dc, serviceIPAddr, node, req, ttl)
	}

	// If the service address is a CNAME for the service we are looking
	// for then use the node address.
	if dns.Fqdn(serviceAddr) == req.Question[0].Name && nodeIPAddr != nil {
		return d.makeRecordFromServiceNode(dc, node, nodeIPAddr, req, ttl)
	}

	// The service address is a FQDN (external service)
	return d.makeRecordFromFQDN(dc, serviceAddr, node, req, ttl, cfg, maxRecursionLevel)
}

func (d *DNSServer) generateMeta(dc string, qName string, node *structs.Node, ttl time.Duration) []dns.RR {
	var extra []dns.RR
	for key, value := range node.Meta {
		txt := value
		if !strings.HasPrefix(strings.ToLower(key), "rfc1035-") {
			txt = encodeKVasRFC1464(key, value)
		}

		extra = append(extra, &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			Txt: []string{txt},
		})
	}
	return extra
}

// serviceARecords is used to add the SRV records for a service lookup
func (d *DNSServer) serviceSRVRecords(cfg *dnsConfig, dc string, nodes structs.CheckServiceNodes, req, resp *dns.Msg, ttl time.Duration, maxRecursionLevel int) {
	handled := make(map[string]struct{})

	for _, node := range nodes {
		// Avoid duplicate entries, possible if a node has
		// the same service the same port, etc.
		serviceAddress := d.agent.TranslateServiceAddress(dc, node.Service.Address, node.Service.TaggedAddresses, TranslateAddressAcceptAny)
		servicePort := d.agent.TranslateServicePort(dc, node.Service.Port, node.Service.TaggedAddresses)
		tuple := fmt.Sprintf("%s:%s:%d", node.Node.Node, serviceAddress, servicePort)
		if _, ok := handled[tuple]; ok {
			continue
		}
		handled[tuple] = struct{}{}

		answers, extra := d.nodeServiceRecords(dc, node, req, ttl, cfg, maxRecursionLevel)

		resp.Answer = append(resp.Answer, answers...)
		resp.Extra = append(resp.Extra, extra...)

		if cfg.NodeMetaTXT {
			resp.Extra = append(resp.Extra, d.generateMeta(dc, fmt.Sprintf("%s.node.%s.%s", node.Node.Node, dc, d.domain), node.Node, ttl)...)
		}
	}
}

// handleRecurse is used to handle recursive DNS queries
func (d *DNSServer) handleRecurse(resp dns.ResponseWriter, req *dns.Msg) {
	cfg := d.config.Load().(*dnsConfig)

	q := req.Question[0]
	network := "udp"
	defer func(s time.Time) {
		d.logger.Debug("request served from client",
			"question", q,
			"network", network,
			"latency", time.Since(s).String(),
			"client", resp.RemoteAddr().String(),
			"client_network", resp.RemoteAddr().Network(),
		)
	}(time.Now())

	// Switch to TCP if the client is
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	// Recursively resolve
	c := &dns.Client{Net: network, Timeout: cfg.RecursorTimeout}
	var r *dns.Msg
	var rtt time.Duration
	var err error
	for _, recursor := range cfg.Recursors {
		r, rtt, err = c.Exchange(req, recursor)
		// Check if the response is valid and has the desired Response code
		if r != nil && (r.Rcode != dns.RcodeSuccess && r.Rcode != dns.RcodeNameError) {
			d.logger.Debug("recurse failed for question",
				"question", q,
				"rtt", rtt,
				"recursor", recursor,
				"rcode", dns.RcodeToString[r.Rcode],
			)
			// If we still have recursors to forward the query to,
			// we move forward onto the next one else the loop ends
			continue
		} else if err == nil || (r != nil && r.Truncated) {
			// Compress the response; we don't know if the incoming
			// response was compressed or not, so by not compressing
			// we might generate an invalid packet on the way out.
			r.Compress = !cfg.DisableCompression

			// Forward the response
			d.logger.Debug("recurse succeeded for question",
				"question", q,
				"rtt", rtt,
				"recursor", recursor,
			)
			if err := resp.WriteMsg(r); err != nil {
				d.logger.Warn("failed to respond", "error", err)
			}
			return
		}
		d.logger.Error("recurse failed", "error", err)
	}

	// If all resolvers fail, return a SERVFAIL message
	d.logger.Error("all resolvers failed for question from client",
		"question", q,
		"client", resp.RemoteAddr().String(),
		"client_network", resp.RemoteAddr().Network(),
	)
	m := &dns.Msg{}
	m.SetReply(req)
	m.Compress = !cfg.DisableCompression
	m.RecursionAvailable = true
	m.SetRcode(req, dns.RcodeServerFailure)
	if edns := req.IsEdns0(); edns != nil {
		setEDNS(req, m, true)
	}
	resp.WriteMsg(m)
}

// resolveCNAME is used to recursively resolve CNAME records
func (d *DNSServer) resolveCNAME(cfg *dnsConfig, name string, maxRecursionLevel int) []dns.RR {
	// If the CNAME record points to a Consul address, resolve it internally
	// Convert query to lowercase because DNS is case insensitive; d.domain and
	// d.altDomain are already converted

	if ln := strings.ToLower(name); strings.HasSuffix(ln, "."+d.domain) || strings.HasSuffix(ln, "."+d.altDomain) {
		if maxRecursionLevel < 1 {
			d.logger.Error("Infinite recursion detected for name, won't perform any CNAME resolution.", "name", name)
			return nil
		}
		req := &dns.Msg{}
		resp := &dns.Msg{}

		req.SetQuestion(name, dns.TypeANY)
		d.doDispatch("udp", nil, req, resp, maxRecursionLevel-1)

		return resp.Answer
	}

	// Do nothing if we don't have a recursor
	if len(cfg.Recursors) == 0 {
		return nil
	}

	// Ask for any A records
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)

	// Make a DNS lookup request
	c := &dns.Client{Net: "udp", Timeout: cfg.RecursorTimeout}
	var r *dns.Msg
	var rtt time.Duration
	var err error
	for _, recursor := range cfg.Recursors {
		r, rtt, err = c.Exchange(m, recursor)
		if err == nil {
			d.logger.Debug("cname recurse RTT for name",
				"name", name,
				"rtt", rtt,
			)
			return r.Answer
		}
		d.logger.Error("cname recurse failed for name",
			"name", name,
			"error", err,
		)
	}
	d.logger.Error("all resolvers failed for name", "name", name)
	return nil
}
