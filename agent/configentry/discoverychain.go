// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

// DiscoveryChainSet is a wrapped set of raw cross-referenced config entries
// necessary for the DiscoveryChain.Get RPC process.
//
// None of these are defaulted.
type DiscoveryChainSet struct {
	Routers              map[structs.ServiceID]*structs.ServiceRouterConfigEntry
	Splitters            map[structs.ServiceID]*structs.ServiceSplitterConfigEntry
	Resolvers            map[structs.ServiceID]*structs.ServiceResolverConfigEntry
	Services             map[structs.ServiceID]*structs.ServiceConfigEntry
	Peers                map[string]*pbpeering.Peering
	DefaultSamenessGroup *structs.SamenessGroupConfigEntry
	SamenessGroups       map[string]*structs.SamenessGroupConfigEntry
	ProxyDefaults        map[string]*structs.ProxyConfigEntry
}

func NewDiscoveryChainSet() *DiscoveryChainSet {
	return &DiscoveryChainSet{
		Routers:        make(map[structs.ServiceID]*structs.ServiceRouterConfigEntry),
		Splitters:      make(map[structs.ServiceID]*structs.ServiceSplitterConfigEntry),
		Resolvers:      make(map[structs.ServiceID]*structs.ServiceResolverConfigEntry),
		Services:       make(map[structs.ServiceID]*structs.ServiceConfigEntry),
		Peers:          make(map[string]*pbpeering.Peering),
		ProxyDefaults:  make(map[string]*structs.ProxyConfigEntry),
		SamenessGroups: make(map[string]*structs.SamenessGroupConfigEntry),
	}
}

func (e *DiscoveryChainSet) GetRouter(sid structs.ServiceID) *structs.ServiceRouterConfigEntry {
	if e.Routers != nil {
		return e.Routers[sid]
	}
	return nil
}

func (e *DiscoveryChainSet) GetSplitter(sid structs.ServiceID) *structs.ServiceSplitterConfigEntry {
	if e.Splitters != nil {
		return e.Splitters[sid]
	}
	return nil
}

func (e *DiscoveryChainSet) GetResolver(sid structs.ServiceID) *structs.ServiceResolverConfigEntry {
	if e.Resolvers != nil {
		return e.Resolvers[sid]
	}
	return nil
}

func (e *DiscoveryChainSet) GetService(sid structs.ServiceID) *structs.ServiceConfigEntry {
	if e.Services != nil {
		return e.Services[sid]
	}
	return nil
}

func (e *DiscoveryChainSet) GetSamenessGroup(name string) *structs.SamenessGroupConfigEntry {
	if e.SamenessGroups != nil {
		return e.SamenessGroups[name]
	}
	return nil
}

func (e *DiscoveryChainSet) GetDefaultSamenessGroup() *structs.SamenessGroupConfigEntry {
	return e.DefaultSamenessGroup
}

func (e *DiscoveryChainSet) GetProxyDefaults(partition string) *structs.ProxyConfigEntry {
	if e.ProxyDefaults != nil {
		return e.ProxyDefaults[partition]
	}
	return nil
}

// AddRouters adds router configs. Convenience function for testing.
func (e *DiscoveryChainSet) AddRouters(entries ...*structs.ServiceRouterConfigEntry) {
	if e.Routers == nil {
		e.Routers = make(map[structs.ServiceID]*structs.ServiceRouterConfigEntry)
	}
	for _, entry := range entries {
		e.Routers[structs.NewServiceID(entry.Name, &entry.EnterpriseMeta)] = entry
	}
}

// AddSplitters adds splitter configs. Convenience function for testing.
func (e *DiscoveryChainSet) AddSplitters(entries ...*structs.ServiceSplitterConfigEntry) {
	if e.Splitters == nil {
		e.Splitters = make(map[structs.ServiceID]*structs.ServiceSplitterConfigEntry)
	}
	for _, entry := range entries {
		e.Splitters[structs.NewServiceID(entry.Name, entry.GetEnterpriseMeta())] = entry
	}
}

