// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
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

// meshPQCMetricsInterval is how often the leader refreshes the
// mesh.pqc_outdated_agents gauge.
var meshPQCMetricsInterval = time.Minute

const meshPQCMetricsRoutineName = "mesh PQC metrics"

var ConfigEntryGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"mesh", "pqc_outdated_agents"},
		Help: "Number of alive agents older than 2.1.0 in an admin partition whose mesh config entry enables post-quantum key exchange. Those agents ignore the setting until they are upgraded. Reported by the leader.",
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
// in the given datacenters runs a release that does not know about
// ECDHCurves. Older servers silently drop the field, including when the entry
// is replicated to secondary datacenters, so a write accepted during a
// mixed-version window could be lost or diverge between servers. Failed
// servers count because they can rejoin with the old version.
func serversSupportMeshECDHCurves(provider checkServersProvider, datacenters []string) error {
	var alive, failed []string
	for _, dc := range datacenters {
		ServersInDCMeetRequirements(provider, dc, func(srv *metadata.Server) (bool, bool) {
			if srv.Status != serf.StatusAlive && srv.Status != serf.StatusFailed {
				return true, true
			}
			if meshPQCVersionSupported(&srv.Build) {
				return true, false
			}
			name := fmt.Sprintf("%s/%s (%s)", srv.Datacenter, srv.ShortName, srv.Build.String())
			if srv.Status == serf.StatusFailed {
				failed = append(failed, name)
			} else {
				alive = append(alive, name)
			}
			// Keep iterating so every outdated server is reported.
			return true, false
		})
	}
	if len(alive) == 0 && len(failed) == 0 {
		return nil
	}
	sort.Strings(alive)
	sort.Strings(failed)

	var b strings.Builder
	fmt.Fprintf(&b, "all servers in every datacenter must run Consul %s or later before configuring ECDHCurves in the mesh config entry, "+
		"because older servers drop the field when they store or replicate the entry", minMeshPQCVersion.String())
	if len(alive) > 0 {
		fmt.Fprintf(&b, "; servers on older versions: %s", formatNodeNames(alive, maxReportedOutdatedAgents))
	}
	if len(failed) > 0 {
		fmt.Fprintf(&b, "; failed servers on older versions: %s. Upgrade them, or remove them with 'consul force-leave' if they will not return",
			formatNodeNames(failed, maxReportedOutdatedAgents))
	}
	return errors.New(b.String())
}

// meshPQCDatacenters returns the local datacenter and every WAN-federated
// datacenter known to this server.
func (s *Server) meshPQCDatacenters() []string {
	dcs := []string{s.config.Datacenter}
	for _, dc := range s.router.GetDatacenters() {
		if !slices.Contains(dcs, dc) {
			dcs = append(dcs, dc)
		}
	}
	return dcs
}

// meshPQCMembers returns the LAN members that render Envoy configuration for
// proxies in the given admin partition.
func (s *Server) meshPQCMembers(partition string) ([]serf.Member, error) {
	return s.LANMembers(LANMemberFilter{Partition: partition})
}

