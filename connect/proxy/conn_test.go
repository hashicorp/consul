package proxy

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// Assert io.Closer implementation
var _ io.Closer = new(Conn)

// testConnPairSetup creates a TCP connection by listening on a random port, and
// returns both ends. Ready to have data sent down them. It also returns a
// closer function that will close both conns and the listener.
func testConnPairSetup(t *testing.T) (net.Conn, net.Conn, func()) {
	t.Helper()

	l, err := net.Listen("tcp", "localhost:0")
	require.Nil(t, err)

	ch := make(chan net.Conn, 1)
	go func() {
		src, err := l.Accept()
		require.Nil(t, err)
		ch <- src
	}()

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

// testConnPipelineSetup creates a pipeline consiting of two TCP connection
// pairs and a Conn that copies bytes between them. Data flow looks like this:
//
//	src1 <---> dst1 <== Conn.CopyBytes ==> src2 <---> dst2
//
// The returned values are the src1 and dst2 which should be able to send and
// receive to each other via the Conn, the Conn itself (not running), and a
// stopper func to close everything.
func testConnPipelineSetup(t *testing.T) (net.Conn, net.Conn, *Conn, func()) {
	src1, dst1, stop1 := testConnPairSetup(t)
	src2, dst2, stop2 := testConnPairSetup(t)
	c := NewConn(dst1, src2)
	return src1, dst2, c, func() {
		c.Close()
		stop1()
		stop2()
	}
}

func TestConn(t *testing.T) {
	src, dst, c, stop := testConnPipelineSetup(t)
	defer stop()

	retCh := make(chan error, 1)
	go func() {
		retCh <- c.CopyBytes()
	}()

	// Now write/read into the other ends of the pipes (src1, dst2)
	srcR := bufio.NewReader(src)
	dstR := bufio.NewReader(dst)

	_, err := src.Write([]byte("ping 1\n"))
	require.Nil(t, err)
	_, err = dst.Write([]byte("ping 2\n"))
	require.Nil(t, err)

	got, err := dstR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "ping 1\n", got)

	got, err = srcR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "ping 2\n", got)

	retry.Run(t, func(r *retry.R) {
		tx, rx := c.Stats()
		assert.Equal(r, uint64(7), tx)
		assert.Equal(r, uint64(7), rx)
	})

	_, err = src.Write([]byte("pong 1\n"))
	require.Nil(t, err)
	_, err = dst.Write([]byte("pong 2\n"))
	require.Nil(t, err)

	got, err = dstR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "pong 1\n", got)

	got, err = srcR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "pong 2\n", got)

	retry.Run(t, func(r *retry.R) {
		tx, rx := c.Stats()
		assert.Equal(r, uint64(14), tx)
		assert.Equal(r, uint64(14), rx)
	})

	c.Close()

	ret := <-retCh
	require.Nil(t, ret, "Close() should not cause error return")
}

func TestConnSrcClosing(t *testing.T) {
	src, dst, c, stop := testConnPipelineSetup(t)
	defer stop()

	retCh := make(chan error, 1)
	go func() {
		retCh <- c.CopyBytes()
	}()

	// Wait until we can actually get some bytes through both ways so we know that
	// the copy goroutines are running.
	srcR := bufio.NewReader(src)
	dstR := bufio.NewReader(dst)

	_, err := src.Write([]byte("ping 1\n"))
	require.Nil(t, err)
	_, err = dst.Write([]byte("ping 2\n"))
	require.Nil(t, err)

	got, err := dstR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "ping 1\n", got)
	got, err = srcR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "ping 2\n", got)

	// If we close the src conn, we expect CopyBytes to return and dst to be
	// closed too. No good way to assert that the conn is closed really other than
	// assume the retCh receive will hang unless CopyBytes exits and that
	// CopyBytes defers Closing both.
	testTimer := time.AfterFunc(3*time.Second, func() {
		panic("test timeout")
	})
	src.Close()
	<-retCh
	testTimer.Stop()
}

func TestConnDstClosing(t *testing.T) {
	src, dst, c, stop := testConnPipelineSetup(t)
	defer stop()

	retCh := make(chan error, 1)
	go func() {
		retCh <- c.CopyBytes()
	}()

	// Wait until we can actually get some bytes through both ways so we know that
	// the copy goroutines are running.
	srcR := bufio.NewReader(src)
	dstR := bufio.NewReader(dst)

	_, err := src.Write([]byte("ping 1\n"))
	require.Nil(t, err)
	_, err = dst.Write([]byte("ping 2\n"))
	require.Nil(t, err)

	got, err := dstR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "ping 1\n", got)
	got, err = srcR.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, "ping 2\n", got)

	// If we close the dst conn, we expect CopyBytes to return and src to be
	// closed too. No good way to assert that the conn is closed really other than
	// assume the retCh receive will hang unless CopyBytes exits and that
	// CopyBytes defers Closing both. i.e. if this test doesn't time out it's
	// good!
	testTimer := time.AfterFunc(3*time.Second, func() {
		panic("test timeout")
	})
	src.Close()
	<-retCh
	testTimer.Stop()
}
