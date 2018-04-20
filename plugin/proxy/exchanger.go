package proxy

import (
	"github.com/coredns/coredns/request"

	"context"

	"github.com/miekg/dns"
)

// Exchanger is an interface that specifies a type implementing a DNS resolver that
// can use whatever transport it likes.
type Exchanger interface {
	Exchange(ctx context.Context, addr string, state request.Request) (*dns.Msg, error)
	Protocol() string

	// Transport returns the only transport protocol used by this Exchanger or "".
	// If the return value is "", Exchange must use `state.Proto()`.
	Transport() string

	OnStartup(*Proxy) error
	OnShutdown(*Proxy) error
}
