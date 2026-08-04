// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package middleware

import (
	"net"

	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
)

var _ net.Listener = (*ConnLimitListener)(nil)

// ConnLimitListener wraps a net.Listener and enforces a per-client-IP limit on
// the number of concurrent connections using the provided limiter.
//
// Connections that exceed the limit are closed immediately and Accept moves on
// to the next incoming connection, so an over-limit client can never cause the
// wrapped server to observe (and allocate resources for) the connection. This
// protects the external gRPC listeners, whose request-stage controls (ACLs and
// rate limiting) cannot run until after a client has completed the gRPC/TLS
// handshake, from clients that open many connections but never do so.
type ConnLimitListener struct {
	net.Listener
	limiter *connlimit.Limiter
	logger  hclog.Logger
}

// NewConnLimitListener returns a ConnLimitListener wrapping l. limiter must be
// non-nil.
func NewConnLimitListener(l net.Listener, limiter *connlimit.Limiter, logger hclog.Logger) *ConnLimitListener {
	return &ConnLimitListener{Listener: l, limiter: limiter, logger: logger}
}

// Accept accepts the next connection that is within the configured per-client
// connection limit. Connections that would exceed the limit are closed and
// skipped.
func (l *ConnLimitListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		free, err := l.limiter.Accept(conn)
		if err != nil {
			if l.logger != nil {
				l.logger.Warn("rejecting gRPC conn because grpc_max_conns_per_client exceeded",
					"conn", conn.RemoteAddr().String())
			}
			_ = conn.Close()
			continue
		}

		// Wrap conn so the limiter slot is released automatically when the
		// connection is closed.
		return connlimit.Wrap(conn, free), nil
	}
}
