package peering

import (
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
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

	if len(tok.ServerAddresses) == 0 && len(tok.ManualServerAddresses) == 0 {
		return errPeeringTokenEmptyServerAddresses
	}
	validAddr := func(addr string) error {
		_, portRaw, err := net.SplitHostPort(addr)
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
		return nil
	}
	for _, addr := range tok.ManualServerAddresses {
		if err := validAddr(addr); err != nil {
			return err
		}
	}
	for _, addr := range tok.ServerAddresses {
		if err := validAddr(addr); err != nil {
			return err
		}
	}

	if len(tok.CA) > 0 && tok.ServerName == "" {
		return errPeeringTokenEmptyServerName
	}

	if tok.PeerID == "" {
		return errPeeringTokenEmptyPeerID
	}

	return nil
}
