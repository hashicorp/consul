package response

import (
	"fmt"
	"time"

	"github.com/miekg/dns"
)

// Type is the type of the message.
type Type int

const (
	// NoError indicates a positive reply
	NoError Type = iota
	// NameError is a NXDOMAIN in header, SOA in auth.
	NameError
	// NoData indicates name found, but not the type: NOERROR in header, SOA in auth.
	NoData
	// Delegation is a msg with a pointer to another nameserver: NOERROR in header, NS in auth, optionally fluff in additional (not checked).
	Delegation
	// Meta indicates a meta message, NOTIFY, or a transfer:  qType is IXFR or AXFR.
	Meta
	// Update is an dynamic update message.
	Update
	// OtherError indicates any other error: don't cache these.
	OtherError
)

var toString = map[Type]string{
	NoError:    "NOERROR",
	NameError:  "NXDOMAIN",
	NoData:     "NODATA",
	Delegation: "DELEGATION",
	Meta:       "META",
	Update:     "UPDATE",
	OtherError: "OTHERERROR",
}

func (t Type) String() string { return toString[t] }

// TypeFromString returns the type from the string s. If not type matches
// the OtherError type and an error are returned.
func TypeFromString(s string) (Type, error) {
	for t, str := range toString {
		if s == str {
			return t, nil
		}
	}
	return NoError, fmt.Errorf("invalid Type: %s", s)
}

// Typify classifies a message, it returns the Type.
func Typify(m *dns.Msg, t time.Time) (Type, *dns.OPT) {
	if m == nil {
		return OtherError, nil
	}
	opt := m.IsEdns0()
	do := false
	if opt != nil {
		do = opt.Do()
	}

	if m.Opcode == dns.OpcodeUpdate {
		return Update, opt
	}

	// Check transfer and update first
	if m.Opcode == dns.OpcodeNotify {
		return Meta, opt
	}

	if len(m.Question) > 0 {
		if m.Question[0].Qtype == dns.TypeAXFR || m.Question[0].Qtype == dns.TypeIXFR {
			return Meta, opt
		}
	}

	// If our message contains any expired sigs and we care about that, we should return expired
	if do {
		if expired := typifyExpired(m, t); expired {
			return OtherError, opt
		}
	}

	if len(m.Answer) > 0 && m.Rcode == dns.RcodeSuccess {
		return NoError, opt
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

	if soa && m.Rcode == dns.RcodeSuccess {
		return NoData, opt
	}
	if soa && m.Rcode == dns.RcodeNameError {
		return NameError, opt
	}

	if ns > 0 && m.Rcode == dns.RcodeSuccess {
		return Delegation, opt
	}

	if m.Rcode == dns.RcodeSuccess {
		return NoError, opt
	}

	return OtherError, opt
}

func typifyExpired(m *dns.Msg, t time.Time) bool {
	if expired := typifyExpiredRRSIG(m.Answer, t); expired {
		return true
	}
	if expired := typifyExpiredRRSIG(m.Ns, t); expired {
		return true
	}
	if expired := typifyExpiredRRSIG(m.Extra, t); expired {
		return true
	}
	return false
}

func typifyExpiredRRSIG(rrs []dns.RR, t time.Time) bool {
	for _, r := range rrs {
		if r.Header().Rrtype != dns.TypeRRSIG {
			continue
		}
		ok := r.(*dns.RRSIG).ValidityPeriod(t)
		if !ok {
			return true
		}
	}
	return false
}
