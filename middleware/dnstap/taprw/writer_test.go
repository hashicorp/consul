package taprw

import (
	"errors"
	"testing"

	"github.com/coredns/coredns/middleware/dnstap/msg"
	"github.com/coredns/coredns/middleware/dnstap/test"
	mwtest "github.com/coredns/coredns/middleware/test"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

type TapFailer struct {
}

func (TapFailer) TapMessage(*tap.Message) error {
	return errors.New("failed")
}
func (TapFailer) TapBuilder() msg.Builder {
	return msg.Builder{Full: true}
}

func TestDnstapError(t *testing.T) {
	rw := ResponseWriter{
		Query:          new(dns.Msg),
		ResponseWriter: &mwtest.ResponseWriter{},
		Tapper:         TapFailer{},
	}
	if err := rw.WriteMsg(new(dns.Msg)); err != nil {
		t.Errorf("dnstap error during Write: %s", err)
	}
	if rw.DnstapError() == nil {
		t.Fatal("no dnstap error")
	}
}

func testingMsg() (m *dns.Msg) {
	m = new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(4097, true)
	return
}

func TestClientQueryResponse(t *testing.T) {
	trapper := test.TrapTapper{Full: true}
	m := testingMsg()
	rw := ResponseWriter{
		Query:          m,
		Tapper:         &trapper,
		ResponseWriter: &mwtest.ResponseWriter{},
	}
	d := test.TestingData()

	// will the wire-format msg be reported?
	bin, err := m.Pack()
	if err != nil {
		t.Fatal(err)
		return
	}
	d.Packed = bin

	if err := rw.WriteMsg(m); err != nil {
		t.Fatal(err)
		return
	}
	if l := len(trapper.Trap); l != 2 {
		t.Fatalf("%d msg trapped", l)
		return
	}
	want := d.ToClientQuery()
	have := trapper.Trap[0]
	if !test.MsgEqual(want, have) {
		t.Fatalf("query: want: %v\nhave: %v", want, have)
	}
	want = d.ToClientResponse()
	have = trapper.Trap[1]
	if !test.MsgEqual(want, have) {
		t.Fatalf("response: want: %v\nhave: %v", want, have)
	}
}
