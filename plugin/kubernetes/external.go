package kubernetes

import (
	"strings"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// External implements the ExternalFunc call from the external plugin.
// It returns any services matching in the services' ExternalIPs.
func (k *Kubernetes) External(state request.Request) ([]msg.Service, int) {
	base, _ := dnsutil.TrimZone(state.Name(), state.Zone)

	segs := dns.SplitDomainName(base)
	last := len(segs) - 1
	if last < 0 {
		return nil, dns.RcodeServerFailure
	}
	// We are dealing with a fairly normal domain name here, but we still need to have the service
	// and the namespace:
	// service.namespace.<base>
	//
	// for service (and SRV) you can also say _tcp, and port (i.e. _http), we need those be picked
	// up, unless they are not specified, then we use an internal wildcard.
	port := "*"
	protocol := "*"
	namespace := segs[last]
	if !k.namespaceExposed(namespace) {
		return nil, dns.RcodeNameError
	}

	last--
	if last < 0 {
		return nil, dns.RcodeSuccess
	}

	service := segs[last]
	last--
	if last == 1 {
		protocol = stripUnderscore(segs[last])
		port = stripUnderscore(segs[last-1])
		last -= 2
	}

	if last != -1 {
		// too long
		return nil, dns.RcodeNameError
	}

	idx := object.ServiceKey(service, namespace)
	serviceList := k.APIConn.SvcIndex(idx)

	services := []msg.Service{}
	zonePath := msg.Path(state.Zone, coredns)
	rcode := dns.RcodeNameError

	for _, svc := range serviceList {
		if namespace != svc.Namespace {
			continue
		}
		if service != svc.Name {
			continue
		}

		for _, ip := range svc.ExternalIPs {
			for _, p := range svc.Ports {
				if !(match(port, p.Name) && match(protocol, string(p.Protocol))) {
					continue
				}
				rcode = dns.RcodeSuccess
				s := msg.Service{Host: ip, Port: int(p.Port), TTL: k.ttl}
				s.Key = strings.Join([]string{zonePath, svc.Namespace, svc.Name}, "/")

				services = append(services, s)
			}
		}
	}
	return services, rcode
}

// ExternalAddress returns the external service address(es) for the CoreDNS service.
func (k *Kubernetes) ExternalAddress(state request.Request) []dns.RR {
	// If CoreDNS is running inside the Kubernetes cluster: k.nsAddrs() will return the external IPs of the services
	// targeting the CoreDNS Pod.
	// If CoreDNS is running outside of the Kubernetes cluster: k.nsAddrs() will return the first non-loopback IP
	// address seen on the local system it is running on. This could be the wrong answer if coredns is using the *bind*
	// plugin to bind to a different IP address.
	return k.nsAddrs(true, state.Zone)
}
