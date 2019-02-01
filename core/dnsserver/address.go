package dnsserver

import (
	"fmt"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/transport"

	"github.com/miekg/dns"
)

type zoneAddr struct {
	Zone      string
	Port      string
	Transport string     // dns, tls or grpc
	IPNet     *net.IPNet // if reverse zone this hold the IPNet
	Address   string     // used for bound zoneAddr - validation of overlapping
}

// String returns the string representation of z.
func (z zoneAddr) String() string {
	s := z.Transport + "://" + z.Zone + ":" + z.Port
	if z.Address != "" {
		s += " on " + z.Address
	}
	return s
}

// normalizeZone parses a zone string into a structured format with separate
// host, and port portions, as well as the original input string.
func normalizeZone(str string) (zoneAddr, error) {
	trans, str := parse.Transport(str)

	host, port, ipnet, err := plugin.SplitHostPort(str)
	if err != nil {
		return zoneAddr{}, err
	}

	if port == "" {
		switch trans {
		case transport.DNS:
			port = Port
		case transport.TLS:
			port = transport.TLSPort
		case transport.GRPC:
			port = transport.GRPCPort
		case transport.HTTPS:
			port = transport.HTTPSPort
		}
	}

	return zoneAddr{Zone: dns.Fqdn(host), Port: port, Transport: trans, IPNet: ipnet}, nil
}

// SplitProtocolHostPort splits a full formed address like "dns://[::1]:53" into parts.
func SplitProtocolHostPort(address string) (protocol string, ip string, port string, err error) {
	parts := strings.Split(address, "://")
	switch len(parts) {
	case 1:
		ip, port, err := net.SplitHostPort(parts[0])
		return "", ip, port, err
	case 2:
		ip, port, err := net.SplitHostPort(parts[1])
		return parts[0], ip, port, err
	default:
		return "", "", "", fmt.Errorf("provided value is not in an address format : %s", address)
	}
}

type zoneOverlap struct {
	registeredAddr map[zoneAddr]zoneAddr // each zoneAddr is registered once by its key
	unboundOverlap map[zoneAddr]zoneAddr // the "no bind" equiv ZoneAddr is registered by its original key
}

func newOverlapZone() *zoneOverlap {
	return &zoneOverlap{registeredAddr: make(map[zoneAddr]zoneAddr), unboundOverlap: make(map[zoneAddr]zoneAddr)}
}

// registerAndCheck adds a new zoneAddr for validation, it returns information about existing or overlapping with already registered
// we consider that an unbound address is overlapping all bound addresses for same zone, same port
func (zo *zoneOverlap) registerAndCheck(z zoneAddr) (existingZone *zoneAddr, overlappingZone *zoneAddr) {

	if exist, ok := zo.registeredAddr[z]; ok {
		// exact same zone already registered
		return &exist, nil
	}
	uz := zoneAddr{Zone: z.Zone, Address: "", Port: z.Port, Transport: z.Transport}
	if already, ok := zo.unboundOverlap[uz]; ok {
		if z.Address == "" {
			// current is not bound to an address, but there is already another zone with a bind address registered
			return nil, &already
		}
		if _, ok := zo.registeredAddr[uz]; ok {
			// current zone is bound to an address, but there is already an overlapping zone+port with no bind address
			return nil, &uz
		}
	}
	// there is no overlap, keep the current zoneAddr for future checks
	zo.registeredAddr[z] = z
	zo.unboundOverlap[uz] = z
	return nil, nil
}
