// Package proxy is middleware that proxies requests.
package proxy

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
)

type ReverseProxy struct {
	Host    string
	Client  Client
	Options Options
}

func (p ReverseProxy) ServeDNS(w dns.ResponseWriter, r *dns.Msg, extra []dns.RR) error {
	var (
		reply *dns.Msg
		err   error
	)

	switch {
	case middleware.Proto(w) == "tcp":
		reply, err = middleware.Exchange(p.Client.TCP, r, p.Host)
	default:
		reply, err = middleware.Exchange(p.Client.UDP, r, p.Host)
	}

	if reply != nil && reply.Truncated {
		// Suppress proxy error for truncated responses
		err = nil
	}

	if err != nil {
		return err
	}

	reply.Compress = true
	reply.Id = r.Id
	w.WriteMsg(reply)
	return nil
}
