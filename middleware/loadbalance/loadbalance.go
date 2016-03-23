package loadbalance

import "github.com/miekg/dns"

type RoundRobinResponseWriter struct {
	dns.ResponseWriter
}

func NewRoundRobinResponseWriter(w dns.ResponseWriter) *RoundRobinResponseWriter {
	return &RoundRobinResponseWriter{w}
}

func (r *RoundRobinResponseWriter) WriteMsg(res *dns.Msg) error {
	if res.Rcode != dns.RcodeSuccess {
		return r.ResponseWriter.WriteMsg(res)
	}
	if len(res.Answer) == 1 {
		return r.ResponseWriter.WriteMsg(res)
	}

	// put CNAMEs first, randomize a/aaaa's and put packet back together.
	// TODO(miek): check family and give v6 more prio?
	cname := []dns.RR{}
	address := []dns.RR{}
	rest := []dns.RR{}
	for _, r := range res.Answer {
		switch r.Header().Rrtype {
		case dns.TypeCNAME:
			cname = append(cname, r)
		case dns.TypeA, dns.TypeAAAA:
			address = append(address, r)
		default:
			rest = append(rest, r)
		}
	}

	switch l := len(address); l {
	case 0, 1:
		return r.ResponseWriter.WriteMsg(res)
	case 2:
		if dns.Id()%2 == 0 {
			address[0], address[1] = address[1], address[0]
		}
	default:
		for j := 0; j < l*(int(dns.Id())%4+1); j++ {
			q := int(dns.Id()) % l
			p := int(dns.Id()) % l
			if q == p {
				p = (p + 1) % l
			}
			address[q], address[p] = address[p], address[q]
		}
	}
	res.Answer = append(cname, rest...)
	res.Answer = append(res.Answer, address...)
	return r.ResponseWriter.WriteMsg(res)
}

func (r *RoundRobinResponseWriter) Write(buf []byte) (int, error) {
	// pack and unpack? Not likely
	n, err := r.ResponseWriter.Write(buf)
	return n, err
}

func (r *RoundRobinResponseWriter) Hijack() {
	r.ResponseWriter.Hijack()
	return
}
