package file

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file/tree"

	"github.com/fsnotify/fsnotify"
	"github.com/miekg/dns"
)

type Zone struct {
	SOA    *dns.SOA
	SIG    []dns.RR
	origin string
	file   string
	*tree.Tree

	TransferTo   []string
	StartupOnce  sync.Once
	TransferFrom []string
	Expired      *bool

	NoReload bool
	reloadMu sync.RWMutex
	// TODO: shutdown watcher channel
}

// NewZone returns a new zone.
func NewZone(name, file string) *Zone {
	z := &Zone{origin: dns.Fqdn(name), file: path.Clean(file), Tree: &tree.Tree{}, Expired: new(bool)}
	*z.Expired = false
	return z
}

// Copy copies a zone *without* copying the zone's content. It is not a deep copy.
func (z *Zone) Copy() *Zone {
	z1 := NewZone(z.origin, z.file)
	z1.TransferTo = z.TransferTo
	z1.TransferFrom = z.TransferFrom
	z1.Expired = z.Expired
	z1.SOA = z.SOA
	z1.SIG = z.SIG
	return z1
}

// Insert inserts r into z.
func (z *Zone) Insert(r dns.RR) error {
	switch h := r.Header().Rrtype; h {
	case dns.TypeSOA:
		z.SOA = r.(*dns.SOA)
		return nil
	case dns.TypeNSEC3, dns.TypeNSEC3PARAM:
		return fmt.Errorf("NSEC3 zone is not supported, dropping")
	case dns.TypeRRSIG:
		if x, ok := r.(*dns.RRSIG); ok && x.TypeCovered == dns.TypeSOA {
			z.SIG = append(z.SIG, x)
			return nil
		}
		fallthrough
	default:
		z.Tree.Insert(r)
	}
	return nil
}

// Delete deletes r from z.
func (z *Zone) Delete(r dns.RR) { z.Tree.Delete(r) }

// TransferAllowed checks if incoming request for transferring the zone is allowed according to the ACLs.
func (z *Zone) TransferAllowed(state middleware.State) bool {
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
	z.reloadMu.RLock()
	defer z.reloadMu.RUnlock()
	records := []dns.RR{}
	allNodes := z.Tree.All()
	for _, a := range allNodes {
		records = append(records, a.All()...)
	}

	if len(z.SIG) > 0 {
		records = append(z.SIG, records...)
	}
	return append([]dns.RR{z.SOA}, records...)
}

func (z *Zone) Reload(shutdown chan bool) error {
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
					z.reloadMu.Lock()
					zone, err := Parse(reader, z.origin, z.file)
					if err != nil {
						log.Printf("[ERROR] Failed to parse `%s': %v", z.origin, err)
						z.reloadMu.Unlock()
						continue
					}
					// copy elements we need
					z.SOA = zone.SOA
					z.SIG = zone.SIG
					z.Tree = zone.Tree
					z.reloadMu.Unlock()
					log.Printf("[INFO] Successfully reloaded zone `%s'", z.origin)
				}
			case <-shutdown:
				watcher.Close()
				return
			}
		}
	}()
	return nil
}
