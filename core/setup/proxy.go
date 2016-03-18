package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/proxy"
)

// Proxy configures a new Proxy middleware instance.
func Proxy(c *Controller) (middleware.Middleware, error) {
	upstreams, err := proxy.NewStaticUpstreams(c.Dispenser)
	if err != nil {
		return nil, err
	}
	return func(next middleware.Handler) middleware.Handler {
		return proxy.Proxy{Next: next, Client: proxy.Clients(), Upstreams: upstreams}
	}, nil
}
