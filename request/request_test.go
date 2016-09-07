package request

import (
	"testing"

	"github.com/miekg/coredns/middleware/test"

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
