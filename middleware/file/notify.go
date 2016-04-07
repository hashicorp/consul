package file

import (
	"fmt"
	"log"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
)

// isNotify checks if state is a notify message and if so, will *also* check if it
// is from one of the configured masters. If not it will not be a valid notify
// message. If the zone z is not a secondary zone the message will also be ignored.
func (z *Zone) isNotify(state middleware.State) bool {
	if state.Req.Opcode != dns.OpcodeNotify {
		return false
	}
	if len(z.TransferFrom) == 0 {
		return false
	}
	remote := state.RemoteAddr()
	for _, from := range z.TransferFrom {
		if from == remote {
			return true
		}
	}
	return false
}

// Notify will send notifies to all configured TransferTo IP addresses.
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

	for _, t := range to {
		if t == "*" {
			continue
		}
		if err := notifyAddr(c, m, t); err != nil {
			log.Printf("[ERROR] " + err.Error())
		} else {
			log.Printf("[INFO] Sent notify for zone %s to %s", zone, t)
		}
	}
	return nil
}

func notifyAddr(c *dns.Client, m *dns.Msg, s string) error {
	for i := 0; i < 3; i++ {
		ret, err := middleware.Exchange(c, m, s)
		if err != nil {
			continue
		}
		if ret.Rcode == dns.RcodeSuccess || ret.Rcode == dns.RcodeNotImplemented {
			return nil
		}
	}
	return fmt.Errorf("Failed to send notify for zone '%s' to '%s'", m.Question[0].Name, s)
}
