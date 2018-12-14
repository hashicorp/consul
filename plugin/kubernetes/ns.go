package kubernetes

import (
	"net"
	"strings"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
)

func isDefaultNS(name, zone string) bool {
	return strings.Index(name, defaultNSName) == 0 && strings.Index(name, zone) == len(defaultNSName)
}

// nsAddr return the A record for the CoreDNS service in the cluster. If it fails that it fallsback
// on the local address of the machine we're running on.
//
// This function is rather expensive to run.
func (k *Kubernetes) nsAddr() *dns.A {
	var (
		svcName      string
		svcNamespace string
	)

	rr := new(dns.A)
	localIP := k.interfaceAddrsFunc()
	rr.A = localIP

FindEndpoint:
	for _, ep := range k.APIConn.EpIndexReverse(localIP.String()) {
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

FindService:
	for _, svc := range k.APIConn.ServiceList() {
		if svcName == svc.Name && svcNamespace == svc.Namespace {
			if svc.ClusterIP == api.ClusterIPNone {
				rr.A = localIP
			} else {
				rr.A = net.ParseIP(svc.ClusterIP)
			}
			break FindService
		}
	}

	rr.Hdr.Name = strings.Join([]string{svcName, svcNamespace, "svc."}, ".")

	return rr
}

const defaultNSName = "ns.dns."
