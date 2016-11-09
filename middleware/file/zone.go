package file

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/miekg/coredns/middleware/file/tree"
	"github.com/miekg/coredns/request"

	"github.com/fsnotify/fsnotify"
	"github.com/miekg/dns"
)

// Zone defines a structure that contains all data related to a DNS zone.
type Zone struct {
	origin  string
	origLen int
	file    string
	*tree.Tree
	Apex Apex

	TransferTo   []string
	StartupOnce  sync.Once
	TransferFrom []string
	Expired      *bool

	NoReload       bool
	reloadMu       sync.RWMutex
	ReloadShutdown chan bool
}

// Apex contains the apex records of a zone: SOA, NS and their potential signatures.
type Apex struct {
	SOA    *dns.SOA
	NS     []dns.RR
	SIGSOA []dns.RR
	SIGNS  []dns.RR
}

// NewZone returns a new zone.
func NewZone(name, file string) *Zone {
	z := &Zone{
		origin:         dns.Fqdn(name),
		origLen:        dns.CountLabel(dns.Fqdn(name)),
		file:           path.Clean(file),
		Tree:           &tree.Tree{},
		Expired:        new(bool),
		ReloadShutdown: make(chan bool),
	}
	*z.Expired = false

	return z
}

// Copy copies a zone *without* copying the zone's content. It is not a deep copy.
func (z *Zone) Copy() *Zone {
	z1 := NewZone(z.origin, z.file)
	z1.TransferTo = z.TransferTo
	z1.TransferFrom = z.TransferFrom
	z1.Expired = z.Expired
	z1.Apex = z.Apex
	return z1
}

// Insert inserts r into z.
func (z *Zone) Insert(r dns.RR) error {
	r.Header().Name = strings.ToLower(r.Header().Name)

	switch h := r.Header().Rrtype; h {
	case dns.TypeNS:
		r.(*dns.NS).Ns = strings.ToLower(r.(*dns.NS).Ns)

		if r.Header().Name == z.origin {
			z.Apex.NS = append(z.Apex.NS, r)
			return nil
		}
	case dns.TypeSOA:
		r.(*dns.SOA).Ns = strings.ToLower(r.(*dns.SOA).Ns)
		r.(*dns.SOA).Mbox = strings.ToLower(r.(*dns.SOA).Mbox)

		z.Apex.SOA = r.(*dns.SOA)
		return nil
	case dns.TypeNSEC3, dns.TypeNSEC3PARAM:
		return fmt.Errorf("NSEC3 zone is not supported, dropping RR: %s for zone: %s", r.Header().Name, z.origin)
	case dns.TypeRRSIG:
		x := r.(*dns.RRSIG)
		switch x.TypeCovered {
		case dns.TypeSOA:
			z.Apex.SIGSOA = append(z.Apex.SIGSOA, x)
			return nil
		case dns.TypeNS:
			if r.Header().Name == z.origin {
				z.Apex.SIGNS = append(z.Apex.SIGNS, x)
				return nil
			}
		}
	case dns.TypeCNAME:
		r.(*dns.CNAME).Target = strings.ToLower(r.(*dns.CNAME).Target)
	case dns.TypeMX:
		r.(*dns.MX).Mx = strings.ToLower(r.(*dns.MX).Mx)
	case dns.TypeSRV:
		r.(*dns.SRV).Target = strings.ToLower(r.(*dns.SRV).Target)
	}

	z.Tree.Insert(r)
	return nil
}

// Delete deletes r from z.
func (z *Zone) Delete(r dns.RR) { z.Tree.Delete(r) }

// TransferAllowed checks if incoming request for transferring the zone is allowed according to the ACLs.
func (z *Zone) TransferAllowed(req request.Request) bool {
	for _, t := range z.TransferTo {
		if t == "*" {
			return true
		}
	}
	// TODO(miek): future matching against IP/CIDR notations
	return false
}

// All returns all records from the zone, the first record will be the SOA record,
// otionally followed by all RRSIG(SOA)s.
func (z *Zone) All() []dns.RR {
	if !z.NoReload {
		z.reloadMu.RLock()
		defer z.reloadMu.RUnlock()
	}

	records := []dns.RR{}
	allNodes := z.Tree.All()
	for _, a := range allNodes {
		records = append(records, a.All()...)
	}

	if len(z.Apex.SIGNS) > 0 {
		records = append(z.Apex.SIGNS, records...)
	}
	records = append(z.Apex.NS, records...)

	if len(z.Apex.SIGSOA) > 0 {
		records = append(z.Apex.SIGSOA, records...)
	}
	return append([]dns.RR{z.Apex.SOA}, records...)
}

// Reload reloads a zone when it is changed on disk. If z.NoRoload is true, no reloading will be done.
func (z *Zone) Reload() error {
	if z.NoReload {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add(path.Dir(z.file))
	if err != nil {
		return err
	}

	go func() {
		// TODO(miek): needs to be killed on reload.
		for {
			select {
			case event := <-watcher.Events:
				if path.Clean(event.Name) == z.file {

					reader, err := os.Open(z.file)
					if err != nil {
						log.Printf("[ERROR] Failed to open `%s' for `%s': %v", z.file, z.origin, err)
						continue
					}
					zone, err := Parse(reader, z.origin, z.file)
					if err != nil {
						log.Printf("[ERROR] Failed to parse `%s': %v", z.origin, err)
						continue
					}

					// copy elements we need
					z.reloadMu.Lock()
					z.Apex = zone.Apex
					z.Tree = zone.Tree
					z.reloadMu.Unlock()

					log.Printf("[INFO] Successfully reloaded zone `%s'", z.origin)
					z.Notify()
				}
			case <-z.ReloadShutdown:
				watcher.Close()
				return
			}
		}
	}()
	return nil
}

// Print prints the zone's tree to stdout.
func (z *Zone) Print() {
	z.Tree.Print()
}

// NameFromRight returns the labels from the right, staring with the
// origin and then i labels extra. When we are overshooting the name
// the returned boolean is set to true.
func (z *Zone) nameFromRight(qname string, i int) (string, bool) {
	if i <= 0 {
		return z.origin, false
	}

	for j := 1; j <= z.origLen; j++ {
		if _, shot := dns.PrevLabel(qname, j); shot {
			return qname, shot
		}
	}

	k := 0
	shot := false
	for j := 1; j <= i; j++ {
		k, shot = dns.PrevLabel(qname, j+z.origLen)
		if shot {
			return qname, shot
		}
	}
	return qname[k:], false
}
