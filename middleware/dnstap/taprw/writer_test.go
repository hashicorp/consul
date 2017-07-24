package taprw

import (
	"errors"
	"testing"

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

func TestDnstapError(t *testing.T) {
	rw := ResponseWriter{
		Query:          new(dns.Msg),
		ResponseWriter: &mwtest.ResponseWriter{},
		Taper:          TapFailer{},
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

func TestClientResponse(t *testing.T) {
	trapper := test.TrapTaper{}
	rw := ResponseWriter{
		Pack:           true,
		Taper:          &trapper,
		ResponseWriter: &mwtest.ResponseWriter{},
	}
	d := test.TestingData()
	m := testingMsg()

	// will the wire-format msg be reported?
	bin, err := m.Pack()
	if err != nil {
		t.Fatal(err)
		return
	}
	d.Packed = bin

	if err := tapResponse(&rw, m); err != nil {
		t.Fatal(err)
		return
	}
	want := d.ToClientResponse()
	if l := len(trapper.Trap); l != 1 {
		t.Fatalf("%d msg trapped", l)
		return
	}
	have := trapper.Trap[0]
	if !test.MsgEqual(want, have) {
		t.Fatalf("want: %v\nhave: %v", want, have)
	}
}

func TestClientQuery(t *testing.T) {
	trapper := test.TrapTaper{}
	rw := ResponseWriter{
		Pack:           false, // no binary this time
		Taper:          &trapper,
		ResponseWriter: &mwtest.ResponseWriter{},
		Query:          testingMsg(),
	}
	if err := tapQuery(&rw); err != nil {
		t.Fatal(err)
		return
	}
	want := test.TestingData().ToClientQuery()
	if l := len(trapper.Trap); l != 1 {
		t.Fatalf("%d msg trapped", l)
		return
	}
	have := trapper.Trap[0]
	if !test.MsgEqual(want, have) {
		t.Fatalf("want: %v\nhave: %v", want, have)
	}
}
