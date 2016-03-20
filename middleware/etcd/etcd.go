// Package etcd provides the etcd backend.
package etcd

import (
	"github.com/miekg/coredns/middleware"
	"github.com/skynetservices/skydns/singleflight"

	etcd "github.com/coreos/etcd/client"
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

func NewEtcd(client etcd.KeysAPI, ctx context.Context) Etcd {
	return Etcd{
		client:   client,
		ctx:      ctx,
		inflight: &singleflight.Group{},
	}
}
