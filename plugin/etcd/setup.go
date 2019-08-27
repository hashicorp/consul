package etcd

import (
	"crypto/tls"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	mwtls "github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/caddyserver/caddy"
	etcdcv3 "go.etcd.io/etcd/clientv3"
)

var log = clog.NewWithPlugin("etcd")

func init() {
	caddy.RegisterPlugin("etcd", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	e, err := etcdParse(c)
	if err != nil {
		return plugin.Error("etcd", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}

func etcdParse(c *caddy.Controller) (*Etcd, error) {
	etc := Etcd{PathPrefix: "skydns"}
	var (
		tlsConfig *tls.Config
		err       error
		endpoints = []string{defaultEndpoint}
		username  string
		password  string
	)

	etc.Upstream = upstream.New()

	for c.Next() {
		etc.Zones = c.RemainingArgs()
		if len(etc.Zones) == 0 {
			etc.Zones = make([]string, len(c.ServerBlockKeys))
			copy(etc.Zones, c.ServerBlockKeys)
		}
		for i, str := range etc.Zones {
			etc.Zones[i] = plugin.Host(str).Normalize()
		}

		for c.NextBlock() {
			switch c.Val() {
			case "stubzones":
				// ignored, remove later.
			case "fallthrough":
				etc.Fall.SetZonesFromArgs(c.RemainingArgs())
			case "debug":
				/* it is a noop now */
			case "path":
				if !c.NextArg() {
					return &Etcd{}, c.ArgErr()
				}
				etc.PathPrefix = c.Val()
			case "endpoint":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return &Etcd{}, c.ArgErr()
				}
				endpoints = args
			case "upstream":
				// remove soon
				c.RemainingArgs()
			case "tls": // cert key cacertfile
				args := c.RemainingArgs()
				tlsConfig, err = mwtls.NewTLSConfigFromArgs(args...)
				if err != nil {
					return &Etcd{}, err
				}
			case "credentials":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return &Etcd{}, c.ArgErr()
				}
				if len(args) != 2 {
					return &Etcd{}, c.Errf("credentials requires 2 arguments, username and password")
				}
				username, password = args[0], args[1]
			default:
				if c.Val() != "}" {
					return &Etcd{}, c.Errf("unknown property '%s'", c.Val())
				}
			}
		}
		client, err := newEtcdClient(endpoints, tlsConfig, username, password)
		if err != nil {
			return &Etcd{}, err
		}
		etc.Client = client
		etc.endpoints = endpoints

		return &etc, nil
	}
	return &Etcd{}, nil
}

func newEtcdClient(endpoints []string, cc *tls.Config, username, password string) (*etcdcv3.Client, error) {
	etcdCfg := etcdcv3.Config{
		Endpoints: endpoints,
		TLS:       cc,
	}
	if username != "" && password != "" {
		etcdCfg.Username = username
		etcdCfg.Password = password
	}
	cli, err := etcdcv3.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

const defaultEndpoint = "http://localhost:2379"
