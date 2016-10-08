package test

import (
	"io/ioutil"
	"log"

	"github.com/miekg/coredns/core/dnsserver"

	// Hook in CoreDNS.
	_ "github.com/miekg/coredns/core"

	"github.com/mholt/caddy"
)

// CoreDNSServer returns a CoreDNS test server. It just takes a normal Corefile as input.
func CoreDNSServer(corefile string) (*caddy.Instance, error) {
	caddy.Quiet = true
	dnsserver.Quiet = true
	log.SetOutput(ioutil.Discard)

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
