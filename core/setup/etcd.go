package setup

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd"
	"github.com/miekg/coredns/middleware/etcd/singleflight"
	"github.com/miekg/coredns/middleware/proxy"

	etcdc "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const defaultEndpoint = "http://127.0.0.1:2379"

// Etcd sets up the etcd middleware.
func Etcd(c *Controller) (middleware.Middleware, error) {
	etcd, stubzones, err := etcdParse(c)
	if err != nil {
		return nil, err
	}
	if stubzones {
		c.Startup = append(c.Startup, func() error {
			etcd.UpdateStubZones()
			return nil
		})
	}

	return func(next middleware.Handler) middleware.Handler {
		etcd.Next = next
		return etcd
	}, nil
}

func etcdParse(c *Controller) (etcd.Etcd, bool, error) {
	stub := make(map[string]proxy.Proxy)
	etc := etcd.Etcd{
		Proxy:      proxy.New([]string{"8.8.8.8:53", "8.8.4.4:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
		Stubmap:    &stub,
	}
	var (
		client        etcdc.KeysAPI
		tlsCertFile   = ""
		tlsKeyFile    = ""
		tlsCAcertFile = ""
		endpoints     = []string{defaultEndpoint}
		stubzones     = false
	)
	for c.Next() {
		if c.Val() == "etcd" {
			etc.Client = client
			etc.Zones = c.RemainingArgs()
			if len(etc.Zones) == 0 {
				etc.Zones = c.ServerBlockHosts
			}
			middleware.Zones(etc.Zones).FullyQualify()
			if c.NextBlock() {
				// TODO(miek): 2 switches?
				switch c.Val() {
				case "stubzones":
					stubzones = true
				case "path":
					if !c.NextArg() {
						return etcd.Etcd{}, false, c.ArgErr()
					}
					etc.PathPrefix = c.Val()
				case "endpoint":
					args := c.RemainingArgs()
					if len(args) == 0 {
						return etcd.Etcd{}, false, c.ArgErr()
					}
					endpoints = args
				case "upstream":
					args := c.RemainingArgs()
					if len(args) == 0 {
						return etcd.Etcd{}, false, c.ArgErr()
					}
					for i := 0; i < len(args); i++ {
						h, p, e := net.SplitHostPort(args[i])
						if e != nil && p == "" {
							args[i] = h + ":53"
						}
					}
					endpoints = args
					etc.Proxy = proxy.New(args)
				case "tls": // cert key cacertfile
					args := c.RemainingArgs()
					if len(args) != 3 {
						return etcd.Etcd{}, false, c.ArgErr()
					}
					tlsCertFile, tlsKeyFile, tlsCAcertFile = args[0], args[1], args[2]
				}
				for c.Next() {
					switch c.Val() {
					case "stubzones":
						stubzones = true
					case "path":
						if !c.NextArg() {
							return etcd.Etcd{}, false, c.ArgErr()
						}
						etc.PathPrefix = c.Val()
					case "endpoint":
						args := c.RemainingArgs()
						if len(args) == 0 {
							return etcd.Etcd{}, false, c.ArgErr()
						}
						endpoints = args
					case "upstream":
						args := c.RemainingArgs()
						if len(args) == 0 {
							return etcd.Etcd{}, false, c.ArgErr()
						}
						for i := 0; i < len(args); i++ {
							h, p, e := net.SplitHostPort(args[i])
							if e != nil && p == "" {
								args[i] = h + ":53"
							}
						}
						endpoints = args
						etc.Proxy = proxy.New(args)
					case "tls": // cert key cacertfile
						args := c.RemainingArgs()
						if len(args) != 3 {
							return etcd.Etcd{}, false, c.ArgErr()
						}
						tlsCertFile, tlsKeyFile, tlsCAcertFile = args[0], args[1], args[2]
					}
				}
			}
			client, err := newEtcdClient(endpoints, tlsCertFile, tlsKeyFile, tlsCAcertFile)
			if err != nil {
				return etcd.Etcd{}, false, err
			}
			etc.Client = client
			return etc, stubzones, nil
		}
	}
	return etcd.Etcd{}, false, nil
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
	var cc *tls.Config = nil

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
