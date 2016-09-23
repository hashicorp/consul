// Package proxy is middleware that proxies requests.
package proxy

import (
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

// ReverseProxy is a basic reverse proxy
type ReverseProxy struct {
	Host    string
	Client  Client
	Options Options
}

// ServeDNS implements the middleware.Handler interface.
func (p ReverseProxy) ServeDNS(w dns.ResponseWriter, r *dns.Msg, extra []dns.RR) error {
	var (
		reply *dns.Msg
		err   error
	)

	switch {
	case request.Proto(w) == "tcp": // TODO(miek): keep this in request
		reply, _, err = p.Client.TCP.Exchange(r, p.Host)
	default:
		reply, _, err = p.Client.UDP.Exchange(r, p.Host)
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
