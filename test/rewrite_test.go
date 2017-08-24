package test

import (
	"bytes"
	"io/ioutil"
	"log"
	"testing"

	"github.com/miekg/dns"
)

func TestRewrite(t *testing.T) {
	t.Parallel()
	corefile := `.:0 {
       rewrite type MX a
       rewrite edns0 local set 0xffee hello-world
       erratic . {
	drop 0
	}
}`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	testMX(t, udp)
	testEdns0(t, udp)
}

func testMX(t *testing.T, server string) {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeMX)

	r, err := dns.Exchange(m, server)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}

	// expect answer section with A record in it
	if len(r.Answer) == 0 {
		t.Error("Expected to at least one RR in the answer section, got none")
	}
	if r.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", r.Answer[0].Header().Rrtype)
	}
	if r.Answer[0].(*dns.A).A.String() != "192.0.2.53" {
		t.Errorf("Expected 192.0.2.53, got: %s", r.Answer[0].(*dns.A).A.String())
	}
}

func testEdns0(t *testing.T, server string) {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)

	r, err := dns.Exchange(m, server)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}

	// expect answer section with A record in it
	if len(r.Answer) == 0 {
		t.Error("Expected to at least one RR in the answer section, got none")
	}
	if r.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", r.Answer[0].Header().Rrtype)
	}
	if r.Answer[0].(*dns.A).A.String() != "192.0.2.53" {
		t.Errorf("Expected 192.0.2.53, got: %s", r.Answer[0].(*dns.A).A.String())
	}
	o := r.IsEdns0()
	if o == nil || len(o.Option) == 0 {
		t.Error("Expected EDNS0 options but got none")
	} else {
		if e, ok := o.Option[0].(*dns.EDNS0_LOCAL); ok {
			if e.Code != 0xffee {
				t.Errorf("Expected EDNS_LOCAL code 0xffee but got %x", e.Code)
			}
			if !bytes.Equal(e.Data, []byte("hello-world")) {
				t.Errorf("Expected EDNS_LOCAL data 'hello-world' but got %q", e.Data)
			}
		} else {
			t.Errorf("Expected EDNS0_LOCAL but got %v", o.Option[0])
		}
	}
}
