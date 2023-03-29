// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package middleware

import (
	"fmt"
	"net"

	"google.golang.org/grpc/credentials"
)

var _ net.Listener = (*LabelledListener)(nil)
var _ net.Conn = (*LabelledConn)(nil)

type Protocol int

var (
	ProtocolPlaintext Protocol = 0
	ProtocolTLS       Protocol = 1
)

// LabelledListener wraps a listener and attaches pre-specified
// fields to each spawned connection.
type LabelledListener struct {
	net.Listener
	Protocol Protocol
}

func (l LabelledListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if conn != nil {
		conn = LabelledConn{conn, l.Protocol}
	}
	return conn, err
}

// LabelledConn wraps a connection and provides extra metadata fields.
type LabelledConn struct {
	net.Conn
	protocol Protocol
}

var _ credentials.TransportCredentials = (*optionalTransportCredentials)(nil)

// optionalTransportCredentials provides a way to selectively perform a TLS handshake
// based on metadata extracted from the underlying connection object.
type optionalTransportCredentials struct {
	credentials.TransportCredentials
	logger Logger
}

func NewOptionalTransportCredentials(creds credentials.TransportCredentials, logger Logger) credentials.TransportCredentials {
	return &optionalTransportCredentials{creds, logger}
}

// ServerHandshake will attempt to detect the underlying connection protocol (TLS or Plaintext)
// based on metadata attached to the underlying connection. If TLS is detected, then a handshake
// will be performed, and the corresponding AuthInfo will be attached to the gRPC context.
// For plaintext connections, this is effectively a no-op, since there is no TLS info to attach.
// If the underlying connection is not a LabelledConn with a valid protocol, then this method will
// panic and prevent the gRPC connection from successfully progressing further.
func (tc *optionalTransportCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	// This should always be a LabelledConn, so no check is necessary.
	nc := conn.(LabelledConn)
	switch nc.protocol {
	case ProtocolPlaintext:
		// This originated from a plaintext listener, so do not use TLS auth.
		return nc, nil, nil
	case ProtocolTLS:
		// This originated from a TLS listener, so it should have a full handshake performed.
		c, ai, err := tc.TransportCredentials.ServerHandshake(conn)
		if err == nil && ai == nil {
			// This should not be possible, but ensure that it's non-nil for safety.
			return nil, nil, fmt.Errorf("missing auth info after handshake")
		}
		return c, ai, err
	default:
		return nil, nil, fmt.Errorf("invalid protocol for grpc connection")
	}
}
