package kubernetes

import (
	"time"

	"github.com/coredns/coredns/request"
)

// Serial implements the Transferer interface.
func (e *Kubernetes) Serial(state request.Request) uint32 {
	return uint32(time.Now().Unix())
}

// MinTTL implements the Transferer interface.
func (e *Kubernetes) MinTTL(state request.Request) uint32 {
	return 30
}
