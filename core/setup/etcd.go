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
	etcd, err := etcdParse(c)
	if err != nil {
		return nil, err
	}

	return func(next middleware.Handler) middleware.Handler {
		etcd.Next = next
		return etcd
	}, nil
}

func etcdParse(c *Controller) (etcd.Etcd, error) {
	etc := etcd.Etcd{
		// make stuff configurable
		Proxy:      proxy.New([]string{"8.8.8.8:53"}),
		PathPrefix: "skydns",
		Ctx:        context.Background(),
		Inflight:   &singleflight.Group{},
	}
	for c.Next() {
		if c.Val() == "etcd" {
			// etcd [origin...]
			client, err := newEtcdClient([]string{defaultEndpoint}, "", "", "")
			if err != nil {
				return etcd.Etcd{}, err
			}
			etc.Client = client
			etc.Zones = c.RemainingArgs()
			if len(etc.Zones) == 0 {
				etc.Zones = c.ServerBlockHosts
			}
			middleware.Zones(etc.Zones).FullyQualify()
			return etc, nil
		}
	}
	return etcd.Etcd{}, nil
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
