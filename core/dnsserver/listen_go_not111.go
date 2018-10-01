// +build !go1.11 !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd

package dnsserver

import "net"

func listen(network, addr string) (net.Listener, error) { return net.Listen(network, addr) }

func listenPacket(network, addr string) (net.PacketConn, error) {
	return net.ListenPacket(network, addr)
}
