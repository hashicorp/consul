package kubernetes

import (
	"net"
	"strings"

	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
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
	rr.A = localIP

	ep := k.APIConn.EpIndexReverse(localIP.String())
	if ep != nil {
	FindEndpoint:
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if localIP.Equal(net.ParseIP(addr.IP)) {
					svcNamespace = ep.Namespace
					svcName = ep.Name
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

	svc := k.APIConn.SvcIndex(object.ServiceKey(svcNamespace, svcName))
	if svc != nil {
		if svc.ClusterIP == api.ClusterIPNone {
			rr.A = localIP
		} else {
			rr.A = net.ParseIP(svc.ClusterIP)
		}
	}

	rr.Hdr.Name = strings.Join([]string{svcName, svcNamespace, "svc."}, ".")

	return rr
}

const defaultNSName = "ns.dns."
