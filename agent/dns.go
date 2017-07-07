package agent

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/miekg/dns"
)

const (
	// UDP can fit ~25 A records in a 512B response, and ~14 AAAA
	// records.  Limit further to prevent unintentional configuration
	// abuse that would have a negative effect on application response
	// times.
	maxUDPAnswerLimit = 8
	maxRecurseRecords = 5

	// Increment a counter when requests staler than this are served
	staleCounterThreshold = 5 * time.Second

	defaultMaxUDPSize = 512
)

// DNSServer is used to wrap an Agent and expose various
// service discovery endpoints using a DNS interface.
type DNSServer struct {
	*dns.Server
	agent     *Agent
	config    *DNSConfig
	domain    string
	recursors []string
	logger    *log.Logger

	// disableCompression is the config.DisableCompression flag that can
	// be safely changed at runtime. It always contains a bool and is
	// initialized with the value from config.DisableCompression.
	disableCompression atomic.Value
}

func NewDNSServer(a *Agent) (*DNSServer, error) {
	var recursors []string
	for _, r := range a.config.DNSRecursors {
		ra, err := recursorAddr(r)
		if err != nil {
			return nil, fmt.Errorf("Invalid recursor address: %v", err)
		}
		recursors = append(recursors, ra)
	}

	// Make sure domain is FQDN, make it case insensitive for ServeMux
	domain := dns.Fqdn(strings.ToLower(a.config.Domain))

	srv := &DNSServer{
		agent:     a,
		config:    &a.config.DNSConfig,
		domain:    domain,
		logger:    a.logger,
		recursors: recursors,
	}
	srv.disableCompression.Store(a.config.DNSConfig.DisableCompression)

	return srv, nil
}

func (s *DNSServer) ListenAndServe(network, addr string, notif func()) error {
	mux := dns.NewServeMux()
	mux.HandleFunc("arpa.", s.handlePtr)
	mux.HandleFunc(s.domain, s.handleQuery)
	if len(s.recursors) > 0 {
		mux.HandleFunc(".", s.handleRecurse)
	}

	s.Server = &dns.Server{
		Addr:              addr,
		Net:               network,
		Handler:           mux,
		NotifyStartedFunc: notif,
	}
	if network == "udp" {
		s.UDPSize = 65535
	}
	return s.Server.ListenAndServe()
}

