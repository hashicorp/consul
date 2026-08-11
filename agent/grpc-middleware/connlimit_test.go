// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package middleware

import (
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLimiter returns a Limiter configured with the given per-IP limit.
func newTestLimiter(maxConns int) *connlimit.Limiter {
	l := &connlimit.Limiter{}
	l.SetConfig(connlimit.Config{MaxConnsPerClientIP: maxConns})
	return l
}

// tcpListener opens a localhost TCP listener and registers a cleanup to close
// it when the test finishes.
func tcpListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	return ln
}

// dialAsync dials addr in a goroutine and returns a channel that receives the
// resulting conn (or nil on error).
func dialAsync(addr string) chan net.Conn {
	ch := make(chan net.Conn, 1)
	go func() {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			ch <- nil
			return
		}
		ch <- c
	}()
	return ch
}

// TestConnLimitListener_AcceptsUnderLimit verifies that connections up to the
// configured per-IP limit are forwarded to the caller and that the limiter
// slot is held until the connection is closed.
func TestConnLimitListener_AcceptsUnderLimit(t *testing.T) {
	raw := tcpListener(t)
	limiter := newTestLimiter(3)
	limited := NewConnLimitListener(raw, limiter, hclog.NewNullLogger())

	var serverConns []net.Conn
	for i := 0; i < 3; i++ {
		ch := dialAsync(raw.Addr().String())
		conn, err := limited.Accept()
		require.NoError(t, err, "connection %d should be accepted under limit", i+1)
		serverConns = append(serverConns, conn)
		client := <-ch
		require.NotNil(t, client)
		defer client.Close()
	}

	// All three limiter slots must be held.
	assert.Equal(t, 3, limiter.NumOpen(raw.Addr()))

	// Closing the server-side conns must release every slot.
	for _, c := range serverConns {
		c.Close()
	}
	assert.Equal(t, 0, limiter.NumOpen(raw.Addr()))
}

// TestConnLimitListener_RejectsOverLimit verifies that a connection that
// exceeds the per-IP limit is immediately closed by Accept. Accept then loops
// and returns the next valid connection — the gRPC server never observes the
// rejected one.
func TestConnLimitListener_RejectsOverLimit(t *testing.T) {
	raw := tcpListener(t)
	limiter := newTestLimiter(1)
	limited := NewConnLimitListener(raw, limiter, hclog.NewNullLogger())

	// Accept the first connection — fills the only available slot.
	ch1 := dialAsync(raw.Addr().String())
	first, err := limited.Accept()
	require.NoError(t, err)
	client1 := <-ch1
	require.NotNil(t, client1)

	// Dial a second connection while the slot is full; the limiter should
	// close it on the server side.
	ch2 := dialAsync(raw.Addr().String())
	client2 := <-ch2
	require.NotNil(t, client2)

	// Free the first slot so Accept can progress past the rejected conn.
	first.Close()
	client1.Close()

	// Dial a third connection which should be accepted after the over-limit
	// second one is rejected.
	ch3 := dialAsync(raw.Addr().String())

	// Accept loops: skips the over-limit conn, then returns the third.
	third, err := limited.Accept()
	require.NoError(t, err, "Accept should return the next valid connection after skipping the over-limit one")
	defer third.Close()

	client3 := <-ch3
	require.NotNil(t, client3)
	defer client3.Close()

	// client2 should have been closed by the limiter — a Read must return an
	// error (EOF or connection reset).
	client2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, readErr := client2.Read(buf)
	assert.Error(t, readErr, "over-limit connection should have been closed by the limiter")
	client2.Close()
}

// TestConnLimitListener_SlotReleasedOnClose verifies that closing a server-side
// connection releases the limiter slot so the same IP can reconnect.
func TestConnLimitListener_SlotReleasedOnClose(t *testing.T) {
	raw := tcpListener(t)
	limiter := newTestLimiter(1)
	limited := NewConnLimitListener(raw, limiter, hclog.NewNullLogger())

	// Accept and immediately close the first connection.
	ch1 := dialAsync(raw.Addr().String())
	first, err := limited.Accept()
	require.NoError(t, err)
	client1 := <-ch1
	require.NotNil(t, client1)
	client1.Close()
	first.Close() // releases the slot

	assert.Equal(t, 0, limiter.NumOpen(raw.Addr()),
		"slot must be released after server-side close")

	// The same IP should now be able to reconnect.
	ch2 := dialAsync(raw.Addr().String())
	second, err := limited.Accept()
	require.NoError(t, err, "reconnection from same IP should succeed after slot is released")
	defer second.Close()
	client2 := <-ch2
	require.NotNil(t, client2)
	defer client2.Close()
}

// TestConnLimitListener_NilLogger verifies that a nil logger does not cause a
// panic when a connection is rejected. We use a limit of 1 and hold the slot
// open so that a second connection triggers the rejection path (nil logger
// guard), then close the listener to unblock Accept.
func TestConnLimitListener_NilLogger(t *testing.T) {
	raw := tcpListener(t)
	limiter := newTestLimiter(1)
	limited := NewConnLimitListener(raw, limiter, nil)

	// Accept the first connection to fill the only slot.
	ch1 := dialAsync(raw.Addr().String())
	first, err := limited.Accept()
	require.NoError(t, err)
	client1 := <-ch1
	require.NotNil(t, client1)
	defer client1.Close()
	defer first.Close()

	// Dial a second connection; it will be over-limit and hit the nil-logger
	// code path. Then close the raw listener to unblock Accept.
	ch2 := dialAsync(raw.Addr().String())
	go func() {
		c := <-ch2
		if c != nil {
			c.Close()
		}
		raw.Close()
	}()

	_, err = limited.Accept()
	// The expected outcome is an error from the closed listener, not a panic.
	assert.Error(t, err, "Accept should return an error when the listener is closed")
}
