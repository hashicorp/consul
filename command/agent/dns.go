package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/miekg/dns"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

const (
	testQuery         = "_test.consul."
	consulDomain      = "consul."
	maxRecurseRecords = 3
)

// DNSServer is used to wrap an Agent and expose various
// service discovery endpoints using a DNS interface.
type DNSServer struct {
	agent        *Agent
	config       *DNSConfig
	dnsHandler   *dns.ServeMux
	dnsServer    *dns.Server
	dnsServerTCP *dns.Server
	domain       string
	recursor     string
	logger       *log.Logger
}

// NewDNSServer starts a new DNS server to provide an agent interface
func NewDNSServer(agent *Agent, config *DNSConfig, logOutput io.Writer, domain, bind, recursor string) (*DNSServer, error) {
	// Make sure domain is FQDN
	domain = dns.Fqdn(domain)

	// Construct the DNS components
	mux := dns.NewServeMux()

	// Setup the servers
	server := &dns.Server{
		Addr:    bind,
		Net:     "udp",
		Handler: mux,
		UDPSize: 65535,
	}
	serverTCP := &dns.Server{
		Addr:    bind,
		Net:     "tcp",
		Handler: mux,
	}

	// Create the server
	srv := &DNSServer{
		agent:        agent,
		config:       config,
		dnsHandler:   mux,
		dnsServer:    server,
		dnsServerTCP: serverTCP,
		domain:       domain,
		recursor:     recursor,
		logger:       log.New(logOutput, "", log.LstdFlags),
	}

	// Register mux handlers, always handle "consul."
	mux.HandleFunc(domain, srv.handleQuery)
	if domain != consulDomain {
		mux.HandleFunc(consulDomain, srv.handleTest)
	}
	if recursor != "" {
		recursor, err := recursorAddr(recursor)
		if err != nil {
			return nil, fmt.Errorf("Invalid recursor address: %v", err)
		}
		srv.recursor = recursor
		mux.HandleFunc(".", srv.handleRecurse)
	}

	// Async start the DNS Servers, handle a potential error
	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		srv.logger.Printf("[ERR] dns: error starting udp server: %v", err)
		errCh <- fmt.Errorf("dns udp setup failed: %v", err)
	}()

	errChTCP := make(chan error, 1)
	go func() {
		err := serverTCP.ListenAndServe()
		srv.logger.Printf("[ERR] dns: error starting tcp server: %v", err)
		errChTCP <- fmt.Errorf("dns tcp setup failed: %v", err)
	}()

	// Check the server is running, do a test lookup
	checkCh := make(chan error, 1)
	go func() {
		// This is jank, but we have no way to edge trigger on
		// the start of our server, so we just wait and hope it is up.
		time.Sleep(50 * time.Millisecond)

		m := new(dns.Msg)
		m.SetQuestion(testQuery, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, bind)
		if err != nil {
			checkCh <- fmt.Errorf("dns test query failed: %v", err)
			return
		}

		if len(in.Answer) == 0 {
			checkCh <- fmt.Errorf("no response to test message")
			return
		}
		close(checkCh)
	}()

	// Wait for either the check, listen error, or timeout
	select {
	case e := <-errCh:
		return srv, e
	case e := <-errChTCP:
		return srv, e
	case e := <-checkCh:
		return srv, e
	case <-time.After(time.Second):
		return srv, fmt.Errorf("timeout setting up DNS server")
	}
	return srv, nil
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

// handleQUery is used to handle DNS queries in the configured domain
func (d *DNSServer) handleQuery(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	defer func(s time.Time) {
		d.logger.Printf("[DEBUG] dns: request for %v (%v)", q, time.Now().Sub(s))
	}(time.Now())

	// Check if this is potentially a test query
	if q.Name == testQuery {
		d.handleTest(resp, req)
		return
	}

	// Switch to TCP if the client is
	network := "udp"
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	// Setup the message response
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	m.RecursionAvailable = (d.recursor != "")

	// Only add the SOA if requested
	if req.Question[0].Qtype == dns.TypeSOA {
		d.addSOA(d.domain, m)
	}

	// Dispatch the correct handler
	d.dispatch(network, req, m)

	// Write out the complete response
	if err := resp.WriteMsg(m); err != nil {
		d.logger.Printf("[WARN] dns: failed to respond: %v", err)
	}
}