// recursorAddr is used to add a port to the recursor if omitted.
func recursorAddr(recursor string) (string, error) {
	// Add the port if none
START:
	_, _, err := net.SplitHostPort(recursor)
	if ae, ok := err.(*net.AddrError); ok && ae.Err == "missing port in address" {
		recursor = fmt.Sprintf("%s:%d", recursor, 53)
		goto START
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

// handlePtr is used to handle "reverse" DNS queries
func (d *DNSServer) handlePtr(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	defer func(s time.Time) {
		metrics.MeasureSince([]string{"consul", "dns", "ptr_query", d.agent.config.NodeName}, s)
		d.logger.Printf("[DEBUG] dns: request for %v (%v) from client %s (%s)",
			q, time.Now().Sub(s), resp.RemoteAddr().String(),
			resp.RemoteAddr().Network())
	}(time.Now())

	// Setup the message response
	m := new(dns.Msg)
	m.SetReply(req)
	m.Compress = !d.disableCompression.Load().(bool)
	m.Authoritative = true
	m.RecursionAvailable = (len(d.recursors) > 0)

	// Only add the SOA if requested
	if req.Question[0].Qtype == dns.TypeSOA {
		d.addSOA(d.domain, m)
	}

	datacenter := d.agent.config.Datacenter

	// Get the QName without the domain suffix
	qName := strings.ToLower(dns.Fqdn(req.Question[0].Name))

	args := structs.DCSpecificRequest{
		Datacenter: datacenter,
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.config.ACLToken,
			AllowStale: *d.config.AllowStale,
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

	// nothing found locally, recurse
	if len(m.Answer) == 0 {
		d.handleRecurse(resp, req)
		return
	}

	// Enable EDNS if enabled
	if edns := req.IsEdns0(); edns != nil {
		m.SetEdns0(edns.UDPSize(), false)
	}

	// Write out the complete response
	if err := resp.WriteMsg(m); err != nil {
		d.logger.Printf("[WARN] dns: failed to respond: %v", err)
	}
}

// handleQuery is used to handle DNS queries in the configured domain
func (d *DNSServer) handleQuery(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	defer func(s time.Time) {
		metrics.MeasureSince([]string{"consul", "dns", "domain_query", d.agent.config.NodeName}, s)
		d.logger.Printf("[DEBUG] dns: request for %v (%v) from client %s (%s)",
			q, time.Now().Sub(s), resp.RemoteAddr().String(),
			resp.RemoteAddr().Network())
	}(time.Now())

	// Switch to TCP if the client is
	network := "udp"
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	// Setup the message response
	m := new(dns.Msg)
	m.SetReply(req)
	m.Compress = !d.disableCompression.Load().(bool)
	m.Authoritative = true
	m.RecursionAvailable = (len(d.recursors) > 0)

	// Only add the SOA if requested
	if req.Question[0].Qtype == dns.TypeSOA {
		d.addSOA(d.domain, m)
	}

	// Dispatch the correct handler
	d.dispatch(network, req, m)

	// Handle EDNS
	if edns := req.IsEdns0(); edns != nil {
		m.SetEdns0(edns.UDPSize(), false)
	}

	// Write out the complete response
	if err := resp.WriteMsg(m); err != nil {
		d.logger.Printf("[WARN] dns: failed to respond: %v", err)
	}
}

// addSOA is used to add an SOA record to a message for the given domain
func (d *DNSServer) addSOA(domain string, msg *dns.Msg) {
	soa := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		Ns:      "ns." + domain,
		Mbox:    "postmaster." + domain,
		Serial:  uint32(time.Now().Unix()),
		Refresh: 3600,
		Retry:   600,
		Expire:  86400,
		Minttl:  0,
	}
	msg.Ns = append(msg.Ns, soa)
}

// dispatch is used to parse a request and invoke the correct handler
func (d *DNSServer) dispatch(network string, req, resp *dns.Msg) {
	// By default the query is in the default datacenter
	datacenter := d.agent.config.Datacenter

	// Get the QName without the domain suffix
	qName := strings.ToLower(dns.Fqdn(req.Question[0].Name))
	qName = strings.TrimSuffix(qName, d.domain)

	// Split into the label parts
	labels := dns.SplitDomainName(qName)

	// The last label is either "node", "service", "query", "_<protocol>", or a datacenter name
PARSE:
	n := len(labels)
	if n == 0 {
		goto INVALID
	}

	// If this is a SRV query the "service" label is optional, we add it back to use the
	// existing code-path.
	if req.Question[0].Qtype == dns.TypeSRV && strings.HasPrefix(labels[n-1], "_") {
		labels = append(labels, "service")
		n = n + 1
	}

	switch labels[n-1] {
	case "service":
		if n == 1 {
			goto INVALID
		}

		// Support RFC 2782 style syntax
		if n == 3 && strings.HasPrefix(labels[n-2], "_") && strings.HasPrefix(labels[n-3], "_") {

			// Grab the tag since we make nuke it if it's tcp
			tag := labels[n-2][1:]

			// Treat _name._tcp.service.consul as a default, no need to filter on that tag
			if tag == "tcp" {
				tag = ""
			}

			// _name._tag.service.consul
			d.serviceLookup(network, datacenter, labels[n-3][1:], tag, req, resp)

			// Consul 0.3 and prior format for SRV queries
		} else {

			// Support "." in the label, re-join all the parts
			tag := ""
			if n >= 3 {
				tag = strings.Join(labels[:n-2], ".")
			}

			// tag[.tag].name.service.consul
			d.serviceLookup(network, datacenter, labels[n-2], tag, req, resp)
		}

	case "node":
		if n == 1 {
			goto INVALID
		}

		// Allow a "." in the node name, just join all the parts
		node := strings.Join(labels[:n-1], ".")
		d.nodeLookup(network, datacenter, node, req, resp)

	case "query":
		if n == 1 {
			goto INVALID
		}

		// Allow a "." in the query name, just join all the parts.
		query := strings.Join(labels[:n-1], ".")
		d.preparedQueryLookup(network, datacenter, query, req, resp)

	case "addr":
		if n != 2 {
			goto INVALID
		}

		switch len(labels[0]) / 2 {
		// IPv4
		case 4:
			ip, err := hex.DecodeString(labels[0])
			if err != nil {
				goto INVALID
			}

			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   qName + d.domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(d.config.NodeTTL / time.Second),
				},
				A: ip,
			})
		// IPv6
		case 16:
			ip, err := hex.DecodeString(labels[0])
			if err != nil {
				goto INVALID
			}

			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   qName + d.domain,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(d.config.NodeTTL / time.Second),
				},
				AAAA: ip,
			})
		}

	default:
		// Store the DC, and re-parse
		datacenter = labels[n-1]
		labels = labels[:n-1]
		goto PARSE
	}
	return
