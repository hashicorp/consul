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
	// NoData indicated name found, but not the type: NOERROR in header, SOA in auth.
	NoData
	// Delegation is a msg with a pointer to another nameserver: NOERROR in header, NS in auth, optionally fluff in additional (not checked).
	Delegation
	// OtherError indicated any other error: don't cache these.
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
	case "OTHERERROR":
		return OtherError, nil
	}
	return NoError, fmt.Errorf("invalid Type: %s", s)
}

// Typify classifies a message, it returns the Type.
func Typify(m *dns.Msg) (Type, *dns.OPT) {
	opt := m.IsEdns0()

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
