// +build go1.11
// +build aix darwin dragonfly freebsd linux netbsd openbsd

package dnsserver

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

const supportsReusePort = true

func reuseportControl(network, address string, c syscall.RawConn) (opErr error) {
	err := c.Control(func(fd uintptr) {
		opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}
	return opErr
}

func listen(network, addr string) (net.Listener, error) {
	lc := net.ListenConfig{Control: reuseportControl}
	return lc.Listen(context.Background(), network, addr)
}

func listenPacket(network, addr string) (net.PacketConn, error) {
	lc := net.ListenConfig{Control: reuseportControl}
	return lc.ListenPacket(context.Background(), network, addr)
}
