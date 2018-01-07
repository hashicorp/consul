package etcd

import (
	"crypto/tls"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	mwtls "github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/coredns/coredns/plugin/proxy"

	etcdc "github.com/coreos/etcd/client"
	"github.com/mholt/caddy"
	"golang.org/x/net/context"
)

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
					if len(args) == 0 {
						return &Etcd{}, false, c.ArgErr()
					}
					ups, err := dnsutil.ParseHostPortOrFile(args...)
					if err != nil {
						return &Etcd{}, false, err
					}
					etc.Proxy = proxy.NewLookup(ups)
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

func newEtcdClient(endpoints []string, cc *tls.Config) (etcdc.KeysAPI, error) {
	etcdCfg := etcdc.Config{
		Endpoints: endpoints,
		Transport: mwtls.NewHTTPSTransport(cc),
	}
	cli, err := etcdc.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return etcdc.NewKeysAPI(cli), nil
}

const defaultEndpoint = "http://localhost:2379"
