package etcd

import (
	"context"
	"crypto/tls"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	mwtls "github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/plugin/proxy"

	etcdcv3 "github.com/coreos/etcd/clientv3"
	"github.com/mholt/caddy"
)

var log = clog.NewWithPlugin("etcd")

func init() {
	caddy.RegisterPlugin("etcd", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	e, stubzones, err := etcdParse(c)
	if err != nil {
		return plugin.Error("etcd", err)
	}

	if stubzones {
		c.OnStartup(func() error {
			e.UpdateStubZones()
			return nil
		})
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}

func etcdParse(c *caddy.Controller) (*Etcd, bool, error) {
	stub := make(map[string]proxy.Proxy)
	etc := Etcd{
		// Don't default to a proxy for lookups.
		//		Proxy:      proxy.NewLookup([]string{"8.8.8.8:53", "8.8.4.4:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Stubmap:    &stub,
	}
	var (
		tlsConfig *tls.Config
		err       error
		endpoints = []string{defaultEndpoint}
		stubzones = false
	)
	for c.Next() {
		etc.Zones = c.RemainingArgs()
		if len(etc.Zones) == 0 {
			etc.Zones = make([]string, len(c.ServerBlockKeys))
			copy(etc.Zones, c.ServerBlockKeys)
		}
		for i, str := range etc.Zones {
			etc.Zones[i] = plugin.Host(str).Normalize()
		}

		if c.NextBlock() {
			for {
				switch c.Val() {
				case "stubzones":
					stubzones = true
				case "fallthrough":
					etc.Fall.SetZonesFromArgs(c.RemainingArgs())
				case "debug":
					/* it is a noop now */
				case "path":
					if !c.NextArg() {
						return &Etcd{}, false, c.ArgErr()
					}
					etc.PathPrefix = c.Val()
				case "endpoint":
					args := c.RemainingArgs()
					if len(args) == 0 {
						return &Etcd{}, false, c.ArgErr()
					}
					endpoints = args
				case "upstream":
					args := c.RemainingArgs()
					u, err := upstream.New(args)
					if err != nil {
						return nil, false, err
					}
					etc.Upstream = u
				case "tls": // cert key cacertfile
					args := c.RemainingArgs()
					tlsConfig, err = mwtls.NewTLSConfigFromArgs(args...)
					if err != nil {
						return &Etcd{}, false, err
					}
				default:
					if c.Val() != "}" {
						return &Etcd{}, false, c.Errf("unknown property '%s'", c.Val())
					}
				}

				if !c.Next() {
					break
				}
			}

		}
		client, err := newEtcdClient(endpoints, tlsConfig)
		if err != nil {
			return &Etcd{}, false, err
		}
		etc.Client = client
		etc.endpoints = endpoints

		return &etc, stubzones, nil
	}
	return &Etcd{}, false, nil
}

func newEtcdClient(endpoints []string, cc *tls.Config) (*etcdcv3.Client, error) {
	etcdCfg := etcdcv3.Config{
		Endpoints: endpoints,
		TLS:       cc,
	}
	cli, err := etcdcv3.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

const defaultEndpoint = "http://localhost:2379"
