package proxy

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/mholt/caddy/caddyfile"
	"github.com/miekg/dns"
)

type staticUpstream struct {
	from string

	healthcheck.HealthCheck

	IgnoredSubDomains []string
	ex                Exchanger
}

// NewStaticUpstreams parses the configuration input and sets up
// static upstreams for the proxy plugin.
func NewStaticUpstreams(c *caddyfile.Dispenser) ([]Upstream, error) {
	var upstreams []Upstream
	for c.Next() {
		u, err := NewStaticUpstream(c)
		if err != nil {
			return upstreams, err
		}
		upstreams = append(upstreams, u)
	}
	return upstreams, nil
}

// NewStaticUpstream parses the configuration of a single upstream
// starting from the FROM
func NewStaticUpstream(c *caddyfile.Dispenser) (Upstream, error) {
	upstream := &staticUpstream{
		from: ".",
		HealthCheck: healthcheck.HealthCheck{
			FailTimeout: 5 * time.Second,
			MaxFails:    3,
		},
		ex: newDNSEx(),
	}

	if !c.Args(&upstream.from) {
		return upstream, c.ArgErr()
	}
	upstream.from = plugin.Host(upstream.from).Normalize()

	to := c.RemainingArgs()
	if len(to) == 0 {
		return upstream, c.ArgErr()
	}

	// process the host list, substituting in any nameservers in files
	toHosts, err := parse.HostPortOrFile(to...)
	if err != nil {
		return upstream, err
	}

	if len(toHosts) > max {
		return upstream, fmt.Errorf("more than %d TOs configured: %d", max, len(toHosts))
	}

	for c.NextBlock() {
		if err := parseBlock(c, upstream); err != nil {
			return upstream, err
		}
	}

	upstream.Hosts = make([]*healthcheck.UpstreamHost, len(toHosts))

	for i, host := range toHosts {
		uh := &healthcheck.UpstreamHost{
			Name:        host,
			FailTimeout: upstream.FailTimeout,
			CheckDown:   checkDownFunc(upstream),
		}
		upstream.Hosts[i] = uh
	}
	upstream.Start()

	return upstream, nil
}

func parseBlock(c *caddyfile.Dispenser, u *staticUpstream) error {
	switch c.Val() {
	case "policy":
		if !c.NextArg() {
			return c.ArgErr()
		}
		policyCreateFunc, ok := healthcheck.SupportedPolicies[c.Val()]
		if !ok {
			return c.ArgErr()
		}
		u.Policy = policyCreateFunc()
	case "fail_timeout":
		if !c.NextArg() {
			return c.ArgErr()
		}
		dur, err := time.ParseDuration(c.Val())
		if err != nil {
			return err
		}
		u.FailTimeout = dur
	case "max_fails":
		if !c.NextArg() {
			return c.ArgErr()
		}
		n, err := strconv.Atoi(c.Val())
		if err != nil {
			return err
		}
		u.MaxFails = int32(n)
	case "health_check":
		if !c.NextArg() {
			return c.ArgErr()
		}
		var err error
		u.HealthCheck.Path, u.HealthCheck.Port, err = net.SplitHostPort(c.Val())
		if err != nil {
			return err
		}
		u.HealthCheck.Interval = 4 * time.Second
		if c.NextArg() {
			dur, err := time.ParseDuration(c.Val())
			if err != nil {
				return err
			}
			u.HealthCheck.Interval = dur
		}
	case "except":
		ignoredDomains := c.RemainingArgs()
		if len(ignoredDomains) == 0 {
			return c.ArgErr()
		}
		for i := 0; i < len(ignoredDomains); i++ {
			ignoredDomains[i] = plugin.Host(ignoredDomains[i]).Normalize()
		}
		u.IgnoredSubDomains = ignoredDomains
	case "spray":
		u.Spray = &healthcheck.Spray{}
	case "protocol":
		encArgs := c.RemainingArgs()
		if len(encArgs) == 0 {
			return c.ArgErr()
		}
		switch encArgs[0] {
		case "dns":
			if len(encArgs) > 1 {
				if encArgs[1] == "force_tcp" {
					opts := Options{ForceTCP: true}
					u.ex = newDNSExWithOption(opts)
				} else {
					return fmt.Errorf("only force_tcp allowed as parameter to dns")
				}
			} else {
				u.ex = newDNSEx()
			}
		case "grpc":
			if len(encArgs) == 2 && encArgs[1] == "insecure" {
				u.ex = newGrpcClient(nil, u)
				return nil
			}
			tls, err := tls.NewTLSConfigFromArgs(encArgs[1:]...)
			if err != nil {
				return err
			}
			u.ex = newGrpcClient(tls, u)
		default:
			return fmt.Errorf("%s: %s", errInvalidProtocol, encArgs[0])
		}

	default:
		return c.Errf("unknown property '%s'", c.Val())
	}
	return nil
}

func (u *staticUpstream) IsAllowedDomain(name string) bool {
	if dns.Name(name) == dns.Name(u.From()) {
		return true
	}

	for _, ignoredSubDomain := range u.IgnoredSubDomains {
		if plugin.Name(ignoredSubDomain).Matches(name) {
			return false
		}
	}
	return true
}

func (u *staticUpstream) Exchanger() Exchanger { return u.ex }
func (u *staticUpstream) From() string         { return u.from }

const max = 15
