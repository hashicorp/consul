package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/miekg/dns"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const (
	testQuery    = "_test.consul."
	consulDomain = "consul."
)

// DNSServer is used to wrap an Agent and expose various
// service discovery endpoints using a DNS interface.
type DNSServer struct {
	agent      *Agent
	dnsHandler *dns.ServeMux
	dnsServer  *dns.Server
	domain     string
	logger     *log.Logger
}

// NewDNSServer starts a new DNS server to provide an agent interface
func NewDNSServer(agent *Agent, logOutput io.Writer, domain, bind string) (*DNSServer, error) {
	// Make sure domain is FQDN
	domain = dns.Fqdn(domain)

	// Construct the DNS components
	mux := dns.NewServeMux()

	// Setup the server
	server := &dns.Server{
		Addr:    bind,
		Net:     "udp",
		Handler: mux,
		UDPSize: 65535,
	}

	// Create the server
	srv := &DNSServer{
		agent:      agent,
		dnsHandler: mux,
		dnsServer:  server,
		domain:     domain,
		logger:     log.New(logOutput, "", log.LstdFlags),
	}

	// Register mux handlers, always handle "consul."
	mux.HandleFunc(domain, srv.handleQuery)
	if domain != consulDomain {
		mux.HandleFunc(consulDomain, srv.handleTest)
	}

	// Async start the DNS Server, handle a potential error
	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		srv.logger.Printf("[ERR] dns: error starting server: %v", err)
		errCh <- err
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
			checkCh <- err
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
	case e := <-checkCh:
		return srv, e
	case <-time.After(time.Second):
		return srv, fmt.Errorf("timeout setting up DNS server")
	}
	return srv, nil
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

	// Setup the message response
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	d.addSOA(d.domain, m)
	defer resp.WriteMsg(m)

	// Dispatch the correct handler
	d.dispatch(req, m)
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
	header := dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0}
	txt := &dns.TXT{header, []string{"ok"}}
	m.Answer = append(m.Answer, txt)
	d.addSOA(consulDomain, m)
	resp.WriteMsg(m)
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
func (d *DNSServer) dispatch(req, resp *dns.Msg) {
	// By default the query is in the default datacenter
	datacenter := d.agent.config.Datacenter

	// Get the QName without the domain suffix
	qName := dns.Fqdn(req.Question[0].Name)
	qName = strings.TrimSuffix(qName, d.domain)

	// Split into the label parts
	labels := dns.SplitDomainName(qName)

	// The last label is either "node", "service" or a datacenter name
PARSE:
	if len(labels) == 0 {
		goto INVALID
	}
	switch labels[len(labels)-1] {
	case "service":
		// Handle lookup with and without tag
		switch len(labels) {
		case 2:
			d.serviceLookup(datacenter, labels[0], "", req, resp)
		case 3:
			d.serviceLookup(datacenter, labels[1], labels[0], req, resp)
		default:
			goto INVALID
		}

	case "node":
		if len(labels) != 2 {
			goto INVALID
		}
		d.nodeLookup(datacenter, labels[0], req, resp)

	default:
		// Store the DC, and re-parse
		datacenter = labels[len(labels)-1]
		labels = labels[:len(labels)-1]
		goto PARSE
	}
	return
INVALID:
	d.logger.Printf("[WARN] dns: QName invalid: %s", qName)
	resp.SetRcode(req, dns.RcodeNameError)
}

// nodeLookup is used to handle a node query
func (d *DNSServer) nodeLookup(datacenter, node string, req, resp *dns.Msg) {
	// Only handle ANY and A type requests
	qType := req.Question[0].Qtype
	if qType != dns.TypeANY && qType != dns.TypeA {
		return
	}

	// Make an RPC request
	args := structs.NodeServicesRequest{
		Datacenter: datacenter,
		Node:       node,
	}
	var out structs.NodeServices
	if err := d.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// If we have no address, return not found!
	if out.Address == "" {
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Parse the IP
	ip := net.ParseIP(out.Address)
	if ip == nil {
		d.logger.Printf("[ERR] dns: failed to parse IP %v for %v", out.Address, node)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Format A record
	aRec := &dns.A{
		Hdr: dns.RR_Header{
			Name:   req.Question[0].Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		A: ip,
	}

	// Add the response
	resp.Answer = append(resp.Answer, aRec)
}

// serviceLookup is used to handle a service query
func (d *DNSServer) serviceLookup(datacenter, service, tag string, req, resp *dns.Msg) {
	// Make an RPC request
	args := structs.ServiceNodesRequest{
		Datacenter:  datacenter,
		ServiceName: service,
		ServiceTag:  tag,
		TagFilter:   tag != "",
	}
	var out structs.ServiceNodes
	if err := d.agent.RPC("Catalog.ServiceNodes", &args, &out); err != nil {
		d.logger.Printf("[ERR] dns: rpc error: %v", err)
		resp.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// If we have no nodes, return not found!
	if len(out) == 0 {
		resp.SetRcode(req, dns.RcodeNameError)
		return
	}

	// Add various responses depending on the request
	qType := req.Question[0].Qtype
	if qType == dns.TypeANY || qType == dns.TypeA {
		d.serviceARecords(out, req, resp)
	}
	if qType == dns.TypeANY || qType == dns.TypeSRV {
		d.serviceSRVRecords(datacenter, out, req, resp)
	}
}

// serviceARecords is used to add the A records for a service lookup
func (d *DNSServer) serviceARecords(nodes structs.ServiceNodes, req, resp *dns.Msg) {
	for _, node := range nodes {
		ip := net.ParseIP(node.Address)
		if ip == nil {
			d.logger.Printf("[ERR] dns: failed to parse IP %v for %v", node.Address, node.Node)
			continue
		}
		aRec := &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: ip,
		}
		resp.Answer = append(resp.Answer, aRec)
	}
}

// serviceARecords is used to add the SRV records for a service lookup
func (d *DNSServer) serviceSRVRecords(dc string, nodes structs.ServiceNodes, req, resp *dns.Msg) {
	for _, node := range nodes {
		srvRec := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			Priority: 1,
			Weight:   1,
			Port:     uint16(node.ServicePort),
			Target:   fmt.Sprintf("%s.node.%s.%s", node.Node, dc, d.domain),
		}
		resp.Answer = append(resp.Answer, srvRec)

		ip := net.ParseIP(node.Address)
		if ip == nil {
			d.logger.Printf("[ERR] dns: failed to parse IP %v for %v", node.Address, node.Node)
			continue
		}
		aRec := &dns.A{
			Hdr: dns.RR_Header{
				Name:   srvRec.Target,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: ip,
		}
		resp.Extra = append(resp.Extra, aRec)
	}
}
