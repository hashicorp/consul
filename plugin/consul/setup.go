package consul

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/caddyserver/caddy"
	"github.com/hashicorp/consul/api"
)

func init() { plugin.Register("consul", setup) }

func setup(c *caddy.Controller) error {
	e, err := consulParse(c)
	if err != nil {
		return plugin.Error("consul", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}

func consulParse(c *caddy.Controller) (*Consul, error) {
	consul := Consul{PathPrefix: "skydns", Address: defaultAddress}

	consul.Upstream = upstream.New()

	for c.Next() {
		consul.Zones = c.RemainingArgs()
		if len(consul.Zones) == 0 {
			consul.Zones = make([]string, len(c.ServerBlockKeys))
			copy(consul.Zones, c.ServerBlockKeys)
		}
		for i, str := range consul.Zones {
			consul.Zones[i] = plugin.Host(str).Normalize()
		}

		for c.NextBlock() {
			switch c.Val() {
			case "stubzones":
				// ignored, remove later.
			case "fallthrough":
				consul.Fall.SetZonesFromArgs(c.RemainingArgs())
			case "debug":
				/* it is a noop now */
			case "path":
				if !c.NextArg() {
					return &Consul{}, c.ArgErr()
				}
				consul.PathPrefix = c.Val()
			case "token":
				if !c.NextArg() {
					return &Consul{}, c.ArgErr()
				}
				consul.Token = c.Val()
			case "address":
				if !c.NextArg() {
					return &Consul{}, c.ArgErr()
				}
				consul.Address = c.Val()
			case "upstream":
				// remove soon
				c.RemainingArgs()

			default:
				if c.Val() != "}" {
					return &Consul{}, c.Errf("unknown property '%s'", c.Val())
				}
			}
		}
		client, err := newConsulClient(consul.Address, consul.Token)
		if err != nil {
			return &Consul{}, err
		}
		consul.Client = client

		return &consul, nil
	}
	return &Consul{}, nil
}

func newConsulClient(address string, token string) (*api.Client, error) {
	consulCfg := &api.Config{
		Address: address,
	}
	if token != "" {
		consulCfg.Token = token
	}
	cli, err := api.NewClient(consulCfg)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

const defaultAddress = "http://localhost:8500"
const defaulttoken = ""
