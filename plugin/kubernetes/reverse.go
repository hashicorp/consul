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
		return nil, nil
	}

	records := k.serviceRecordForIP(ip, state.Name())
	return records, nil
}

// serviceRecordForIP gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) serviceRecordForIP(ip, name string) []msg.Service {
	// First check services with cluster ips
	for _, service := range k.APIConn.SvcIndexReverse(ip) {
		if len(k.Namespaces) > 0 && !k.namespaceExposed(service.Namespace) {
			continue
		}
		domain := strings.Join([]string{service.Name, service.Namespace, Svc, k.primaryZone()}, ".")
		return []msg.Service{{Host: domain, TTL: k.ttl}}
	}
	// If no cluster ips match, search endpoints
	for _, ep := range k.APIConn.EpIndexReverse(ip) {
		if len(k.Namespaces) > 0 && !k.namespaceExposed(ep.ObjectMeta.Namespace) {
			continue
		}
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if addr.IP == ip {
					domain := strings.Join([]string{endpointHostname(addr, k.endpointNameMode), ep.ObjectMeta.Name, ep.ObjectMeta.Namespace, Svc, k.primaryZone()}, ".")
					return []msg.Service{{Host: domain, TTL: k.ttl}}
				}
			}
		}
	}
	return nil
}
