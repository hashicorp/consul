package test

import (
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// TCPServer starts a DNS server with a TCP listener on laddr.
func TCPServer(laddr string) (*dns.Server, string, error) {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, "", err
	}

	server := &dns.Server{Listener: l, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = func() { waitLock.Unlock() }

	go func() {
		server.ActivateAndServe()
		l.Close()
	}()

	waitLock.Lock()
	return server, l.Addr().String(), nil
}

// UDPServer starts a DNS server with an UDP listener on laddr.
func UDPServer(laddr string) (*dns.Server, string, error) {
	pc, err := net.ListenPacket("udp", laddr)
	if err != nil {
		return nil, "", err
	}
	server := &dns.Server{PacketConn: pc, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = func() { waitLock.Unlock() }

	go func() {
		server.ActivateAndServe()
		pc.Close()
	}()

	waitLock.Lock()
	return server, pc.LocalAddr().String(), nil
}
