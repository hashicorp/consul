package request

import (
	"fmt"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestRequestDo(t *testing.T) {
	st := testRequest()

	st.Do()
	if st.do == 0 {
		t.Fatalf("Expected st.do to be set")
	}
}

func TestRequestRemote(t *testing.T) {
	st := testRequest()
	if st.IP() != "10.240.0.1" {
		t.Fatalf("Wrong IP from request")
	}
	p := st.Port()
	if p == "" {
		t.Fatalf("Failed to get Port from request")
	}
	if p != "40212" {
		t.Fatalf("Wrong port from request")
	}
}

func TestRequestMalformed(t *testing.T) {
	m := new(dns.Msg)
	st := Request{Req: m}

	if x := st.QType(); x != 0 {
		t.Errorf("Expected 0 Qtype, got %d", x)
	}

	if x := st.QClass(); x != 0 {
		t.Errorf("Expected 0 QClass, got %d", x)
	}

	if x := st.QName(); x != "." {
		t.Errorf("Expected . Qname, got %s", x)
	}

	if x := st.Name(); x != "." {
		t.Errorf("Expected . Name, got %s", x)
	}

	if x := st.Type(); x != "" {
		t.Errorf("Expected empty Type, got %s", x)
	}

	if x := st.Class(); x != "" {
		t.Errorf("Expected empty Class, got %s", x)
	}
}

func TestRequestScrub(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion("large.example.com.", dns.TypeSRV)
	req := Request{W: &test.ResponseWriter{}, Req: m}

	reply := new(dns.Msg)
	reply.SetReply(m)
	for i := 1; i < 200; i++ {
		reply.Answer = append(reply.Answer, test.SRV(fmt.Sprintf(
			"large.example.com. 10 IN SRV 0 0 80 10-0-0-%d.default.pod.k8s.example.com.",
			i,
		)))
	}

	msg, got := req.Scrub(reply)
	if want := ScrubDone; want != got {
		t.Errorf("want scrub result %d, got %d", want, got)
	}
	if want, got := req.Size(), msg.Len(); want < got {
		t.Errorf("want scrub to reduce message length below %d bytes, got %d bytes", want, got)
	}
	if !msg.Truncated {
		t.Errorf("want scrub to set truncated bit")
	}
}

func BenchmarkRequestDo(b *testing.B) {
	st := testRequest()

	for i := 0; i < b.N; i++ {
		st.Do()
	}
}

func BenchmarkRequestSize(b *testing.B) {
	st := testRequest()

	for i := 0; i < b.N; i++ {
		st.Size()
	}
}

func testRequest() Request {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(4097, true)
	return Request{W: &test.ResponseWriter{}, Req: m}
}
