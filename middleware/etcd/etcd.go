// Package etcd provides the etcd backend.
package etcd

import (
	"github.com/skynetservices/skydns/singleflight"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type (
	Etcd struct {
		Ttl      uint32
		Priority uint16
		Backend  *Backend
	}
)

type Backend struct {
	client   etcd.KeysAPI
	ctx      context.Context
	config   *Config
	inflight *singleflight.Group
}

// NewBackend returns a new Backend.
func NewBackend(client etcd.KeysAPI, ctx context.Context, config *Config) *Backend {
	return &Backend{
		client:   client,
		ctx:      ctx,
		config:   config,
		inflight: &singleflight.Group{},
	}
}
