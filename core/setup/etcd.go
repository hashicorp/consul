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

	etcdc "github.com/coreos/etcd/client"
)

const defaultEndpoint = "http://127.0.0.1:2379"

// Etcd sets up the etcd middleware.
func Etcd(c *Controller) (middleware.Middleware, error) {
	client, err := etcdParse(c)
	if err != nil {
		return nil, err
	}

	return func(next middleware.Handler) middleware.Handler {
		return etcd.NewEtcd(client, next, c.ServerBlockHosts)
	}, nil
}

func etcdParse(c *Controller) (etcdc.KeysAPI, error) {
	for c.Next() {
		if c.Val() == "etcd" {
			// etcd [address...]
			if !c.NextArg() {
				// TODO(certs) and friends, this is client side
				client, err := newEtcdClient([]string{defaultEndpoint}, "", "", "")
				return client, err
			}
			client, err := newEtcdClient(c.RemainingArgs(), "", "", "")
			return client, err
		}
	}
	return nil, nil
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
