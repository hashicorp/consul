// Package fall handles the fallthrough logic used in plugins that support it.
package fall

import (
	"github.com/coredns/coredns/plugin"
)

// F can be nil to allow for no fallthrough, empty allow all zones to fallthrough or
// contain a zone list that is checked.
type F struct {
	Zones []string
}

// Through will check if we should fallthrough for qname. Note that we've named the
// variable in each plugin "Fall", so this then reads Fall.Through().
func (f F) Through(qname string) bool {
	return plugin.Zones(f.Zones).Matches(qname) != ""
}

// setZones will set zones in f.
func (f *F) setZones(zones []string) {
	for i := range zones {
		zones[i] = plugin.Host(zones[i]).Normalize()
	}
	f.Zones = zones
}

// SetZonesFromArgs sets zones in f to the passed value or to "." if the slice is empty.
func (f *F) SetZonesFromArgs(zones []string) {
	if len(zones) == 0 {
		f.setZones(Root.Zones)
		return
	}
	f.setZones(zones)
}

// Equal returns true if f and g are equal.
func (f F) Equal(g F) bool {
	if len(f.Zones) != len(g.Zones) {
		return false
	}
	for i := range f.Zones {
		if f.Zones[i] != g.Zones[i] {
			return false
		}
	}
	return true
}

// Zero returns a zero valued F.
var Zero = func() F {
	return F{[]string{}}
}()

// Root returns F set to only ".".
var Root = func() F {
	return F{[]string{"."}}
}()
