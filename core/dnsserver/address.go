package dnsserver

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type zoneAddr struct {
	Zone      string
	Port      string
	Transport string // dns, tls or grpc
}

// String return the string representation of z.
func (z zoneAddr) String() string { return z.Transport + "://" + z.Zone + ":" + z.Port }

// Transport returns the protocol of the string s
func Transport(s string) string {
	switch {
	case strings.HasPrefix(s, TransportTLS+"://"):
		return TransportTLS
	case strings.HasPrefix(s, TransportDNS+"://"):
		return TransportDNS
	case strings.HasPrefix(s, TransportGRPC+"://"):
		return TransportGRPC
	}
	return TransportDNS
}

// normalizeZone parses an zone string into a structured format with separate
// host, and port portions, as well as the original input string.
//
// TODO(miek): possibly move this to middleware/normalize.go
func normalizeZone(str string) (zoneAddr, error) {
	var err error

	// Default to DNS if there isn't a transport protocol prefix.
	trans := TransportDNS

	switch {
	case strings.HasPrefix(str, TransportTLS+"://"):
		trans = TransportTLS
		str = str[len(TransportTLS+"://"):]
	case strings.HasPrefix(str, TransportDNS+"://"):
		trans = TransportDNS
		str = str[len(TransportDNS+"://"):]
	case strings.HasPrefix(str, TransportGRPC+"://"):
		trans = TransportGRPC
		str = str[len(TransportGRPC+"://"):]
	}

	host, port, err := net.SplitHostPort(str)
	if err != nil {
		host, port, err = net.SplitHostPort(str + ":")
		// no error check here; return err at end of function
	}

	if len(host) > 255 { // TODO(miek): this should take escaping into account.
		return zoneAddr{}, fmt.Errorf("specified zone is too long: %d > 255", len(host))
	}
	_, d := dns.IsDomainName(host)
	if !d {
		return zoneAddr{}, fmt.Errorf("zone is not a valid domain name: %s", host)
	}

	if port == "" {
		if trans == TransportDNS {
			port = Port
		}
		if trans == TransportTLS {
			port = TLSPort
		}
		if trans == TransportGRPC {
			port = GRPCPort
		}
	}

	return zoneAddr{Zone: strings.ToLower(dns.Fqdn(host)), Port: port, Transport: trans}, err
}

// Supported transports.
const (
	TransportDNS  = "dns"
	TransportTLS  = "tls"
	TransportGRPC = "grpc"
)
