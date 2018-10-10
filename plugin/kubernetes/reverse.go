package kubernetes

import (
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
)

// Reverse implements the ServiceBackend interface.
func (k *Kubernetes) Reverse(state request.Request, exact bool, opt plugin.Options) ([]msg.Service, error) {

	ip := dnsutil.ExtractAddressFromReverse(state.Name())
	if ip == "" {
		_, e := k.Records(state, exact)
		return nil, e
	}

	record := k.serviceRecordForIP(ip, state.Name())
	if record == nil {
		return nil, errNoItems
	}
	return []msg.Service{*record}, nil
}

// serviceRecordForIP gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) serviceRecordForIP(ip, name string) *msg.Service {
	// First check services with cluster ips
	service := k.APIConn.SvcIndexReverse(ip)
	if service != nil {
		if len(k.Namespaces) > 0 && !k.namespaceExposed(service.Namespace) {
			return nil
		}
		domain := strings.Join([]string{service.Name, service.Namespace, Svc, k.primaryZone()}, ".")
		return &msg.Service{Host: domain, TTL: k.ttl}
	}
	// If no cluster ips match, search endpoints
	ep := k.APIConn.EpIndexReverse(ip)
	if ep == nil || len(k.Namespaces) > 0 && !k.namespaceExposed(ep.Namespace) {
		return nil
	}
	for _, eps := range ep.Subsets {
		for _, addr := range eps.Addresses {
			if addr.IP == ip {
				domain := strings.Join([]string{endpointHostname(addr, k.endpointNameMode), ep.Name, ep.Namespace, Svc, k.primaryZone()}, ".")
				return &msg.Service{Host: domain, TTL: k.ttl}
			}
		}
	}
	return nil
}
