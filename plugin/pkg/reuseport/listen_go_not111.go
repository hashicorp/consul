// +build !go1.11 !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd

package reuseport

import "net"

// Listen is a wrapper around net.Listen.
func Listen(network, addr string) (net.Listener, error) { return net.Listen(network, addr) }

// ListenPacket is a wrapper around net.ListenPacket.
func ListenPacket(network, addr string) (net.PacketConn, error) {
	return net.ListenPacket(network, addr)
}
