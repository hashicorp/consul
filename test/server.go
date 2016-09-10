package test

import (
	"net"
	"sync"
	"testing"
	"time"

	_ "github.com/miekg/coredns/core"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

// TCPServer returns a generic DNS server listening for TCP connections on laddr.
func TCPServer(t *testing.T, laddr string) (*dns.Server, string, error) {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, "", err
	}

	server := &dns.Server{Listener: l, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = func() { t.Logf("started TCP server on %s", l.Addr()); waitLock.Unlock() }

	go func() {
		server.ActivateAndServe()
		l.Close()
	}()

	waitLock.Lock()
	return server, l.Addr().String(), nil
}

// UDPServer returns a generic DNS server listening for UDP connections on laddr.
func UDPServer(t *testing.T, laddr string) (*dns.Server, string, error) {
	pc, err := net.ListenPacket("udp", laddr)
	if err != nil {
		return nil, "", err
	}
	server := &dns.Server{PacketConn: pc, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = func() { t.Logf("started UDP server on %s", pc.LocalAddr()); waitLock.Unlock() }

	go func() {
		server.ActivateAndServe()
		pc.Close()
	}()

	waitLock.Lock()
	return server, pc.LocalAddr().String(), nil
}

// CoreDNSServer returns a CoreDNS test server. It just takes a normal Corefile as input.
func CoreDNSServer(corefile string) (*caddy.Instance, error) {
	caddy.Quiet = true
	return caddy.Start(NewInput(corefile))
}

// CoreDNSSserverStop stops a server.
func CoreDNSServerStop(i *caddy.Instance) { i.Stop() }

// CoreDNSServeRPorts returns the ports the instance is listening on. The integer k indicates
// which ServerListener you want.
func CoreDNSServerPorts(i *caddy.Instance, k int) (udp, tcp string) {
	srvs := i.Servers()
	if len(srvs) < k+1 {
		return "", ""
	}
	u := srvs[k].LocalAddr()
	t := srvs[k].Addr()

	if u != nil {
		udp = u.String()
	}
	if t != nil {
		tcp = t.String()
	}
	return
}

type Input struct {
	corefile []byte
}

func NewInput(corefile string) *Input {
	return &Input{corefile: []byte(corefile)}
}

func (i *Input) Body() []byte       { return i.corefile }
func (i *Input) Path() string       { return "Corefile" }
func (i *Input) ServerType() string { return "dns" }
