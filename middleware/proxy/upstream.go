package proxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy/caddyfile"
	"github.com/miekg/dns"
)

var (
	supportedPolicies = make(map[string]func() Policy)
)

type staticUpstream struct {
	from   string
	Hosts  HostPool
	Policy Policy
	Spray  Policy

	FailTimeout time.Duration
	MaxFails    int32
	HealthCheck struct {
		Path     string
		Port     string
		Interval time.Duration
	}
	WithoutPathPrefix string
	IgnoredSubDomains []string
	options           Options
}

// Options ...
type Options struct {
	Ecs []*net.IPNet // EDNS0 CLIENT SUBNET address (v4/v6) to add in CIDR notaton.
}

// NewStaticUpstreams parses the configuration input and sets up
// static upstreams for the proxy middleware.
func NewStaticUpstreams(c *caddyfile.Dispenser) ([]Upstream, error) {
	var upstreams []Upstream
	for c.Next() {
		upstream := &staticUpstream{
			from:        "",
			Hosts:       nil,
			Policy:      &Random{},
			Spray:       nil,
			FailTimeout: 10 * time.Second,
			MaxFails:    1,
		}

		if !c.Args(&upstream.from) {
			return upstreams, c.ArgErr()
		}
		to := c.RemainingArgs()
		if len(to) == 0 {
			return upstreams, c.ArgErr()
		}

		// process the host list, substituting in any nameservers in files
		var toHosts []string
		for _, host := range to {
			h, _, err := net.SplitHostPort(host)
			if err != nil {
				h = host
			}
			if x := net.ParseIP(h); x == nil {
				// it's a file, parse as resolv.conf
				c, err := dns.ClientConfigFromFile(host)
				if err == os.ErrNotExist {
					return upstreams, fmt.Errorf("not an IP address or file: `%s'", h)
				} else if err != nil {
					return upstreams, err
				}
				for _, s := range c.Servers {
					toHosts = append(toHosts, net.JoinHostPort(s, c.Port))
				}
			} else {
				toHosts = append(toHosts, host)
			}
		}

		for c.NextBlock() {
			if err := parseBlock(c, upstream); err != nil {
				return upstreams, err
			}
		}

		upstream.Hosts = make([]*UpstreamHost, len(toHosts))
		for i, host := range toHosts {
			uh := &UpstreamHost{
				Name:        defaultHostPort(host),
				Conns:       0,
				Fails:       0,
				FailTimeout: upstream.FailTimeout,
				Unhealthy:   false,

				CheckDown: func(upstream *staticUpstream) UpstreamHostDownFunc {
					return func(uh *UpstreamHost) bool {
						if uh.Unhealthy {
							return true
						}

						fails := atomic.LoadInt32(&uh.Fails)
						if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
							return true
						}
						return false
					}
				}(upstream),
				WithoutPathPrefix: upstream.WithoutPathPrefix,
			}
			upstream.Hosts[i] = uh
		}

		if upstream.HealthCheck.Path != "" {
			go upstream.HealthCheckWorker(nil)
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams, nil
}

// RegisterPolicy adds a custom policy to the proxy.
func RegisterPolicy(name string, policy func() Policy) {
	supportedPolicies[name] = policy
}

func (u *staticUpstream) From() string {
	return u.from
}

func (u *staticUpstream) Options() Options {
	return u.options
}

func parseBlock(c *caddyfile.Dispenser, u *staticUpstream) error {
	switch c.Val() {
	case "policy":
		if !c.NextArg() {
			return c.ArgErr()
		}
		policyCreateFunc, ok := supportedPolicies[c.Val()]
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
		u.HealthCheck.Interval = 30 * time.Second
		if c.NextArg() {
			dur, err := time.ParseDuration(c.Val())
			if err != nil {
				return err
			}
			u.HealthCheck.Interval = dur
		}
	case "without":
		if !c.NextArg() {
			return c.ArgErr()
		}
		u.WithoutPathPrefix = c.Val()
	case "except":
		ignoredDomains := c.RemainingArgs()
		if len(ignoredDomains) == 0 {
			return c.ArgErr()
		}
		for i := 0; i < len(ignoredDomains); i++ {
			ignoredDomains[i] = strings.ToLower(dns.Fqdn(ignoredDomains[i]))
		}
		u.IgnoredSubDomains = ignoredDomains
	case "spray":
		u.Spray = &Spray{}

	default:
		return c.Errf("unknown property '%s'", c.Val())
	}
	return nil
}

func (u *staticUpstream) healthCheck() {
	for _, host := range u.Hosts {
		port := ""
		if u.HealthCheck.Port != "" {
			port = ":" + u.HealthCheck.Port
		}
		hostURL := host.Name + port + u.HealthCheck.Path
		if r, err := http.Get(hostURL); err == nil {
			io.Copy(ioutil.Discard, r.Body)
			r.Body.Close()
			host.Unhealthy = r.StatusCode < 200 || r.StatusCode >= 400
		} else {
			host.Unhealthy = true
		}
	}
}

func (u *staticUpstream) HealthCheckWorker(stop chan struct{}) {
	ticker := time.NewTicker(u.HealthCheck.Interval)
	u.healthCheck()
	for {
		select {
		case <-ticker.C:
			u.healthCheck()
		case <-stop:
			// TODO: the library should provide a stop channel and global
			// waitgroup to allow goroutines started by plugins a chance
			// to clean themselves up.
		}
	}
}

func (u *staticUpstream) Select() *UpstreamHost {
	pool := u.Hosts
	if len(pool) == 1 {
		if pool[0].Down() && u.Spray == nil {
			return nil
		}
		return pool[0]
	}
	allDown := true
	for _, host := range pool {
		if !host.Down() {
			allDown = false
			break
		}
	}
	if allDown {
		if u.Spray == nil {
			return nil
		}
		return u.Spray.Select(pool)
	}

	if u.Policy == nil {
		h := (&Random{}).Select(pool)
		if h == nil && u.Spray == nil {
			return nil
		}
		return u.Spray.Select(pool)
	}

	h := u.Policy.Select(pool)
	if h != nil {
		return h
	}

	if u.Spray == nil {
		return nil
	}
	return u.Spray.Select(pool)
}

func (u *staticUpstream) IsAllowedPath(name string) bool {
	for _, ignoredSubDomain := range u.IgnoredSubDomains {
		if dns.Name(name) == dns.Name(u.From()) {
			return true
		}
		if middleware.Name(name).Matches(ignoredSubDomain + u.From()) {
			return false
		}
	}
	return true
}

func defaultHostPort(s string) string {
	_, _, e := net.SplitHostPort(s)
	if e == nil {
		return s
	}
	return net.JoinHostPort(s, "53")
}
