package etcd

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/middleware/pkg/singleflight"
	mwtls "github.com/coredns/coredns/middleware/pkg/tls"
	"github.com/coredns/coredns/middleware/proxy"

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
		return middleware.Error("etcd", err)
	}

	if stubzones {
		c.OnStartup(func() error {
			e.UpdateStubZones()
			return nil
		})
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		e.Next = next
		return e
	})

	return nil
}

func etcdParse(c *caddy.Controller) (*Etcd, bool, error) {
	stub := make(map[string]proxy.Proxy)
	etc := Etcd{
		Proxy:      proxy.NewLookup([]string{"8.8.8.8:53", "8.8.4.4:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
		Stubmap:    &stub,
	}
	var (
		tlsConfig *tls.Config
		err       error
		endpoints = []string{defaultEndpoint}
		stubzones = false
	)
	for c.Next() {
		if c.Val() == "etcd" {
			etc.Zones = c.RemainingArgs()
			if len(etc.Zones) == 0 {
				etc.Zones = make([]string, len(c.ServerBlockKeys))
				copy(etc.Zones, c.ServerBlockKeys)
			}
			middleware.Zones(etc.Zones).Normalize()
			if c.NextBlock() {
				// TODO(miek): 2 switches?
				switch c.Val() {
				case "stubzones":
					stubzones = true
				case "debug":
					etc.Debugging = true
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
				for c.Next() {
					switch c.Val() {
					case "stubzones":
						stubzones = true
					case "debug":
						etc.Debugging = true
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
							return &Etcd{}, false, c.ArgErr()
						}
						etc.Proxy = proxy.NewLookup(ups)
					case "tls": // cert key cacertfile
						args := c.RemainingArgs()
						tlsConfig, err = mwtls.NewTLSConfigFromArgs(args...)
						if err != nil {
							return &Etcd{}, false, err
						}
					default:
						if c.Val() != "}" { // TODO(miek): this feels like I'm doing it completely wrong.
							return &Etcd{}, false, c.Errf("unknown property '%s'", c.Val())
						}
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
	}
	return &Etcd{}, false, nil
}

func newEtcdClient(endpoints []string, cc *tls.Config) (etcdc.KeysAPI, error) {
	etcdCfg := etcdc.Config{
		Endpoints: endpoints,
		Transport: newHTTPSTransport(cc),
	}
	cli, err := etcdc.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return etcdc.NewKeysAPI(cli), nil
}

func newHTTPSTransport(cc *tls.Config) etcdc.CancelableTransport {
	// this seems like a bad idea but was here in the previous version
	if cc != nil {
		cc.InsecureSkipVerify = true
	}

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cc,
	}

	return tr
}

const defaultEndpoint = "http://localhost:2379"
