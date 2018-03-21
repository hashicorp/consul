package proxy

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	m := NewManager()

	addrs := TestLocalBindAddrs(t, 3)

	for i := 0; i < len(addrs); i++ {
		name := fmt.Sprintf("proxier-%d", i)
		// Run proxy
		err := m.RunProxier(name, &TestProxier{
			Addr:   addrs[i],
			Prefix: name + ": ",
		})
		require.Nil(t, err)
	}

	// Make sure each one is echoing correctly now all are running
	for i := 0; i < len(addrs); i++ {
		conn, err := net.Dial("tcp", addrs[i])
		require.Nil(t, err)
		TestEchoConn(t, conn, fmt.Sprintf("proxier-%d: ", i))
		conn.Close()
	}

	// Stop first proxier
	err := m.StopProxier("proxier-0")
	require.Nil(t, err)

	// We should fail to dial it now. Note that Runner.Stop is synchronous so
	// there should be a strong guarantee that it's stopped listening by now.
	_, err = net.Dial("tcp", addrs[0])
	require.NotNil(t, err)

	// Rest of proxiers should still be running
	for i := 1; i < len(addrs); i++ {
		conn, err := net.Dial("tcp", addrs[i])
		require.Nil(t, err)
		TestEchoConn(t, conn, fmt.Sprintf("proxier-%d: ", i))
		conn.Close()
	}

	// Stop non-existent proxier should fail
	err = m.StopProxier("foo")
	require.Equal(t, ErrNotExist, err)

	// Add already-running proxier should fail
	err = m.RunProxier("proxier-1", &TestProxier{})
	require.Equal(t, ErrExists, err)

	// But rest should stay running
	for i := 1; i < len(addrs); i++ {
		conn, err := net.Dial("tcp", addrs[i])
		require.Nil(t, err)
		TestEchoConn(t, conn, fmt.Sprintf("proxier-%d: ", i))
		conn.Close()
	}

	// StopAll should stop everything
	err = m.StopAll()
	require.Nil(t, err)

	// Verify failures
	for i := 0; i < len(addrs); i++ {
		_, err = net.Dial("tcp", addrs[i])
		require.NotNilf(t, err, "proxier-%d should not be running", i)
	}
}