// agentsWithoutMeshPQCSupport returns the sorted, de-duplicated names of alive
// Consul agents whose version renders Envoy configuration without mesh
// post-quantum settings.
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
		if !meshPQCVersionSupported(v) && !slices.Contains(names, m.Name) {
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

// checkMeshPQCCompatibility runs after the write is authorized and is known to
// change the stored entry. It rejects Outgoing curve lists that cannot work
// with TLS 1.2, rejects ECDHCurves while any server is too old to keep the
// field, and warns when agents in the entry's partition will ignore it.
func (c *ConfigEntry) checkMeshPQCCompatibility(updated, current structs.ConfigEntry) error {
	mesh, ok := updated.(*structs.MeshConfigEntry)
	if !ok {
		return nil
	}
	if err := structs.ValidateMeshOutgoingECDHCurvesUpdate(mesh, current); err != nil {
		return err
	}
	if !meshEntryEnablesPQC(mesh) {
		return nil
	}

	if meshEntryConfiguresECDHCurves(mesh) {
		if err := serversSupportMeshECDHCurves(c.srv, c.srv.meshPQCDatacenters()); err != nil {
			return err
		}
	}

	partition := mesh.GetEnterpriseMeta().PartitionOrDefault()
	members, err := c.srv.meshPQCMembers(partition)
	if err != nil {
		c.logger.Debug("unable to list LAN members to check post-quantum TLS support", "partition", partition, "error", err)
		return nil
	}
	logAgentsWithoutMeshPQCSupport(c.logger, partition, members)
	return nil
}

// logAgentsWithoutMeshPQCSupport logs a warning when alive agents in the
// partition are too old to apply mesh post-quantum settings. It returns the
// number of outdated agents.
func logAgentsWithoutMeshPQCSupport(logger hclog.Logger, partition string, members []serf.Member) int {
	outdated := agentsWithoutMeshPQCSupport(members)
	if len(outdated) == 0 {
		return 0
	}
	logger.Warn("mesh config entry enables post-quantum TLS key exchange, but some agents run a Consul version that ignores it; "+
		"sidecar proxies and gateways managed by these agents keep their previous TLS settings until the agents are upgraded",
		"partition", partition,
		"min_version", minMeshPQCVersion.String(),
		"count", len(outdated),
		"agents", formatNodeNames(outdated, maxReportedOutdatedAgents),
	)
	return len(outdated)
}

// meshPQCOutdatedAgentCounts returns, for each partition with a mesh config
// entry, the number of alive agents that ignore its post-quantum settings.
// Partitions whose mesh entry does not enable post-quantum key exchange
// report zero.
func meshPQCOutdatedAgentCounts(entries []structs.ConfigEntry, membersFn func(partition string) ([]serf.Member, error), logger hclog.Logger) map[string]int {
	counts := make(map[string]int)
	for _, entry := range entries {
		mesh, ok := entry.(*structs.MeshConfigEntry)
		if !ok {
			continue
		}
		partition := mesh.GetEnterpriseMeta().PartitionOrDefault()
		if !meshEntryEnablesPQC(mesh) {
			counts[partition] = 0
			continue
		}
		members, err := membersFn(partition)
		if err != nil {
			logger.Debug("unable to list LAN members for post-quantum TLS metrics", "partition", partition, "error", err)
			continue
		}
		counts[partition] = len(agentsWithoutMeshPQCSupport(members))
	}
	return counts
}

// emitMeshPQCOutdatedAgentGauges sets mesh.pqc_outdated_agents for every
// partition in counts and resets partitions reported previously but no longer
// present. It returns the partitions reported in this round.
func emitMeshPQCOutdatedAgentGauges(counts map[string]int, previous map[string]struct{}, setGauge func(key []string, val float32, labels []metrics.Label)) map[string]struct{} {
	key := []string{"mesh", "pqc_outdated_agents"}
	reported := make(map[string]struct{}, len(counts))
	partitions := make([]string, 0, len(counts))
	for p := range counts {
		partitions = append(partitions, p)
	}
	sort.Strings(partitions)
	for _, p := range partitions {
		setGauge(key, float32(counts[p]), []metrics.Label{{Name: "partition", Value: p}})
		reported[p] = struct{}{}
	}
	for p := range previous {
		if _, ok := reported[p]; !ok {
			setGauge(key, 0, []metrics.Label{{Name: "partition", Value: p}})
		}
	}
	return reported
}

// updateMeshPQCOutdatedAgentGauges computes and emits the gauge for every mesh
// config entry stored on this server.
func (s *Server) updateMeshPQCOutdatedAgentGauges(previous map[string]struct{}, setGauge func(key []string, val float32, labels []metrics.Label)) map[string]struct{} {
	_, entries, err := s.fsm.State().ConfigEntriesByKindAllPartitions(nil, structs.MeshConfig)
	if err != nil {
		s.logger.Debug("unable to read mesh config entries for post-quantum TLS metrics", "error", err)
		return previous
	}
	counts := meshPQCOutdatedAgentCounts(entries, s.meshPQCMembers, s.logger)
	return emitMeshPQCOutdatedAgentGauges(counts, previous, setGauge)
}

func (s *Server) startMeshPQCMetrics(ctx context.Context) {
	s.leaderRoutineManager.Start(ctx, meshPQCMetricsRoutineName, s.runMeshPQCMetrics)
}

func (s *Server) stopMeshPQCMetrics() {
	s.leaderRoutineManager.Stop(meshPQCMetricsRoutineName)
}

func (s *Server) runMeshPQCMetrics(ctx context.Context) error {
	ticker := time.NewTicker(meshPQCMetricsInterval)
	defer ticker.Stop()

	var reported map[string]struct{}
	for {
		reported = s.updateMeshPQCOutdatedAgentGauges(reported, metrics.SetGaugeWithLabels)
		select {
		case <-ctx.Done():
			// Stop reporting stale values after losing leadership.
			emitMeshPQCOutdatedAgentGauges(nil, reported, metrics.SetGaugeWithLabels)
			return nil
		case <-ticker.C:
		}
	}
}
