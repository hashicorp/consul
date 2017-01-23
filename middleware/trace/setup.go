package trace

import (
	"fmt"
	"strings"
	"sync"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("trace", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	t, err := traceParse(c)
	if err != nil {
		return middleware.Error("trace", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		t.Next = next
		return t
	})

	traceOnce.Do(func() {
		c.OnStartup(t.OnStartup)
	})

	return nil
}

func traceParse(c *caddy.Controller) (*Trace, error) {
	var (
		tr = &Trace{Endpoint: defEP, EndpointType: defEpType}
		err error
	)

	cfg := dnsserver.GetConfig(c)
	tr.ServiceEndpoint = cfg.ListenHost + ":" + cfg.Port
	for c.Next() {
		if c.Val() == "trace" {
			var err error
			args := c.RemainingArgs()
			switch len(args) {
			case 0:
				tr.Endpoint, err = normalizeEndpoint(tr.EndpointType, defEP)
			case 1:
				tr.Endpoint, err = normalizeEndpoint(defEpType, args[0])
			case 2:
				tr.EndpointType = strings.ToLower(args[0])
				tr.Endpoint, err = normalizeEndpoint(tr.EndpointType, args[1])
			default:
				err = c.ArgErr()
			}
			if err != nil {
				return tr, err
			}
		}
	}
	return tr, err
}

func normalizeEndpoint(epType, ep string) (string, error) {
	switch epType {
	case "zipkin":
		if strings.Index(ep, "http") == -1 {
			ep = "http://" + ep + "/api/v1/spans"
		}
		return ep, nil
	default:
		return "", fmt.Errorf("Tracing endpoint type '%s' is not supported.", epType)
	}
}

var traceOnce sync.Once

const (
	defEP = "localhost:9411"
	defEpType = "zipkin"
)
