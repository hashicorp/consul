package kubernetes

import (
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

type recordRequest struct {
	// The named port from the kubernetes DNS spec, this is the service part (think _https) from a well formed
	// SRV record.
	port string
	// The protocol is usually _udp or _tcp (if set), and comes from the protocol part of a well formed
	// SRV record.
	protocol string
	endpoint string
	// The servicename used in Kubernetes.
	service string
	// The namespace used in Kubernetes.
	namespace string
	// A each name can be for a pod or a service, here we track what we've seen, either "pod" or "service".
	podOrSvc string
}

// parseRequest parses the qname to find all the elements we need for querying k8s. Anything
// that is not parsed will have the wildcard "*" value (except r.endpoint).
// Potential underscores are stripped from _port and _protocol.
func parseRequest(state request.Request) (r recordRequest, err error) {
	// 3 Possible cases:
	// 1. _port._protocol.service.namespace.pod|svc.zone
	// 2. (endpoint): endpoint.service.namespace.pod|svc.zone
	// 3. (service): service.namespace.pod|svc.zone
	//
	// Federations are handled in the federation plugin. And aren't parsed here.

	base, _ := dnsutil.TrimZone(state.Name(), state.Zone)
	segs := dns.SplitDomainName(base)

	r.port = "*"
	r.protocol = "*"
	r.service = "*"
	r.namespace = "*"
	// r.endpoint is the odd one out, we need to know if it has been set or not. If it is
	// empty we should skip the endpoint check in k.get(). Hence we cannot set if to "*".

	// start at the right and fill out recordRequest with the bits we find, so we look for
	// pod|svc.namespace.service and then either
	// * endpoint
	// *_protocol._port

	last := len(segs) - 1
	if last < 0 {
		return r, nil
	}
	r.podOrSvc = segs[last]
	if r.podOrSvc != Pod && r.podOrSvc != Svc {
		return r, errInvalidRequest
	}
	last--
	if last < 0 {
		return r, nil
	}

	r.namespace = segs[last]
	last--
	if last < 0 {
		return r, nil
	}

	r.service = segs[last]
	last--
	if last < 0 {
		return r, nil
	}

	// Because of ambiquity we check the labels left: 1: an endpoint. 2: port and protocol.
	// Anything else is a query that is too long to answer and can safely be delegated to return an nxdomain.
	switch last {

	case 0: // endpoint only
		r.endpoint = segs[last]
	case 1: // service and port
		r.protocol = stripUnderscore(segs[last])
		r.port = stripUnderscore(segs[last-1])

	default: // too long
		return r, errInvalidRequest
	}

	return r, nil
}

// stripUnderscore removes a prefixed underscore from s.
func stripUnderscore(s string) string {
	if s[0] != '_' {
		return s
	}
	return s[1:]
}

// String return a string representation of r, it just returns all fields concatenated with dots.
// This is mostly used in tests.
func (r recordRequest) String() string {
	s := r.port
	s += "." + r.protocol
	s += "." + r.endpoint
	s += "." + r.service
	s += "." + r.namespace
	s += "." + r.podOrSvc
	return s
}
