package kubernetes

import (
	"github.com/coredns/coredns/middleware/pkg/dnsutil"

	"github.com/miekg/dns"
)

type recordRequest struct {
	// The named port from the kubernetes DNS spec, this is the service part (think _https) from a well formed
	// SRV record.
	port string
	// The protocol is usually _udp or _tcp (if set), and comes from the protocol part of a well formed
	// SRV record.
	protocol  string
	endpoint  string
	service   string
	namespace string
	// A each name can be for a pod or a service, here we track what we've seen. This value is true for
	// pods and false for services. If we ever need to extend this well use a typed value.
	podOrSvc   string
	zone       string
	federation string
}

// TODO(miek): make it use request.Request.
func (k *Kubernetes) parseRequest(lowerCasedName string, qtype uint16, zone ...string) (r recordRequest, err error) {
	// 3 Possible cases
	//   SRV Request: _port._protocol.service.namespace.[federation.]type.zone
	//   A Request (endpoint): endpoint.service.namespace.[federation.]type.zone
	//   A Request (service): service.namespace.[federation.]type.zone

	if len(zone) == 0 {
		panic("parseRequest must be called with a zone")
	}

	base, _ := dnsutil.TrimZone(lowerCasedName, zone[0])
	segs := dns.SplitDomainName(base)

	r.zone = zone[0]
	r.federation, segs = k.stripFederation(segs)

	if qtype == dns.TypeNS {
		return r, nil
	}

	if qtype == dns.TypeA && isDefaultNS(lowerCasedName, r) {
		return r, nil
	}

	offset := 0
	if qtype == dns.TypeSRV {
		// The kubernetes peer-finder expects queries with empty port and service to resolve
		// If neither is specified, treat it as a wildcard
		if len(segs) == 3 {
			r.port = "*"
			r.service = "*"
			offset = 0
		} else {
			if len(segs) != 5 {
				return r, errInvalidRequest
			}
			// This is a SRV style request, get first two elements as port and
			// protocol, stripping leading underscores if present.
			if segs[0][0] == '_' {
				r.port = segs[0][1:]
			} else {
				r.port = segs[0]
				if !wildcard(r.port) {
					return r, errInvalidRequest
				}
			}
			if segs[1][0] == '_' {
				r.protocol = segs[1][1:]
				if r.protocol != "tcp" && r.protocol != "udp" {
					return r, errInvalidRequest
				}
			} else {
				r.protocol = segs[1]
				if !wildcard(r.protocol) {
					return r, errInvalidRequest
				}
			}
			if r.port == "" || r.protocol == "" {
				return r, errInvalidRequest
			}
			offset = 2
		}
	}
	if (qtype == dns.TypeA || qtype == dns.TypeAAAA) && len(segs) == 4 {
		// This is an endpoint A/AAAA record request. Get first element as endpoint.
		r.endpoint = segs[0]
		offset = 1
	}

	if len(segs) == (offset + 3) {
		r.service = segs[offset]
		r.namespace = segs[offset+1]
		r.podOrSvc = segs[offset+2]

		return r, nil
	}

	return r, errInvalidRequest
}

// String return a string representation of r, it just returns all
// fields concatenated with dots.
// This is mostly used in tests.
func (r recordRequest) String() string {
	s := r.port
	s += "." + r.protocol
	s += "." + r.endpoint
	s += "." + r.service
	s += "." + r.namespace
	s += "." + r.podOrSvc
	s += "." + r.zone
	s += "." + r.federation
	return s
}
