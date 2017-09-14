package test

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestAuto(t *testing.T) {
	t.Parallel()
	tmpdir, err := ioutil.TempDir(os.TempDir(), "coredns")
	if err != nil {
		t.Fatal(err)
	}

	corefile := `org:0 {
		auto {
			directory ` + tmpdir + ` db\.(.*) {1} 1
		}
	}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "www.example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("Expected reply to be a SERVFAIL, got %d", resp.Rcode)
	}

	// Write db.example.org to get example.org.
	if err = ioutil.WriteFile(path.Join(tmpdir, "db.example.org"), []byte(zoneContent), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1500 * time.Millisecond) // wait for it to be picked up

	resp, err = p.Lookup(state, "www.example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("Expected 1 RR in the answer section, got %d", len(resp.Answer))
	}

	// Remove db.example.org again.
	os.Remove(path.Join(tmpdir, "db.example.org"))

	time.Sleep(1100 * time.Millisecond) // wait for it to be picked up
	resp, err = p.Lookup(state, "www.example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("Expected reply to be a SERVFAIL, got %d", resp.Rcode)
	}
}

func TestAutoNonExistentZone(t *testing.T) {
	t.Parallel()
	tmpdir, err := ioutil.TempDir(os.TempDir(), "coredns")
	if err != nil {
		t.Fatal(err)
	}
	log.SetOutput(ioutil.Discard)

	corefile := `.:0 {
		auto {
			directory ` + tmpdir + ` (.*) {1} 1
		}
		errors stdout
	}
`

	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatal("Could not get UDP listening port")
	}
	defer i.Stop()

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("Expected reply to be a SERVFAIL, got %d", resp.Rcode)
	}
}

func TestAutoAXFR(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	tmpdir, err := ioutil.TempDir(os.TempDir(), "coredns")
	if err != nil {
		t.Fatal(err)
	}

	corefile := `org:0 {
		auto {
			directory ` + tmpdir + ` db\.(.*) {1} 1
			transfer to *
		}
	}
`

	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatal("Could not get UDP listening port")
	}
	defer i.Stop()

	// Write db.example.org to get example.org.
	if err = ioutil.WriteFile(path.Join(tmpdir, "db.example.org"), []byte(zoneContent), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1100 * time.Millisecond) // wait for it to be picked up

	p := proxy.NewLookup([]string{udp})
	m := new(dns.Msg)
	m.SetAxfr("example.org.")
	state := request.Request{W: &test.ResponseWriter{}, Req: m}

	resp, err := p.Lookup(state, "example.org.", dns.TypeAXFR)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if len(resp.Answer) != 5 {
		t.Fatalf("Expected response with %d RRs, got %d", 5, len(resp.Answer))
	}
}

const zoneContent = `; testzone
@	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2016082534 7200 3600 1209600 3600
		NS	a.iana-servers.net.
		NS	b.iana-servers.net.

www IN A 127.0.0.1
`
