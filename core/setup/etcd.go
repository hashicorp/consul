package setup

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	etcdc "github.com/coreos/etcd/client"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file"
)

const defaultAddress = "http://127.0.0.1:2379"

// Etcd sets up the etcd middleware.
func Etcd(c *Controller) (middleware.Middleware, error) {
	keysapi, err := etcdParse(c)
	if err != nil {
		return nil, err
	}

	return func(next middleware.Handler) middleware.Handler {
		return file.File{Next: next, Zones: zones}
	}, nil

}

func etcdParse(c *Controller) (etcdc.KeysAPI, error) {
	for c.Next() {
		if c.Val() == "etcd" {
			// etcd [address...]
			if !c.NextArg() {

				return file.Zones{}, c.ArgErr()
			}
			args1 := c.RemainingArgs()
			fileName := c.Val()

			origin := c.ServerBlockHosts[c.ServerBlockHostIndex]
			if c.NextArg() {
				c.Next()
				origin = c.Val()
			}
			// normalize this origin
			origin = middleware.Host(origin).StandardHost()

			zone, err := parseZone(origin, fileName)
			if err == nil {
				z[origin] = zone
			}
			names = append(names, origin)
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}

func newEtcdClient(machines []string, tlsCert, tlsKey, tlsCACert string) (etcd.KeysAPI, error) {
	etcdCfg := etcd.Config{
		Endpoints: machines,
		Transport: newHTTPSTransport(tlsCert, tlsKey, tlsCACert),
	}
	cli, err := etcd.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return etcd.NewKeysAPI(cli), nil
}

func newHTTPSTransport(tlsCertFile, tlsKeyFile, tlsCACertFile string) etcd.CancelableTransport {
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