INVALID:
	d.logger.Printf("[WARN] dns: QName invalid: %s", qName)
	d.addSOA(d.domain, resp)
	resp.SetRcode(req, dns.RcodeNameError)
}

// nodeLookup is used to handle a node query
func (d *DNSServer) nodeLookup(network, datacenter, node string, req, resp *dns.Msg) {
	// Only handle ANY, A and AAAA type requests
	qType := req.Question[0].Qtype
	if qType != dns.TypeANY && qType != dns.TypeA && qType != dns.TypeAAAA {
		return
	}

	// Make an RPC request
	args := structs.NodeSpecificRequest{
		Datacenter: datacenter,
		Node:       node,
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.config.ACLToken,
			AllowStale: *d.config.AllowStale,
		},
	}
	var out structs.IndexedNodeServices
RPC:
	if err := d.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Verify that request is not too stale, redo the request
	if args.AllowStale {
		if out.LastContact > d.config.MaxStale {
			args.AllowStale = false
			d.logger.Printf("[WARN] dns: Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"consul", "dns", "stale_queries"}, 1)
		}
	}

	// If we have no address, return not found!
	if out.NodeServices == nil {
		d.addSOA(d.domain, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Add the node record
	n := out.NodeServices.Node
	edns := req.IsEdns0() != nil
	addr := d.agent.TranslateAddress(datacenter, n.Address, n.TaggedAddresses)
	records := d.formatNodeRecord(out.NodeServices.Node, addr,
		req.Question[0].Name, qType, d.config.NodeTTL, edns)
	if records != nil {
		resp.Answer = append(resp.Answer, records...)
	}
}

// formatNodeRecord takes a Node and returns an A, AAAA, or CNAME record
func (d *DNSServer) formatNodeRecord(node *structs.Node, addr, qName string, qType uint16, ttl time.Duration, edns bool) (records []dns.RR) {
	// Parse the IP
	ip := net.ParseIP(addr)
	var ipv4 net.IP
	if ip != nil {
		ipv4 = ip.To4()
	}
	switch {
	case ipv4 != nil && (qType == dns.TypeANY || qType == dns.TypeA):
		return []dns.RR{&dns.A{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			A: ip,
		}}

	case ip != nil && ipv4 == nil && (qType == dns.TypeANY || qType == dns.TypeAAAA):
		return []dns.RR{&dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			AAAA: ip,
		}}

	case ip == nil && (qType == dns.TypeANY || qType == dns.TypeCNAME ||
		qType == dns.TypeA || qType == dns.TypeAAAA):
		// Get the CNAME
		cnRec := &dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			Target: dns.Fqdn(addr),
		}
		records = append(records, cnRec)

		// Recurse
		more := d.resolveCNAME(cnRec.Target)
		extra := 0
	MORE_REC:
		for _, rr := range more {
			switch rr.Header().Rrtype {
			case dns.TypeCNAME, dns.TypeA, dns.TypeAAAA:
				records = append(records, rr)
				extra++
				if extra == maxRecurseRecords && !edns {
					break MORE_REC
				}
			}
		}
	}
	return records
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

// trimUDPResponse makes sure a UDP response is not longer than allowed by RFC
// 1035. Enforce an arbitrary limit that can be further ratcheted down by
// config, and then make sure the response doesn't exceed 512 bytes. Any extra
// records will be trimmed along with answers.
func trimUDPResponse(config *DNSConfig, req, resp *dns.Msg) (trimmed bool) {
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
	maxAnswers := lib.MinInt(maxUDPAnswerLimit, config.UDPAnswerLimit)
	if maxSize == defaultMaxUDPSize && numAnswers > maxAnswers {
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
	compress := resp.Compress
	resp.Compress = false
	for len(resp.Answer) > 0 && resp.Len() > maxSize {
		resp.Answer = resp.Answer[:len(resp.Answer)-1]
		if hasExtra {
			syncExtra(index, resp)
		}
	}
	resp.Compress = compress

	return len(resp.Answer) < numAnswers
}

// serviceLookup is used to handle a service query
func (d *DNSServer) serviceLookup(network, datacenter, service, tag string, req, resp *dns.Msg) {
	// Make an RPC request
	args := structs.ServiceSpecificRequest{
		Datacenter:  datacenter,
		ServiceName: service,
		ServiceTag:  tag,
		TagFilter:   tag != "",
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.config.ACLToken,
			AllowStale: *d.config.AllowStale,
		},
	}
	var out structs.IndexedCheckServiceNodes
RPC:
	if err := d.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Verify that request is not too stale, redo the request
	if args.AllowStale {
		if out.LastContact > d.config.MaxStale {
			args.AllowStale = false
			d.logger.Printf("[WARN] dns: Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"consul", "dns", "stale_queries"}, 1)
		}
	}

	// Determine the TTL
	var ttl time.Duration
	if d.config.ServiceTTL != nil {
		var ok bool
		ttl, ok = d.config.ServiceTTL[service]
		if !ok {
			ttl = d.config.ServiceTTL["*"]
		}
	}

	// Filter out any service nodes due to health checks
	out.Nodes = out.Nodes.Filter(d.config.OnlyPassing)

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		d.addSOA(d.domain, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Perform a random shuffle
	out.Nodes.Shuffle()

	// Add various responses depending on the request
	qType := req.Question[0].Qtype
	if qType == dns.TypeSRV {
		d.serviceSRVRecords(datacenter, out.Nodes, req, resp, ttl)
	} else {
		d.serviceNodeRecords(datacenter, out.Nodes, req, resp, ttl)
	}

	// If the network is not TCP, restrict the number of responses
	if network != "tcp" {
		wasTrimmed := trimUDPResponse(d.config, req, resp)

		// Flag that there are more records to return in the UDP response
		if wasTrimmed && d.config.EnableTruncate {
			resp.Truncated = true
		}
	}

	// If the answer is empty and the response isn't truncated, return not found
	if len(resp.Answer) == 0 && !resp.Truncated {
		d.addSOA(d.domain, resp)
		return
	}
}

// preparedQueryLookup is used to handle a prepared query.
func (d *DNSServer) preparedQueryLookup(network, datacenter, query string, req, resp *dns.Msg) {
	// Execute the prepared query.
	args := structs.PreparedQueryExecuteRequest{
		Datacenter:    datacenter,
		QueryIDOrName: query,
		QueryOptions: structs.QueryOptions{
			Token:      d.agent.config.ACLToken,
			AllowStale: *d.config.AllowStale,
		},

		// Always pass the local agent through. In the DNS interface, there
		// is no provision for passing additional query parameters, so we
		// send the local agent's data through to allow distance sorting
		// relative to ourself on the server side.
		Agent: structs.QuerySource{
			Datacenter: d.agent.config.Datacenter,
			Node:       d.agent.config.NodeName,
		},
	}

	// TODO (slackpad) - What's a safe limit we can set here? It seems like
	// with dup filtering done at this level we need to get everything to
	// match the previous behavior. We can optimize by pushing more filtering
	// into the query execution, but for now I think we need to get the full
	// response. We could also choose a large arbitrary number that will
	// likely work in practice, like 10*maxUDPAnswerLimit which should help
	// reduce bandwidth if there are thousands of nodes available.

	var out structs.PreparedQueryExecuteResponse
RPC:
	if err := d.agent.RPC("PreparedQuery.Execute", &args, &out); err != nil {
		// If they give a bogus query name, treat that as a name error,
		// not a full on server error. We have to use a string compare
		// here since the RPC layer loses the type information.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			d.addSOA(d.domain, resp)
			resp.SetRcode(req, dns.RcodeNameError)
			return
		}

		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Verify that request is not too stale, redo the request.
	if args.AllowStale {
		if out.LastContact > d.config.MaxStale {
			args.AllowStale = false
			d.logger.Printf("[WARN] dns: Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"consul", "dns", "stale_queries"}, 1)
		}
	}

	// Determine the TTL. The parse should never fail since we vet it when
	// the query is created, but we check anyway. If the query didn't
	// specify a TTL then we will try to use the agent's service-specific
	// TTL configs.
	var ttl time.Duration
	if out.DNS.TTL != "" {
		var err error
		ttl, err = time.ParseDuration(out.DNS.TTL)
		if err != nil {
			d.logger.Printf("[WARN] dns: Failed to parse TTL '%s' for prepared query '%s', ignoring", out.DNS.TTL, query)
		}
	} else if d.config.ServiceTTL != nil {
		var ok bool
		ttl, ok = d.config.ServiceTTL[out.Service]
		if !ok {
			ttl = d.config.ServiceTTL["*"]
		}
	}

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		d.addSOA(d.domain, resp)
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Add various responses depending on the request.
	qType := req.Question[0].Qtype
	if qType == dns.TypeSRV {
		d.serviceSRVRecords(out.Datacenter, out.Nodes, req, resp, ttl)
	} else {
		d.serviceNodeRecords(out.Datacenter, out.Nodes, req, resp, ttl)
	}

	// If the network is not TCP, restrict the number of responses.
	if network != "tcp" {
		wasTrimmed := trimUDPResponse(d.config, req, resp)

		// Flag that there are more records to return in the UDP response
		if wasTrimmed && d.config.EnableTruncate {
			resp.Truncated = true
		}
	}

	// If the answer is empty and the response isn't truncated, return not found
	if len(resp.Answer) == 0 && !resp.Truncated {
		d.addSOA(d.domain, resp)
		return
	}
}

// serviceNodeRecords is used to add the node records for a service lookup
func (d *DNSServer) serviceNodeRecords(dc string, nodes structs.CheckServiceNodes, req, resp *dns.Msg, ttl time.Duration) {
	qName := req.Question[0].Name
	qType := req.Question[0].Qtype
	handled := make(map[string]struct{})
	edns := req.IsEdns0() != nil

	for _, node := range nodes {
		// Start with the translated address but use the service address,
		// if specified.
		addr := d.agent.TranslateAddress(dc, node.Node.Address, node.Node.TaggedAddresses)
		if node.Service.Address != "" {
			addr = node.Service.Address
		}

		// If the service address is a CNAME for the service we are looking
		// for then use the node address.
		if qName == strings.TrimSuffix(addr, ".")+"." {
			addr = node.Node.Address
		}

		// Avoid duplicate entries, possible if a node has
		// the same service on multiple ports, etc.
		if _, ok := handled[addr]; ok {
			continue
		}
		handled[addr] = struct{}{}

		// Add the node record
		records := d.formatNodeRecord(node.Node, addr, qName, qType, ttl, edns)
		if records != nil {
			resp.Answer = append(resp.Answer, records...)
		}
	}
}

// serviceARecords is used to add the SRV records for a service lookup
func (d *DNSServer) serviceSRVRecords(dc string, nodes structs.CheckServiceNodes, req, resp *dns.Msg, ttl time.Duration) {
	handled := make(map[string]struct{})
	edns := req.IsEdns0() != nil

	for _, node := range nodes {
		// Avoid duplicate entries, possible if a node has
		// the same service the same port, etc.
		tuple := fmt.Sprintf("%s:%s:%d", node.Node.Node, node.Service.Address, node.Service.Port)
		if _, ok := handled[tuple]; ok {
			continue
		}
		handled[tuple] = struct{}{}

		// Add the SRV record
		srvRec := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl / time.Second),
			},
			Priority: 1,
			Weight:   1,
			Port:     uint16(node.Service.Port),
			Target:   fmt.Sprintf("%s.node.%s.%s", node.Node.Node, dc, d.domain),
		}
		resp.Answer = append(resp.Answer, srvRec)

		// Start with the translated address but use the service address,
		// if specified.
		addr := d.agent.TranslateAddress(dc, node.Node.Address, node.Node.TaggedAddresses)
		if node.Service.Address != "" {
			addr = node.Service.Address
		}

		// Add the extra record
		records := d.formatNodeRecord(node.Node, addr, srvRec.Target, dns.TypeANY, ttl, edns)
		if len(records) > 0 {
			// Use the node address if it doesn't differ from the service address
			if addr == node.Node.Address {
				resp.Extra = append(resp.Extra, records...)
			} else {
				// If it differs from the service address, give a special response in the
				// 'addr.consul' domain with the service IP encoded in it. We have to do
				// this because we can't put an IP in the target field of an SRV record.
				switch record := records[0].(type) {
				// IPv4
				case *dns.A:
					addr := hex.EncodeToString(record.A)

					// Take the last 8 chars (4 bytes) of the encoded address to avoid junk bytes
					srvRec.Target = fmt.Sprintf("%s.addr.%s.%s", addr[len(addr)-(net.IPv4len*2):], dc, d.domain)
					record.Hdr.Name = srvRec.Target
					resp.Extra = append(resp.Extra, record)

				// IPv6
				case *dns.AAAA:
					srvRec.Target = fmt.Sprintf("%s.addr.%s.%s", hex.EncodeToString(record.AAAA), dc, d.domain)
					record.Hdr.Name = srvRec.Target
					resp.Extra = append(resp.Extra, record)

				// Something else (probably a CNAME; just add the records).
				default:
					resp.Extra = append(resp.Extra, records...)
				}
			}
		}
	}
}

// handleRecurse is used to handle recursive DNS queries
func (d *DNSServer) handleRecurse(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	network := "udp"
	defer func(s time.Time) {
		d.logger.Printf("[DEBUG] dns: request for %v (%s) (%v) from client %s (%s)",
			q, network, time.Now().Sub(s), resp.RemoteAddr().String(),
			resp.RemoteAddr().Network())
	}(time.Now())

	// Switch to TCP if the client is
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	// Recursively resolve
	c := &dns.Client{Net: network, Timeout: d.config.RecursorTimeout}
	var r *dns.Msg
	var rtt time.Duration
	var err error
	for _, recursor := range d.recursors {
		r, rtt, err = c.Exchange(req, recursor)
		if err == nil || err == dns.ErrTruncated {
			// Compress the response; we don't know if the incoming
			// response was compressed or not, so by not compressing
			// we might generate an invalid packet on the way out.
			r.Compress = !d.disableCompression.Load().(bool)

			// Forward the response
			d.logger.Printf("[DEBUG] dns: recurse RTT for %v (%v)", q, rtt)
			if err := resp.WriteMsg(r); err != nil {
				d.logger.Printf("[WARN] dns: failed to respond: %v", err)
			}
			return
		}
		d.logger.Printf("[ERR] dns: recurse failed: %v", err)
	}

	// If all resolvers fail, return a SERVFAIL message
	d.logger.Printf("[ERR] dns: all resolvers failed for %v from client %s (%s)",
		q, resp.RemoteAddr().String(), resp.RemoteAddr().Network())
	m := &dns.Msg{}
	m.SetReply(req)
	m.Compress = !d.disableCompression.Load().(bool)
	m.RecursionAvailable = true
	m.SetRcode(req, dns.RcodeServerFailure)
	if edns := req.IsEdns0(); edns != nil {
		m.SetEdns0(edns.UDPSize(), false)
	}
	resp.WriteMsg(m)
}

// resolveCNAME is used to recursively resolve CNAME records
func (d *DNSServer) resolveCNAME(name string) []dns.RR {
	// If the CNAME record points to a Consul address, resolve it internally
	// Convert query to lowercase because DNS is case insensitive; d.domain is
	// already converted
	if strings.HasSuffix(strings.ToLower(name), "."+d.domain) {
		req := &dns.Msg{}
		resp := &dns.Msg{}

		req.SetQuestion(name, dns.TypeANY)
		d.dispatch("udp", req, resp)

		return resp.Answer
	}

	// Do nothing if we don't have a recursor
	if len(d.recursors) == 0 {
		return nil
	}

	// Ask for any A records
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)

	// Make a DNS lookup request
	c := &dns.Client{Net: "udp", Timeout: d.config.RecursorTimeout}
	var r *dns.Msg
	var rtt time.Duration
	var err error
	for _, recursor := range d.recursors {
		r, rtt, err = c.Exchange(m, recursor)
		if err == nil {
			d.logger.Printf("[DEBUG] dns: cname recurse RTT for %v (%v)", name, rtt)
			return r.Answer
		}
		d.logger.Printf("[ERR] dns: cname recurse failed for %v: %v", name, err)
	}
	d.logger.Printf("[ERR] dns: all resolvers failed for %v", name)
	return nil
}
