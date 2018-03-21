package proxy

import (
	"bufio"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// testConnSetup listens on a random TCP port and passes the accepted net.Conn
// back to test code on returned channel. It then creates a source and
// destination Conn. And a cleanup func
func testConnSetup(t *testing.T) (net.Conn, net.Conn, func()) {
	t.Helper()

	l, err := net.Listen("tcp", "localhost:0")
	require.Nil(t, err)

	ch := make(chan net.Conn, 1)
	go func(ch chan net.Conn) {
		src, err := l.Accept()
		require.Nil(t, err)
		ch <- src
	}(ch)

	dst, err := net.Dial("tcp", l.Addr().String())
	require.Nil(t, err)

	src := <-ch

	stopper := func() {
		l.Close()
		src.Close()
		dst.Close()
	}

	return src, dst, stopper
}

func TestConn(t *testing.T) {
	src, dst, stop := testConnSetup(t)
	defer stop()

	c := NewConn(src, dst)

	retCh := make(chan error, 1)
	go func() {
		retCh <- c.CopyBytes()
	}()

	srcR := bufio.NewReader(src)
	dstR := bufio.NewReader(dst)

	_, err := src.Write([]byte("ping 1\n"))
	require.Nil(t, err)
	_, err = dst.Write([]byte("ping 2\n"))
	require.Nil(t, err)

	got, err := dstR.ReadString('\n')
	require.Equal(t, "ping 1\n", got)

	got, err = srcR.ReadString('\n')
	require.Equal(t, "ping 2\n", got)

	_, err = src.Write([]byte("pong 1\n"))
	require.Nil(t, err)
	_, err = dst.Write([]byte("pong 2\n"))
	require.Nil(t, err)

	got, err = dstR.ReadString('\n')
	require.Equal(t, "pong 1\n", got)

	got, err = srcR.ReadString('\n')
	require.Equal(t, "pong 2\n", got)

	c.Close()

	ret := <-retCh
	require.Nil(t, ret, "Close() should not cause error return")
}

func TestConnSrcClosing(t *testing.T) {
	src, dst, stop := testConnSetup(t)
	defer stop()

	c := NewConn(src, dst)
	retCh := make(chan error, 1)
	go func() {
		retCh <- c.CopyBytes()
	}()

	// If we close the src conn, we expect CopyBytes to return and src to be
	// closed too. No good way to assert that the conn is closed really other than
	// assume the retCh receive will hand unless CopyBytes exits and that
	// CopyBytes defers Closing both. i.e. if this test doesn't time out it's
	// good!
	src.Close()
	<-retCh
}

func TestConnDstClosing(t *testing.T) {
	src, dst, stop := testConnSetup(t)
	defer stop()

	c := NewConn(src, dst)
	retCh := make(chan error, 1)
	go func() {
		retCh <- c.CopyBytes()
	}()

	// If we close the dst conn, we expect CopyBytes to return and src to be
	// closed too. No good way to assert that the conn is closed really other than
	// assume the retCh receive will hand unless CopyBytes exits and that
	// CopyBytes defers Closing both. i.e. if this test doesn't time out it's
	// good!
	dst.Close()
	<-retCh
}
