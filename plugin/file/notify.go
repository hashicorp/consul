package file

import (
	"fmt"
	"log"
	"net"

	"github.com/coredns/coredns/plugin/pkg/rcode"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// isNotify checks if state is a notify message and if so, will *also* check if it
// is from one of the configured masters. If not it will not be a valid notify
// message. If the zone z is not a secondary zone the message will also be ignored.
func (z *Zone) isNotify(state request.Request) bool {
	if state.Req.Opcode != dns.OpcodeNotify {
		return false
	}
	if len(z.TransferFrom) == 0 {
		return false
	}
	// If remote IP matches we accept.
	remote := state.IP()
	for _, f := range z.TransferFrom {
		from, _, err := net.SplitHostPort(f)
		if err != nil {
			continue
		}
		if from == remote {
			return true
		}
	}
	return false
}

// Notify will send notifies to all configured TransferTo IP addresses.
func (z *Zone) Notify() {
	go notify(z.origin, z.TransferTo)
}

// notify sends notifies to the configured remote servers. It will try up to three times
// before giving up on a specific remote. We will sequentially loop through "to"
// until they all have replied (or have 3 failed attempts).
func notify(zone string, to []string) error {
	m := new(dns.Msg)
	m.SetNotify(zone)
	c := new(dns.Client)

	for _, t := range to {
		if t == "*" {
			continue
		}
		if err := notifyAddr(c, m, t); err != nil {
			log.Printf("[ERROR] " + err.Error())
		} else {
			log.Printf("[INFO] Sent notify for zone %q to %q", zone, t)
		}
	}
	return nil
}

func notifyAddr(c *dns.Client, m *dns.Msg, s string) error {
	var err error

	code := dns.RcodeServerFailure
	for i := 0; i < 3; i++ {
		ret, _, err := c.Exchange(m, s)
		if err != nil {
			continue
		}
		code = ret.Rcode
		if code == dns.RcodeSuccess {
			return nil
		}
	}
	if err != nil {
		return fmt.Errorf("notify for zone %q was not accepted by %q: %q", m.Question[0].Name, s, err)
	}
	return fmt.Errorf("notify for zone %q was not accepted by %q: rcode was %q", m.Question[0].Name, s, rcode.ToString(code))
}
