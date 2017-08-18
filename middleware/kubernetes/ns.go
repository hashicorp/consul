package kubernetes

import (
	"net"
	"strings"

	"github.com/miekg/dns"
	"k8s.io/client-go/1.5/pkg/api"
)

func isDefaultNS(name, zone string) bool {
	return strings.Index(name, defaultNSName) == 0 && strings.Index(name, zone) == len(defaultNSName)
}

func (k *Kubernetes) nsAddr() *dns.A {
	var (
		svcName      string
		svcNamespace string
	)

	rr := new(dns.A)
	localIP := k.interfaceAddrsFunc()
	endpointsList := k.APIConn.EndpointsList()

	rr.A = localIP

FindEndpoint:
	for _, ep := range endpointsList.Items {
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if localIP.Equal(net.ParseIP(addr.IP)) {
					svcNamespace = ep.ObjectMeta.Namespace
					svcName = ep.ObjectMeta.Name
					break FindEndpoint
				}
			}
		}
	}

	if len(svcName) == 0 {
		rr.Hdr.Name = defaultNSName
		rr.A = localIP
		return rr
	}
	// Find service to get ClusterIP
	serviceList := k.APIConn.ServiceList()

FindService:
	for _, svc := range serviceList {
		if svcName == svc.Name && svcNamespace == svc.Namespace {
			if svc.Spec.ClusterIP == api.ClusterIPNone {
				rr.A = localIP
			} else {
				rr.A = net.ParseIP(svc.Spec.ClusterIP)
			}
			break FindService
		}
	}

	rr.Hdr.Name = strings.Join([]string{svcName, svcNamespace, "svc."}, ".")

	return rr
}

const defaultNSName = "ns.dns."