// handleTest is used to handle DNS queries in the ".consul." domain
func (d *DNSServer) handleTest(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	defer func(s time.Time) {
		d.logger.Printf("[DEBUG] dns: request for %v (%v)", q, time.Now().Sub(s))
	}(time.Now())

	if !(q.Qtype == dns.TypeANY || q.Qtype == dns.TypeTXT) {
		return
	}
	if q.Name != testQuery {
		return
	}

	// Always respond with TXT "ok"
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	m.RecursionAvailable = true
	header := dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0}
	txt := &dns.TXT{header, []string{"ok"}}
	m.Answer = append(m.Answer, txt)
	d.addSOA(consulDomain, m)
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

	// The last label is either "node", "service" or a datacenter name
PARSE:
	n := len(labels)
	if n == 0 {
		goto INVALID
	}
	switch labels[n-1] {
	case "service":
		if n == 1 {
			goto INVALID
		}

		// Extract the service
		service := labels[n-2]

		// Support "." in the label, re-join all the parts
		tag := ""
		if n >= 3 {
			tag = strings.Join(labels[:n-2], ".")
		}

		// Handle lookup with and without tag
		d.serviceLookup(network, datacenter, service, tag, req, resp)

	case "node":
		if len(labels) == 1 {
			goto INVALID
		}
		// Allow a "." in the node name, just join all the parts
		node := strings.Join(labels[:n-1], ".")
		d.nodeLookup(network, datacenter, node, req, resp)

	default:
		// Store the DC, and re-parse
		datacenter = labels[n-1]
		labels = labels[:n-1]
		goto PARSE
	}
	return
INVALID:
	d.logger.Printf("[WARN] dns: QName invalid: %s", qName)
	resp.SetRcode(req, dns.RcodeNameError)
}

// nodeLookup is used to handle a node query
func (d *DNSServer) nodeLookup(network, datacenter, node string, req, resp *dns.Msg) {
	// Only handle ANY and A type requests
	qType := req.Question[0].Qtype
	if qType != dns.TypeANY && qType != dns.TypeA {
		return
	}

	// Make an RPC request
	args := structs.NodeSpecificRequest{
		Datacenter:   datacenter,
		Node:         node,
		QueryOptions: structs.QueryOptions{AllowStale: d.config.AllowStale},
	}
	var out structs.IndexedNodeServices
RPC:
	if err := d.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Verify that request is not too stale, redo the request
	if args.AllowStale && out.LastContact > d.config.MaxStale {
		args.AllowStale = false
		d.logger.Printf("[WARN] dns: Query results too stale, re-requesting")
		goto RPC
	}

	// If we have no address, return not found!
	if out.NodeServices == nil {
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Add the node record
	records := d.formatNodeRecord(&out.NodeServices.Node, req.Question[0].Name,
		qType, d.config.NodeTTL)
	if records != nil {
		resp.Answer = append(resp.Answer, records...)
	}
}

// formatNodeRecord takes a Node and returns an A, AAAA, or CNAME record
func (d *DNSServer) formatNodeRecord(node *structs.Node, qName string, qType uint16, ttl time.Duration) (records []dns.RR) {
	// Parse the IP
	ip := net.ParseIP(node.Address)
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
			Target: dns.Fqdn(node.Address),
		}
		records = append(records, cnRec)

		// Recurse
		more := d.resolveCNAME(cnRec.Target)
		extra := 0
	MORE_REC:
		for _, rr := range more {
			switch rr.Header().Rrtype {
			case dns.TypeA:
				fallthrough
			case dns.TypeAAAA:
				records = append(records, rr)
				extra++
				if extra == maxRecurseRecords {
					break MORE_REC
				}
			}
		}
	}
	return records
}

// serviceLookup is used to handle a service query
func (d *DNSServer) serviceLookup(network, datacenter, service, tag string, req, resp *dns.Msg) {
	// Make an RPC request
	args := structs.ServiceSpecificRequest{
		Datacenter:   datacenter,
		ServiceName:  service,
		ServiceTag:   tag,
		TagFilter:    tag != "",
		QueryOptions: structs.QueryOptions{AllowStale: d.config.AllowStale},
	}
	var out structs.IndexedCheckServiceNodes
RPC:
	if err := d.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Verify that request is not too stale, redo the request
	if args.AllowStale && out.LastContact > d.config.MaxStale {
		args.AllowStale = false
		d.logger.Printf("[WARN] dns: Query results too stale, re-requesting")
		goto RPC
	}

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		resp.SetRcode(req, dns.RcodeNameError)
		return
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
	out.Nodes = d.filterServiceNodes(out.Nodes)

	// Perform a random shuffle
	shuffleServiceNodes(out.Nodes)

	// If the network is not TCP, restrict the number of responses
	if network != "tcp" && len(out.Nodes) > d.config.MaxUDPResponses {
		out.Nodes = out.Nodes[:d.config.MaxUDPResponses]
	}

	// Add various responses depending on the request
	qType := req.Question[0].Qtype
	d.serviceNodeRecords(out.Nodes, req, resp, ttl)

	if qType == dns.TypeSRV {
		d.serviceSRVRecords(datacenter, out.Nodes, req, resp, ttl)
	}
}

