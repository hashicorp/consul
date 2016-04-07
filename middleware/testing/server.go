package testing

import (
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

func TCPServer(laddr string) (*dns.Server, string, error) {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, "", err
	}

	server := &dns.Server{Listener: l, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = waitLock.Unlock

	go func() {
		server.ActivateAndServe()
		l.Close()
	}()

	waitLock.Lock()
	return server, l.Addr().String(), nil
}

func UDPServer(laddr string) (*dns.Server, string, chan bool, error) {
	pc, err := net.ListenPacket("udp", laddr)
	if err != nil {
		return nil, "", nil, err
	}
	server := &dns.Server{PacketConn: pc, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = waitLock.Unlock

	stop := make(chan bool)

	go func() {
		server.ActivateAndServe()
		close(stop)
		pc.Close()
	}()

	waitLock.Lock()
	return server, pc.LocalAddr().String(), stop, nil
}
