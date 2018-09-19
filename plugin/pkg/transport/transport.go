package transport

import (
	"strings"
)

// Parse returns the transport defined in s and a string where the
// transport prefix is removed (if there was any). If no transport is defined
// we default to TransportDNS
func Parse(s string) (transport string, addr string) {
	switch {
	case strings.HasPrefix(s, TLS+"://"):
		s = s[len(TLS+"://"):]
		return TLS, s

	case strings.HasPrefix(s, DNS+"://"):
		s = s[len(DNS+"://"):]
		return DNS, s

	case strings.HasPrefix(s, GRPC+"://"):
		s = s[len(GRPC+"://"):]
		return GRPC, s

	case strings.HasPrefix(s, HTTPS+"://"):
		s = s[len(HTTPS+"://"):]

		return HTTPS, s
	}

	return DNS, s
}

// Supported transports.
const (
	DNS   = "dns"
	TLS   = "tls"
	GRPC  = "grpc"
	HTTPS = "https"
)

// Port numbers for the various protocols
const (
	// TLSPort is the default port for DNS-over-TLS.
	TLSPort = "853"
	// GRPCPort is the default port for DNS-over-gRPC.
	GRPCPort = "443"
	// HTTPSPort is the default port for DNS-over-HTTPS.
	HTTPSPort = "443"
)
