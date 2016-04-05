package file

import (
	"log"
	"time"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
)

// TransferIn retrieves the zone from the masters, parses it and sets it live.
func (z *Zone) TransferIn() error {
	if len(z.TransferFrom) == 0 {
		return nil
	}
	m := new(dns.Msg)
	m.SetAxfr(z.name)

	z1 := z.Copy()
	var Err error

Transfer:
	for _, tr := range z.TransferFrom {
		t := new(dns.Transfer)
		c, err := t.In(m, tr)
		if err != nil {
			log.Printf("[ERROR] Failed to setup transfer %s with %s: %v", z.name, tr, err)
			Err = err
			continue Transfer
		}
		for env := range c {
			if env.Error != nil {
				log.Printf("[ERROR] Failed to parse transfer %s: %v", z.name, env.Error)
				Err = env.Error
				continue Transfer
			}
			for _, rr := range env.RR {
				if rr.Header().Rrtype == dns.TypeSOA {
					z1.SOA = rr.(*dns.SOA)
					continue
				}
				if rr.Header().Rrtype == dns.TypeRRSIG {
					if x, ok := rr.(*dns.RRSIG); ok && x.TypeCovered == dns.TypeSOA {
						z1.SIG = append(z1.SIG, x)
					}
				}
				z1.Insert(rr)
			}
		}
		Err = nil
		break
	}
	if Err != nil {
		log.Printf("[ERROR] Failed to transfer %s", z.name)
		return nil
	}

	z.Tree = z1.Tree
	*z.Expired = false
	log.Printf("[INFO] Transfered: %s", z.name)
	return nil
}

// shouldTransfer checks the primaries of zone, retrieves the SOA record, checks the current serial
// and the remote serial and will return true if the remote one is higher than the locally configured one.
func (z *Zone) shouldTransfer() (bool, error) {
	c := new(dns.Client)
	c.Net = "tcp" // do this query over TCP to minimize spoofing
	m := new(dns.Msg)
	m.SetQuestion(z.name, dns.TypeSOA)

	var Err error
	serial := -1

	for _, tr := range z.TransferFrom {
		Err = nil
		ret, err := middleware.Exchange(c, m, tr)
		if err != nil || ret.Rcode != dns.RcodeSuccess {
			Err = err
			continue
		}
		for _, a := range ret.Answer {
			if a.Header().Rrtype == dns.TypeSOA {
				serial = int(a.(*dns.SOA).Serial)
			}
		}
	}
	if serial == -1 {
		return false, Err
	}
	return less(z.SOA.Serial, uint32(serial)), Err
}

// less return true of a is smaller than b when taking RFC 1982 serial arithmetic into account.
func less(a, b uint32) bool {
	// TODO(miek): implement!
	return a < b
}

// Update updates the secondary zone according to its SOA. It will run for the life time of the server
// and uses the SOA parameters. Every refresh it will check for a new SOA number. If that fails (for all
// server) it wil retry every retry interval. If the zone failed to transfer before the expire, the zone
// will be marked expired.
func (z *Zone) Update() error {
	// TODO(miek): if SOA changes we need to redo this with possible different timer values.
	// TODO(miek): yeah...
	for z.SOA == nil {
		time.Sleep(1 * time.Second)
	}

	refresh := time.Second * time.Duration(z.SOA.Refresh)
	retry := time.Second * time.Duration(z.SOA.Retry)
	expire := time.Second * time.Duration(z.SOA.Expire)
	retryActive := false

	// TODO(miek): check max as well?
	if refresh < time.Hour {
		refresh = time.Hour
	}
	if retry < time.Hour {
		retry = time.Hour
	}

	refreshTicker := time.NewTicker(refresh)
	retryTicker := time.NewTicker(retry)
	expireTicker := time.NewTicker(expire)

	for {
		select {
		case <-expireTicker.C:
			if !retryActive {
				break
			}
			// TODO(miek): should actually keep track of last succesfull transfer
			*z.Expired = true

		case <-retryTicker.C:
			if !retryActive {
				break
			}
			ok, err := z.shouldTransfer()
			if err != nil && ok {
				log.Printf("[INFO] Refreshing zone: %s: initiating transfer", z.name)
				z.TransferIn()
				retryActive = false
			}

		case <-refreshTicker.C:
			ok, err := z.shouldTransfer()
			retryActive = err != nil
			if err != nil && ok {
				log.Printf("[INFO] Refreshing zone: %s: initiating transfer", z.name)
				z.TransferIn()
			}
		}
	}

	refreshTicker.Stop()
	retryTicker.Stop()
	expireTicker.Stop()
	return nil
}
