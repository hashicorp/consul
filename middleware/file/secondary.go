package file

import (
	"log"

	"github.com/miekg/dns"
)

// TransferIn retrieves the zone from the masters, parses it and sets it live.
func (z *Zone) TransferIn() error {
	if len(z.TransferFrom) == 0 {
		return nil
	}
	t := new(dns.Transfer)
	m := new(dns.Msg)
	m.SetAxfr(z.name)

	var Err error
Transfer:
	for _, tr := range z.TransferFrom {
		c, err := t.In(m, tr)
		if err != nil {
			log.Printf("[ERROR] failed to setup transfer %s with %s: %v", z.name, z.TransferFrom[0], err)
			Err = err
			continue Transfer
		}
		for env := range c {
			if env.Error != nil {
				log.Printf("[ERROR] failed to parse transfer %s: %v", z.name, env.Error)
				Err = env.Error
				continue Transfer
			}
			for _, rr := range env.RR {
				if rr.Header().Rrtype == dns.TypeSOA {
					z.SOA = rr.(*dns.SOA)
					continue
				}
				if rr.Header().Rrtype == dns.TypeRRSIG {
					if x, ok := rr.(*dns.RRSIG); ok && x.TypeCovered == dns.TypeSOA {
						z.SIG = append(z.SIG, x)
					}
				}
				z.Insert(rr)
			}
		}
	}
	return nil
	return Err // ignore errors for now. TODO(miek)
}
