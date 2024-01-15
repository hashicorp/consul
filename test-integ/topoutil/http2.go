// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/net/http2"
)

// EnableHTTP2 returns a new shallow copy of client that has been tweaked to do
// h2c (cleartext http2).
//
// Note that this clears the Client.Transport.Proxy trick because http2 and
// http proxies are incompatible currently in Go.
func EnableHTTP2(client *http.Client) *http.Client {
	// Shallow copy, and swap the transport
	client2 := *client
	client = &client2
	client.Transport = &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
	return client
}
