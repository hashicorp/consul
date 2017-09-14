package msg

import (
	"net"
	"reflect"
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

func testRequest(t *testing.T, expected Data, r request.Request) {
	d := Data{}
	if err := d.RemoteAddr(r.W.RemoteAddr()); err != nil {
		t.Fail()
		return
	}
	if d.SocketProto != expected.SocketProto ||
		d.SocketFam != expected.SocketFam ||
		!reflect.DeepEqual(d.Address, expected.Address) ||
		d.Port != expected.Port {
		t.Fatalf("expected: %v, have: %v", expected, d)
		return
	}
}
func TestRequest(t *testing.T) {
	testRequest(t, Data{
		SocketProto: tap.SocketProtocol_UDP,
		SocketFam:   tap.SocketFamily_INET,
		Address:     net.ParseIP("10.240.0.1"),
		Port:        40212,
	}, testingRequest())
}
func testingRequest() request.Request {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(4097, true)
	return request.Request{W: &test.ResponseWriter{}, Req: m}
}
