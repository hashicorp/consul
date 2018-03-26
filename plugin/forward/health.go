package forward

import (
	"sync/atomic"

	"github.com/miekg/dns"
)

// For HC we send to . IN NS +norec message to the upstream. Dial timeouts and empty
// replies are considered fails, basically anything else constitutes a healthy upstream.

// Check is used as the up.Func in the up.Probe.
func (p *Proxy) Check() error {
	err := p.send()
	if err != nil {
		HealthcheckFailureCount.WithLabelValues(p.addr).Add(1)
		atomic.AddUint32(&p.fails, 1)
		return err
	}

	atomic.StoreUint32(&p.fails, 0)
	return nil
}

func (p *Proxy) send() error {
	hcping := new(dns.Msg)
	hcping.SetQuestion(".", dns.TypeNS)

	m, _, err := p.client.Exchange(hcping, p.addr)
	// If we got a header, we're alright, basically only care about I/O errors 'n stuff
	if err != nil && m != nil {
		// Silly check, something sane came back
		if m.Response || m.Opcode == dns.OpcodeQuery {
			err = nil
		}
	}

	return err
}
