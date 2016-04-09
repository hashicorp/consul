package middleware

import (
	"errors"

	"github.com/miekg/dns"
)

// Edns0Version checks the EDNS version in the request. If error
// is nil everything is OK and we can invoke the middleware. If non-nil, the
// returned Msg is valid to be returned to the client (and should). For some
// reason this response should not contain a question RR in the question section.
func Edns0Version(req *dns.Msg) (*dns.Msg, error) {
	opt := req.IsEdns0()
	if opt == nil {
		return nil, nil
	}
	if opt.Version() == 0 {
		return nil, nil
	}
	m := new(dns.Msg)
	m.SetReply(req)
	// zero out question section, wtf.
	m.Question = nil

	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	o.SetVersion(0)
	o.SetExtendedRcode(dns.RcodeBadVers)
	m.Extra = []dns.RR{o}

	return m, errors.New("EDNS0 BADVERS")
}
