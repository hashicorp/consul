// Package auto implements a on-the-fly loading file backend.
package auto

import (
	"sync"

	"github.com/miekg/coredns/middleware/file"
)

// Zones maps zone names to a *Zone. This keep track of what we zones we have loaded at
// any one time.
type Zones struct {
	Z     map[string]*file.Zone // A map mapping zone (origin) to the Zone's data.
	names []string              // All the keys from the map Z as a string slice.

	origins []string // Any origins from the server block.

	sync.RWMutex
}

// Names returns the names from z.
func (z *Zones) Names() []string {
	z.RLock()
	n := z.names
	z.RUnlock()
	return n
}

// Origins returns the origins from z.
func (z *Zones) Origins() []string {
	// doesn't need locking, because there aren't multiple Go routines accessing it.
	return z.origins
}

// Zones returns a zone with origin name from z, nil when not found.
func (z *Zones) Zones(name string) *file.Zone {
	z.RLock()
	zo := z.Z[name]
	z.RUnlock()
	return zo
}

// Add adds a new zone into z. If zo.NoReload is false, the
// reload goroutine is started.
func (z *Zones) Add(zo *file.Zone, name string) {
	z.Lock()

	if z.Z == nil {
		z.Z = make(map[string]*file.Zone)
	}

	z.Z[name] = zo
	z.names = append(z.names, name)
	zo.Reload()

	z.Unlock()
}

// Remove removes the zone named name from z. It also stop the the zone's reload goroutine.
func (z *Zones) Remove(name string) {
	z.Lock()

	if zo, ok := z.Z[name]; ok && !zo.NoReload {
		zo.ReloadShutdown <- true
	}

	delete(z.Z, name)

	// TODO(miek): just regenerate Names (might be bad if you have a lot of zones...)
	z.names = []string{}
	for n := range z.Z {
		z.names = append(z.names, n)
	}

	z.Unlock()
}