// AddResolvers adds resolver configs. Convenience function for testing.
func (e *DiscoveryChainSet) AddResolvers(entries ...*structs.ServiceResolverConfigEntry) {
	if e.Resolvers == nil {
		e.Resolvers = make(map[structs.ServiceID]*structs.ServiceResolverConfigEntry)
	}
	for _, entry := range entries {
		e.Resolvers[structs.NewServiceID(entry.Name, entry.GetEnterpriseMeta())] = entry
	}
}

// AddServices adds service configs. Convenience function for testing.
func (e *DiscoveryChainSet) AddServices(entries ...*structs.ServiceConfigEntry) {
	if e.Services == nil {
		e.Services = make(map[structs.ServiceID]*structs.ServiceConfigEntry)
	}
	for _, entry := range entries {
		e.Services[structs.NewServiceID(entry.Name, entry.GetEnterpriseMeta())] = entry
	}
}

// AddSamenessGroup adds a sameness group. Convenience function for testing.
func (e *DiscoveryChainSet) AddSamenessGroup(entries ...*structs.SamenessGroupConfigEntry) {
	if e.SamenessGroups == nil {
		e.SamenessGroups = make(map[string]*structs.SamenessGroupConfigEntry)
	}
	for _, entry := range entries {
		e.SamenessGroups[entry.Name] = entry
	}
}

// SetDefaultSamenessGroup sets the default sameness group. Convenience function for testing.
func (e *DiscoveryChainSet) SetDefaultSamenessGroup(entry *structs.SamenessGroupConfigEntry) {
	if e.SamenessGroups == nil {
		e.SamenessGroups = make(map[string]*structs.SamenessGroupConfigEntry)
	}

	if entry == nil {
		return
	}

	e.SamenessGroups[entry.Name] = entry
	e.DefaultSamenessGroup = entry
}

// AddProxyDefaults adds proxy-defaults configs. Convenience function for testing.
func (e *DiscoveryChainSet) AddProxyDefaults(entries ...*structs.ProxyConfigEntry) {
	if e.ProxyDefaults == nil {
		e.ProxyDefaults = make(map[string]*structs.ProxyConfigEntry)
	}
	for _, entry := range entries {
		e.ProxyDefaults[entry.PartitionOrDefault()] = entry
	}
}

// AddPeers adds cluster peers. Convenience function for testing.
func (e *DiscoveryChainSet) AddPeers(entries ...*pbpeering.Peering) {
	if e.Peers == nil {
		e.Peers = make(map[string]*pbpeering.Peering)
	}
	for _, entry := range entries {
		e.Peers[entry.Name] = entry
	}
}

// AddEntries adds generic configs. Convenience function for testing. Panics on
// operator error.
func (e *DiscoveryChainSet) AddEntries(entries ...structs.ConfigEntry) {
	for _, rawEntry := range entries {
		switch entry := rawEntry.(type) {
		case *structs.ServiceRouterConfigEntry:
			e.AddRouters(entry)
		case *structs.ServiceSplitterConfigEntry:
			e.AddSplitters(entry)
		case *structs.ServiceResolverConfigEntry:
			e.AddResolvers(entry)
		case *structs.ServiceConfigEntry:
			e.AddServices(entry)
		case *structs.SamenessGroupConfigEntry:
			if entry.DefaultForFailover {
				e.DefaultSamenessGroup = entry
			}
			e.AddSamenessGroup(entry)
		case *structs.ProxyConfigEntry:
			if entry.GetName() != structs.ProxyConfigGlobal {
				panic("the only supported proxy-defaults name is '" + structs.ProxyConfigGlobal + "'")
			}
			e.AddProxyDefaults(entry)
		default:
			panic("unhandled config entry kind: " + entry.GetKind())
		}
	}
}

// IsEmpty returns true if there are no config entries at all in the response.
// You should prefer this over IsChainEmpty() in most cases.
func (e *DiscoveryChainSet) IsEmpty() bool {
	return e.IsChainEmpty() && len(e.Services) == 0 && len(e.ProxyDefaults) == 0
}

// IsChainEmpty returns true if there are no service-routers,
// service-splitters, or service-resolvers that are present. These config
// entries are the primary parts of the discovery chain.
func (e *DiscoveryChainSet) IsChainEmpty() bool {
	return len(e.Routers) == 0 && len(e.Splitters) == 0 && len(e.Resolvers) == 0 && e.DefaultSamenessGroup == nil
}
