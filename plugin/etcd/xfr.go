package etcd

import (
	"time"

	"github.com/coredns/coredns/request"
)

// Serial implements the Transferer interface.
func (e *Etcd) Serial(state request.Request) uint32 {
	return uint32(time.Now().Unix())
}

// MinTTL implements the Transferer interface.
func (e *Etcd) MinTTL(state request.Request) uint32 {
	return 30
}
