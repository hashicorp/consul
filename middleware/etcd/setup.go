package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/singleflight"
	"github.com/miekg/coredns/middleware/proxy"

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
		Proxy:      proxy.New([]string{"8.8.8.8:53", "8.8.4.4:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
		Stubmap:    &stub,
	}
	var (
		tlsCertFile   = ""
		tlsKeyFile    = ""
		tlsCAcertFile = ""
		endpoints     = []string{defaultEndpoint}
		stubzones     = false
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
					for i := 0; i < len(args); i++ {
						h, p, e := net.SplitHostPort(args[i])
						if e != nil && p == "" {
							args[i] = h + ":53"
						}
					}
					etc.Proxy = proxy.New(args)
				case "tls": // cert key cacertfile
					args := c.RemainingArgs()
					if len(args) != 3 {
						return &Etcd{}, false, c.ArgErr()
					}
					tlsCertFile, tlsKeyFile, tlsCAcertFile = args[0], args[1], args[2]
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
						for i := 0; i < len(args); i++ {
							h, p, e := net.SplitHostPort(args[i])
							if e != nil && p == "" {
								args[i] = h + ":53"
							}
						}
						etc.Proxy = proxy.New(args)
					case "tls": // cert key cacertfile
						args := c.RemainingArgs()
						if len(args) != 3 {
							return &Etcd{}, false, c.ArgErr()
						}
						tlsCertFile, tlsKeyFile, tlsCAcertFile = args[0], args[1], args[2]
					default:
						if c.Val() != "}" { // TODO(miek): this feels like I'm doing it completely wrong.
							return &Etcd{}, false, c.Errf("unknown property '%s'", c.Val())
						}
					}
				}

			}
			client, err := newEtcdClient(endpoints, tlsCertFile, tlsKeyFile, tlsCAcertFile)
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

func newEtcdClient(endpoints []string, tlsCert, tlsKey, tlsCACert string) (etcdc.KeysAPI, error) {
	etcdCfg := etcdc.Config{
		Endpoints: endpoints,
		Transport: newHTTPSTransport(tlsCert, tlsKey, tlsCACert),
	}
	cli, err := etcdc.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return etcdc.NewKeysAPI(cli), nil
}

func newHTTPSTransport(tlsCertFile, tlsKeyFile, tlsCACertFile string) etcdc.CancelableTransport {
	var cc *tls.Config

	if tlsCertFile != "" && tlsKeyFile != "" {
		var rpool *x509.CertPool
		if tlsCACertFile != "" {
			if pemBytes, err := ioutil.ReadFile(tlsCACertFile); err == nil {
				rpool = x509.NewCertPool()
				rpool.AppendCertsFromPEM(pemBytes)
			}
		}

		if tlsCert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile); err == nil {
			cc = &tls.Config{
				RootCAs:            rpool,
				Certificates:       []tls.Certificate{tlsCert},
				InsecureSkipVerify: true,
			}
		}
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
