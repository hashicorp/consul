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
	/*
	   t.TsigSecret = map[string]string{"axfr.": "so6ZGir4GPAqINNh9U5c3A=="}
	   m.SetTsig("axfr.", dns.HmacMD5, 300, time.Now().Unix())
	*/

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
	return Err
}

/*

				28800      ; refresh (8 hours)
				7200       ; retry (2 hours)
				604800     ; expire (1 week)
				3600       ; minimum (1 hour)
// Check SOA
// Just check every refresh hours, if fail set to retry until succeeds
// expire is need: to give SERVFAIL.
*/
