package kubernetes

import (
	"log"

	"github.com/miekg/dns"
)

// NormalizeZoneList filters the zones argument to remove
// array items that conflict with other items in zones.
// For example, providing the following zones array:
//    [ "a.b.c", "b.c", "a", "e.d.f", "a.b" ]
// Returns:
//    [ "a.b.c", "a", "e.d.f", "a.b" ]
// Zones filted out:
//    - "b.c" because "a.b.c" and "b.c" share the common top
//      level "b.c". First listed zone wins if there is a conflict.
//
// Note: This may prove to be too restrictive in practice.
//       Need to find counter-example use-cases.
func NormalizeZoneList(zones []string) []string {
	filteredZones := []string{}

	for _, z := range zones {
		zoneConflict, _ := subzoneConflict(filteredZones, z)
		if zoneConflict {
			log.Printf("[WARN] new zone '%v' from Corefile conflicts with existing zones: %v\n        Ignoring zone '%v'\n", z, filteredZones, z)
		} else {
			filteredZones = append(filteredZones, z)
		}
	}

	return filteredZones
}

// subzoneConflict returns true if name is a child or parent zone of
// any element in zones. If conflicts exist, return the conflicting zones.
func subzoneConflict(zones []string, name string) (bool, []string) {
	conflicts := []string{}

	for _, z := range zones {
		if dns.IsSubDomain(z, name) || dns.IsSubDomain(name, z) {
			conflicts = append(conflicts, z)
		}
	}

	return (len(conflicts) != 0), conflicts
}
