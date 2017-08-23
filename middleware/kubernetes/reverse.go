package kubernetes

import (
	"strings"

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

// serviceRecordForIP gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) serviceRecordForIP(ip, name string) []msg.Service {
	// First check services with cluster ips
	svcList := k.APIConn.ServiceList()

	for _, service := range svcList {
		if (len(k.Namespaces) > 0) && !k.namespaceExposed(service.Namespace) {
			continue
		}
		if service.Spec.ClusterIP == ip {
			domain := strings.Join([]string{service.Name, service.Namespace, Svc, k.primaryZone()}, ".")
			return []msg.Service{{Host: domain}}
		}
	}
	// If no cluster ips match, search endpoints
	epList := k.APIConn.EndpointsList()
	for _, ep := range epList.Items {
		if (len(k.Namespaces) > 0) && !k.namespaceExposed(ep.ObjectMeta.Namespace) {
			continue
		}
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if addr.IP == ip {
					domain := strings.Join([]string{endpointHostname(addr), ep.ObjectMeta.Name, ep.ObjectMeta.Namespace, Svc, k.primaryZone()}, ".")
					return []msg.Service{{Host: domain}}
				}
			}
		}
	}
	return nil
}
