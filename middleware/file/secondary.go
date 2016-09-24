package file

import (
	"log"
	"time"

	"github.com/miekg/dns"
)

// TransferIn retrieves the zone from the masters, parses it and sets it live.
func (z *Zone) TransferIn() error {
	if len(z.TransferFrom) == 0 {
		return nil
	}
	m := new(dns.Msg)
	m.SetAxfr(z.origin)

	z1 := z.Copy()
	var (
		Err error
		tr  string
	)

Transfer:
	for _, tr = range z.TransferFrom {
		t := new(dns.Transfer)
		c, err := t.In(m, tr)
		if err != nil {
			log.Printf("[ERROR] Failed to setup transfer `%s' with `%s': %v", z.origin, tr, err)
			Err = err
			continue Transfer
		}
		for env := range c {
			if env.Error != nil {
				log.Printf("[ERROR] Failed to parse transfer `%s': %v", z.origin, env.Error)
				Err = env.Error
				continue Transfer
			}
			for _, rr := range env.RR {
				if err := z1.Insert(rr); err != nil {
					log.Printf("[ERROR] Failed to parse transfer `%s': %v", z.origin, err)
					Err = err
					continue Transfer
				}
			}
		}
		Err = nil
		break
	}
	if Err != nil {
		log.Printf("[ERROR] Failed to transfer %s: %s", z.origin, Err)
		return Err
	}

	z.Tree = z1.Tree
	z.Apex = z1.Apex
	*z.Expired = false
	log.Printf("[INFO] Transferred: %s from %s", z.origin, tr)
	return nil
}

// shouldTransfer checks the primaries of zone, retrieves the SOA record, checks the current serial
// and the remote serial and will return true if the remote one is higher than the locally configured one.
func (z *Zone) shouldTransfer() (bool, error) {
	c := new(dns.Client)
	c.Net = "tcp" // do this query over TCP to minimize spoofing
	m := new(dns.Msg)
	m.SetQuestion(z.origin, dns.TypeSOA)

	var Err error
	serial := -1

Transfer:
	for _, tr := range z.TransferFrom {
		Err = nil
		ret, _, err := c.Exchange(m, tr)
		if err != nil || ret.Rcode != dns.RcodeSuccess {
			Err = err
			continue
		}
		for _, a := range ret.Answer {
			if a.Header().Rrtype == dns.TypeSOA {
				serial = int(a.(*dns.SOA).Serial)
				break Transfer
			}
		}
	}
	if serial == -1 {
		return false, Err
	}
	if z.Apex.SOA == nil {
		return true, Err
	}
	return less(z.Apex.SOA.Serial, uint32(serial)), Err
}

// less return true of a is smaller than b when taking RFC 1982 serial arithmetic into account.
func less(a, b uint32) bool {
	if a < b {
		return (b - a) <= MaxSerialIncrement
	}
	return (a - b) > MaxSerialIncrement
}

// Update updates the secondary zone according to its SOA. It will run for the life time of the server
// and uses the SOA parameters. Every refresh it will check for a new SOA number. If that fails (for all
// server) it wil retry every retry interval. If the zone failed to transfer before the expire, the zone
// will be marked expired.
func (z *Zone) Update() error {
	// If we don't have a SOA, we don't have a zone, wait for it to appear.
	for z.Apex.SOA == nil {
		time.Sleep(1 * time.Second)
	}
	retryActive := false

Restart:
	refresh := time.Second * time.Duration(z.Apex.SOA.Refresh)
	retry := time.Second * time.Duration(z.Apex.SOA.Retry)
	expire := time.Second * time.Duration(z.Apex.SOA.Expire)

	if refresh < time.Hour {
		refresh = time.Hour
	}
	if retry < time.Hour {
		retry = time.Hour
	}
	if refresh > 24*time.Hour {
		refresh = 24 * time.Hour
	}
	if retry > 12*time.Hour {
		retry = 12 * time.Hour
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
			*z.Expired = true

		case <-retryTicker.C:
			if !retryActive {
				break
			}
			ok, err := z.shouldTransfer()
			if err != nil && ok {
				if err := z.TransferIn(); err != nil {
					// transfer failed, leave retryActive true
					break
				}
				retryActive = false
				// transfer OK, possible new SOA, stop timers and redo
				refreshTicker.Stop()
				retryTicker.Stop()
				expireTicker.Stop()
				goto Restart
			}

		case <-refreshTicker.C:
			ok, err := z.shouldTransfer()
			retryActive = err != nil
			if err != nil && ok {
				if err := z.TransferIn(); err != nil {
					// transfer failed
					retryActive = true
					break
				}
				retryActive = false
				// transfer OK, possible new SOA, stop timers and redo
				refreshTicker.Stop()
				retryTicker.Stop()
				expireTicker.Stop()
				goto Restart
			}
		}
	}
}

// MaxSerialIncrement is the maximum difference between two serial numbers. If the difference between
// two serials is greater than this number, the smaller one is considered greater.
const MaxSerialIncrement uint32 = 2147483647
