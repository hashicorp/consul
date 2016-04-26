package middleware

import "github.com/miekg/dns"

type MsgType int

const (
	Success    MsgType = iota
	NameError          // NXDOMAIN in header, SOA in auth.
	NoData             // NOERROR in header, SOA in auth.
	Delegation         // NOERROR in header, NS in auth, optionally fluff in additional (not checked).
	OtherError         // Don't cache these.
)

// Classify classifies a message, it returns the MessageType.
func Classify(m *dns.Msg) (MsgType, *dns.OPT) {
	opt := m.IsEdns0()

	if len(m.Answer) > 0 && m.Rcode == dns.RcodeSuccess {
		return Success, opt
	}

	soa := false
	ns := 0
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeSOA {
			soa = true
			continue
		}
		if r.Header().Rrtype == dns.TypeNS {
			ns++
		}
	}

	// Check length of different sections, and drop stuff that is just to large? TODO(miek).
	if soa && m.Rcode == dns.RcodeSuccess {
		return NoData, opt
	}
	if soa && m.Rcode == dns.RcodeNameError {
		return NameError, opt
	}

	if ns > 0 && ns == len(m.Ns) && m.Rcode == dns.RcodeSuccess {
		return Delegation, opt
	}

	if m.Rcode == dns.RcodeSuccess {
		return Success, opt
	}

	return OtherError, opt
}
