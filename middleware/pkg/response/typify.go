package response

import (
	"fmt"

	"github.com/miekg/dns"
)

// Type is the type of the message
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

func (t Type) String() string {
	switch t {
	case NoError:
		return "NOERROR"
	case NameError:
		return "NXDOMAIN"
	case NoData:
		return "NODATA"
	case Delegation:
		return "DELEGATION"
	case Meta:
		return "META"
	case Update:
		return "UPDATE"
	case OtherError:
		return "OTHERERROR"
	}
	return ""
}

// TypeFromString returns the type from the string s. If not type matches
// the OtherError type and an error are returned.
func TypeFromString(s string) (Type, error) {
	switch s {
	case "NOERROR":
		return NoError, nil
	case "NXDOMAIN":
		return NameError, nil
	case "NODATA":
		return NoData, nil
	case "DELEGATION":
		return Delegation, nil
	case "META":
		return Meta, nil
	case "UPDATE":
		return Update, nil
	case "OTHERERROR":
		return OtherError, nil
	}
	return NoError, fmt.Errorf("invalid Type: %s", s)
}

// Typify classifies a message, it returns the Type.
func Typify(m *dns.Msg) (Type, *dns.OPT) {
	if m == nil {
		return OtherError, nil
	}
	opt := m.IsEdns0()

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
		return NoError, opt
	}

	return OtherError, opt
}
