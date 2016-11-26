package httpproxy

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/miekg/dns"
)

// Exchanger is an interface that specifies a type implementing a DNS resolver that
// uses a HTTPS server.
type Exchanger interface {
	Exchange(*dns.Msg) (*dns.Msg, error)

	SetUpstream(*simpleUpstream) error
	OnStartup() error
	OnShutdown() error
}

func newClient(sni string) *http.Client {
	tls := &tls.Config{ServerName: sni}

	c := &http.Client{
		Timeout:   time.Second * timeOut,
		Transport: &http.Transport{TLSClientConfig: tls},
	}

	return c
}

const timeOut = 5
