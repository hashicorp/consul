// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

// matches valid DNS labels according to RFC 1123, should be at most 63
// characters according to the RFC. This does not allow uppercase letters, unlike
// node / service validation. All lowercase is enforced to reduce potential issues
// relating to case-mismatch throughout the codebase (state-store lookups,
// envoy listeners, etc).
var validPeeringName = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

// validatePeerName returns an error if the peer name does not match
// the expected format. Returns nil on valid names.
func validatePeerName(name string) error {
	if !validPeeringName.MatchString(name) {
		return errors.New("a valid peering name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}
	return nil
}

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
