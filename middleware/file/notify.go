package file

import (
	"fmt"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
)

// Notify will send notifies to all configured IP addresses.
func (z *Zone) Notify() {
	go notify(z.name, z.TransferTo)
}

// notify sends notifies to the configured remote servers. It will try up to three times
// before giving up on a specific remote. We will sequentially loop through "to"
// until they all have replied (or have 3 failed attempts).
func notify(zone string, to []string) error {
	m := new(dns.Msg)
	m.SetNotify(zone)
	c := new(dns.Client)

	// TODO(miek): error handling? Run this in a goroutine?
	for _, t := range to {
		notifyAddr(c, m, t)
	}
	return nil
}

func notifyAddr(c *dns.Client, m *dns.Msg, s string) error {
	for i := 0; i < 3; i++ {
		ret, err := middleware.Exchange(c, m, s)
		if err == nil && ret.Rcode == dns.RcodeSuccess || ret.Rcode == dns.RcodeNotImplemented {
			return nil
		}
		// timeout? mean don't want it. should stop sending as well?
	}
	return fmt.Errorf("failed to send notify for zone '%s' to '%s'", m.Question[0].Name, s)
}
