// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// exportTo returns an exported-services entry that exports each named service
// to the peer.
func exportTo(peer string, services ...string) *api.ExportedServicesConfigEntry {
	entry := &api.ExportedServicesConfigEntry{Name: "default"}
	for _, svc := range services {
		entry.Services = append(entry.Services, api.ExportedService{
			Name:      svc,
			Consumers: []api.ServiceConsumer{{Peer: peer}},
		})
	}
	return entry
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// exchange returns the services the acceptor exports over the peering and the
// services the dialer imports from it.
func (p *peeringPair) exchange(t retry.TestingTB) (exported, imported []string) {
	exported = sorted(peeringState(t, p.acc, acceptorPeerName).StreamStatus.ExportedServices)
	imported = sorted(peeringState(t, p.dial, dialerPeerName).StreamStatus.ImportedServices)
	return exported, imported
}

// importedInstances returns the instances of service the dialer can see
// through the peering.
func (p *peeringPair) importedInstances(t retry.TestingTB, service string) []*api.ServiceEntry {
	entries, _, err := p.dial.Health().Service(service, "", false, &api.QueryOptions{Peer: dialerPeerName})
	require.NoError(t, err)
	return entries
}

// TestPeering_ExportedServices verifies which services cross a peering as the
// exported-services entry changes, and that the importing datacenter sees
// instances and their health.
func TestPeering_ExportedServices(t *testing.T) {
	t.Parallel()

	p := newPeeringPair(t, false)
	p.connect(t)

	register := func(name string, port int, withCheck bool) {
		reg := &api.AgentServiceRegistration{ID: name + "-1", Name: name, Port: port}
		if withCheck {
			reg.Check = &api.AgentServiceCheck{TTL: "10m", Status: api.HealthPassing}
		}
		require.NoError(t, p.acc.Agent().ServiceRegister(reg))
	}
	register("web", 8080, true)
	register("api", 8081, false)

	t.Run("nothing is shared until it is exported", func(t *testing.T) {
		exported, imported := p.exchange(t)
		require.Empty(t, exported)
		require.Empty(t, imported)
		require.Empty(t, p.importedInstances(t, "web"))
	})

	t.Run("an exported service becomes visible to the peer", func(t *testing.T) {
		setEntry(t, p.acc, exportTo(acceptorPeerName, "web"))

		eventually(t, func(r *retry.R) {
			exported, imported := p.exchange(r)
			require.Equal(r, []string{"web"}, exported)
			require.Equal(r, []string{"web"}, imported)

			instances := p.importedInstances(r, "web")
			require.Len(r, instances, 1)
			require.Equal(r, 8080, instances[0].Service.Port)
			require.Empty(r, p.importedInstances(r, "api"), "services that were not exported stay private")
		})
	})

	t.Run("health changes cross the peering", func(t *testing.T) {
		require.NoError(t, p.acc.Agent().UpdateTTL("service:web-1", "down", api.HealthCritical))

		eventually(t, func(r *retry.R) {
			instances := p.importedInstances(r, "web")
			require.Len(r, instances, 1)
			require.Equal(r, api.HealthCritical, instances[0].Checks.AggregatedStatus())
		})

		require.NoError(t, p.acc.Agent().UpdateTTL("service:web-1", "up", api.HealthPassing))
		eventually(t, func(r *retry.R) {
			require.Equal(r, api.HealthPassing, p.importedInstances(r, "web")[0].Checks.AggregatedStatus())
		})
	})

	t.Run("exporting a second service adds it", func(t *testing.T) {
		setEntry(t, p.acc, exportTo(acceptorPeerName, "web", "api"))

		eventually(t, func(r *retry.R) {
			exported, imported := p.exchange(r)
			require.Equal(r, []string{"api", "web"}, exported)
			require.Equal(r, []string{"api", "web"}, imported)
			require.Len(r, p.importedInstances(r, "api"), 1)
		})
	})

	t.Run("un-exporting a service removes it from the peer", func(t *testing.T) {
		setEntry(t, p.acc, exportTo(acceptorPeerName, "api"))

		eventually(t, func(r *retry.R) {
			exported, imported := p.exchange(r)
			require.Equal(r, []string{"api"}, exported)
			require.Equal(r, []string{"api"}, imported)
			require.Empty(r, p.importedInstances(r, "web"))
		})
	})

	t.Run("a service exported to a different peer is not shared with this one", func(t *testing.T) {
		setEntry(t, p.acc, exportTo("some-other-peer", "api"))

		eventually(t, func(r *retry.R) {
			exported, imported := p.exchange(r)
			require.Empty(r, exported)
			require.Empty(r, imported)
			require.Empty(r, p.importedInstances(r, "api"))
		})
	})

	t.Run("deregistering the last instance leaves nothing to import", func(t *testing.T) {
		setEntry(t, p.acc, exportTo(acceptorPeerName, "api"))
		eventually(t, func(r *retry.R) {
			require.Len(r, p.importedInstances(r, "api"), 1)
		})

		require.NoError(t, p.acc.Agent().ServiceDeregister("api-1"))
		eventually(t, func(r *retry.R) {
			require.Empty(r, p.importedInstances(r, "api"))
		})
	})

	t.Run("the entry must be named default", func(t *testing.T) {
		entry := exportTo(acceptorPeerName, "api")
		entry.Name = "not-default"

		_, _, err := p.acc.ConfigEntries().Set(entry, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `exported-services Name must be "default"`)
	})
}

func setEntry(t *testing.T, c *api.Client, entry api.ConfigEntry) {
	t.Helper()
	ok, _, err := c.ConfigEntries().Set(entry, nil)
	require.NoError(t, err)
	require.True(t, ok)
}
