package request

import (
	"testing"

	"github.com/coredns/coredns/middleware/test"

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
