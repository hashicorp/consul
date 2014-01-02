package agent

import (
	"fmt"
	"github.com/miekg/dns"
	"io"
	"log"
	"time"
)

// DNSServer is used to wrap an Agent and expose various
// service discovery endpoints using a DNS interface.
type DNSServer struct {
	agent      *Agent
	dnsHandler *dns.ServeMux
	dnsServer  *dns.Server
	logger     *log.Logger
}

// NewDNSServer starts a new DNS server to provide an agent interface
func NewDNSServer(agent *Agent, logOutput io.Writer, bind string) (*DNSServer, error) {
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
		logger:     log.New(logOutput, "", log.LstdFlags),
	}

	// Register mux handlers
	mux.HandleFunc("consul.", srv.handleConsul)

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
		m.SetQuestion("_test.consul.", dns.TypeANY)

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

// handleConsul is used to handle DNS queries in the ".consul." domain
func (d *DNSServer) handleConsul(resp dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	d.logger.Printf("[DEBUG] dns: request for %v", q)

	if q.Qtype != dns.TypeANY && q.Qtype != dns.TypeTXT {
		return
	}

	// Always respond with TXT "ok"
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	header := dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0}
	txt := &dns.TXT{header, []string{"ok"}}
	m.Answer = append(m.Answer, txt)
	d.addSOA("consul.", m)
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
