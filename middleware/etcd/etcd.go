// Package etcd provides the etcd backend.
package etcd

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
	"github.com/skynetservices/skydns/singleflight"

	etcdc "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type (
	Etcd struct {
		Next middleware.Handler

		client   etcd.KeysAPI
		ctx      context.Context
		inflight *singleflight.Group
	}
)

func NewEtcd(client etcdc.KeysAPI, next middleware.Handler) Etcd {
	return Etcd{
		Next:     next,
		client:   client,
		ctx:      context.Background(),
		inflight: &singleflight.Group{},
	}
}

func (e Etcd) ServerDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return 0, nil
}
