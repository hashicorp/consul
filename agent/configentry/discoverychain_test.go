// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func makeDiscoveryChainSetForHashTest() *DiscoveryChainSet {
	dcs := NewDiscoveryChainSet()

	dcs.AddRouters(&structs.ServiceRouterConfigEntry{
		Kind: structs.ServiceRouter,
		Name: "router1",
	})
	dcs.AddSplitters(&structs.ServiceSplitterConfigEntry{
		Kind: structs.ServiceSplitter,
		Name: "splitter1",
	})
	dcs.AddResolvers(&structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "resolver1",
	})
	dcs.AddServices(&structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "svc1",
	})
	dcs.AddPeers(&pbpeering.Peering{Name: "peer1"})
	dcs.SetDefaultSamenessGroup(&structs.SamenessGroupConfigEntry{
		Name: "sg-default",
	})
	dcs.AddSamenessGroup(&structs.SamenessGroupConfigEntry{
		Name: "sg1",
	})
	dcs.AddProxyDefaults(&structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	})

	return dcs
}

func populateDiscoveryChainSetHashField(t *testing.T, dcs *DiscoveryChainSet, field string) {
	t.Helper()

	switch field {
	case "Routers":
		dcs.AddRouters(&structs.ServiceRouterConfigEntry{Kind: structs.ServiceRouter, Name: "router2"})
	case "Splitters":
		dcs.AddSplitters(&structs.ServiceSplitterConfigEntry{Kind: structs.ServiceSplitter, Name: "splitter2"})
	case "Resolvers":
		dcs.AddResolvers(&structs.ServiceResolverConfigEntry{Kind: structs.ServiceResolver, Name: "resolver2"})
	case "Services":
		dcs.AddServices(&structs.ServiceConfigEntry{Kind: structs.ServiceDefaults, Name: "svc2"})
	case "Peers":
		dcs.AddPeers(&pbpeering.Peering{Name: "peer2"})
	case "DefaultSamenessGroup":
		dcs.SetDefaultSamenessGroup(&structs.SamenessGroupConfigEntry{Name: "sg-default-2"})
	case "SamenessGroups":
		dcs.AddSamenessGroup(&structs.SamenessGroupConfigEntry{Name: "sg2"})
	case "ProxyDefaults":
		entry := &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
		}
		entry.SetHash(12345)
		dcs.AddProxyDefaults(entry)
	default:
		t.Fatalf("populateDiscoveryChainSetHashField missing support for field %q", field)
	}
}

// TestDiscoveryChainSetHashConsistency ensures hashing is deterministic.
func TestDiscoveryChainSetHashConsistency(t *testing.T) {
	dcs := makeDiscoveryChainSetForHashTest()

	hash1 := dcs.Hash()
	hash2 := dcs.Hash()
	hash3 := dcs.Hash()

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash() is not consistent. Got: %d, %d, %d", hash1, hash2, hash3)
	}
}

func TestDiscoveryChainSetHashChangesForEachExportedField(t *testing.T) {
	dcs := makeDiscoveryChainSetForHashTest()
	baseHash := dcs.Hash()

	for _, field := range discoveryChainSetExportedFields() {
		field := field
		t.Run(field, func(t *testing.T) {
			mutated := makeDiscoveryChainSetForHashTest()
			populateDiscoveryChainSetHashField(t, mutated, field)

			if got := mutated.Hash(); got == baseHash {
				t.Fatalf("expected hash to change when mutating %s; hash=%d", field, got)
			}
		})
	}
}

func discoveryChainSetExportedFields() []string {
	return []string{
		"Routers",
		"Splitters",
		"Resolvers",
		"Services",
		"Peers",
		"DefaultSamenessGroup",
		"SamenessGroups",
		"ProxyDefaults",
	}
}
