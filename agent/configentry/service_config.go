package configentry

import (
	"github.com/hashicorp/consul/agent/structs"
)

// ResolvedServiceConfigSet is a wrapped set of raw cross-referenced config
// entries necessary for the ConfigEntry.ResolveServiceConfig RPC process.
//
// None of these are defaulted.
type ResolvedServiceConfigSet struct {
	ServiceDefaults map[structs.ServiceID]*structs.ServiceConfigEntry
	GlobalProxy     *structs.ProxyConfigEntry
}

func (r *ResolvedServiceConfigSet) IsEmpty() bool {
	return len(r.ServiceDefaults) == 0 && r.GlobalProxy == nil
}

func (r *ResolvedServiceConfigSet) GetServiceDefaults(sid structs.ServiceID) *structs.ServiceConfigEntry {
	if r.ServiceDefaults == nil {
		return nil
	}
	return r.ServiceDefaults[sid]
}

func (r *ResolvedServiceConfigSet) AddServiceDefaults(entry *structs.ServiceConfigEntry) {
	if entry == nil {
		return
	}

	if r.ServiceDefaults == nil {
		r.ServiceDefaults = make(map[structs.ServiceID]*structs.ServiceConfigEntry)
	}

	sid := structs.NewServiceID(entry.Name, &entry.EnterpriseMeta)
	r.ServiceDefaults[sid] = entry
}

func (r *ResolvedServiceConfigSet) AddProxyDefaults(entry *structs.ProxyConfigEntry) {
	if entry == nil {
		return
	}
	r.GlobalProxy = entry
}
