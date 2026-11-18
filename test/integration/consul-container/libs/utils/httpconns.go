// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"bufio"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// HostPort returns the host:port of an agent address as found in an API client
// configuration, which may or may not carry a scheme.
func HostPort(address string) string {
	if !strings.Contains(address, "://") {
		return address
	}
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	return u.Host
}

// HTTPConn is a persistent client connection to an agent's HTTP port.
type HTTPConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// OpenHTTPConn opens a connection to addr and makes one request on it,
// returning the status code that the agent answered with. The connection stays
// open so that it counts against the agent's per-client connection limit.
func OpenHTTPConn(t *testing.T, addr string) (*HTTPConn, int) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	require.NoError(t, err)
	c := &HTTPConn{conn: conn, reader: bufio.NewReader(conn)}

	status := c.Get(t, "/v1/status/leader")
	return c, status
}

// Get makes a request on the open connection.
func (c *HTTPConn) Get(t *testing.T, path string) int {
	t.Helper()

	require.NoError(t, c.conn.SetDeadline(time.Now().Add(10*time.Second)))
	_, err := c.conn.Write([]byte("GET " + path + " HTTP/1.1\r\nHost: consul\r\n\r\n"))
	require.NoError(t, err)

	resp, err := http.ReadResponse(c.reader, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = bufio.NewReader(resp.Body).Discard(1 << 20)
	return resp.StatusCode
}

func (c *HTTPConn) Close() { _ = c.conn.Close() }

// FillHTTPConns opens connections until the agent turns one away or max
// connections are open. It returns the connections that were accepted, which
// the caller must close, and the status of the one that was refused (zero if
// none was).
func FillHTTPConns(t *testing.T, addr string, max int) (accepted []*HTTPConn, refusedStatus int) {
	t.Helper()
	for i := 0; i < max; i++ {
		c, status := OpenHTTPConn(t, addr)
		if status != http.StatusOK {
			c.Close()
			return accepted, status
		}
		accepted = append(accepted, c)
	}
	return accepted, 0
}

// CloseHTTPConns closes all of conns.
func CloseHTTPConns(conns []*HTTPConn) {
	for _, c := range conns {
		c.Close()
	}
}
