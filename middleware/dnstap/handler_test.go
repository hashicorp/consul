package dnstap

import (
	"errors"
	"fmt"
	"testing"

	"github.com/coredns/coredns/middleware/dnstap/test"
	mwtest "github.com/coredns/coredns/middleware/test"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func testCase(t *testing.T, tapq, tapr *tap.Message, q, r *dns.Msg) {
	w := writer{}
	w.queue = append(w.queue, tapq, tapr)
	h := Dnstap{
		Next: mwtest.HandlerFunc(func(_ context.Context,
			w dns.ResponseWriter, _ *dns.Msg) (int, error) {

			return 0, w.WriteMsg(r)
		}),
		Out:  &w,
		Pack: false,
	}
	_, err := h.ServeDNS(context.TODO(), &mwtest.ResponseWriter{}, q)
	if err != nil {
		t.Fatal(err)
	}
}

type writer struct {
	queue []*tap.Message
}

func (w *writer) Write(b []byte) (int, error) {
	e := tap.Dnstap{}
	if err := proto.Unmarshal(b, &e); err != nil {
		return 0, err
	}
	if len(w.queue) == 0 {
		return 0, errors.New("message not expected")
	}
	if !test.MsgEqual(w.queue[0], e.Message) {
		return 0, fmt.Errorf("want: %v, have: %v", w.queue[0], e.Message)
	}
	w.queue = w.queue[1:]
	return len(b), nil
}

func TestDnstap(t *testing.T) {
	q := mwtest.Case{Qname: "example.org", Qtype: dns.TypeA}.Msg()
	r := mwtest.Case{
		Qname: "example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			mwtest.A("example.org. 3600	IN	A 10.0.0.1"),
		},
	}.Msg()
	tapq := test.TestingData().ToClientQuery()
	tapr := test.TestingData().ToClientResponse()
	testCase(t, tapq, tapr, q, r)
}
