package dns

import (
	"path/filepath"

	"github.com/miekg/coredns/core/assets"
)

var storage = Storage(assets.Path())

// Storage is a root directory and facilitates
// forming file paths derived from it.
type Storage string

// Zones gets the directory that stores zones data.
func (s Storage) Zones() string {
	return filepath.Join(string(s), "zones")
}

// Zone returns the path to the folder containing assets for domain.
func (s Storage) Zone(domain string) string {
	return filepath.Join(s.Zones(), domain)
}

// SecondaryZoneFile returns the path to domain's secondary zone file (when fetched).
func (s Storage) SecondaryZoneFile(domain string) string {
	return filepath.Join(s.Zone(domain), "db."+domain)
}
