// Reflect provides middleware that reflects back some client properties.
// This is the default middleware when Caddy is run without configuration.
//
// The left-most label must be `who`.
// When queried for type A (resp. AAAA), it sends back the IPv4 (resp. v6) address.
// In the additional section the port number and transport are shown.
// Basic use pattern:
//
//	dig @localhost -p 1053 who.miek.nl A
//
//	;; ANSWER SECTION:
//	who.miek.nl.		0	IN	A	127.0.0.1
//
//	;; ADDITIONAL SECTION:
//	who.miek.nl.		0	IN	TXT	"Port: 56195 (udp)"
package reflect

import (
	"errors"
	"net"
	"strings"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

type Reflect struct {
	Next middleware.Handler
}

func (rl Reflect) ServeDNS(w dns.ResponseWriter, r *dns.Msg) (int, error) {
	context := middleware.Context{Req: r, W: w}

	class := r.Question[0].Qclass
	qname := r.Question[0].Name
	i, ok := dns.NextLabel(qname, 0)

	if strings.ToLower(qname[:i]) != who || ok {
		err := context.ErrorMessage(dns.RcodeFormatError)
		w.WriteMsg(err)
		return dns.RcodeFormatError, errors.New(dns.RcodeToString[dns.RcodeFormatError])
	}

	answer := new(dns.Msg)
	answer.SetReply(r)
	answer.Compress = true
	answer.Authoritative = true

	ip := context.IP()
	proto := context.Proto()
	port, _ := context.Port()
	family := context.Family()
	var rr dns.RR

	switch family {
	case 1:
		rr = new(dns.A)
		rr.(*dns.A).Hdr = dns.RR_Header{Name: qname, Rrtype: dns.TypeA, Class: class, Ttl: 0}
		rr.(*dns.A).A = net.ParseIP(ip).To4()
	case 2:
		rr = new(dns.AAAA)
		rr.(*dns.AAAA).Hdr = dns.RR_Header{Name: qname, Rrtype: dns.TypeAAAA, Class: class, Ttl: 0}
		rr.(*dns.AAAA).AAAA = net.ParseIP(ip)
	}

	t := new(dns.TXT)
	t.Hdr = dns.RR_Header{Name: qname, Rrtype: dns.TypeTXT, Class: class, Ttl: 0}
	t.Txt = []string{"Port: " + port + " (" + proto + ")"}

	switch context.Type() {
	case "TXT":
		answer.Answer = append(answer.Answer, t)
		answer.Extra = append(answer.Extra, rr)
	default:
		fallthrough
	case "AAAA", "A":
		answer.Answer = append(answer.Answer, rr)
		answer.Extra = append(answer.Extra, t)
	}
	w.WriteMsg(answer)
	return 0, nil
}

const who = "who."
