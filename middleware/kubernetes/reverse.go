package kubernetes

import (
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/etcd/msg"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/request"
)

// Reverse implements the ServiceBackend interface.
func (k *Kubernetes) Reverse(state request.Request, exact bool, opt middleware.Options) ([]msg.Service, []msg.Service, error) {

	ip := dnsutil.ExtractAddressFromReverse(state.Name())
	if ip == "" {
		return nil, nil, nil
	}

	records := k.serviceRecordForIP(ip, state.Name())
	return records, nil, nil
}
