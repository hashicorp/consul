package proxy

import (
	"fmt"

	"github.com/miekg/dns"
)

// toMsg converts a googleMsg into the dns message.
func toMsg(g *googleMsg) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.Response = true
	m.Rcode = g.Status
	m.Truncated = g.TC
	m.RecursionDesired = g.RD
	m.RecursionAvailable = g.RA
	m.AuthenticatedData = g.AD
	m.CheckingDisabled = g.CD

	m.Question = make([]dns.Question, 1)
	m.Answer = make([]dns.RR, len(g.Answer))
	m.Ns = make([]dns.RR, len(g.Authority))
	m.Extra = make([]dns.RR, len(g.Additional))

	m.Question[0] = dns.Question{Name: g.Question[0].Name, Qtype: g.Question[0].Type, Qclass: dns.ClassINET}

	var err error
	for i := 0; i < len(m.Answer); i++ {
		m.Answer[i], err = toRR(g.Answer[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(m.Ns); i++ {
		m.Ns[i], err = toRR(g.Authority[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(m.Extra); i++ {
		m.Extra[i], err = toRR(g.Additional[i])
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

// toRR transforms a "google" RR to a dns.RR.
func toRR(g googleRR) (dns.RR, error) {
	typ, ok := dns.TypeToString[g.Type]
	if !ok {
		return nil, fmt.Errorf("failed to convert type %q", g.Type)
	}

	str := fmt.Sprintf("%s %d %s %s", g.Name, g.TTL, typ, g.Data)
	rr, err := dns.NewRR(str)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q: %s", str, err)
	}
	return rr, nil
}

// googleRR represents a dns.RR in another form.
type googleRR struct {
	Name string
	Type uint16
	TTL  uint32
	Data string
}

// googleMsg is a JSON representation of the dns.Msg.
type googleMsg struct {
	Status   int
	TC       bool
	RD       bool
	RA       bool
	AD       bool
	CD       bool
	Question []struct {
		Name string
		Type uint16
	}
	Answer     []googleRR
	Authority  []googleRR
	Additional []googleRR
	Comment    string
}
