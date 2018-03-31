package forward

import "github.com/miekg/dns"

// truncated looks at the error and if truncated return a nil errror
// and a possible reconstructed dns message if that was nil.
func truncated(ret *dns.Msg, err error) (*dns.Msg, error) {
	// If you query for instance ANY isc.org; you get a truncated query back which miekg/dns fails to unpack
	// because the RRs are not finished. The returned message can be useful or useless. Return the original
	// query with some header bits set that they should retry with TCP.
	if err != dns.ErrTruncated {
		return ret, err
	}

	// We may or may not have something sensible... if not reassemble something to send to the client.
	m := ret
	if ret == nil {
		m = new(dns.Msg)
		m.SetReply(ret)
		m.Truncated = true
		m.Authoritative = true
		m.Rcode = dns.RcodeSuccess
	}
	return m, nil
}
