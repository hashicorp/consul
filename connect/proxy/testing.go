package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/connect"
)

// TestLocalAddr makes a localhost address on the given port
func TestLocalAddr(port int) string {
	return fmt.Sprintf("localhost:%d", port)
}

// TestTCPServer is a simple TCP echo server for use during tests.
type TestTCPServer struct {
	l                        net.Listener
	stopped                  int32
	accepted, closed, active int32
}

// NewTestTCPServer opens as a listening socket on the given address and returns
// a TestTCPServer serving requests to it. The server is already started and can
// be stopped by calling Close().
func NewTestTCPServer(t testing.T) *TestTCPServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	log.Printf("test tcp server listening on %s", l.Addr())
	s := &TestTCPServer{l: l}
	go s.accept()

	return s
}

// Close stops the server
func (s *TestTCPServer) Close() {
	atomic.StoreInt32(&s.stopped, 1)
	if s.l != nil {
		s.l.Close()
	}
}

// Addr returns the address that this server is listening on.
func (s *TestTCPServer) Addr() net.Addr {
	return s.l.Addr()
}

func (s *TestTCPServer) accept() error {
	for {
		conn, err := s.l.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.stopped) == 1 {
				log.Printf("test tcp echo server %s stopped", s.l.Addr())
				return nil
			}
			log.Printf("test tcp echo server %s failed: %s", s.l.Addr(), err)
			return err
		}

		atomic.AddInt32(&s.accepted, 1)
		atomic.AddInt32(&s.active, 1)

		go func(c net.Conn) {
			io.Copy(c, c)
			atomic.AddInt32(&s.closed, 1)
			atomic.AddInt32(&s.active, -1)
		}(conn)
	}
}

// TestEchoConn attempts to write some bytes to conn and expects to read them
// back within a short timeout (10ms). If prefix is not empty we expect it to be
// poresent at the start of all echoed responses (for example to distinguish
// between multiple echo server instances).
func TestEchoConn(t testing.T, conn net.Conn, prefix string) {
	t.Helper()

	// Write some bytes and read them back
	n, err := conn.Write([]byte("Hello World"))
	require.Equal(t, 11, n)
	require.Nil(t, err)

	expectLen := 11 + len(prefix)

	buf := make([]byte, expectLen)
	// read until our buffer is full - it might be separate packets if prefix is
	// in use.
	got := 0
	for got < expectLen {
		n, err = conn.Read(buf[got:])
		require.Nilf(t, err, "err: %s", err)
		got += n
	}
	require.Equal(t, expectLen, got)
	require.Equal(t, prefix+"Hello World", string(buf[:]))

	// Addresses test flakiness around returning before Write or Read finish
	// see PR #4498
	time.Sleep(time.Millisecond)
}

// TestStaticUpstreamResolverFunc returns a function that will return a static
// resolver for testing UpstreamListener.
func TestStaticUpstreamResolverFunc(r connect.Resolver) func(UpstreamConfig) (connect.Resolver, error) {
	return func(UpstreamConfig) (connect.Resolver, error) {
		return r, nil
	}
}
