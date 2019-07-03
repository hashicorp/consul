package test

import (
	"sync"

	"github.com/coredns/coredns/core/dnsserver"

	// Hook in CoreDNS.
	_ "github.com/coredns/coredns/core"

	"github.com/caddyserver/caddy"
)

var mu sync.Mutex

// CoreDNSServer returns a CoreDNS test server. It just takes a normal Corefile as input.
func CoreDNSServer(corefile string) (*caddy.Instance, error) {
	mu.Lock()
	defer mu.Unlock()
	caddy.Quiet = true
	dnsserver.Quiet = true

	return caddy.Start(NewInput(corefile))
}

// CoreDNSServerStop stops a server.
func CoreDNSServerStop(i *caddy.Instance) { i.Stop() }

// CoreDNSServerPorts returns the ports the instance is listening on. The integer k indicates
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

// CoreDNSServerAndPorts combines CoreDNSServer and CoreDNSServerPorts to start a CoreDNS
// server and returns the udp and tcp ports of the first instance.
func CoreDNSServerAndPorts(corefile string) (i *caddy.Instance, udp, tcp string, err error) {
	i, err = CoreDNSServer(corefile)
	if err != nil {
		return nil, "", "", err
	}
	udp, tcp = CoreDNSServerPorts(i, 0)
	return i, udp, tcp, nil
}

// Input implements the caddy.Input interface and acts as an easy way to use a string as a Corefile.
type Input struct {
	corefile []byte
}

// NewInput returns a pointer to Input, containing the corefile string as input.
func NewInput(corefile string) *Input {
	return &Input{corefile: []byte(corefile)}
}

// Body implements the Input interface.
func (i *Input) Body() []byte { return i.corefile }

// Path implements the Input interface.
func (i *Input) Path() string { return "Corefile" }

// ServerType implements the Input interface.
func (i *Input) ServerType() string { return "dns" }
