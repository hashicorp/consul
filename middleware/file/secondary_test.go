package file

import (
	"net"
	"sync"
	"testing"
	"time"

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

func TCPServer(laddr string) (*dns.Server, string, error) {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, "", err
	}

	server := &dns.Server{Listener: l, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = waitLock.Unlock

	go func() {
		server.ActivateAndServe()
		l.Close()
	}()

	waitLock.Lock()
	return server, l.Addr().String(), nil
}

func UDPServer(laddr string) (*dns.Server, string, chan bool, error) {
	pc, err := net.ListenPacket("udp", laddr)
	if err != nil {
		return nil, "", nil, err
	}
	server := &dns.Server{PacketConn: pc, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = waitLock.Unlock

	stop := make(chan bool)

	go func() {
		server.ActivateAndServe()
		close(stop)
		pc.Close()
	}()

	waitLock.Lock()
	return server, pc.LocalAddr().String(), stop, nil
}

type soa struct {
	serial uint32
}

func (s *soa) Handler(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Answer = make([]dns.RR, 1)
	m.Answer[0] = &dns.SOA{Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 100}, Ns: "bla.", Mbox: "bla.", Serial: s.serial}
	w.WriteMsg(m)
}

func TestShouldTransfer(t *testing.T) {
	soa := soa{250}

	dns.HandleFunc("secondary.miek.nl.", soa.Handler)
	defer dns.HandleRemove("secondary.miek.nl.")

	s, addrstr, err := TCPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to run test server: %v", err)
	}
	defer s.Shutdown()

	z := new(Zone)
	z.name = "secondary.miek.nl."
	z.TransferFrom = []string{addrstr}

	// Serial smaller
	z.SOA = &dns.SOA{Hdr: dns.RR_Header{Name: "secondary.miek.nl.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 100}, Ns: "bla.", Mbox: "bla.", Serial: soa.serial - 1}
	should, err := z.shouldTransfer()
	if err != nil {
		t.Fatalf("unable to run shouldTransfer: %v", err)
	}
	if !should {
		t.Fatalf("shouldTransfer should return true for serial: %q", soa.serial-1)
	}
	// Serial equal
	z.SOA = &dns.SOA{Hdr: dns.RR_Header{Name: "secondary.miek.nl.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 100}, Ns: "bla.", Mbox: "bla.", Serial: soa.serial}
	should, err = z.shouldTransfer()
	if err != nil {
		t.Fatalf("unable to run shouldTransfer: %v", err)
	}
	if should {
		t.Fatalf("shouldTransfer should return false for serial: %d", soa.serial)
	}
}
