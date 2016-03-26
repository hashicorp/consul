// Package proxy is middleware that proxies requests.
package proxy

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
)

type ReverseProxy struct {
	Host   string
	Client Client
}

func (p ReverseProxy) ServeDNS(w dns.ResponseWriter, r *dns.Msg, extra []dns.RR) error {
	var (
		reply *dns.Msg
		err   error
	)
	state := middleware.State{W: w, Req: r}

	// We forward the original request, no need to fiddle with EDNS0 opt sizes.
	if state.Proto() == "tcp" {
		reply, err = middleware.Exchange(p.Client.TCP, r, p.Host)
	} else {
		reply, err = middleware.Exchange(p.Client.UDP, r, p.Host)
	}

	if err != nil {
		return err
	}
	reply.Compress = true
	reply.Id = r.Id
	w.WriteMsg(reply)
	return nil
}