// filterServiceNodes is used to filter out nodes that are failing
// health checks to prevent routing to unhealthy nodes
func (d *DNSServer) filterServiceNodes(nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	n := len(nodes)
OUTER:
	for i := 0; i < n; i++ {
		node := nodes[i]
		for _, check := range node.Checks {
			if check.Status == structs.HealthCritical {
				d.logger.Printf("[WARN] dns: node '%s' failing health check '%s: %s', dropping from service '%s'",
					node.Node.Node, check.CheckID, check.Name, node.Service.Service)
				nodes[i], nodes[n-1] = nodes[n-1], structs.CheckServiceNode{}
				n--
				i--
				continue OUTER
			}
		}
	}
	return nodes[:n]
}

// shuffleServiceNodes does an in-place random shuffle using the Fisher-Yates algorithm
func shuffleServiceNodes(nodes structs.CheckServiceNodes) {
	for i := len(nodes) - 1; i > 0; i-- {
		j := rand.Int31() % int32(i+1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

// serviceNodeRecords is used to add the node records for a service lookup
func (d *DNSServer) serviceNodeRecords(nodes structs.CheckServiceNodes, req, resp *dns.Msg, ttl time.Duration) {
	qName := req.Question[0].Name
	qType := req.Question[0].Qtype
	handled := make(map[string]struct{})
	for _, node := range nodes {
		// Avoid duplicate entries, possible if a node has
		// the same service on multiple ports, etc.
		addr := node.Node.Address
		if _, ok := handled[addr]; ok {
			continue
		}
		handled[addr] = struct{}{}

		// Add the node record
		records := d.formatNodeRecord(&node.Node, qName, qType, ttl)
		if records != nil {
			resp.Answer = append(resp.Answer, records...)
		}
	}
}

// serviceARecords is used to add the SRV records for a service lookup
func (d *DNSServer) serviceSRVRecords(dc string, nodes structs.CheckServiceNodes, req, resp *dns.Msg, ttl time.Duration) {
	handled := make(map[string]struct{})
	for _, node := range nodes {
		// Avoid duplicate entries, possible if a node has
		// the same service the same port, etc.
		tuple := fmt.Sprintf("%s:%d", node.Node.Node, node.Service.Port)
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

		// Add the extra record
		records := d.formatNodeRecord(&node.Node, srvRec.Target, dns.TypeANY, ttl)
		if records != nil {
			resp.Extra = append(resp.Extra, records...)
		}
	}
}

// handleRecurse is used to handle recursive DNS queries
func (d *DNSServer) handleRecurse(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	network := "udp"
	defer func(s time.Time) {
		d.logger.Printf("[DEBUG] dns: request for %v (%s) (%v)", q, network, time.Now().Sub(s))
	}(time.Now())

	// Switch to TCP if the client is
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	// Recursively resolve
	c := &dns.Client{Net: network}
	r, rtt, err := c.Exchange(req, d.recursor)

	// On failure, return a SERVFAIL message
	if err != nil {
		d.logger.Printf("[ERR] dns: recurse failed: %v", err)
		m := &dns.Msg{}
		m.SetReply(req)
		m.RecursionAvailable = true
		m.SetRcode(req, dns.RcodeServerFailure)
		resp.WriteMsg(m)
		return
	}
	d.logger.Printf("[DEBUG] dns: recurse RTT for %v (%v)", q, rtt)

	// Forward the response
	if err := resp.WriteMsg(r); err != nil {
		d.logger.Printf("[WARN] dns: failed to respond: %v", err)
	}
}

// resolveCNAME is used to recursively resolve CNAME records
func (d *DNSServer) resolveCNAME(name string) []dns.RR {
	// Do nothing if we don't have a recursor
	if d.recursor == "" {
		return nil
	}

	// Ask for any A records
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)

	// Make a DNS lookup request
	c := &dns.Client{Net: "udp"}
	r, rtt, err := c.Exchange(m, d.recursor)
	if err != nil {
		d.logger.Printf("[ERR] dns: cname recurse failed: %v", err)
		return nil
	}
	d.logger.Printf("[DEBUG] dns: cname recurse RTT for %v (%v)", name, rtt)

	// Return all the answers
	return r.Answer
}
