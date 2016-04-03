package file

import (
	"fmt"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

// Notify will send notifies to all configured IP addresses.
func (z *Zone) Notify() {
	go notify(z.name, z.Peers)
}

// notify sends notifies to the configured remotes. It will try up to three times
// before giving up on a specific remote. We will sequentially loop through the remotes
// until they all have replied (or have 3 failed attempts).
func notify(zone string, remotes []string) error {
	m := new(dns.Msg)
	m.SetNotify(zone)
	c := new(dns.Client)

	// TODO(miek): error handling? Run this in a goroutine?
	for _, remote := range remotes {
		notifyRemote(c, m, middleware.Addr(remote).Standard())
	}
	return nil
}

func notifyRemote(c *dns.Client, m *dns.Msg, s string) error {
	for i := 0; i < 3; i++ {
		ret, err := middleware.Exchange(c, m, s)
		if err == nil && ret.Rcode == dns.RcodeSuccess || ret.Rcode == dns.RcodeNotImplemented {
			return nil
		}
		// timeout? mean don't want it. should stop sending as well
	}
	return fmt.Errorf("failed to send notify for zone '%s' to '%s'", m.Question[0].Name, s)
}
