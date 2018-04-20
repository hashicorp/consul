package dnstap

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/dnstap/test"
	mwtest "github.com/coredns/coredns/plugin/test"

	"context"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

func testCase(t *testing.T, tapq, tapr *tap.Message, q, r *dns.Msg) {
	w := writer{t: t}
	w.queue = append(w.queue, tapq, tapr)
	h := Dnstap{
		Next: mwtest.HandlerFunc(func(_ context.Context,
			w dns.ResponseWriter, _ *dns.Msg) (int, error) {

			return 0, w.WriteMsg(r)
		}),
		IO:             &w,
		JoinRawMessage: false,
	}
	_, err := h.ServeDNS(context.TODO(), &mwtest.ResponseWriter{}, q)
	if err != nil {
		t.Fatal(err)
	}
}

type writer struct {
	t     *testing.T
	queue []*tap.Message
}

func (w *writer) Dnstap(e tap.Dnstap) {
	if len(w.queue) == 0 {
		w.t.Error("Message not expected.")
	}
	if !test.MsgEqual(w.queue[0], e.Message) {
		w.t.Errorf("want: %v, have: %v", w.queue[0], e.Message)
	}
	w.queue = w.queue[1:]
}

func TestDnstap(t *testing.T) {
	q := mwtest.Case{Qname: "example.org", Qtype: dns.TypeA}.Msg()
	r := mwtest.Case{
		Qname: "example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			mwtest.A("example.org. 3600	IN	A 10.0.0.1"),
		},
	}.Msg()
	tapq, _ := test.TestingData().ToClientQuery()
	tapr, _ := test.TestingData().ToClientResponse()
	testCase(t, tapq, tapr, q, r)
}

type noWriter struct {
}

func (n noWriter) Dnstap(d tap.Dnstap) {
}

func endWith(c int, err error) plugin.Handler {
	return mwtest.HandlerFunc(func(_ context.Context, w dns.ResponseWriter, _ *dns.Msg) (int, error) {
		w.WriteMsg(nil) // trigger plugin dnstap to log client query and response
		// maybe dnstap should log the client query when no message is written...
		return c, err
	})
}

type badAddr struct {
}

func (bad badAddr) Network() string {
	return "bad network"
}
func (bad badAddr) String() string {
	return "bad address"
}

type badRW struct {
	dns.ResponseWriter
}

func (bad *badRW) RemoteAddr() net.Addr {
	return badAddr{}
}

func TestError(t *testing.T) {
	h := Dnstap{
		Next:           endWith(0, nil),
		IO:             noWriter{},
		JoinRawMessage: false,
	}
	rw := &badRW{&mwtest.ResponseWriter{}}

	// the dnstap error will show only if there is no plugin error
	_, err := h.ServeDNS(context.TODO(), rw, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "plugin/dnstap") {
		t.Fatal("must return the dnstap error but have:", err)
	}

	// plugin errors will always overwrite dnstap errors
	pluginErr := errors.New("plugin error")
	h.Next = endWith(0, pluginErr)
	_, err = h.ServeDNS(context.TODO(), rw, nil)
	if err != pluginErr {
		t.Fatal("must return the plugin error but have:", err)
	}
}
