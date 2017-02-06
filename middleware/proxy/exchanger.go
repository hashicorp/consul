package proxy

import (
	"github.com/miekg/coredns/request"
	"github.com/miekg/dns"
)

// Exchanger is an interface that specifies a type implementing a DNS resolver that
// can use whatever transport it likes.
type Exchanger interface {
	Exchange(addr string, state request.Request) (*dns.Msg, error)
	Protocol() string

	OnStartup(*Proxy) error
	OnShutdown(*Proxy) error
}
