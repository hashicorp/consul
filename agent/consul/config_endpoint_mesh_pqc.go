// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-metrics/prometheus"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

// minMeshPQCVersion is the first Consul release that stores mesh ECDHCurves and
// renders post-quantum key exchange settings into Envoy configuration.
// Pre-releases of this version (for example 2.1.0-rc1) are treated as supported.
var minMeshPQCVersion = version.Must(version.NewVersion("2.1.0"))

// maxReportedOutdatedAgents bounds the number of node names included in the
// warning logged when agents cannot apply mesh post-quantum settings.
const maxReportedOutdatedAgents = 10

var ConfigEntryCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"config_entry", "mesh", "pqc_outdated_agents"},
		Help: "Increments by the number of alive agents older than 2.1.0 each time a mesh config entry that enables post-quantum key exchange is written. Those agents ignore the setting until they are upgraded.",
	},
}

func meshPQCVersionSupported(v *version.Version) bool {
	if v == nil {
		return false
	}
	return !v.Core().LessThan(minMeshPQCVersion)
}

func meshDirectionalTLSConfigs(entry structs.ConfigEntry) []*structs.MeshDirectionalTLSConfig {
	mesh, ok := entry.(*structs.MeshConfigEntry)
	if !ok || mesh == nil || mesh.TLS == nil {
		return nil
	}
	var out []*structs.MeshDirectionalTLSConfig
	for _, cfg := range []*structs.MeshDirectionalTLSConfig{mesh.TLS.Incoming, mesh.TLS.Outgoing} {
		if cfg != nil {
			out = append(out, cfg)
		}
	}
	return out
}

// meshEntryConfiguresECDHCurves reports whether the entry is a mesh config entry
// that explicitly sets ECDHCurves in either direction.
func meshEntryConfiguresECDHCurves(entry structs.ConfigEntry) bool {
	for _, cfg := range meshDirectionalTLSConfigs(entry) {
		if len(cfg.ECDHCurves) > 0 {
			return true
		}
	}
	return false
}

// meshEntryEnablesPQC reports whether the entry results in post-quantum key
// exchange settings on 2.1.0+ proxies, either through explicit ECDHCurves or
// through automatic curve injection for a TLSv1_3 minimum.
func meshEntryEnablesPQC(entry structs.ConfigEntry) bool {
	for _, cfg := range meshDirectionalTLSConfigs(entry) {
		if len(cfg.ECDHCurves) > 0 || cfg.TLSMinVersion == types.TLSv1_3 {
			return true
		}
	}
	return false
}

// serversSupportMeshECDHCurves returns an error when any alive or failed server
// in the datacenter runs a release that does not know about ECDHCurves. Older
// servers silently drop the field, so a write accepted during a mixed-version
// window could be lost or diverge between servers.
func serversSupportMeshECDHCurves(provider checkServersProvider, datacenter string) error {
	var outdated []string
	ServersInDCMeetRequirements(provider, datacenter, func(srv *metadata.Server) (bool, bool) {
		if srv.Status != serf.StatusAlive && srv.Status != serf.StatusFailed {
			return true, true
		}
		if !meshPQCVersionSupported(&srv.Build) {
			outdated = append(outdated, fmt.Sprintf("%s (%s)", srv.ShortName, srv.Build.String()))
			return false, false
		}
		return true, false
	})
	if len(outdated) == 0 {
		return nil
	}
	sort.Strings(outdated)
	return fmt.Errorf("all servers in the datacenter must run Consul %s or later before configuring ECDHCurves in the mesh config entry; servers on older versions: %s",
		minMeshPQCVersion.String(), formatNodeNames(outdated, maxReportedOutdatedAgents))
}

// agentsWithoutMeshPQCSupport returns the sorted names of alive Consul agents
// whose version renders Envoy configuration without mesh post-quantum settings.
func agentsWithoutMeshPQCSupport(members []serf.Member) []string {
	var names []string
	for i := range members {
		m := &members[i]
		if m.Status != serf.StatusAlive {
			continue
		}
		if role := m.Tags["role"]; role != "consul" && role != "node" {
			continue
		}
		v, err := metadata.Build(m)
		if err != nil || v == nil {
			continue
		}
		if !meshPQCVersionSupported(v) {
			names = append(names, m.Name)
		}
	}
	sort.Strings(names)
	return names
}

func formatNodeNames(names []string, limit int) string {
	if limit <= 0 || len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s (and %d more)", strings.Join(names[:limit], ", "), len(names)-limit)
}

// checkMeshPQCCompatibility rejects ECDHCurves writes while servers are being
// upgraded and warns when client agents will not apply post-quantum settings.
func (c *ConfigEntry) checkMeshPQCCompatibility(entry structs.ConfigEntry) error {
	if !meshEntryEnablesPQC(entry) {
		return nil
	}

	if meshEntryConfiguresECDHCurves(entry) {
		if err := serversSupportMeshECDHCurves(c.srv, c.srv.config.Datacenter); err != nil {
			return err
		}
	}

	members, err := c.srv.lanPoolAllMembers()
	if err != nil {
		c.logger.Debug("unable to list LAN members to check post-quantum TLS support", "error", err)
		return nil
	}
	if outdated := agentsWithoutMeshPQCSupport(members); len(outdated) > 0 {
		metrics.IncrCounter([]string{"config_entry", "mesh", "pqc_outdated_agents"}, float32(len(outdated)))
		c.logger.Warn("mesh config entry enables post-quantum TLS key exchange, but some agents run a Consul version that ignores it; "+
			"sidecar proxies and gateways managed by these agents keep their previous TLS settings until the agents are upgraded",
			"min_version", minMeshPQCVersion.String(),
			"count", len(outdated),
			"agents", formatNodeNames(outdated, maxReportedOutdatedAgents),
		)
	}
	return nil
}
