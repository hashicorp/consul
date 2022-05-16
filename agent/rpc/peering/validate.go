package peering

import (
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"

	// TODO: replace this with net/netip when we upgrade to go1.18
	"inet.af/netaddr"
)

// validatePeeringToken ensures that the token has valid values.
func validatePeeringToken(tok *structs.PeeringToken) error {
	// the CA values here should be valid x509 certs
	for _, certStr := range tok.CA {
		// TODO(peering): should we put these in a cert pool on the token?
		// maybe there's a better place to do the parsing?
		if _, err := connect.ParseCert(certStr); err != nil {
			return fmt.Errorf("peering token invalid CA: %w", err)
		}
	}

	if len(tok.ServerAddresses) == 0 {
		return errPeeringTokenEmptyServerAddresses
	}
	for _, addr := range tok.ServerAddresses {
		host, portRaw, err := net.SplitHostPort(addr)
		if err != nil {
			return &errPeeringInvalidServerAddress{addr}
		}

		port, err := strconv.Atoi(portRaw)
		if err != nil {
			return &errPeeringInvalidServerAddress{addr}
		}
		if port < 1 || port > 65535 {
			return &errPeeringInvalidServerAddress{addr}
		}
		if _, err := netaddr.ParseIP(host); err != nil {
			return &errPeeringInvalidServerAddress{addr}
		}
	}

	// TODO(peering): validate name matches SNI?
	// TODO(peering): validate name well formed?
	if tok.ServerName == "" {
		return errPeeringTokenEmptyServerName
	}

	if tok.PeerID == "" {
		return errPeeringTokenEmptyPeerID
	}

	return nil
}
