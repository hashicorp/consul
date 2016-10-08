package file

import (
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

// TODO(miek): should test notifies as well, ie start test server (a real coredns one)...
// setup other test server that sends notify, see if CoreDNS comes calling for a zone
// tranfer

func TestLess(t *testing.T) {
	const (
		min  = 0
		max  = 4294967295
		low  = 12345
		high = 4000000000
	)

	if less(min, max) {
		t.Fatalf("less: should be false")
	}
	if !less(max, min) {
		t.Fatalf("less: should be true")
	}
	if !less(high, low) {
		t.Fatalf("less: should be true")
	}
	if !less(7, 9) {
		t.Fatalf("less; should be true")
	}
}

type soa struct {
	serial uint32
}

func (s *soa) Handler(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	switch req.Question[0].Qtype {
	case dns.TypeSOA:
		m.Answer = make([]dns.RR, 1)
		m.Answer[0] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
		w.WriteMsg(m)
	case dns.TypeAXFR:
		m.Answer = make([]dns.RR, 4)
		m.Answer[0] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
		m.Answer[1] = test.A(fmt.Sprintf("%s IN A 127.0.0.1", testZone))
		m.Answer[2] = test.A(fmt.Sprintf("%s IN A 127.0.0.1", testZone))
		m.Answer[3] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
		w.WriteMsg(m)
	}
}

func (s *soa) TransferHandler(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Answer = make([]dns.RR, 1)
	m.Answer[0] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
	w.WriteMsg(m)
}

const testZone = "secondary.miek.nl."

func TestShouldTransfer(t *testing.T) {
	soa := soa{250}
	log.SetOutput(ioutil.Discard)

	dns.HandleFunc(testZone, soa.Handler)
	defer dns.HandleRemove(testZone)

	s, addrstr, err := test.TCPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to run test server: %v", err)
	}
	defer s.Shutdown()

	z := new(Zone)
	z.origin = testZone
	z.TransferFrom = []string{addrstr}

	// when we have a nil SOA (initial state)
	should, err := z.shouldTransfer()
	if err != nil {
		t.Fatalf("unable to run shouldTransfer: %v", err)
	}
	if !should {
		t.Fatalf("shouldTransfer should return true for serial: %d", soa.serial)
	}
	// Serial smaller
	z.Apex.SOA = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, soa.serial-1))
	should, err = z.shouldTransfer()
	if err != nil {
		t.Fatalf("unable to run shouldTransfer: %v", err)
	}
	if !should {
		t.Fatalf("shouldTransfer should return true for serial: %q", soa.serial-1)
	}
	// Serial equal
	z.Apex.SOA = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, soa.serial))
	should, err = z.shouldTransfer()
	if err != nil {
		t.Fatalf("unable to run shouldTransfer: %v", err)
	}
	if should {
		t.Fatalf("shouldTransfer should return false for serial: %d", soa.serial)
	}
}

func TestTransferIn(t *testing.T) {
	soa := soa{250}
	log.SetOutput(ioutil.Discard)

	dns.HandleFunc(testZone, soa.Handler)
	defer dns.HandleRemove(testZone)

	s, addrstr, err := test.TCPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to run test server: %v", err)
	}
	defer s.Shutdown()

	z := new(Zone)
	z.Expired = new(bool)
	z.origin = testZone
	z.TransferFrom = []string{addrstr}

	err = z.TransferIn()
	if err != nil {
		t.Fatalf("unable to run TransferIn: %v", err)
	}
	if z.Apex.SOA.String() != fmt.Sprintf("%s	3600	IN	SOA	bla. bla. 250 0 0 0 0", testZone) {
		t.Fatalf("unknown SOA transferred")
	}
}

func TestIsNotify(t *testing.T) {
	z := new(Zone)
	z.Expired = new(bool)
	z.origin = testZone
	state := newRequest(testZone, dns.TypeSOA)
	// need to set opcode
	state.Req.Opcode = dns.OpcodeNotify

	z.TransferFrom = []string{"10.240.0.1:53"} // IP from from testing/responseWriter
	if !z.isNotify(state) {
		t.Fatal("should have been valid notify")
	}
	z.TransferFrom = []string{"10.240.0.2:53"}
	if z.isNotify(state) {
		t.Fatal("should have been invalid notify")
	}
}

func newRequest(zone string, qtype uint16) request.Request {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(4097, true)
	return request.Request{W: &test.ResponseWriter{}, Req: m}
}
