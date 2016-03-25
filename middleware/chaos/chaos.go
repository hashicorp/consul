package chaos

import (
	"os"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type Chaos struct {
	Next    middleware.Handler
	Version string
	Authors map[string]bool // get randomization for free \o/
}

func (c Chaos) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	if state.QClass() != dns.ClassCHAOS || state.QType() != dns.TypeTXT {
		return c.Next.ServeDNS(ctx, w, r)
	}
	m := new(dns.Msg)
	hdr := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeTXT, Class: dns.ClassCHAOS, Ttl: 0}
	switch state.Name() {
	default:
		return c.Next.ServeDNS(ctx, w, r)
	case "authors.bind.":
		for a, _ := range c.Authors {
			m.Answer = append(m.Answer, &dns.TXT{Hdr: hdr, Txt: []string{trim(a)}})
		}
	case "version.bind.", "version.server.":
		m.Answer = []dns.RR{&dns.TXT{Hdr: hdr, Txt: []string{trim(c.Version)}}}
	case "hostname.bind.", "id.server.":
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "localhost"
		}
		m.Answer = []dns.RR{&dns.TXT{Hdr: hdr, Txt: []string{trim(hostname)}}}
	}
	w.WriteMsg(m)
	return 0, nil
}

func trim(s string) string {
	if len(s) < 256 {
		return s
	}
	return s[:255]
}
