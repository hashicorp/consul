package haproxy2consul

// This file comes from https://github.com/haproxytech/haproxy-consul-connect/
// Please don't modify it without syncing it with its origin
import (
	"crypto/x509"
	"fmt"
	"reflect"
)

type Config struct {
	ServiceName string
	ServiceID   string
	CAsPool     *x509.CertPool
	Downstream  Downstream
	Upstreams   []Upstream
}

type Upstream struct {
	Service          string
	LocalBindAddress string
	LocalBindPort    int

	TLS

	Nodes []UpstreamNode
}

func (n Upstream) Equal(o Upstream) bool {
	return n.LocalBindAddress == o.LocalBindAddress &&
		n.LocalBindPort == o.LocalBindPort &&
		n.TLS.Equal(o.TLS)
}

type UpstreamNode struct {
	Host   string
	Port   int
	Weight int
}

func (n UpstreamNode) ID() string {
	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}

func (n UpstreamNode) Equal(o UpstreamNode) bool {
	return n == o
}

type Downstream struct {
	LocalBindAddress string
	LocalBindPort    int
	TargetAddress    string
	TargetPort       int

	TLS
}

func (d Downstream) Equal(o Downstream) bool {
	return reflect.DeepEqual(d, o)
}

type TLS struct {
	Cert []byte
	Key  []byte
	CAs  [][]byte
}

func (t TLS) Equal(o TLS) bool {
	return reflect.DeepEqual(t, o)
}
