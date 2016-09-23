package etcd

import (
	"errors"
	"log"

	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Stub wraps an Etcd. We have this type so that it can have a ServeDNS method.
type Stub struct {
	*Etcd
	Zone string // for what zone (and thus what nameservers are we called)
}

// ServeDNS implements the middleware.Handler interface.
func (s Stub) ServeDNS(ctx context.Context, w dns.ResponseWriter, req *dns.Msg) (int, error) {
	if hasStubEdns0(req) {
		log.Printf("[WARNING] Forwarding cycle detected, refusing msg: %s", req.Question[0].Name)
		return dns.RcodeRefused, errors.New("stub forward cycle")
	}
	req = addStubEdns0(req)
	proxy, ok := (*s.Etcd.Stubmap)[s.Zone]
	if !ok { // somebody made a mistake..
		return dns.RcodeServerFailure, nil
	}

	state := request.Request{W: w, Req: req}
	m, e := proxy.Forward(state)
	if e != nil {
		return dns.RcodeServerFailure, e
	}
	m.RecursionAvailable, m.Compress = true, true
	state.SizeAndDo(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// hasStubEdns0 checks if the message is carrying our special edns0 zero option.
func hasStubEdns0(m *dns.Msg) bool {
	option := m.IsEdns0()
	if option == nil {
		return false
	}
	for _, o := range option.Option {
		if o.Option() == ednsStubCode && len(o.(*dns.EDNS0_LOCAL).Data) == 1 &&
			o.(*dns.EDNS0_LOCAL).Data[0] == 1 {
			return true
		}
	}
	return false
}

// addStubEdns0 adds our special option to the message's OPT record.
func addStubEdns0(m *dns.Msg) *dns.Msg {
	option := m.IsEdns0()
	// Add a custom EDNS0 option to the packet, so we can detect loops when 2 stubs are forwarding to each other.
	if option != nil {
		option.Option = append(option.Option, &dns.EDNS0_LOCAL{Code: ednsStubCode, Data: []byte{1}})
		return m
	}

	m.Extra = append(m.Extra, ednsStub)
	return m
}

const (
	ednsStubCode = dns.EDNS0LOCALSTART + 10
	stubDomain   = "stub.dns"
)

var ednsStub = func() *dns.OPT {
	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	o.SetUDPSize(4096)

	e := new(dns.EDNS0_LOCAL)
	e.Code = ednsStubCode
	e.Data = []byte{1}
	o.Option = append(o.Option, e)
	return o
}()
